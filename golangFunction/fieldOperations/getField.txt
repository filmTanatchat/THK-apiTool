package fieldOperations

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"thinkerTools/models"
	"thinkerTools/types"
)

// Env represents the environment configuration
type Env struct {
	BaseURL  string `json:"BASE_URL"`
	Email    string `json:"EMAIL"`
	Password string `json:"PASSWORD"`
}

// ResponseData represents the data section of a response
type ResponseData struct {
	CaseID           string  `json:"case_id"`
	Fields           []Field `json:"fields"`
	AdditionalFields []Field `json:"additional_fields"`
}

// ResponseJSON represents a typical JSON response structure
type ResponseJSON struct {
	Response map[string]interface{} `json:"Response"`
	Status   int                    `json:"Status"`
}

type ApplyProductResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CaseID           string  `json:"case_id"`
		AnswerToken      string  `json:"answer_token"`
		Field            Field   `json:"field"`
		AdditionalFields []Field `json:"additional_fields"`
	} `json:"data"`
}

type Label struct {
	Text     string `json:"text"`
	ImageURL string `json:"image_url"`
}

type Choice struct {
	Value string           `json:"value"`
	Label map[string]Label `json:"label"`
}

type Field struct {
	FieldName               string           `json:"field_name"`
	DataType                string           `json:"data_type"`
	CurrentValue            string           `json:"current_value"`
	Label                   map[string]Label `json:"label"`
	Choices                 []Choice         `json:"choices"`
	IsMandatory             bool             `json:"is_mandatory"`
	InputSource             string           `json:"input_source"`
	IsMultipleValuesAllowed bool             `json:"is_multiple_values_allowed"`
	Alias                   string           `json:"alias"`
}

type FormData struct {
	CaseID           string         `json:"case_id"`
	Fields           []models.Field `json:"fields"`
	AdditionalFields []models.Field `json:"additional_fields"`
}

type GetFullFormResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    FormData `json:"data"`
}

var basePath string

// loadJSONFromPath loads JSON from a given file path.
func loadJSONFromPath(path string) (map[string]interface{}, error) {
	filePath := filepath.Join(basePath, path)
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	jsonStr, err := types.RemoveComments(string(file)), nil

	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	return result, err
}

// makeRequest makes an HTTP request and returns the response.
func makeRequest(client *http.Client, apiURL string, headers map[string]string, payload interface{}) (*ResponseJSON, []byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return nil, nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	fmt.Printf("Sending request to URL: %s with payload: %s\n", apiURL, string(data))

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var responseJSON ResponseJSON
	if err := json.Unmarshal(bodyBytes, &responseJSON); err != nil {
		return nil, bodyBytes, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, bodyBytes, fmt.Errorf("received non-200 status: %d", resp.StatusCode)
	}

	return &responseJSON, bodyBytes, nil
}

func handleErr(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %v\n", msg, err)
		os.Exit(1)
	}
}

func makeApplyForProductRequest(client *http.Client, env types.Environment, headers map[string]string) ApplyProductResponse {

	payload, err := loadJSONFromPath(filepath.Join(basePath, "3. dataSource/productName.json"))
	handleErr(err, "Failed to load payload")

	_, bodyBytes, err := makeRequest(client, env.BaseURL+"/question-taskpool/api/v1/apply-for-product", headers, payload)
	handleErr(err, "Error applying for product")

	var applyProductResp ApplyProductResponse
	err = json.Unmarshal(bodyBytes, &applyProductResp)
	handleErr(err, "Error unmarshalling apply product response")

	return applyProductResp
}

func makeGetFullFormRequest(client *http.Client, env types.Environment, headers map[string]string, caseID string) GetFullFormResponse {

	payloadGetFullForm := map[string]string{"case_id": caseID}
	_, bodyBytes, err := makeRequest(client, env.BaseURL+"/question-taskpool/api/v1/get-full-form", headers, payloadGetFullForm)
	handleErr(err, "Error in get-full-form request")

	var fullFormResponse GetFullFormResponse
	err = json.Unmarshal(bodyBytes, &fullFormResponse)
	handleErr(err, "Error decoding full form response")

	return fullFormResponse
}

func extractCSVData(formData FormData, fieldType string) ([]string, []string) {
	var allFields []models.Field // Changed type to models.Field

	if fieldType == "fields" || fieldType == "all" {
		allFields = append(allFields, formData.Fields...)
	}
	if fieldType == "additional_fields" || fieldType == "all" {
		allFields = append(allFields, formData.AdditionalFields...)
	}

	header := []string{"case_id"}
	exampleDataRow := []string{formData.CaseID}

	for _, field := range allFields {
		if field.InputSource != "" {
			continue
		}

		formattedFieldName := formatFieldName(field)
		header = append(header, formattedFieldName)

		exampleValue := exampleDataForField(field)
		exampleDataRow = append(exampleDataRow, exampleValue)
	}

	return header, exampleDataRow
}

func formatFieldName(field models.Field) string {
	formattedName := field.FieldName + "||" + field.DataType
	if field.IsMultipleValuesAllowed {
		formattedName += "||MULTI"
	}
	return formattedName
}

func exampleDataForField(field models.Field) string {

	switch field.DataType {
	case "date":
		if field.IsMultipleValuesAllowed {
			return "DD-MM-YYYY\\DD-MM-YYYY"
		}
		return "DD-MM-YYYY"
	case "date_time":
		if field.IsMultipleValuesAllowed {
			return "DD-MM-YYYY hh:mm:ss\\DD-MM-YYYY hh:mm:ss"
		}
		return "DD-MM-YYYY hh:mm:ss"
	case "boolean":
		if field.IsMultipleValuesAllowed {
			return "true\\false"
		}
		return "true"
	case "number":
		if field.IsMultipleValuesAllowed {
			return "0\\0"
		}
		return "0"
	case "file":
		if field.IsMultipleValuesAllowed {
			return "test1.pdf\\test2.pdf"
		}
		return "test1.pdf"
	case "text":
		if field.IsMultipleValuesAllowed {
			return "text1\\text2"
		}
		return "text"
	default:
		return ""
	}
}

// writeCSV writes header and rows to a CSV file.
func writeCSV(header []string, row []string, path string) error {
	filePath := filepath.Join(basePath, path)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(header); err != nil {
		return err
	}
	if err := writer.Write(row); err != nil {
		return err
	}

	return nil
}

func GetAllField(env types.Environment) ([]models.Field, error) {
	session := &http.Client{}

	// Authenticate and obtain headers
	headers, err := types.Authenticate(session, env.BaseURL+"/authentication/api/v1/login", env.Email, env.Password)
	if err != nil {
		return nil, fmt.Errorf("error during authentication: %w", err)
	}

	applyProductResp := makeApplyForProductRequest(session, env, headers)
	fullFormResponse := makeGetFullFormRequest(session, env, headers, applyProductResp.Data.CaseID)

	// Extract CSV data and write to file (if needed)
	header, rowData := extractCSVData(fullFormResponse.Data, "all")
	if err := writeCSV(header, rowData, "4. answerAndQuestion/questionAllFields.csv"); err != nil {
		return nil, fmt.Errorf("error writing CSV: %w", err)
	}

	return fullFormResponse.Data.Fields, nil
}
