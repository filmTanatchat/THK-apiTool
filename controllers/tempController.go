package controllers

import (
	"fmt"
	"strconv"
	"thinkerTools/models"
	"thinkerTools/types"
	"thinkerTools/views"
)

type MainController struct {
	Config   types.Config
	Packages []struct {
		Name   string
		Action func(types.Environment) error
	}
}

func NewMainController(config types.Config) *MainController {
	// Initialize your packages/actions here similar to how it's done in main.go
	return &MainController{
		Config: config,
		Packages: []struct {
			Name   string
			Action func(types.Environment) error
		}{
			{"Form Add Field", models.AddFieldsFromCSV},
			{"Apply Product", models.ApplyProductMultiCaseId},
			{"Assign Role", models.AssignRoleModel},
		},
	}
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

		if err := mc.Packages[choice-1].Action(selectedEnv); err != nil {
			views.DisplayError(err)
			// Optional: Decide if you want to exit or continue after an error
			continue // Go back to selecting environment
		}
		// Optional: Add a message or logic here if you want something to happen after a successful action
		// Go back to selecting environment after completing the action
	}
}
