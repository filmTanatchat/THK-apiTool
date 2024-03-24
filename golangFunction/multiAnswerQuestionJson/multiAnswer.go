package multiAnswerQuestionJson

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"MoneyDD/types"

	"gopkg.in/yaml.v2"
)

// EndpointConfig represents a single endpoint configuration in YAML.
type EndpointConfig struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	Method   string `yaml:"method"`
}

type Endpoints struct {
	Configs []EndpointConfig `yaml:"endpoints-config"`
}

// LoadEndpoints loads endpoints from a YAML file
func LoadEndpoints(filePath string) (Endpoints, error) {
	var endpoints Endpoints

	file, err := os.ReadFile(filePath)
	if err != nil {
		return endpoints, fmt.Errorf("error reading endpoints file: %w", err)
	}

	err = yaml.Unmarshal(file, &endpoints)
	if err != nil {
		return endpoints, fmt.Errorf("error unmarshaling YAML: %w", err)
	}

	return endpoints, nil
}

// SelectEndpoint allows the user to choose an endpoint and returns its URL and method.
func SelectEndpoint(endpoints Endpoints) (string, string, error) {
	fmt.Println("Select an endpoint:")
	for i, config := range endpoints.Configs {
		fmt.Printf("%d: %s - %s (%s)\n", i+1, config.Name, config.Endpoint, config.Method)
	}

	fmt.Print("Enter your choice: ")
	var choice int
	_, err := fmt.Scan(&choice)
	if err != nil {
		return "", "", fmt.Errorf("error reading input: %w", err)
	}

	if choice < 1 || choice > len(endpoints.Configs) {
		return "", "", fmt.Errorf("invalid choice")
	}

	selectedEndpoint := endpoints.Configs[choice-1].Endpoint
	selectedMethod := endpoints.Configs[choice-1].Method
	return selectedEndpoint, selectedMethod, nil
}

// RemoveComments removes comments from a JSON string.
func RemoveComments(jsonStr string) string {
	var inString bool
	var inBlockComment bool
	var inLineComment bool
	var result strings.Builder

	for i := 0; i < len(jsonStr); i++ {
		switch jsonStr[i] {
		case '"':
			if !inLineComment && !inBlockComment && (i == 0 || jsonStr[i-1] != '\\') {
				inString = !inString
			}
			if !inLineComment && !inBlockComment {
				result.WriteByte(jsonStr[i])
			}
		case '/':
			if !inString {
				if !inLineComment && !inBlockComment && i+1 < len(jsonStr) && jsonStr[i+1] == '/' {
					inLineComment = true
				} else if !inLineComment && !inBlockComment && i+1 < len(jsonStr) && jsonStr[i+1] == '*' {
					inBlockComment = true
				} else if !inLineComment && !inBlockComment {
					result.WriteByte(jsonStr[i])
				}
			} else if !inLineComment && !inBlockComment {
				result.WriteByte(jsonStr[i])
			}
		case '*':
			if !inString && inBlockComment && i+1 < len(jsonStr) && jsonStr[i+1] == '/' {
				inBlockComment = false
				i++ // Skip '/'
			} else if !inLineComment && !inBlockComment {
				result.WriteByte(jsonStr[i])
			}
		case '\n':
			if inLineComment {
				inLineComment = false
			}
			if !inBlockComment && !inLineComment {
				result.WriteByte(jsonStr[i])
			}
		default:
			if !inLineComment && !inBlockComment {
				result.WriteByte(jsonStr[i])
			}
		}
	}
	return result.String()
}

// ReadJSONTemplate reads the JSON template from the file.
func ReadJSONTemplate(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Remove comments from JSON content
	return RemoveComments(string(content)), nil
}

