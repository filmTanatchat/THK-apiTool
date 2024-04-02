package fieldOperations

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"thinkerTools/types"
)

var basePath string

func makeApplyForProductRequest(client *http.Client, env types.Environment, headers map[string]string) types.ApplyProductResponse {
	payload, err := types.LoadJSONFromPath(filepath.Join(basePath, "3. dataSource/productName.json"))
	types.HandleErr(err, "Failed to load payload")

	resp, err := types.MakeRequest(client, env.BaseURL+"/question-taskpool/api/v1/apply-for-product", headers, payload)
	types.HandleErr(err, "Error applying for product")

	defer resp.Body.Close()
	var applyProductResp types.ApplyProductResponse
	err = json.NewDecoder(resp.Body).Decode(&applyProductResp)
	types.HandleErr(err, "Error unmarshalling apply product response")

	return applyProductResp
}

func makeGetFullFormRequest(client *http.Client, env types.Environment, headers map[string]string, caseID string) types.GetFullFormResponse {
	payloadGetFullForm := map[string]string{"case_id": caseID}
	resp, err := types.MakeRequest(client, env.BaseURL+"/question-taskpool/api/v1/get-full-form", headers, payloadGetFullForm)
	types.HandleErr(err, "Error in get-full-form request")

	defer resp.Body.Close()
	var fullFormResponse types.GetFullFormResponse
	err = json.NewDecoder(resp.Body).Decode(&fullFormResponse)
	types.HandleErr(err, "Error decoding full form response")

	return fullFormResponse
}

func extractCSVData(formData types.FormData, fieldType string) ([]string, []string) {
	var allFields []types.Field

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

func formatFieldName(field types.Field) string {
	formattedName := field.FieldName + "||" + field.DataType
	if field.IsMultipleValuesAllowed {
		formattedName += "||MULTI"
	}
	return formattedName
}

func exampleDataForField(field types.Field) string {

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

func generateSummary(fields []types.Field) string {
	summary := fmt.Sprintf("Total Fields: %d\n", len(fields))
	mandatoryCount := countMandatoryFields(fields)
	fieldTypes := countFieldTypes(fields)

	summary += fmt.Sprintf("Mandatory Fields: %d\n", mandatoryCount)
	summary += "Field Types Count:\n"
	for dataType, count := range fieldTypes {
		summary += fmt.Sprintf("- %s: %d\n", dataType, count)
	}

	return summary
}

func countMandatoryFields(fields []types.Field) int {
	count := 0
	for _, field := range fields {
		if field.IsMandatory {
			count++
		}
	}
	return count
}

func countFieldTypes(fields []types.Field) map[string]int {
	fieldTypes := make(map[string]int)
	for _, field := range fields {
		fieldTypes[field.DataType]++
	}
	return fieldTypes
}

func GetAllField(env types.Environment) ([]types.Field, error) {
	session := &http.Client{}

	// Authenticate and obtain headers
	headers, err := types.Authenticate(session, env.BaseURL+"/authentication/api/v1/login", env.Email, env.Password)
	if err != nil {
		return nil, fmt.Errorf("error during authentication: %w", err)
	}

	applyProductResp := makeApplyForProductRequest(session, env, headers)
	fullFormResponse := makeGetFullFormRequest(session, env, headers, applyProductResp.Data.CaseID)

	// Extract CSV data
	header, rowData := extractCSVData(fullFormResponse.Data, "all")
	if err := writeCSV(header, rowData, "4. answerAndQuestion/questionAllFields.csv"); err != nil {
		return nil, fmt.Errorf("error writing CSV: %w", err)
	}

	// Generate and print summary
	summary := generateSummary(fullFormResponse.Data.Fields)
	fmt.Println("_____________________\nSummary:\n" + summary + "_____________________")

	return fullFormResponse.Data.Fields, nil
}
