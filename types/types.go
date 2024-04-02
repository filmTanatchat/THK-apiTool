package types

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

var basePath string

// Authenticate handles user authentication and returns a headers map if successful.
func Authenticate(session *http.Client, loginURL, email, password string) (map[string]string, error) {
	payload := AuthPayload{Email: email, Password: password}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := session.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("authentication failed: " + resp.Status)
	}

	var authResponse AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + authResponse.Data.SessionID,
	}, nil
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

// SelectEnvironment displays a list of environments and allows the user to select one.
func SelectEnvironment(cfg Config) (Environment, bool, bool) {
	if len(cfg.Environments) == 0 {
		fmt.Println("No environments found in the configuration.")
		return Environment{}, false, false
	}

	fmt.Println("Select environment (or type 'exit' to quit):")
	for i, env := range cfg.Environments {
		fmt.Printf("%d: %s\n", i+1, env.Name)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your choice: ")
	choiceStr, _ := reader.ReadString('\n')
	choiceStr = strings.TrimSpace(choiceStr)

	// Check for exit command
	if strings.ToLower(choiceStr) == "exit" {
		return Environment{}, false, true
	}

	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > len(cfg.Environments) {
		fmt.Println("Invalid choice")
		return Environment{}, false, false
	}

	selectedEnv := cfg.Environments[choice-1]
	return selectedEnv, true, false
}

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

// loadJSONFromPath loads JSON from a given file path.
func LoadJSONFromPath(path string) (map[string]interface{}, error) {
	filePath := filepath.Join(basePath, path)
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	jsonStr := RemoveComments(string(fileData))
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}
	return result, nil
}

// makeRequest makes an HTTP request and returns the response.
func MakeRequest(client *http.Client, apiURL string, headers map[string]string, payload interface{}) (*http.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return client.Do(req)
}

// handleErr prints an error message and exits if there's an error.
func HandleErr(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %v\n", msg, err)
		os.Exit(1)
	}
}
