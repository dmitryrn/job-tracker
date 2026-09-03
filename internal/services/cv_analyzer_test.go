package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openai"
)

func TestCVAnalyzerReturnsVersionedEvidenceBackedProfile(t *testing.T) {
	client := &fakeCVCompletionClient{response: openai.ChatResponse{
		Model: "test-model",
		Content: `{
			"name":"Riley Morgan",
			"headline":"Senior Backend Engineer",
			"location":"Berlin, Germany",
			"workAuthorization":[{"value":"Germany and EU","evidence":"Eligible to work in Germany and other EU member states."}],
			"experience":[{"id":"experience-1","company":"RelayFox GmbH","title":"Senior Backend Engineer","startDate":"May 2022","endDate":"Present","evidence":["Designed and operated Go services using PostgreSQL, Kafka, Redis, Docker, and Kubernetes."]}],
			"skills":[{"name":"Go","concept":"golang","experienceIds":["experience-1"],"evidence":["Designed and operated Go services using PostgreSQL, Kafka, Redis, Docker, and Kubernetes."]}],
			"constraints":[{"kind":"employment_type","value":"permanent","evidence":"Looking for a permanent senior backend or platform role."}],
			"unknowns":["The CV does not state a target start date."]
		}`,
	}}
	analyzer := NewCVAnalyzer(client, "cv-test-model")

	analysis, err := analyzer.Analyze(context.Background(), `Riley Morgan
Senior Backend Engineer
Eligible to work in Germany and other EU member states.
Designed and operated Go services using PostgreSQL, Kafka, Redis, Docker, and Kubernetes.
Looking for a permanent senior backend or platform role.`)
	require.NoError(t, err)
	assert.Equal(t, CVAnalyzerVersion, analysis.AnalyzerVersion)
	assert.Equal(t, CVPromptVersion, analysis.PromptVersion)
	assert.Equal(t, "test-model", analysis.Model)
	assert.Equal(t, "cv-test-model", client.model)
	assert.NotEmpty(t, client.session)
	assert.NotEmpty(t, analysis.InputSHA256)
	assert.NotEmpty(t, analysis.AnalyzedAt)
	assert.Equal(t, "Go", analysis.Profile.Skills[0].Name)
	assert.Equal(t, "go", analysis.Profile.Skills[0].Concept)
	assert.Equal(t, []string{"experience-1"}, analysis.Profile.Skills[0].ExperienceIDs)
}

func TestCVAnalyzerRejectsClaimsWithoutEvidence(t *testing.T) {
	analyzer := NewCVAnalyzer(&fakeCVCompletionClient{response: openai.ChatResponse{
		Content: `{"name":"","headline":"","location":"","workAuthorization":[],"experience":[],"skills":[{"name":"Go","concept":"go","experienceIds":[],"evidence":[]}],"constraints":[],"unknowns":[]}`,
	}}, "cv-test-model")

	_, err := analyzer.Analyze(context.Background(), "Go developer")
	assert.ErrorContains(t, err, "skill without evidence")
}

func TestCVAnalyzerRejectsSkillWithAnInvalidExperienceLink(t *testing.T) {
	analyzer := NewCVAnalyzer(&fakeCVCompletionClient{response: openai.ChatResponse{
		Content: `{
			"name":"","headline":"","location":"","workAuthorization":[],
			"experience":[{"id":"experience-1","company":"","title":"Backend Engineer","startDate":"","endDate":"","evidence":["Built Go APIs."]}],
			"skills":[{"name":"Go","concept":"go","experienceIds":["experience-2"],"evidence":["Built Go APIs."]}],
			"constraints":[],"unknowns":[]
		}`,
	}}, "cv-test-model")

	_, err := analyzer.Analyze(context.Background(), "Built Go APIs.")
	assert.ErrorContains(t, err, "invalid experience link")
}

func TestCVAnalyzerRejectsBlankCV(t *testing.T) {
	analyzer := NewCVAnalyzer(&fakeCVCompletionClient{}, "cv-test-model")
	_, err := analyzer.Analyze(context.Background(), " \n ")
	assert.ErrorContains(t, err, "CV text is required")
}

func TestDecodeCVProfileAcceptsJSONWithPreamble(t *testing.T) {
	profile, err := decodeCVProfile("Here is the profile:\n{\"name\":\"\",\"headline\":\"\",\"location\":\"\",\"workAuthorization\":[],\"experience\":[],\"skills\":[],\"constraints\":[],\"unknowns\":[]}")
	require.NoError(t, err)
	assert.Empty(t, profile.Skills)
}

type fakeCVCompletionClient struct {
	response openai.ChatResponse
	err      error
	model    string
	session  string
	request  openai.ChatRequest
}

func (client *fakeCVCompletionClient) Complete(_ context.Context, model, session string, request openai.ChatRequest) (openai.ChatResponse, error) {
	client.model = model
	client.session = session
	client.request = request
	return client.response, client.err
}