// ReadCaseDataFromCSV reads case data from a CSV file and returns a slice of maps.
func ReadCaseDataFromCSV(filePath string) ([]map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	var caseData []map[string]string
	reader := csv.NewReader(file)
	headers, err := reader.Read() // Read headers
	if err != nil {
		return nil, fmt.Errorf("error reading CSV headers: %w", err)
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record: %w", err)
		}

		rowData := make(map[string]string)
		for i, header := range headers {
			rowData[header] = record[i]
		}
		caseData = append(caseData, rowData)
	}
	return caseData, nil
}

// ModifyPayload replaces placeholders in JSON template with values from the CSV.
func ModifyPayload(template string, rowData map[string]string) string {
	modifiedPayload := template
	for key, value := range rowData {
		modifiedPayload = strings.ReplaceAll(modifiedPayload, fmt.Sprintf("{{%s}}", key), value)
	}
	return modifiedPayload
}

// WriteLogToFile creates or overwrites the log file with the specified log message.
func WriteLogToFile(logFilePath, logMessage string) error {
	// Use os.Create to create or truncate the existing file
	file, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("error creating log file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(logMessage + "\n")
	if err != nil {
		return fmt.Errorf("error writing to log file: %w", err)
	}

	return nil
}

// SendRequest sends the request with the modified JSON payload and logs the entire process.
func SendRequest(client *http.Client, method, apiURL, token, originalPayload, modifiedPayload, logFilePath, cid string) error {
	// Determine if any modifications were made
	modificationsMade := originalPayload != modifiedPayload

	// Log message based on whether modifications were made
	logMessage := fmt.Sprintf("Case ID %s: Sending Payload", cid)
	if !modificationsMade {
		logMessage = fmt.Sprintf("Case ID %s: Sending Original Payload (No modifications made)", cid)
	}
	logErr := WriteLogToFile(logFilePath, logMessage)
	if logErr != nil {
		fmt.Printf("Error logging to file: %v\n", logErr)
	}

	// Create and send the request
	req, err := http.NewRequest(method, apiURL, bytes.NewBufferString(modifiedPayload))
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
	if resp.StatusCode >= 300 {
		fmt.Printf("Case ID %s: Received Error - Status Code: %d\n", cid, resp.StatusCode)
		logErr = WriteLogToFile(logFilePath, fmt.Sprintf("Case ID %s: Error Response - Status Code: %d, Body: %s", cid, resp.StatusCode, string(body)))
	} else {
		fmt.Printf("Case ID %s: Success - Status Code: %d\n", cid, resp.StatusCode)
		logErr = WriteLogToFile(logFilePath, fmt.Sprintf("Case ID %s: Success Response - Body: %s", cid, string(body)))
	}
	if logErr != nil {
		fmt.Printf("Error logging to file: %v\n", logErr)
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
func AnswerMultiCaseId(env types.Environment, basePath string) error {
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

	yamlPath := filepath.Join(basePath, "config", "endPoint.yaml")
	endpoints, err := LoadEndpoints(yamlPath)
	if err != nil {
		return fmt.Errorf("error loading endpoints: %w", err)
	}

	selectedEndpoint, selectedMethod, err := SelectEndpoint(endpoints)
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

	csvPath := filepath.Join(basePath, "3. dataSource", "multiCaseId.csv")
	caseData, err := ReadCaseDataFromCSV(csvPath)
	if err != nil {
		return err
	}

	logFilePath := filepath.Join(basePath, "2. log", "multiAnswer.log")

	var wg sync.WaitGroup
	for _, rowData := range caseData {
		wg.Add(1)
		go func(data map[string]string) {
			defer wg.Done()
			cid := data["case_id"]

			originalPayload := jsonTemplate
			modifiedPayload := ModifyPayload(originalPayload, data)

			if err := SendRequest(client, selectedMethod, apiURL, token, originalPayload, modifiedPayload, logFilePath, cid); err != nil {
				fmt.Printf("Case ID %s: Processing Error - %v\n", cid, err)
			}
		}(rowData)
	}
	wg.Wait()
	return nil
}
