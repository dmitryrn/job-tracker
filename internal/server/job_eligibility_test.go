package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nice/internal/clients/typesafe"
)

func TestJobEligibilityAnswerUsesQuestionPolarity(t *testing.T) {
	tests := []struct {
		name     string
		question jobEligibilityQuestion
		noul     float64
		answer   string
		positive bool
	}{
		{name: "missing EU rights requirement is positive", question: jobEligibilityQuestions[0], noul: 0.1, answer: "no", positive: true},
		{name: "EU rights requirement is negative", question: jobEligibilityQuestions[0], noul: 0.9, answer: "yes", positive: false},
		{name: "visa sponsorship is positive", question: jobEligibilityQuestions[2], noul: 0.9, answer: "yes", positive: true},
		{name: "missing visa sponsorship is negative", question: jobEligibilityQuestions[2], noul: 0.1, answer: "no", positive: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := typesafe.Response{Answers: map[string]typesafe.Answer{
				test.question.ID: {Type: "noul", Noul: test.noul},
			}}

			result, err := jobEligibilityAnswer(response, test.question)

			require.NoError(t, err)
			require.Equal(t, test.question.ID, result.ID)
			require.Equal(t, test.question.Text, result.Question)
			require.Equal(t, test.answer, result.Answer)
			require.Equal(t, test.positive, result.Positive)
		})
	}
}

func TestJobEligibilityRequestQuestionsUseTrueAndFalseCriteria(t *testing.T) {
	questions := jobEligibilityRequestQuestions()

	for id, question := range questions {
		criteria, ok := question.Criteria.(map[string]string)
		require.True(t, ok, "%s criteria has unexpected type", id)
		require.Len(t, criteria, 2)
		require.Contains(t, criteria, "true")
		require.Contains(t, criteria, "false")
		require.NotContains(t, criteria, "yes")
		require.NotContains(t, criteria, "no")
	}
}

func TestVisaSponsorshipReturnsBackendDisplayRule(t *testing.T) {
	question := jobEligibilityQuestions[2]
	response := typesafe.Response{Answers: map[string]typesafe.Answer{
		question.ID: {Type: "noul", Noul: 0.9},
	}}

	result, err := jobEligibilityAnswer(response, question)

	require.NoError(t, err)
	require.Equal(t, &jobEligibilityConditionGroup{
		Operator: OperatorAnd,
		Conditions: []jobEligibilityCondition{
			{QuestionID: "european_union_job_rights", Value: false},
			{QuestionID: "specific_country_residence", Value: false},
		},
	}, result.CollapseWhen)
}
