package roleAssignment

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"thinkerTools/models"
	"thinkerTools/types"
)

type Result struct {
	RoleName     string      `json:"RoleName"`
	VmName       string      `json:"VmName"`
	FromRow      int         `json:"FromRow"`
	Status       int         `json:"Status"`
	Response     interface{} `json:"Response,omitempty"`
	Error        string      `json:"Error,omitempty"`
	Endpoint     string      `json:"Endpoint"`
	Payload      string      `json:"Payload"`
	TimeReceived time.Time   `json:"TimeReceived"`
	Duration     string      `json:"Duration"`
}

func AssignRole(env models.Environment) error {
	basePath, _ := os.Getwd()
	logFilePath := filepath.Join(basePath, "2. log", "roleAssignment.log")
	csvFilePath := filepath.Join(basePath, "3. dataSource", "role.csv")

	session, headers, err := setupSession(env)
	if err != nil {
		return fmt.Errorf("setup session: %v", err)
	}

	logFile, err := setupLogFile(logFilePath)
	if err != nil {
		return fmt.Errorf("setup log file: %v", err)
	}
	defer logFile.Close()

	// Validate that baseURL is correct
	if env.BaseURL == "" || !(strings.HasPrefix(env.BaseURL, "http://") || strings.HasPrefix(env.BaseURL, "https://")) {
		return fmt.Errorf("invalid BaseURL: %s", env.BaseURL)
	}

	return processCSV(csvFilePath, session, headers, env.BaseURL, logFile)
}

func setupSession(env models.Environment) (*http.Client, map[string]string, error) {
	session := &http.Client{}
	loginURL := env.BaseURL + "/authentication/api/v1/login"
	headers, err := types.Authenticate(session, loginURL, env.Email, env.Password)
	return session, headers, err
}

func setupLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

func processCSV(filePath string, session *http.Client, headers map[string]string, baseURL string, logFile *os.File) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	results := make(chan Result)
	go handleResults(results, logFile)

	for i, row := range records {
		if i == 0 {
			continue // Skip header
		}
		wg.Add(1)
		go processRow(i, row, session, headers, baseURL, results, &wg)
	}

	wg.Wait()
	close(results)
	return nil
}

func processRow(index int, row []string, session *http.Client, headers map[string]string, baseURL string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	// Confirm baseURL is correct
	fmt.Println("BaseURL:", baseURL)

	startTime := time.Now()
	apiURL := baseURL + "/authentication/api/v1/rbac/add-role-policy"

	// Print the final apiURL to confirm it's correct
	fmt.Println("apiURL:", apiURL)

	payload := createPayload(row)
	response, err := sendRequest(session, apiURL, headers, payload)
	endTime := time.Now()

	result := createResult(index, response, err, apiURL, payload, startTime, endTime)
	results <- result
}

func createPayload(row []string) map[string]interface{} {
	return map[string]interface{}{
		"policy": map[string]string{
			"role_name": row[0],
			"url_path":  row[1],
			"action":    "EDIT",
		},
	}
}

func sendRequest(session *http.Client, url string, headers map[string]string, payload interface{}) (*http.Response, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return session.Do(req)
}

func createResult(index int, response *http.Response, err error, apiURL string, payload map[string]interface{}, startTime, endTime time.Time) Result {
	var resultData interface{}
	var errMsg string
	status := 0

	if err != nil {
		errMsg = err.Error()
	} else if response != nil {
		defer response.Body.Close()
		status = response.StatusCode
		json.NewDecoder(response.Body).Decode(&resultData)
	}

	jsonPayload, _ := json.Marshal(payload)
	duration := endTime.Sub(startTime)

	return Result{
		FromRow:      index,
		Status:       status,
		Response:     resultData,
		Error:        errMsg,
		Endpoint:     apiURL,
		Payload:      string(jsonPayload),
		TimeReceived: endTime,
		Duration:     duration.String(),
	}
}

func handleResults(results <-chan Result, logFile *os.File) {
	for result := range results {
		logEntry := formatLogEntry(result)
		fmt.Println(logEntry)
		_, err := logFile.WriteString(logEntry)
		if err != nil {
			fmt.Printf("Error writing to log file: %v\n", err)
		}
	}
}

func formatLogEntry(result Result) string {
	var logEntry string
	if result.Error != "" {
		logEntry = fmt.Sprintf("Error in row %d: %v\n", result.FromRow, result.Error)
	} else {
		logEntry = fmt.Sprintf("Row %d successful\n", result.FromRow)
		logEntry += fmt.Sprintf("Role Name: %s, VM Name: %s\n", result.RoleName, result.VmName)
		logEntry += fmt.Sprintf("Status Code: %d, Response: %v\n", result.Status, result.Response)
	}
	logEntry += fmt.Sprintf("Endpoint: %s\nPayload: %s\n", result.Endpoint, result.Payload)
	logEntry += fmt.Sprintf("Time Received: %s, Duration: %s\n", result.TimeReceived.Format("2006-01-02 15:04:05"), result.Duration)
	logEntry += "----------------------------\n"
	return logEntry
}
