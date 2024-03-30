package views

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"thinkerTools/types"
)

// DisplayMainMenu shows the main menu options to the user.
func DisplayMainMenu(packages []struct {
	Name   string
	Action func(types.Environment) error
}) string {
	displayMenu("Select the package to run (or type 'exit' to quit):", packages)
	return promptForInput("Enter your choice: ")
}

// DisplayConfigMenu shows the environment selection menu.
func DisplayConfigMenu(cfg types.Config) (int, bool, bool) {
	if len(cfg.Environments) == 0 {
		fmt.Println("No environments found in the configuration.")
		return 0, false, false
	}

	displayEnvironments(cfg.Environments)
	choiceStr := promptForInput("Enter your choice: ")

	if strings.ToLower(choiceStr) == "exit" {
		return 0, false, true
	}

	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > len(cfg.Environments) {
		fmt.Println("Invalid choice")
		return 0, false, false
	}

	return choice, true, false
}

// DisplayError shows any error messages to the user.
func DisplayError(err error) {
	fmt.Printf("Error: %v\n", err)
}

// promptForInput is a helper function to read user input.
func promptForInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input. Please try again.")
		return promptForInput(prompt)
	}
	return strings.TrimSpace(input)
}

// displayMenu helps to display the menu options.
func displayMenu(prompt string, packages []struct {
	Name   string
	Action func(types.Environment) error
}) {
	fmt.Println(prompt)
	for i, pkg := range packages {
		fmt.Printf("%d: %s\n", i+1, pkg.Name)
	}
}

// displayEnvironments helps to display the available environments.
func displayEnvironments(environments []types.Environment) {
	fmt.Println("Select environment (or type 'exit' to quit):")
	for i, env := range environments {
		fmt.Printf("%d: %s\n", i+1, env.Name)
	}
}
