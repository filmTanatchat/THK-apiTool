package models

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"thinkerTools/types"

	"gopkg.in/yaml.v2"
)

// EndpointConfig represents a single endpoint configuration in YAML.
type EndpointConfig struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	Method   string `yaml:"method"`
}

type Endpoints struct {
	Configs []EndpointConfig `yaml:"endpoints-config"`
}

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

// LoadEndpoints loads endpoints from a YAML file
func LoadEndpoints(filePath string) (Endpoints, error) {
	var endpoints Endpoints

	file, err := os.ReadFile(filePath)
	if err != nil {
		return endpoints, fmt.Errorf("error reading endpoints file: %w", err)
	}

	err = yaml.Unmarshal(file, &endpoints)
	if err != nil {
		return endpoints, fmt.Errorf("error unmarshaling YAML: %w", err)
	}

	return endpoints, nil
}

// ReadCaseDataFromCSV reads case data from a CSV file and returns a slice of maps.
func ReadCaseDataFromCSV(filePath string) ([]map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	var caseData []map[string]string
	reader := csv.NewReader(file)
	headers, err := reader.Read() // Read headers
	if err != nil {
		return nil, fmt.Errorf("error reading CSV headers: %w", err)
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record: %w", err)
		}

		rowData := make(map[string]string)
		for i, header := range headers {
			rowData[header] = record[i]
		}
		caseData = append(caseData, rowData)
	}
	return caseData, nil
}

// ModifyPayload replaces placeholders in JSON template with values from CSV row.
func ModifyPayload(template string, rowData map[string]string) string {
	modifiedPayload := template
	for key, value := range rowData {
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(modifiedPayload, placeholder) {
			modifiedPayload = strings.ReplaceAll(modifiedPayload, placeholder, value)
		}
	}
	return modifiedPayload
}
