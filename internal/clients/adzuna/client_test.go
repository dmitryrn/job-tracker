package adzuna

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSalaryIsPredictedAcceptsAdzunaStringValue(t *testing.T) {
	var result response
	require.NoError(t, json.Unmarshal([]byte(`{"results":[{"salary_is_predicted":"1"},{"salary_is_predicted":0}]}`), &result))

	assert.True(t, salaryIsPredicted(result.Results[0].SalaryIsPredicted))
	assert.False(t, salaryIsPredicted(result.Results[1].SalaryIsPredicted))
}
