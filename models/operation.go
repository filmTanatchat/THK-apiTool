package models

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// Environment struct holds the configuration for an environment.
type Environment struct {
	Name         string `yaml:"name"`
	BaseURL      string `yaml:"BASE_URL"`
	Email        string `yaml:"EMAIL"`
	Password     string `yaml:"PASSWORD"`
	SessionToken string
	CSVFilePath  string
}

// Config represents a list of environments.
type Config struct {
	Environments []Environment `yaml:"environments"`
}

// AuthPayload holds the authentication credentials.
type AuthPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is the expected response from the authentication request.
type AuthResponse struct {
	Data struct {
		SessionID string `json:"session_id"`
	} `json:"data"`
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

type ResponseJSON struct {
	Response map[string]interface{} `json:"Response"`
	Status   int                    `json:"Status"`
}

type Label struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type Choice struct {
	Value string           `json:"value"`
	Label map[string]Label `json:"label"`
}

// Field represents a single field structure.
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

// EndpointConfig represents a single endpoint configuration in YAML.
type EndpointConfig struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	Method   string `yaml:"method"`
}

type Endpoints struct {
	Configs []EndpointConfig `yaml:"endpoints-config"`
}

// LoadConfig reads environment configuration from a YAML file.
func LoadConfig(filename string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(filename)
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
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

// ReadCaseDataFromCSV reads case data from a CSV file and returns a slice of maps.
func ReadCaseDataFromCSV(filePath string) ([]map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read the first line to check for BOM
	firstLine, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV first line: %w", err)
	}

	// Remove BOM if present
	if len(firstLine) > 0 && strings.HasPrefix(firstLine[0], "\ufeff") {
		firstLine[0] = strings.TrimPrefix(firstLine[0], "\ufeff")
	}

	headers := firstLine
	fmt.Println("CSV Headers:", headers) // Debugging line

	var caseData []map[string]string
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
		fmt.Println("Row Data:", rowData) // Debugging line
		caseData = append(caseData, rowData)
	}
	return caseData, nil
}

// ModifyPayload replaces placeholders in JSON template with values from CSV row.
func ModifyPayload(template string, rowData map[string]string) string {
	modifiedPayload := template
	for key, value := range rowData {
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(modifiedPayload, placeholder) {
			modifiedPayload = strings.ReplaceAll(modifiedPayload, placeholder, value)
		}
	}
	return modifiedPayload
}
