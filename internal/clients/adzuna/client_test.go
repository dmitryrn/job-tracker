package adzuna

import (
	"encoding/json"
	"testing"
)

func TestSalaryIsPredictedAcceptsAdzunaStringValue(t *testing.T) {
	var result response
	if err := json.Unmarshal([]byte(`{"results":[{"salary_is_predicted":"1"},{"salary_is_predicted":0}]}`), &result); err != nil {
		t.Fatal(err)
	}

	if !salaryIsPredicted(result.Results[0].SalaryIsPredicted) {
		t.Error("string value 1 should be predicted")
	}
	if salaryIsPredicted(result.Results[1].SalaryIsPredicted) {
		t.Error("numeric value 0 should not be predicted")
	}
}
