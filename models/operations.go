package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	formAddField "thinkerTools/golangFunction/addFieldToForm"
	applyProductMultiCaseId "thinkerTools/golangFunction/applyMultiCaseID"
	roleAssignment "thinkerTools/golangFunction/roleAssignment"
	"thinkerTools/types"

	"gopkg.in/yaml.v2"
)

// LoadConfig reads environment configuration from a YAML file.
func LoadConfig(filename string) (types.Config, error) {
	var cfg types.Config
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

// Authenticate handles user authentication and returns a headers map if successful.
func Authenticate(session *http.Client, loginURL, email, password string) (map[string]string, error) {
	payload := types.AuthPayload{Email: email, Password: password}
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
	var authResponse types.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + authResponse.Data.SessionID,
	}, nil
}

// AddFieldsFromCSV calls the external AddFieldsFromCSV function.
func AddFieldsFromCSV(env types.Environment) error {
	return formAddField.AddFieldsFromCSV(env)
}

// ApplyProductMultiCaseId calls the external ApplyProductMultiCaseId function.
func ApplyProductMultiCaseId(env types.Environment) error {
	basePath, err := os.Getwd()
	if err != nil {
		return err
	}
	return applyProductMultiCaseId.ApplyProductMultiCaseId(env, basePath)
}

// AssignRoleModel is a wrapper function that calls the AssignRole from the roleAssignment package
func AssignRoleModel(env types.Environment) error {
	return roleAssignment.AssignRole(env)
}
