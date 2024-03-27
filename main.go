package main

import (
	"fmt"
	"os"
	"path/filepath"
	"thinkerTools/controllers"
	"thinkerTools/models" // Make sure to import the models package
)

func main() {
	basePath, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting working directory:", err)
		return
	}

	configFilePath := filepath.Join(basePath, "config", "config.yaml")

	// Load the configuration using the models package
	config, err := models.LoadConfig(configFilePath)
	if err != nil {
		fmt.Println("Failed to load configuration:", err)
		return
	}

	// Now pass the loaded configuration to NewMainController
	controller := controllers.NewMainController(config)
	controller.Run()
}
