package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	formAddField "MoneyDD/golangFunction/addFieldToForm"
	applyProductMultiCaseId "MoneyDD/golangFunction/applyMultiCaseID"
	multiAnswerQuestionJson "MoneyDD/golangFunction/multiAnswerQuestionJson"
	roleAssignment "MoneyDD/golangFunction/roleAssignment"
	"MoneyDD/types"
)

// Wrapper function for formAddField.AddFieldsFromCSV
func addFieldsFromCSVWrapper(env types.Environment) error {
	return formAddField.AddFieldsFromCSV(env)
}

// Wrapper function for applyProductMultiCaseId.ApplyForProduct
func applyProductMultiCaseIdWrapper(env types.Environment) error {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		return err
	}

	return applyProductMultiCaseId.ApplyProductMultiCaseId(env, basePath)
}

// Wrapper function for multiAnswerQuestionJson.AnswerMultiCaseId
func multiAnswerQuestionJsonWrapper(env types.Environment) error {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		return err
	}

	return multiAnswerQuestionJson.AnswerMultiCaseId(env, basePath)
}

var packages = []struct {
	Name   string
	Action func(types.Environment) error
}{
	{"Form Add Field", addFieldsFromCSVWrapper},
	{"Apply Product", applyProductMultiCaseIdWrapper},
	{"Role Assignment", roleAssignment.AssignRole},
	{"Multi Answer Question Json", multiAnswerQuestionJsonWrapper}, // Add new package here
}

func main() {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		return
	}

	envPath := filepath.Join(basePath, "1. environment", "env.yaml")

	for {
		cfg, err := types.LoadConfig(envPath)
		if err != nil {
			fmt.Printf("Error loading configuration: %v\n", err)
			return
		}

		selectedEnv, ok, exit := types.SelectEnvironment(cfg)
		if exit {
			break // Exit the program if 'exit' is chosen
		}
		if !ok {
			continue
		}

		fmt.Println("Select the package to run (or type 'exit' to quit):")
		for i, pkg := range packages {
			fmt.Printf("%d: %s\n", i+1, pkg.Name)
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your choice: ")
		choiceStr, _ := reader.ReadString('\n')
		choiceStr = strings.TrimSpace(choiceStr)

		// Check if user wants to exit at the package selection stage
		if strings.ToLower(choiceStr) == "exit" {
			break // Exit the program
		}

		choice, err := strconv.Atoi(choiceStr)
		if err != nil || choice < 1 || choice > len(packages) {
			fmt.Println("Invalid choice")
			continue
		}

		if err := packages[choice-1].Action(selectedEnv); err != nil {
			fmt.Printf("Error running %s: %v\n", packages[choice-1].Name, err)
		}
	}
}
