package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/typesafe"
	"nice/internal/models"
)

type fakeTypeSafeClient struct {
	state     any
	questions map[string]typesafe.Question
	response  typesafe.Response
}

func (client *fakeTypeSafeClient) SystemOne(_ context.Context, state any, questions map[string]typesafe.Question) (typesafe.Response, error) {
	client.state = state
	client.questions = questions
	return client.response, nil
}

func TestJobProfileScorerUsesTheSharedSafeProfileProjection(t *testing.T) {
	client := &fakeTypeSafeClient{response: typesafe.Response{
		Model: "jev-1.13.0",
		Answers: map[string]typesafe.Answer{
			"overall_fit": {Type: "score", Score: 7.6},
		},
	}}
	scorer := NewJobProfileScorer(client)
	profile := models.UserProfile{
		ID:          42,
		UpdatedAt:   "2026-09-20T00:00:00Z",
		Headline:    "Platform engineer",
		Summary:     "Builds Go systems. Email me at private@example.test or call +1 555 010 0100.",
		Skills:      []models.UserProfileSkill{{Name: "Go", Notes: "Production work; private@example.test"}},
		WorkHistory: []models.UserProfileWorkHistory{{Company: "Cedar Systems", Title: "Engineer", Body: "Owned Kubernetes platforms."}},
		Education:   []models.UserProfileEducation{{Institution: "Private University", Degree: "BSc", Body: "Computer science."}},
	}

	score, err := scorer.Score(context.Background(), models.Job{Title: "Platform Engineer", BodyText: "Operate Kubernetes."}, profile)

	require.NoError(t, err)
	assert.Equal(t, 8, score)
	require.Len(t, client.questions, 1)
	assert.Len(t, client.questions["overall_fit"].Criteria, 10)
	payload, err := json.Marshal(client.state)
	require.NoError(t, err)
	serialized := string(payload)
	for _, privateValue := range []string{
		"private@example.test",
		"+1 555 010 0100",
		"Private University",
		`"id":42`,
		"2026-09-20T00:00:00Z",
	} {
		assert.NotContains(t, serialized, privateValue)
	}

	assert.Contains(t, serialized, "Cedar Systems")
	assert.Contains(t, serialized, "Kubernetes")
}

func TestJobProfileScorerMapsTheTopTypeSafeLevelToTen(t *testing.T) {
	client := &fakeTypeSafeClient{response: typesafe.Response{
		Answers: map[string]typesafe.Answer{
			"overall_fit": {Type: "score", Score: 9},
		},
	}}
	scorer := NewJobProfileScorer(client)

	score, err := scorer.Score(context.Background(), models.Job{}, models.UserProfile{})

	require.NoError(t, err)
	assert.Equal(t, 10, score)
}

func TestProfileForLLMDoesNotMutateTheStoredProfile(t *testing.T) {
	profile := models.UserProfile{
		Education: []models.UserProfileEducation{{Institution: "Private University"}},
		Summary:   "Email private@example.test",
	}

	projected := profileForLLM(profile)

	assert.Empty(t, projected.Education[0].Body)
	assert.NotContains(t, projected.Summary, "private@example.test")
	assert.Equal(t, "Private University", profile.Education[0].Institution)
	assert.Equal(t, "Email private@example.test", profile.Summary)
}
