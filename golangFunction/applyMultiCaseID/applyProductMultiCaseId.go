package applyProductMultiCaseId

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v2"

	"MoneyDD/types"
)

// Result captures the response and status code of an HTTP request.
type Result struct {
	ResponseData json.RawMessage
	StatusCode   int
}

// ProductName represents the structure of products in the YAML file
type ProductName struct {
	Products map[string]string `yaml:"products"`
}

// MakeAndProcessRequest makes an HTTP request and processes the response.
func MakeAndProcessRequest(client *http.Client, apiURL string, headers map[string]string, payload map[string]interface{}) Result {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling payload: %v\n", err)
		return Result{StatusCode: http.StatusInternalServerError}
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return Result{StatusCode: http.StatusInternalServerError}
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return Result{StatusCode: http.StatusInternalServerError}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return Result{StatusCode: http.StatusInternalServerError}
	}

	return Result{
		ResponseData: json.RawMessage(body),
		StatusCode:   resp.StatusCode,
	}
}

// LoadJSONFromPath loads JSON data from a specified file path.
func LoadJSONFromPath(path string) (map[string]interface{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return data, nil
}

func loadProductsFromYAML(path string) (ProductName, error) {
	var products ProductName
	file, err := os.Open(path)
	if err != nil {
		return products, fmt.Errorf("error opening YAML file %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&products); err != nil {
		return products, fmt.Errorf("error decoding YAML file %s: %w", path, err)
	}

	return products, nil
}

func selectProduct(products ProductName) (string, error) {
	fmt.Println("Select a product by number:")
	productKeys := make([]string, 0, len(products.Products))
	i := 0
	for key, name := range products.Products {
		fmt.Printf("%d: %s\n", i+1, name)
		productKeys = append(productKeys, key)
		i++
	}

	var choice int
	fmt.Print("Enter your choice: ")
	_, err := fmt.Scan(&choice)
	if err != nil || choice < 1 || choice > len(productKeys) {
		return "", fmt.Errorf("invalid choice, must be a number between 1 and %d", len(productKeys))
	}

	return products.Products[productKeys[choice-1]], nil
}

// ApplyProductMultiCaseId is the main function for the applyProductMultiCaseId package.
func ApplyProductMultiCaseId(env types.Environment, basePath string) error {

	// Load product names from YAML
	productNamesPath := filepath.Join(basePath, "config", "productName.yaml")
	products, err := loadProductsFromYAML(productNamesPath)
	if err != nil {
		return fmt.Errorf("error loading product names: %w", err)
	}

	// Display product options and get user selection
	selectedProduct, err := selectProduct(products)
	if err != nil {
		return err
	}

	// Ask for the number of case IDs
	var numRequests int
	fmt.Print("How many case IDs want to apply?: ")
	_, err = fmt.Scan(&numRequests) // Notice the change here, removed the ':'
	if err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}
	if numRequests <= 0 {
		return fmt.Errorf("number of case IDs should be greater than 0")
	}

	// Create payload with selected product
	payload := map[string]interface{}{"product_name": selectedProduct}

	// Authenticate and Set Up HTTP Client
	client := &http.Client{}
	headers, err := types.Authenticate(client, env.BaseURL+"/authentication/api/v1/login", env.Email, env.Password)
	if err != nil {
		return fmt.Errorf("error during authentication: %w", err)
	}

	// Concurrent Requests Setup
	var wg sync.WaitGroup
	results := make(chan Result, numRequests)
	wg.Add(numRequests)
	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			result := MakeAndProcessRequest(client, env.BaseURL+"/question-taskpool/api/v1/apply-for-product", headers, payload)
			results <- result
		}()
	}

	wg.Wait()
	close(results)

	// Process Results
	caseIds, responseCounters, uniqueResponses, err := processResults(results)
	if err != nil {
		return err
	}

	// Writing to CSV and logging response details
	if err := writeToCSV(caseIds, basePath); err != nil {
		return err
	}
	logResponseDetails(responseCounters, uniqueResponses)

	return nil
}

func processResults(results chan Result) ([]string, map[int]int, map[int]map[string]int, error) {
	caseIds := make([]string, 0)
	responseCounters := make(map[int]int)
	uniqueResponses := make(map[int]map[string]int)

	for result := range results {
		var responseData map[string]interface{}
		if err := json.Unmarshal(result.ResponseData, &responseData); err != nil {
			return nil, nil, nil, fmt.Errorf("error unmarshalling response data: %v", err)
		}

		if data, ok := responseData["data"].(map[string]interface{}); ok {
			if caseID, ok := data["case_id"].(string); ok {
				caseIds = append(caseIds, caseID)
			}
		}

		responseCounters[result.StatusCode]++
		if _, found := uniqueResponses[result.StatusCode]; !found {
			uniqueResponses[result.StatusCode] = make(map[string]int)
		}
		uniqueResponses[result.StatusCode][string(result.ResponseData)]++
	}

	return caseIds, responseCounters, uniqueResponses, nil
}

// writeToCSV writes case IDs to a CSV file.
func writeToCSV(caseIds []string, basePath string) error {
	csvPath := filepath.Join(basePath, "3. dataSource", "multiCaseId.csv")
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("error creating CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"case_id"}) // Header
	for _, caseId := range caseIds {
		writer.Write([]string{caseId})
	}
	writer.Flush()
	return nil
}

// logResponseDetails logs the details of HTTP responses.
func logResponseDetails(responseCounters map[int]int, uniqueResponses map[int]map[string]int) {
	for status, count := range responseCounters {
		fmt.Printf("Status %d: %d Requests\n", status, count)
		for responseStr, occurrence := range uniqueResponses[status] {
			fmt.Printf("Response: %s\nOccurrences: %d\n", responseStr, occurrence)
		}
		fmt.Println("----------------------")
	}
}
