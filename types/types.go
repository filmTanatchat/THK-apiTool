package types

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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
