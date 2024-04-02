package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	formAddField "thinkerTools/golangFunction/addFieldToForm"
	applyProductMultiCaseId "thinkerTools/golangFunction/applyMultiCaseID"
	"thinkerTools/golangFunction/fieldOperations"
	roleAssignment "thinkerTools/golangFunction/roleAssignment"
	"thinkerTools/types"
	"thinkerTools/views"
)

type MainController struct {
	Config   types.Config
	Session  *http.Client
	Packages []struct {
		Name   string
		Action func(types.Environment) error
	}
}

func NewMainController(config types.Config) *MainController {
	// Initialize your packages/actions here similar to how it's done in main.go
	return &MainController{
		Config:  config,
		Session: &http.Client{},
		Packages: []struct {
			Name   string
			Action func(types.Environment) error
		}{
			{"Form Add Field", AddFieldsFromCSV},
			{"Apply Product", ApplyProductMultiCaseId},
			{"Assign Role", AssignRoleModel},
			{"Send API JSON ", MultiAnswerQuestionJson},
			{"Get All Field", GetAllFieldWrapper},
		},
	}
}

// Authenticate handles user authentication
func (mc *MainController) Authenticate(env *types.Environment) error {
	if mc.Session == nil {
		mc.Session = &http.Client{}
	}

	payload := types.AuthPayload{
		Email:    env.Email,
		Password: env.Password,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, env.BaseURL+"/authentication/api/v1/login", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := mc.Session.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: %v", resp.Status)
	}

	var authResponse types.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		return err
	}

	env.SessionToken = authResponse.Data.SessionID
	return nil
}

func (mc *MainController) Run() {
	for {
		choiceIndex, ok, exit := views.DisplayConfigMenu(mc.Config)
		if exit {
			return // Exit if chosen by the user
		}
		if !ok || choiceIndex < 1 || choiceIndex > len(mc.Config.Environments) {
			fmt.Println("Invalid choice, try again.")
			continue // Go back to the beginning of the loop for another try
		}
		selectedEnv := mc.Config.Environments[choiceIndex-1]

		menuChoice := views.DisplayMainMenu(mc.Packages)
		if menuChoice == "exit" {
			continue // Go back to selecting environment
		}

		choice, err := strconv.Atoi(menuChoice)
		if err != nil || choice < 1 || choice > len(mc.Packages) {
			views.DisplayError(fmt.Errorf("invalid choice"))
			continue // Go back to the beginning of the loop for another try
		}

		if err := mc.Authenticate(&selectedEnv); err != nil {
			views.DisplayError(fmt.Errorf("authentication failed: %w", err))
			continue // Go back to the beginning of the loop for another try
		}

		if err := mc.Packages[choice-1].Action(selectedEnv); err != nil {
			views.DisplayError(err)
			continue
		}
		// Optional: Add a message or logic here if you want something to happen after a successful action
		// Go back to selecting environment after completing the action
	}
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

// Wrapper function for multiAnswerQuestionJson.AnswerMultiCaseId
func MultiAnswerQuestionJson(env types.Environment) error {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		return err
	}

	return applyProductMultiCaseId.AnswerMultiCaseId(env, basePath)
}

// GetAllFieldWrapper wraps the fieldOperations.GetAllField function
func GetAllFieldWrapper(env types.Environment) error {
	_, err := fieldOperations.GetAllField(env)
	return err // Just return the error, ignore the fields for now
}
