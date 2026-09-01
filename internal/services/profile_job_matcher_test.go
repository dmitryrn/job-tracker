package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openrouter"
	"nice/internal/models"
)

func TestLLMProfileJobMatcherReturnsValidatedAssessment(t *testing.T) {
	client := &recordingMatchCompletionClient{response: openrouter.ChatResponse{
		Model: "test-model",
		Content: `{
			"score":84,
			"summary":"Strong platform engineering fit with an adjacent infrastructure gap.",
			"strengths":["The profile demonstrates ownership of Go services."],
			"gaps":["Kubernetes experience is not established."],
			"questions":["Can the candidate work in Germany?"],
			"applicationAngle":"Lead with production Go service ownership."
		}`,
	}}

	assessment, err := NewLLMProfileJobMatcher(client, "matcher-test-model", "high").Match(context.Background(),
		models.BrowseJob{ID: 1, Title: "Senior Backend Engineer", BodyText: "Build Go services."},
		models.JobAnalysisRecord{JobID: 1, Analysis: models.JobAnalysisDraft{Requirements: []models.JobRequirement{{ID: "go", Concept: "go", Kind: "must_have", Quote: "Build Go services."}}}},
		models.UserProfile{Headline: "Backend engineer", Skills: []models.UserProfileSkill{{Name: "Go", Notes: "Production services"}}},
	)

	require.NoError(t, err)
	assert.Equal(t, 84, assessment.Score)
	assert.Equal(t, "Strong fit", assessment.Label)
	assert.Equal(t, LLMProfileJobMatcherVersion, assessment.MatcherVersion)
	assert.Equal(t, "test-model", assessment.Model)
	assert.Equal(t, "matcher-test-model", client.request.Model)
	assert.Equal(t, "high", client.request.ReasoningEffort)
	assert.Equal(t, profileMatchMaxTokens, client.request.MaxTokens)
	assert.Contains(t, client.request.Messages[1].Content, `"title":"Senior Backend Engineer"`)
	assert.Contains(t, client.request.Messages[1].Content, `"headline":"Backend engineer"`)
}

func TestLLMProfileJobMatcherRejectsInvalidScore(t *testing.T) {
	client := &recordingMatchCompletionClient{response: openrouter.ChatResponse{Model: "test-model", Content: `{
		"score":101,
		"summary":"Too high.",
		"strengths":[],
		"gaps":[],
		"questions":[],
		"applicationAngle":""
	}`}}

	_, err := NewLLMProfileJobMatcher(client, "matcher-test-model", "high").Match(context.Background(), models.BrowseJob{}, models.JobAnalysisRecord{}, models.UserProfile{})
	assert.ErrorContains(t, err, "score must be between 0 and 100")
}

func TestMatchScoreLabel(t *testing.T) {
	assert.Equal(t, "Skip", matchScoreLabel(39))
	assert.Equal(t, "Possible fit", matchScoreLabel(40))
	assert.Equal(t, "Worth applying", matchScoreLabel(60))
	assert.Equal(t, "Strong fit", matchScoreLabel(75))
	assert.Equal(t, "Exceptional fit", matchScoreLabel(90))
}

type recordingMatchCompletionClient struct {
	response openrouter.ChatResponse
	err      error
	request  openrouter.ChatRequest
}

func (client *recordingMatchCompletionClient) Complete(_ context.Context, request openrouter.ChatRequest) (openrouter.ChatResponse, error) {
	client.request = request
	return client.response, client.err
}
