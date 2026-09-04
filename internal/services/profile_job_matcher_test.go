package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openai"
	"nice/internal/models"
)

func TestLLMProfileJobMatcherReturnsValidatedAssessment(t *testing.T) {
	client := &recordingMatchCompletionClient{response: openai.ChatResponse{
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
	assert.Equal(t, "matcher-test-model", client.model)
	assert.NotEmpty(t, client.session)
	assert.Equal(t, "high", client.request.ReasoningEffort)
	assert.Equal(t, profileMatchMaxTokens, client.request.MaxTokens)
	assert.Contains(t, client.request.Messages[1].Content, `"title":"Senior Backend Engineer"`)
	assert.Contains(t, client.request.Messages[1].Content, `"headline":"Backend engineer"`)
}

func TestLLMProfileJobMatcherRejectsInvalidScore(t *testing.T) {
	client := &recordingMatchCompletionClient{response: openai.ChatResponse{Model: "test-model", Content: `{
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

func TestLLMProfileJobMatcherOmitsInstitutionNames(t *testing.T) {
	client := &recordingMatchCompletionClient{response: openai.ChatResponse{Model: "test-model", Content: `{
		"score":84,
		"summary":"Strong fit.",
		"strengths":[],
		"gaps":[],
		"questions":[],
		"applicationAngle":"Highlight platform experience."
	}`}}
	profile := models.UserProfile{
		WorkAuthorization: "Authorized to work in Canada",
		Summary:           "Built systems at Cedar Systems.",
		WorkHistory: []models.UserProfileWorkHistory{{
			Company: "Cedar Systems",
			Body:    "Led Cedar Systems engineering.",
		}},
		Education: []models.UserProfileEducation{{
			Institution: "Riverside College",
			Body:        "Studied at Riverside College.",
		}},
	}

	_, err := NewLLMProfileJobMatcher(client, "matcher-test-model", "low").Match(context.Background(), models.BrowseJob{}, models.JobAnalysisRecord{}, profile)
	require.NoError(t, err)

	providerPayload, err := json.Marshal(client.request)
	require.NoError(t, err)
	for _, sensitiveValue := range []string{"Riverside College"} {
		assert.NotContains(t, string(providerPayload), sensitiveValue)
	}
	assert.Contains(t, string(providerPayload), "Cedar Systems")
	assert.Contains(t, string(providerPayload), "Authorized to work in Canada")
	assert.Equal(t, "Cedar Systems", profile.WorkHistory[0].Company)
}

func TestLLMProfileJobMatcherAcceptsAJSONCodeFence(t *testing.T) {
	client := &recordingMatchCompletionClient{response: openai.ChatResponse{Model: "test-model", Content: "```json\n{\"score\":84,\"summary\":\"Strong fit.\",\"strengths\":[],\"gaps\":[],\"questions\":[],\"applicationAngle\":\"Highlight Go experience.\"}\n```"}}

	assessment, err := NewLLMProfileJobMatcher(client, "matcher-test-model", "low").Match(context.Background(), models.BrowseJob{}, models.JobAnalysisRecord{}, models.UserProfile{})

	require.NoError(t, err)
	assert.Equal(t, 84, assessment.Score)
}

func TestMatchScoreLabel(t *testing.T) {
	assert.Equal(t, "Skip", matchScoreLabel(39))
	assert.Equal(t, "Possible fit", matchScoreLabel(40))
	assert.Equal(t, "Worth applying", matchScoreLabel(60))
	assert.Equal(t, "Strong fit", matchScoreLabel(75))
	assert.Equal(t, "Exceptional fit", matchScoreLabel(90))
}

type recordingMatchCompletionClient struct {
	response openai.ChatResponse
	err      error
	model    string
	session  string
	request  openai.ChatRequest
}

func (client *recordingMatchCompletionClient) Complete(_ context.Context, model, session string, request openai.ChatRequest) (openai.ChatResponse, error) {
	client.model = model
	client.session = session
	client.request = request
	return client.response, client.err
}
