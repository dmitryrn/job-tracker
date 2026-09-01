package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openrouter"
	"nice/internal/models"
)

func TestJobAnalyzerReturnsVersionedEvidenceBackedAnalysis(t *testing.T) {
	client := &fakeJobCompletionClient{response: openrouter.ChatResponse{
		Model: "test-model",
		Content: `{
			"role":{"family":"Full-Stack Software Engineer","seniority":"senior","seniorityConfidence":"high"},
			"constraints":[{"kind":"workplace","value":"remote","quote":"remote","confidence":"high"}],
			"requirements":[{"id":"professional-go-experience","kind":"must_have","concept":"Golang","minimumYears":5,"screeningRisk":"high","quote":"Must have 5 years of professional Golang experience.","confidence":"high"}],
			"responsibilities":[{"concept":"reliable_services","quote":"Build reliable services."}],
			"preferences":[{"concept":"kubernetes","quote":"Experience with Kubernetes is a plus."}],
			"unknowns":["The posting does not state a salary."]
		}`,
	}}
	job := models.Job{
		Source:         "example",
		Title:          "Senior Backend Engineer",
		Workplace:      "remote",
		EmploymentType: "full-time",
		BodyText: `This is a remote role.
Must have 5 years of professional Golang experience.
Build reliable services.
Experience with Kubernetes is a plus.`,
	}

	analysis, err := NewJobAnalyzer(client, "analyzer-test-model", "high").Analyze(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, JobAnalyzerVersion, analysis.AnalyzerVersion)
	assert.Equal(t, JobPromptVersion, analysis.PromptVersion)
	assert.Equal(t, "test-model", analysis.Model)
	assert.NotEmpty(t, analysis.InputSHA256)
	assert.NotEmpty(t, analysis.AnalyzedAt)
	assert.Equal(t, "go", analysis.Analysis.Requirements[0].Concept)
	assert.Equal(t, "full_stack_software_engineer", analysis.Analysis.Role.Family)
	assert.Contains(t, analysis.NormalizedDescription, "Must have 5 years of professional Golang experience.")
	assert.Contains(t, client.request.Messages[1].Content, "Supplementary provider metadata:")
	assert.Equal(t, "analyzer-test-model", client.request.Model)
	assert.Equal(t, "high", client.request.ReasoningEffort)
	assert.Equal(t, jobAnalysisMaxTokens, client.request.MaxTokens)
}

func TestJobAnalyzerRejectsClaimsWithoutPostingQuotes(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openrouter.ChatResponse{
		Content: `{
			"role":{"family":"backend_engineering","seniority":"senior","seniorityConfidence":"high"},
			"constraints":[],
			"requirements":[{"id":"professional-go-experience","kind":"must_have","concept":"go","minimumYears":null,"screeningRisk":"high","quote":"Must have Go experience.","confidence":"high"}],
			"responsibilities":[],
			"preferences":[],
			"unknowns":[]
		}`,
	}}, "analyzer-test-model", "high")

	_, err := analyzer.Analyze(context.Background(), models.Job{Title: "Backend Engineer", BodyText: "Build APIs."})
	assert.ErrorContains(t, err, "quote is not in source")
}

func TestJobAnalyzerRejectsBlankJob(t *testing.T) {
	_, err := NewJobAnalyzer(&fakeJobCompletionClient{}, "analyzer-test-model", "high").Analyze(context.Background(), models.Job{})
	assert.ErrorContains(t, err, "job title or description is required")
}

func TestDecodeJobAnalysisAcceptsAJSONCodeFence(t *testing.T) {
	draft, err := decodeJobAnalysis("```json\n{\"role\":{\"family\":\"backend_engineering\",\"seniority\":\"senior\",\"seniorityConfidence\":\"high\"}}\n```")
	require.NoError(t, err)
	assert.Equal(t, "backend_engineering", draft.Role.Family)
}

func TestDecodeJobAnalysisAcceptsADuplicatedOpeningDelimiter(t *testing.T) {
	draft, err := decodeJobAnalysis("{{\"role\":{\"family\":\"backend_engineering\",\"seniority\":\"senior\",\"seniorityConfidence\":\"high\"}}")
	require.NoError(t, err)
	assert.Equal(t, "backend_engineering", draft.Role.Family)
}

func TestCanonicalJobRequirementID(t *testing.T) {
	assert.Equal(t, "professional-go-experience", canonicalJobRequirementID("Professional_Go Experience"))
}

