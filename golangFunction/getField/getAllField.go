package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"thinkerTools/types"

	"github.com/mitchellh/mapstructure"
)

// Env represents the environment configuration
type Env struct {
	BaseURL  string `json:"BASE_URL"`
	Email    string `json:"EMAIL"`
	Password string `json:"PASSWORD"`
}

// ResponseData represents the data section of a response
type ResponseData struct {
	CaseID           string  `json:"case_id"`
	Fields           []Field `json:"fields"`
	AdditionalFields []Field `json:"additional_fields"`
}

// ResponseJSON represents a typical JSON response structure
type ResponseJSON struct {
	Response map[string]interface{} `json:"Response"`
	Status   int                    `json:"Status"`
}

type ApplyProductResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CaseID           string  `json:"case_id"`
		AnswerToken      string  `json:"answer_token"`
		Field            Field   `json:"field"`
		AdditionalFields []Field `json:"additional_fields"`
	} `json:"data"`
}

type Label struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type Choice struct {
	Value string           `json:"value"`
	Label map[string]Label `json:"label"`
}

type Field struct {
	FieldName               string           `json:"field_name"`
	DataType                string           `json:"data_type"`
	CurrentValue            string           `json:"current_value"`
	Label                   map[string]Label `json:"label"`
	Choices                 []Choice         `json:"choices"`
	IsMandatory             bool             `json:"is_mandatory"`
	InputSource             string           `json:"input_source"`
	IsMultipleValuesAllowed bool             `json:"is_multiple_values_allowed"`
	Alias                   string           `json:"alias"`
}

type FormData struct {
	CaseID           string  `json:"case_id"`
	Fields           []Field `json:"fields"`
	AdditionalFields []Field `json:"additional_fields"`
}

type GetFullFormResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    FormData `json:"data"`
}

var basePath string

// loadJSONFromPath loads JSON from a given file path.
func loadJSONFromPath(path string) (map[string]interface{}, error) {
	filePath := filepath.Join(basePath, path)
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	jsonStr, err := removeComments(string(file))
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}

// removeComments removes comments from a JSON string.
func removeComments(jsonStr string) (string, error) {
	pattern := `//.*?$|/\*.*?\*/|'(?:(?:\\.|[^'\\])*)'|"(?:(?:\\.|[^"\\])*)"`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	cleaned := re.ReplaceAllStringFunc(jsonStr, func(m string) string {
		if strings.HasPrefix(m, `"`) || strings.HasPrefix(m, `'`) {
			return m
		}
		return ""
	})
	return cleaned, nil
}

// makeRequest makes an HTTP request and returns the response.
func makeRequest(client *http.Client, apiURL string, headers map[string]string, payload interface{}) (*ResponseJSON, []byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	fmt.Printf("Sending request to URL: %s with payload: %s\n", apiURL, string(data))

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// Log detailed response information for debugging
	fmt.Printf("HTTP Response Status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Println("HTTP Response Body:", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, bodyBytes, fmt.Errorf("received non-200 status: %d", resp.StatusCode)
	}

	var responseJSON ResponseJSON
	err = json.Unmarshal(bodyBytes, &responseJSON)
	if err != nil {
		return nil, bodyBytes, err
	}

	return &responseJSON, bodyBytes, nil
}

func extractCSVData(responseJSON *ResponseJSON, fieldType string) ([]string, []string) {
	data, ok := responseJSON.Response["data"].(map[string]interface{})
	if !ok {
		return []string{}, []string{}
	}

	var allFields []interface{}
	if fieldType == "fields" || fieldType == "all" {
		fields, ok := data["fields"].([]interface{})
		if ok {
			allFields = append(allFields, fields...)
		}
	}
	if fieldType == "additional_fields" || fieldType == "all" {
		additionalFields, ok := data["additional_fields"].([]interface{})
		if ok {
			allFields = append(allFields, additionalFields...)
		}
	}

	caseID, _ := data["case_id"].(string)
	header := []string{"case_id"}
	exampleDataRow := []string{caseID}

	for _, f := range allFields {
		fieldMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}

		fieldName, _ := fieldMap["field_name"].(string)
		dataType, _ := fieldMap["data_type"].(string)
		isMultipleValuesAllowed, _ := fieldMap["is_multiple_values_allowed"].(bool)

		header = append(header, formatFieldName(fieldName, dataType, isMultipleValuesAllowed))
		exampleDataRow = append(exampleDataRow, exampleDataForField(dataType, isMultipleValuesAllowed))
	}

	return header, exampleDataRow
}

func formatFieldName(fieldName, dataType string, isMultipleValuesAllowed bool) string {
	formattedName := fieldName + "||" + dataType
	if isMultipleValuesAllowed {
		formattedName += "||MULTI"
	}
	return formattedName
}

func exampleDataForField(dataType string, isMultipleValuesAllowed bool) string {
	switch dataType {
	case "date":
		return "DD-MM-YYYY"
	case "date_time":
		return "DD-MM-YYYY hh:mm:ss"
	case "number":
		return "0"
	case "file":
		return "test1.pdf"
	case "text":
		if isMultipleValuesAllowed {
			return "text1\\text2"
		}
		return "sample text"
	default:
		return ""
	}
}

