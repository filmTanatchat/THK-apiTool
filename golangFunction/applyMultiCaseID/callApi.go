package applyProductMultiCaseId

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"thinkerTools/models"
	"thinkerTools/types"
)

// ReadJSONTemplate reads the JSON template from the file.
func ReadJSONTemplate(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Remove comments from JSON content
	return types.RemoveComments(string(content)), nil
}

// WriteLogToFile appends a log message to a log file or creates the log file if it does not exist.
func WriteLogToFile(logFilePath, logMessage string) error {
	// Open the file in append mode (os.O_APPEND), create it if it doesn't exist (os.O_CREATE), and open it for writing (os.O_WRONLY)
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening log file: %w", err)
	}
	defer file.Close()

	// Append the log message followed by a newline
	if _, err := file.WriteString(logMessage + "\n"); err != nil {
		return fmt.Errorf("error writing to log file: %w", err)
	}

	return nil
}

// SendRequest sends the request with the modified JSON payload and logs the entire process.
func SendRequest(client *http.Client, method, apiURL, token, payload, logFilePath string, statusCodes *map[int]int, mu *sync.Mutex) error {
	// Log the request
	requestLogEntry := fmt.Sprintf("Sending Request:\nMethod: %s\nURL: %s\nPayload: %s\n", method, apiURL, payload)
	fmt.Println(requestLogEntry) // Print the request details

	// Create and send the request
	req, err := http.NewRequest(method, apiURL, bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Log and print the response
	responseLogEntry := fmt.Sprintf("\nResponse:\nStatus Code: %d\nBody: %s\n", resp.StatusCode, string(body))
	fmt.Println(responseLogEntry) // Print the response details

	// Write the combined log entry to file
	mu.Lock()
	if err := WriteLogToFile(logFilePath, requestLogEntry+responseLogEntry); err != nil {
		fmt.Printf("Error logging request and response to file: %v\n", err)
	}
	mu.Unlock()

	statusCode := resp.StatusCode
	mu.Lock()
	(*statusCodes)[statusCode]++
	mu.Unlock()

	return nil
}

// ListFiles lists files with the specified extension in the directory.
func ListFiles(dirPath, extension string) ([]string, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}

	var fileList []string
	for _, file := range files {
		if file.Type().IsRegular() && strings.HasSuffix(file.Name(), extension) {
			fileList = append(fileList, file.Name())
		}
	}
	sort.Strings(fileList)
	return fileList, nil
}

// ChooseFile allows the user to choose a file from the list.
func ChooseFile(dirPath, extension string) (string, error) {
	files, err := ListFiles(dirPath, extension)
	if err != nil {
		return "", err
	}

	fmt.Printf("Select a %s file:\n", extension)
	for i, file := range files {
		fmt.Printf("%d: %s\n", i+1, file)
	}

	var choice int
	fmt.Print("Enter your choice: ")
	if _, err := fmt.Scan(&choice); err != nil || choice < 1 || choice > len(files) {
		return "", fmt.Errorf("invalid choice")
	}

	return filepath.Join(dirPath, files[choice-1]), nil
}

// CallApiByFile processes each case ID with the provided JSON template.
func CallApiByFile(env models.Environment, basePath string) error {
	client := &http.Client{}

	authData, err := types.Authenticate(client, env.BaseURL+"/authentication/api/v1/login", env.Email, env.Password)
	if err != nil {
		return fmt.Errorf("error during authentication: %w", err)
	}

	authorizationHeader, ok := authData["Authorization"]
	if !ok {
		return fmt.Errorf("authentication token not found")
	}

	splitToken := strings.Split(authorizationHeader, " ")
	if len(splitToken) != 2 {
		return fmt.Errorf("invalid token format")
	}
	token := splitToken[1]

	yamlPath := filepath.Join(basePath, "config", "config.yaml")
	// Use LoadEndpoints from models package
	endpoints, err := models.LoadEndpoints(yamlPath)
	if err != nil {
		return fmt.Errorf("error loading endpoints: %w", err)
	}

	selectedEndpoint, selectedMethod, err := models.SelectEndpoint(endpoints)
	if err != nil {
		return fmt.Errorf("error selecting endpoint: %w", err)
	}
	apiURL := env.BaseURL + selectedEndpoint

	jsonDirPath := filepath.Join(basePath, "5. jsonTemplate")
	jsonTemplatePath, err := ChooseFile(jsonDirPath, ".json")
	if err != nil {
		return err
	}

	jsonTemplate, err := ReadJSONTemplate(jsonTemplatePath)
	if err != nil {
		return err
	}

	// List and choose CSV file
	csvDirPath := filepath.Join(basePath, "3. dataSource")
	csvPath, err := ChooseFile(csvDirPath, ".csv")
	if err != nil {
		return err
	}

	// Use ReadCaseDataFromCSV from models package
	caseData, err := models.ReadCaseDataFromCSV(csvPath)
	if err != nil {
		return fmt.Errorf("error reading case data: %w", err)
	}

	logFilePath := filepath.Join(basePath, "2. log", "callApi.log")

	statusCodes := make(map[int]int)
	var mu sync.Mutex

	var wg sync.WaitGroup
	interval := 1 * time.Second        // Set the interval between requests
	ticker := time.NewTicker(interval) // Rate limit: one request per interval
	defer ticker.Stop()

	semaphore := make(chan struct{}, 5) // Set the maximum number of concurrent requests

	for _, rowData := range caseData {
		<-ticker.C // Wait for the ticker
		wg.Add(1)
		semaphore <- struct{}{} // acquire a slot
		go func(data map[string]string) {
			defer wg.Done()
			defer func() { <-semaphore }() // release the slot

			// Modify the payload based on the data from CSV
			modifiedPayload := models.ModifyPayload(jsonTemplate, data)

			if err := SendRequest(client, selectedMethod, apiURL, token, modifiedPayload, logFilePath, &statusCodes, &mu); err != nil {
				fmt.Printf("Data Processing Error - %v\n", err)
			}
		}(rowData)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Print summary of status codes
	fmt.Println("\nSummary of Status Codes:")
	for code, count := range statusCodes {
		fmt.Printf("Status Code %d: %d occurrences\n", code, count)
	}

	return nil
}
