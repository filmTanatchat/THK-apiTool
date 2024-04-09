package applyProductMultiCaseId

import (
	"bufio"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"thinkerTools/models"
	"thinkerTools/types"
)

var basePath string

func writePayloadsToFile(payloads []map[string]interface{}, basePath string) error {
	filePath := filepath.Join(basePath, "2. log", "productName.json")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // For pretty printing
	if err := encoder.Encode(payloads); err != nil {
		return fmt.Errorf("error writing JSON to file: %w", err)
	}

	return nil
}

func convertRecordToJSON(record, headers []string, basePath string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"case_id":          record[0],
		"is_question_mode": false,
		"answers":          make([]map[string]interface{}, 0),
	}

	for i, value := range record[1:] {
		fieldName := getFieldName(headers[i+1]) // Extract field name
		convertedValue, err := convertValue(value, headers[i+1], basePath)
		if err != nil {
			return nil, err
		}
		answer := map[string]interface{}{
			"field_name":  fieldName,
			"field_value": convertedValue,
			"source":      "customer",
		}
		payload["answers"] = append(payload["answers"].([]map[string]interface{}), answer)
	}
	return payload, nil
}

func getFieldName(header string) string {
	parts := strings.Split(header, "||")
	return parts[0]
}

func convertSingleValue(value, dataType, basePath string) (string, error) {
	var stringValue string
	switch dataType {
	case "date", "date_time":
		t, err := time.Parse("02-01-2006", value)
		if err != nil {
			t, err = time.Parse("2-1-2006", value)
			if err != nil {
				return "", err
			}
		}
		stringValue = strconv.FormatInt(t.Unix(), 10)
	case "boolean":
		stringValue = strings.ToLower(value)
	case "file":
		fileData, err := os.ReadFile(filepath.Join(basePath, "4. answerAndQuestion/file", value))
		if err != nil {
			return "", err
		}
		stringValue = "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(fileData)
	default:
		stringValue = value
	}
	return stringValue, nil
}

func convertValue(value, header, basePath string) (string, error) {
	// Split the header to get the field name and data type
	parts := strings.Split(header, "||")
	_, dataType := parts[0], parts[1]

	// Handle multiple values
	if len(parts) > 2 && parts[2] == "MULTI" {
		values := strings.Split(value, "\\")
		stringValues := make([]string, len(values))
		for i, v := range values {
			stringValue, err := convertSingleValue(v, dataType, basePath)
			if err != nil {
				return "", err
			}
			stringValues[i] = stringValue
		}
		jsonString, err := json.Marshal(stringValues)
		if err != nil {
			return "", err
		}
		return string(jsonString), nil
	}

	// Handle single value
	return convertSingleValue(value, dataType, basePath)
}

func processRecords(jobs chan []string, wg *sync.WaitGroup, mu *sync.Mutex, client *http.Client, headers map[string]string, fullApiUrl, method string, headersRow []string, csvFilePath string, statusCodes *map[int]int, allPayloads *[]map[string]interface{}) {
	defer wg.Done()
	for record := range jobs {
		jsonPayload, convertErr := convertRecordToJSON(record, headersRow, csvFilePath)
		if convertErr != nil {
			fmt.Printf("Error converting record to JSON: %v\n", convertErr)
			continue
		}

		mu.Lock()
		*allPayloads = append(*allPayloads, jsonPayload)
		mu.Unlock()

		response, sendErr := types.MakeRequest(client, method, fullApiUrl, headers, jsonPayload)
		if sendErr != nil {
			fmt.Printf("Error sending request: %v\n", sendErr)
			continue
		}

		// Track the status code
		statusCode := response.StatusCode
		mu.Lock()
		(*statusCodes)[statusCode]++
		mu.Unlock()

		// Read response body for error debugging
		if statusCode != http.StatusOK && statusCode != http.StatusCreated {
			bodyBytes, err := io.ReadAll(response.Body)
			if err != nil {
				fmt.Printf("Error reading response body: %v\n", err)
			} else {
				fmt.Printf("Response Status Code %d: %s\n", statusCode, string(bodyBytes))
			}
		}
		response.Body.Close() // Close response body immediately after use
	}
}

func ProcessAnswerQuestionFromCSVData(env models.Environment, concurrentRequests int) error {
	// Authenticate once and use the token for all requests
	client := &http.Client{}
	headers, authErr := types.Authenticate(client, env.BaseURL+"/authentication/api/v1/login", env.Email, env.Password)
	if authErr != nil {
		return fmt.Errorf("authentication error: %w", authErr)
	}

	csvPath := filepath.Join(basePath, "4. answerAndQuestion")
	files, err := os.ReadDir(csvPath)
	if err != nil {
		return fmt.Errorf("error reading directory: %w", err)
	}

	// Load and select an API endpoint
	endpointsPath := filepath.Join(basePath, "config", "config.yaml")
	endpoints, err := models.LoadEndpoints(endpointsPath)
	if err != nil {
		return fmt.Errorf("error loading endpoints: %w", err)
	}

	selectedEndpoint, selectedMethod, err := models.SelectEndpoint(endpoints)
	if err != nil {
		return fmt.Errorf("error selecting endpoint: %w", err)
	}
	fullApiUrl := env.BaseURL + selectedEndpoint

	// Display CSV files with a running number
	var csvFiles []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".csv") {
			csvFiles = append(csvFiles, file.Name())
		}
	}

	if len(csvFiles) == 0 {
		return fmt.Errorf("no CSV files found in the directory")
	}

	fmt.Println("Select a CSV file:")
	for index, file := range csvFiles {
		fmt.Printf("%d: %s\n", index+1, file)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the number of the CSV file you want to use: ")
	choiceStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("error reading user input: %w", err)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(choiceStr))
	if err != nil || choice < 1 || choice > len(csvFiles) {
		return fmt.Errorf("invalid choice")
	}

	selectedFile := csvFiles[choice-1]

	// Open and read the selected CSV File
	selectedFilePath := filepath.Join(csvPath, selectedFile)
	file, err := os.Open(selectedFilePath)
	if err != nil {
		return fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading CSV data: %w", err)
	}

	// Map to track the frequency of status codes
	statusCodes := make(map[int]int)

	// Prepare for concurrent processing
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allPayloads []map[string]interface{} // Declare here
	statusCodes = make(map[int]int)
	jobs := make(chan []string, concurrentRequests)

	for w := 0; w < concurrentRequests; w++ {
		wg.Add(1)
		go processRecords(jobs, &wg, &mu, client, headers, fullApiUrl, selectedMethod, records[0], env.CSVFilePath, &statusCodes, &allPayloads) // Pass reference to allPayloads
	}

	// Sending jobs to the channel
	for _, record := range records[1:] {
		jobs <- record
	}
	close(jobs)

	// Wait for all goroutines to finish
	wg.Wait()

	// Print summary
	fmt.Println("\nSummary of Status Codes:")
	for code, count := range statusCodes {
		fmt.Printf("Status Code %d: %d occurrences\n", code, count)
	}

	// Write all payloads to a JSON file
	return writePayloadsToFile(allPayloads, basePath)
}