func getFieldSlice(rawFields []interface{}) []Field {
	var fields []Field
	for _, f := range rawFields {
		var field Field
		mapstructure.Decode(f, &field)
		fields = append(fields, field)
	}
	return fields
}

// writeCSV writes header and rows to a CSV file.
func writeCSV(header []string, row []string, path string) error {
	filePath := filepath.Join(basePath, path)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(header); err != nil {
		return err
	}
	if err := writer.Write(row); err != nil {
		return err
	}

	return nil
}

func main() {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		os.Exit(1)
	}

	// Assuming the config file is in a folder named 'config' within the current working directory
	configPath := filepath.Join(basePath, "config", "config.yaml")

	// Load configuration
	config, err := types.LoadConfig(configPath)
	if err != nil {
		fmt.Println("Failed to load configuration:", err)
		os.Exit(1)
	}

	selectedEnv, ok, exit := types.SelectEnvironment(config)
	if !ok {
		if exit {
			fmt.Println("Exiting...")
			os.Exit(0)
		}
		fmt.Println("Invalid environment selection")
		os.Exit(1)
	}

	session := &http.Client{}
	headers, err := types.Authenticate(session, selectedEnv.BaseURL+"/authentication/api/v1/login", selectedEnv.Email, selectedEnv.Password)
	if err != nil {
		fmt.Println("Error during authentication:", err)
		os.Exit(1)
	}

	payload, err := loadJSONFromPath(filepath.Join(basePath, "3. dataSource/productName.json"))
	if err != nil {
		fmt.Println("Failed to load payload:", err)
		os.Exit(1)
	}

	responseApplyProduct, bodyBytes, err := makeRequest(session, selectedEnv.BaseURL+"/question-taskpool/api/v1/apply-for-product", headers, payload)
	if err != nil {
		fmt.Println("Error applying for product:", err)
		os.Exit(1)
	}

	// Use bodyBytes for further processing
	var responseMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &responseMap); err != nil {
		fmt.Println("Error decoding response:", err)
		os.Exit(1)
	}

	// Access the nested fields as per the observed structure
	dataField, ok := responseMap["data"].(map[string]interface{})
	if !ok {
		fmt.Println("Error accessing 'data' field in response")
		os.Exit(1)
	}

	caseID, ok := dataField["case_id"].(string)
	if !ok || caseID == "" {
		fmt.Println("No case_id received in apply product response")
		os.Exit(1)
	}

	fmt.Printf("Received case_id: %s\n", caseID)

	// Prepare payload for get-full-form API
	payloadGetFullForm := map[string]string{
		"case_id": caseID,
	}

	// Make the request to the get-full-form API
	responseGetFullForm, _, err := makeRequest(session, selectedEnv.BaseURL+"/question-taskpool/api/v1/get-full-form", headers, payloadGetFullForm)
	if err != nil {
		fmt.Printf("Error in get-full-form request: %v\n", err)
		os.Exit(1)
	}

	// Example of using responseGetFullForm - print out some data
	fmt.Printf("Response from get-full-form: %+v\n", responseGetFullForm)

	responseCounters := make(map[int]int)
	uniqueResponses := make(map[int]map[string]int)

	responseStr, err := json.Marshal(responseApplyProduct.Response)
	if err != nil {
		panic(err)
	}

	status := responseApplyProduct.Status
	responseCounters[status]++

	if uniqueResponses[status] == nil {
		uniqueResponses[status] = make(map[string]int)
	}
	uniqueResponses[status][string(responseStr)]++

	if status == 200 {
		var applyProductResp ApplyProductResponse
		responseBytes, err := json.Marshal(responseApplyProduct.Response)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(responseBytes, &applyProductResp)
		if err != nil {
			panic(err)
		}

		caseID := applyProductResp.Data.CaseID

		payloadGetFullForm := map[string]interface{}{
			"case_id": caseID,
		}

		responseGetFullForm, _, err := makeRequest(session, selectedEnv.BaseURL+"/question-taskpool/api/v1/get-full-form", headers, payloadGetFullForm)
		if err != nil {
			panic(err)
		}

		// Write responses to CSV files
		headerAll, rowAll := extractCSVData(responseGetFullForm, "all")
		if err := writeCSV(headerAll, rowAll, "4. answerAndQuestion/questionAllFields.csv"); err != nil {
			panic(err)
		}

		headerFields, rowFields := extractCSVData(responseGetFullForm, "fields")
		if err := writeCSV(headerFields, rowFields, "4. answerAndQuestion/questionMandatoryFields.csv"); err != nil {
			panic(err)
		}

		headerAddFields, rowAddFields := extractCSVData(responseGetFullForm, "additional_fields")
		if err := writeCSV(headerAddFields, rowAddFields, "4. answerAndQuestion/questionAdditionalFields.csv"); err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("Received non-200 status from apply for product: %d\n", responseApplyProduct.Status)
		os.Exit(1)
	}

	// Print summary of responses
	for status, count := range responseCounters {
		fmt.Printf("status %d : %d Requests.\n", status, count)
		fmt.Println("detail of", status, ":")
		for responseStr, occurrence := range uniqueResponses[status] {
			fmt.Printf("Occurrences: %d\n", occurrence)
			fmt.Println("Response:", responseStr)
			fmt.Println("----------------------")
		}
	}
}
