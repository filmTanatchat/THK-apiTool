package types

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"thinkerTools/models"

	"gopkg.in/yaml.v2"
)

var basePath string

// Authenticate handles user authentication and returns a headers map if successful.
func Authenticate(session *http.Client, loginURL, email, password string) (map[string]string, error) {
	payload := models.AuthPayload{Email: email, Password: password}
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

	var authResponse models.AuthResponse
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
func LoadConfig(filename string) (models.Config, error) {
	var cfg models.Config
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
func SelectEnvironment(cfg models.Config) (models.Environment, bool, bool) {
	if len(cfg.Environments) == 0 {
		fmt.Println("No environments found in the configuration.")
		return models.Environment{}, false, false
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
		return models.Environment{}, false, true
	}

	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > len(cfg.Environments) {
		fmt.Println("Invalid choice")
		return models.Environment{}, false, false
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

// makeRequest makes an HTTP request with a given method and returns the response.
func MakeRequest(client *http.Client, method, apiURL string, headers map[string]string, payload interface{}) (*http.Response, error) {
	var reqBody io.Reader

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequest(method, apiURL, reqBody)
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
