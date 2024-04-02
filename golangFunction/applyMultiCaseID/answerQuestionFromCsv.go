package applyProductMultiCaseId

import (
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"thinkerTools/types"
)

func ProcessAndSendCSVData(env types.Environment, csvFile string, endpoint string, concurrentRequests int) error {
	// Authenticate once and use the token for all requests
	client := &http.Client{}
	headers, authErr := types.Authenticate(client, env.BaseURL+"/authentication/api/v1/login", env.Email, env.Password)
	if authErr != nil {
		return fmt.Errorf("authentication error: %w", authErr)
	}

	// Open and read CSV File
	csvPath := filepath.Join(env.CSVFilePath, csvFile)
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("error opening CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading CSV data: %w", err)
	}

	// Channel to process records concurrently
	jobs := make(chan []string, concurrentRequests)

	var wg sync.WaitGroup

	// Start worker goroutines
	for w := 0; w < concurrentRequests; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := range jobs {
				jsonPayload, convertErr := convertRecordToJSON(record, records[0], env.CSVFilePath)
				if convertErr != nil {
					fmt.Printf("Error converting record to JSON: %v\n", convertErr)
					continue
				}

				_, sendErr := types.MakeRequest(client, endpoint, headers, jsonPayload)
				if sendErr != nil {
					fmt.Printf("Error sending request: %v\n", sendErr)
					continue
				}
			}
		}()
	}

	// Sending jobs to the channel
	for _, record := range records[1:] {
		jobs <- record
	}
	close(jobs)

	// Wait for all goroutines to finish
	wg.Wait()

	return nil
}

func convertRecordToJSON(record, headers []string, basePath string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"case_id":          record[0],
		"is_question_mode": false,
		"answers":          make([]map[string]interface{}, 0),
	}

	for i, value := range record[1:] {
		convertedValue, err := convertValue(value, headers[i+1], basePath)
		if err != nil {
			return nil, err
		}
		answer := map[string]interface{}{
			"field_name":  headers[i+1],
			"field_value": convertedValue,
			"source":      "customer",
		}
		payload["answers"] = append(payload["answers"].([]map[string]interface{}), answer)
	}

	return payload, nil
}

func convertValue(value, header, basePath string) (interface{}, error) {
	// Determine the data type from the header
	dataType := strings.Split(header, "||")[1]

	switch dataType {
	case "date", "date_time":
		t, err := time.Parse("02-01-2006", value)
		if err != nil {
			return nil, err
		}
		return t.Unix(), nil
	case "boolean":
		return strings.ToLower(value) == "true", nil
	case "file":
		fileData, err := os.ReadFile(filepath.Join(basePath, value))
		if err != nil {
			return nil, err
		}
		return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(fileData), nil
	default:
		return value, nil
	}
}
