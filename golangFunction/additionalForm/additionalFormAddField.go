package formAddField

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"thinkerTools/models"
	"thinkerTools/types"
)

type Result struct {
	FormName     string      `json:"FormName"`
	FieldName    string      `json:"FieldName"`
	IsMandatory  bool        `json:"IsMandatory"`
	Source       string      `json:"Source"`
	FromRow      int         `json:"FromRow"`
	Status       int         `json:"Status"`
	Response     interface{} `json:"Response,omitempty"`
	Error        string      `json:"Error,omitempty"`
	Endpoint     string      `json:"Endpoint"`
	Payload      string      `json:"Payload"`
	TimeReceived time.Time   `json:"TimeReceived"`
	Duration     string      `json:"Duration"`
}

var verbose bool // Package-level verbose flag

const maxRetriesPerRow = 3

// postRequestWithRetries sends a POST request with retry logic.
func postRequestWithRetries(session *http.Client, headers map[string]string, url string, payload map[string]interface{}) (*http.Response, error) {
	payloadBytes, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("error marshaling payload: %w", err)
	}

	fmt.Printf("Sending JSON Payload:\n%s\n\n", string(payloadBytes))

	var lastErr error
	for i := 0; i < maxRetriesPerRow; i++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}

		for key, value := range headers {
			req.Header.Set(key, value)
		}

		resp, err := session.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp != nil {
			bodyBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				fmt.Println("Error reading response body:", err)
				continue
			}

			if resp.StatusCode == http.StatusOK {
				return &http.Response{
					StatusCode: resp.StatusCode,
					Body:       io.NopCloser(bytes.NewBuffer(bodyBytes)),
				}, nil
			} else {
				fmt.Printf("Unexpected status code: %d, URL: %s, Response Body: %s\n", resp.StatusCode, url, string(bodyBytes))
				lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}
		} else {
			lastErr = fmt.Errorf("received nil response")
		}
	}

	return nil, lastErr
}

// processFile processes a CSV file and sends POST requests based on its content.
func processFile(session *http.Client, headers map[string]string, csvFilePath string, apiUrl string) ([]Result, map[int]int, error) {
	file, err := os.Open(csvFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("error reading CSV records: %w", err)
	}

	results := make([]Result, 0)
	responseCounters := make(map[int]int)

	for i, row := range records[1:] {
		startTime := time.Now()
		isMandatory := strings.ToLower(row[2]) == "true"

		payload := map[string]interface{}{
			"form_name":    row[0],
			"field_name":   row[1],
			"is_mandatory": isMandatory,
			"source":       "selected",
		}

		// Prepare payload for sending
		payloadBytes, err := json.MarshalIndent(payload, "", "    ")
		if err != nil {
			return nil, nil, fmt.Errorf("error marshaling payload: %w", err)
		}

		fmt.Printf("Processing row %d: %v\n", i+1, row)
		fmt.Println("Sending payload:", payload)
		fmt.Printf("Sending JSON Payload:\n%s\n", string(payloadBytes))

		response, err := postRequestWithRetries(session, headers, apiUrl, payload)
		var responseData interface{}
		status := 0
		if response != nil {
			defer response.Body.Close()
			json.NewDecoder(response.Body).Decode(&responseData)
			status = response.StatusCode
		} else if err != nil {
			fmt.Println("Error on request:", err)
		}
		endTime := time.Now()

		result := Result{
			FormName:     row[0],
			FieldName:    row[1],
			IsMandatory:  isMandatory,
			Source:       "selected",
			FromRow:      i + 1,
			Status:       status,
			Response:     responseData,
			Error:        fmt.Sprintf("%v", err),
			Endpoint:     apiUrl,
			Payload:      string(payloadBytes),
			TimeReceived: endTime,
			Duration:     endTime.Sub(startTime).String(),
		}

		responseCounters[status]++
		results = append(results, result)
	}

	return results, responseCounters, nil
}

// handleResults logs each result.
func handleResults(results []Result, logFile *os.File) {
	for _, result := range results {
		logEntry := formatLogEntry(result)
		if verbose {
			fmt.Println(logEntry) // Print detailed log if verbose is true
		}
		_, err := logFile.WriteString(logEntry)
		if err != nil {
			fmt.Printf("Error writing to log file: %v\n", err)
		}
	}
}

// formatLogEntry formats the log entry for a single result.
func formatLogEntry(result Result) string {
	var logEntry string
	if result.Error != "" {
		logEntry = fmt.Sprintf("Error in row %d: %v\n", result.FromRow, result.Error)
	} else {
		logEntry = fmt.Sprintf("Row %d successful\n", result.FromRow)
		logEntry += fmt.Sprintf("Form Name: %s, Field Name: %s, Is Mandatory: %v\n", result.FormName, result.FieldName, result.IsMandatory)
		logEntry += fmt.Sprintf("Status Code: %d, Response: %v\n", result.Status, result.Response)
	}
	logEntry += fmt.Sprintf("Endpoint: %s\nPayload: %s\n", result.Endpoint, result.Payload)
	logEntry += fmt.Sprintf("Time Received: %s, Duration: %s\n", result.TimeReceived.Format("2006-01-02 15:04:05"), result.Duration)
	logEntry += "----------------------------\n"
	return logEntry
}

func setupSession(env models.Environment) (*http.Client, map[string]string, error) {
	session := &http.Client{}
	loginURL := env.BaseURL + "/authentication/api/v1/login"
	headers, err := types.Authenticate(session, loginURL, env.Email, env.Password)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to authenticate: %w", err)
	}
	return session, headers, nil
}

func AddFieldsFromCSV(env models.Environment) error {
	session, headers, err := setupSession(env)
	if err != nil {
		fmt.Println("Failed to setup session:", err)
		return err
	}

	apiUrl := env.BaseURL + "/form/api/v1/add_field"
	basePath, _ := os.Getwd()
	csvFilePath := filepath.Join(basePath, "3. dataSource", "additionalFormAddUpdateField.csv")

	logFilePath := filepath.Join(basePath, "2. log", "formAddField.log")

	logFile, err := os.Create(logFilePath)
	if err != nil {
		fmt.Println("Error creating log file:", err)
		return err
	}
	defer logFile.Close()

	results, responseCounters, err := processFile(session, headers, csvFilePath, apiUrl)
	if err != nil {
		fmt.Println("Error processing CSV file:", err)
		return err
	}

	handleResults(results, logFile)
	printSummary(responseCounters)

	return nil
}

func printSummary(responseCounters map[int]int) {
	fmt.Println("\nSummary of Response Statuses:")
	for status, count := range responseCounters {
		fmt.Printf("Status Code %d: %d Fields Processed\n", status, count)
	}
}
