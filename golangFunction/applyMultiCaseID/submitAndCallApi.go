package applyProductMultiCaseId

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
func SendRequest(client *http.Client, method, apiURL, token, payload, logFilePath, cid string) error {
	// Log the payload being sent
	logPayload := fmt.Sprintf("Case ID %s: Sending Payload - %s\n", cid, payload)
	logErr := WriteLogToFile(logFilePath, logPayload)
	if logErr != nil {
		fmt.Printf("Error logging payload to file: %v\n", logErr)
	}

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

	// Log the response
	logResponse := fmt.Sprintf("Case ID %s: Response - Status Code: %d, Body: %s\n", cid, resp.StatusCode, string(body))
	logErr = WriteLogToFile(logFilePath, logResponse)
	if logErr != nil {
		fmt.Printf("Error logging response to file: %v\n", logErr)
	}

	if resp.StatusCode >= 300 {
		fmt.Printf("Case ID %s: Received Error - Status Code: %d\n", cid, resp.StatusCode)
	} else {
		fmt.Printf("Case ID %s: Success - Status Code: %d\n", cid, resp.StatusCode)
	}

	return nil
}

// ListJSONFiles lists JSON files in the directory.
func ListJSONFiles(dirPath string) ([]string, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}

	var jsonFiles []string
	for _, file := range files {
		if file.Type().IsRegular() && strings.HasSuffix(file.Name(), ".json") {
			jsonFiles = append(jsonFiles, file.Name())
		}
	}
	return jsonFiles, nil
}

// ChooseJSONTemplate allows the user to choose a JSON template file.
func ChooseJSONTemplate(dirPath string) (string, error) {
	files, err := ListJSONFiles(dirPath)
	if err != nil {
		return "", err
	}

	fmt.Println("Select a JSON template file:")
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

// AnswerMultiCaseId processes each case ID with the provided JSON template.
func AnswerMultiCaseId(env models.Environment, basePath string) error {
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
	jsonTemplatePath, err := ChooseJSONTemplate(jsonDirPath)
	if err != nil {
		return err
	}

	jsonTemplate, err := ReadJSONTemplate(jsonTemplatePath)
	if err != nil {
		return err
	}

	// Read case data from CSV
	csvPath := filepath.Join(basePath, "3. dataSource", "multiCaseId.csv")
	// Use ReadCaseDataFromCSV from models package
	caseData, err := models.ReadCaseDataFromCSV(csvPath)
	if err != nil {
		return fmt.Errorf("error reading case data: %w", err)
	}

	logFilePath := filepath.Join(basePath, "2. log", "multiAnswer.log")

	var wg sync.WaitGroup
	for _, rowData := range caseData {
		wg.Add(1)
		go func(data map[string]string) {
			defer wg.Done()

			// Extract case_id from the data
			cid, ok := data["case_id"]
			if !ok {
				fmt.Printf("Case ID missing in row: %v\n", data)
				return
			}

			// Modify the payload based on the data from CSV
			modifiedPayload := models.ModifyPayload(jsonTemplate, data)

			// Send the request
			if err := SendRequest(client, selectedMethod, apiURL, token, modifiedPayload, logFilePath, cid); err != nil {
				fmt.Printf("Case ID %s: Processing Error - %v\n", cid, err)
			}
		}(rowData)
	}
	wg.Wait()

	return nil
}
