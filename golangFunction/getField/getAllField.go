package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Environment struct to hold environment configuration
type Environment struct {
	BaseURL  string `json:"base_url"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthPayload struct for authentication request
type AuthPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse struct to capture the response from authentication
type AuthResponse struct {
	Data struct {
		SessionID string `json:"session_id"`
	} `json:"data"`
}

type APIResponse struct {
	Data     ResponseData    `json:"data"`
	Status   int             `json:"status"`
	Response json.RawMessage `json:"response"` // if needed
}

type Field struct {
	FieldName               string `json:"field_name"`
	DataType                string `json:"data_type"`
	IsMultipleValuesAllowed bool   `json:"is_multiple_values_allowed"`
	InputSource             string `json:"input_source"`
}

type ResponseData struct {
	CaseID           string  `json:"case_id"`
	Fields           []Field `json:"fields"`
	AdditionalFields []Field `json:"additional_fields"`
}

// loadJSONFromPath reads a JSON file and unmarshals it into the provided struct
func loadJSONFromPath(path string, v interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(v); err != nil {
		return err
	}
	return nil
}

func writeJSONToFile(filePath string, data interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// authenticate performs an authentication request
func authenticate(client *http.Client, env *Environment) (map[string]string, error) {
	payload := AuthPayload{
		Email:    env.Email,
		Password: env.Password,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", env.BaseURL+"/authentication/api/v1/login", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed: status code %d", resp.StatusCode)
	}

	var authResponse AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + authResponse.Data.SessionID,
	}

	return headers, nil
}

func main() {
	// Load environment variables
	var env Environment
	err := loadJSONFromPath("1. environment/env.json", &env)
	if err != nil {
		log.Fatalf("Error loading environment: %v", err)
	}

	// Load payload
	var payload interface{}
	err = loadJSONFromPath("3. dataSource/productName.json", &payload)
	if err != nil {
		log.Fatalf("Error loading payload: %v", err)
	}

	// Initialize response counters and unique responses
	responseCounters := make(map[int]int)
	uniqueResponses := make(map[int]map[string]int)

	// Create an HTTP client
	client := &http.Client{}

	// Authenticate
	headers, err := authenticate(client, &env)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// Make 'apply for product' request
	applyProductURL := fmt.Sprintf("%s/question-taskpool/api/v1/apply-for-product", env.BaseURL)
	responseApplyProduct, err := makeRequest(client, applyProductURL, headers, payload)
	if err != nil {
		log.Fatalf("Error making apply product request: %v", err)
	}

	responseStr, _ := json.Marshal(responseApplyProduct)
	status := responseApplyProduct.Status
	responseCounters[status]++
	if uniqueResponses[status] == nil {
		uniqueResponses[status] = make(map[string]int)
	}
	uniqueResponses[status][string(responseStr)]++

	// Process response
	if responseApplyProduct.Status == http.StatusOK {
		var responseData APIResponse
		if err := json.Unmarshal(responseApplyProduct.Response, &responseData); err != nil {
			log.Fatalf("Error unmarshalling response: %v", err)
		}

		if responseData.Data.CaseID != "" {
			caseID := responseData.Data.CaseID

			// Make 'get full form' request
			getFullFormURL := fmt.Sprintf("%s/question-taskpool/api/v1/get-full-form", env.BaseURL)
			payloadGetFullForm := map[string]string{"case_id": caseID}
			responseGetFullForm, err := makeRequest(client, getFullFormURL, headers, payloadGetFullForm)
			if err != nil {
				log.Fatalf("Error making get full form request: %v", err)
			}

			// Write response to log file
			logFilePath := filepath.Join("2. log", "getFullFormLog.json")
			if err := writeJSONToFile(logFilePath, responseGetFullForm); err != nil {
				log.Fatalf("Error writing to log file: %v", err)
			}

			// CSV Writing Logic (assuming extractCSVData and writeCSV are implemented)
			headerAll, rowAll, err := extractCSVData(responseData, "all")
			if err != nil {
				log.Fatalf("Error extracting CSV data: %v", err)
			}
			csvPathAll := filepath.Join("4. answerAndQuestion", "questionAllFields.csv")
			if err := writeCSV(headerAll, rowAll, csvPathAll); err != nil {
				log.Fatalf("Error writing CSV: %v", err)
			}

			// Write CSV for 'fields'
			headerFields, rowFields, err := extractCSVData(responseData, "fields")
			if err != nil {
				log.Fatalf("Error extracting CSV data for fields: %v", err)
			}
			csvPathFields := filepath.Join("4. answerAndQuestion", "questionMandatoryFields.csv")
			if err := writeCSV(headerFields, rowFields, csvPathFields); err != nil {
				log.Fatalf("Error writing CSV for fields: %v", err)
			}

			// Write CSV for 'additional_fields'
			headerAddFields, rowAddFields, err := extractCSVData(responseData, "additional_fields")
			if err != nil {
				log.Fatalf("Error extracting CSV data for additional fields: %v", err)
			}
			csvPathAddFields := filepath.Join("4. answerAndQuestion", "questionAdditionalFields.csv")
			if err := writeCSV(headerAddFields, rowAddFields, csvPathAddFields); err != nil {
				log.Fatalf("Error writing CSV for additional fields: %v", err)
			}

		}
	}

	// Print summary of responses
	for status, count := range responseCounters {
		fmt.Printf("status %d : %d Requests.\n", status, count)
		fmt.Println("detail of ", status, ":")

		if status == http.StatusOK {
			for responseStr, occurrence := range uniqueResponses[status] {
				var resp map[string]interface{}
				if err := json.Unmarshal([]byte(responseStr), &resp); err != nil {
					log.Fatalf("Error unmarshalling response string: %v", err)
				}
				fmt.Printf("Occurrences: %d\n", occurrence)
				fmt.Printf("Code: %v, Message: %v\n", resp["code"], resp["message"])
				fmt.Println("----------------------")
			}
		} else {
			for responseStr, occurrence := range uniqueResponses[status] {
				var resp map[string]interface{}
				if err := json.Unmarshal([]byte(responseStr), &resp); err != nil {
					log.Fatalf("Error unmarshalling response string: %v", err)
				}
				fmt.Printf("Occurrences: %d\n", occurrence)
				fmt.Printf("Code: %v, Message: %v\n", resp["code"], resp["message"])
				fmt.Println("----------------------")
			}
		}
	}
}

func makeRequest(client *http.Client, url string, headers map[string]string, payload interface{}) (*APIResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResponse APIResponse
	apiResponse.Status = resp.StatusCode
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	apiResponse.Response = json.RawMessage(responseBody)

	return &apiResponse, nil
}

func extractCSVData(response APIResponse, fieldType string) ([]string, []string, error) {
	var allFields []Field
	switch fieldType {
	case "fields":
		allFields = response.Data.Fields
	case "additional_fields":
		allFields = response.Data.AdditionalFields
	default: // "all"
		allFields = append(response.Data.Fields, response.Data.AdditionalFields...)
	}

	caseID := response.Data.CaseID
	header := []string{"case_id"}
	exampleDataRow := []string{caseID}

	for _, field := range allFields {
		if field.InputSource == "" {
			fieldHeader := field.FieldName + "||" + field.DataType
			if field.IsMultipleValuesAllowed {
				fieldHeader += "||MULTI"
			}
			header = append(header, fieldHeader)

			var exampleData string
			switch field.DataType {
			case "date":
				exampleData = "DD-MM-YYYY"
			case "date_time":
				exampleData = "DD-MM-YYYY hh:mm:ss"
			case "number":
				exampleData = "0"
				if field.IsMultipleValuesAllowed {
					exampleData = "0\\0"
				}
			case "file":
				exampleData = "test1.pdf"
				if field.IsMultipleValuesAllowed {
					exampleData = "test1.pdf\\test2.pdf"
				}
			case "text":
				if field.IsMultipleValuesAllowed {
					exampleData = "text1\\text2"
				}
			}
			exampleDataRow = append(exampleDataRow, exampleData)
		}
	}

	return header, exampleDataRow, nil
}

func writeCSV(header []string, row []string, path string) error {
	file, err := os.Create(path)
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
