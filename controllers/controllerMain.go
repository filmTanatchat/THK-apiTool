package controllers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	formAddField "thinkerTools/golangFunction/addFieldToForm"
	applyProductMultiCaseId "thinkerTools/golangFunction/applyMultiCaseID"
	"thinkerTools/golangFunction/fieldOperations"
	roleAssignment "thinkerTools/golangFunction/roleAssignment"
	"thinkerTools/models"
	"thinkerTools/views"
)

type MainController struct {
	Config   models.Config
	Session  *http.Client
	Packages []struct {
		Name   string
		Action func(models.Environment) error
	}
}

func NewMainController(config models.Config) *MainController {
	// Initialize your packages/actions here similar to how it's done in main.go
	return &MainController{
		Config:  config,
		Session: &http.Client{},
		Packages: []struct {
			Name   string
			Action func(models.Environment) error
		}{
			{"Form Add Field", AddFieldsFromCSV},
			{"Apply Product", ApplyProductMultiCaseId},
			{"Assign Role", AssignRoleModel},
			{"Send API JSON ", MultiAnswerQuestionJson},
			{"Get All Field", GetAllField},
			{"Answer Question Via CSV", ProcessAnswerQuestionFromCSVData},
		},
	}
}

// Authenticate handles user authentication
func (mc *MainController) Authenticate(env *models.Environment) error {
	if mc.Session == nil {
		mc.Session = &http.Client{}
	}

	payload := models.AuthPayload{
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

	var authResponse models.AuthResponse
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
	}
}

// AddFieldsFromCSV calls the external AddFieldsFromCSV function.
func AddFieldsFromCSV(env models.Environment) error {
	return formAddField.AddFieldsFromCSV(env)
}

// ApplyProductMultiCaseId calls the external ApplyProductMultiCaseId function.
func ApplyProductMultiCaseId(env models.Environment) error {
	basePath, err := os.Getwd()
	if err != nil {
		return err
	}
	return applyProductMultiCaseId.ApplyProductMultiCaseId(env, basePath)
}

// AssignRoleModel is a wrapper function that calls the AssignRole from the roleAssignment package
func AssignRoleModel(env models.Environment) error {
	return roleAssignment.AssignRole(env)
}

// Wrapper function for multiAnswerQuestionJson.AnswerMultiCaseId
func MultiAnswerQuestionJson(env models.Environment) error {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		return err
	}

	return applyProductMultiCaseId.AnswerMultiCaseId(env, basePath)
}

// GetAllField wraps the fieldOperations.GetAllField function
func GetAllField(env models.Environment) error {
	_, err := fieldOperations.GetAllField(env)
	return err // Just return the error, ignore the fields for now
}

// ProcessAnswerQuestionFromCSVData wraps the applyProductMultiCaseId.ProcessAnswerQuestionFromCSVData function
func ProcessAnswerQuestionFromCSVData(env models.Environment) error {
	reader := bufio.NewReader(os.Stdin)

	// Get number of concurrent requests from user
	fmt.Print("Enter the number of concurrent requests: ")
	concurrentStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("error reading number of concurrent requests: %w", err)
	}
	concurrentRequests, err := strconv.Atoi(strings.TrimSpace(concurrentStr))
	if err != nil {
		return fmt.Errorf("invalid number for concurrent requests: %w", err)
	}

	// Call the ProcessAnswerQuestionFromCSVData function from the applyProductMultiCaseId package
	return applyProductMultiCaseId.ProcessAnswerQuestionFromCSVData(env, concurrentRequests)
}