func TestJobAnalyzerDowngradesAnImplicitMustHave(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openrouter.ChatResponse{
		Content: `{
			"role":{"family":"backend engineering","seniority":"senior","seniorityConfidence":"high"},
			"constraints":[],
			"requirements":[{"id":"go-experience","kind":"must_have","concept":"go","minimumYears":null,"screeningRisk":"high","quote":"solid Go experience","confidence":"high"}],
			"responsibilities":[],
			"preferences":[],
			"unknowns":[]
		}`,
	}}, "analyzer-test-model", "high")

	analysis, err := analyzer.Analyze(context.Background(), models.Job{Title: "Backend Engineer", BodyText: "We need solid Go experience."})
	require.NoError(t, err)
	assert.Equal(t, "strong_preference", analysis.Analysis.Requirements[0].Kind)
}

func TestJobAnalyzerReclassifiesANonOptionalPreference(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openrouter.ChatResponse{
		Content: `{
			"role":{"family":"backend engineering","seniority":"senior","seniorityConfidence":"high"},
			"constraints":[],
			"requirements":[],
			"responsibilities":[],
			"preferences":[{"concept":"go_experience","quote":"solid Go experience"}],
			"unknowns":[]
		}`,
	}}, "analyzer-test-model", "high")

	analysis, err := analyzer.Analyze(context.Background(), models.Job{Title: "Backend Engineer", BodyText: "We need solid Go experience."})
	require.NoError(t, err)
	require.Len(t, analysis.Analysis.Requirements, 1)
	assert.Equal(t, "strong_preference", analysis.Analysis.Requirements[0].Kind)
	assert.Empty(t, analysis.Analysis.Preferences)
}

func TestJobAnalyzerRejectsAnEmptyExtraction(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openrouter.ChatResponse{
		Content: `{
			"role":{"family":"backend_engineering","seniority":"senior","seniorityConfidence":"high"},
			"constraints":[],
			"requirements":[],
			"responsibilities":[],
			"preferences":[],
			"unknowns":[]
		}`,
	}}, "analyzer-test-model", "high")

	_, err := analyzer.Analyze(context.Background(), models.Job{Title: "Backend Engineer", BodyText: "Build APIs."})
	assert.ErrorContains(t, err, "no extracted claims")
}

func TestNormalizeJobDescriptionPreservesHeadingsAndListItems(t *testing.T) {
	job := loadJobFixture(t, "jobicy-staff-backend-ai-systems.json")
	normalized := normalizeJobDescription(job.BodyText)

	assert.Contains(t, normalized, "ABOUT THE TEAM\nThe AI Foundations Team")
	assert.Contains(t, normalized, "WHAT YOU'LL DO\nBuild the core backend systems")
	assert.NotContains(t, normalized, "<h3>")
}

type fakeJobCompletionClient struct {
	response openrouter.ChatResponse
	err      error
	request  openrouter.ChatRequest
}

func (client *fakeJobCompletionClient) Complete(_ context.Context, request openrouter.ChatRequest) (openrouter.ChatResponse, error) {
	client.request = request
	return client.response, client.err
}

func loadJobFixture(t *testing.T, name string) models.Job {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repositoryRoot(t), "jobs", name))
	require.NoError(t, err)

	var fixture struct {
		Source         string `json:"source"`
		SourceURL      string `json:"sourceURL"`
		Title          string `json:"title"`
		Company        string `json:"company"`
		Location       string `json:"location"`
		Workplace      string `json:"workplace"`
		EmploymentType string `json:"employmentType"`
		SalaryMin      *int64 `json:"salaryMin"`
		SalaryMax      *int64 `json:"salaryMax"`
		PostedAt       string `json:"postedAt"`
		BodyText       string `json:"bodyText"`
	}
	require.NoError(t, json.Unmarshal(contents, &fixture))
	return models.Job{
		Source:         fixture.Source,
		SourceURL:      fixture.SourceURL,
		Title:          fixture.Title,
		Company:        fixture.Company,
		Location:       fixture.Location,
		Workplace:      fixture.Workplace,
		EmploymentType: fixture.EmploymentType,
		SalaryMin:      fixture.SalaryMin,
		SalaryMax:      fixture.SalaryMax,
		PostedAt:       fixture.PostedAt,
		BodyText:       fixture.BodyText,
	}
}
