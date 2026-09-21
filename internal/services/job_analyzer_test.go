package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openai"
	"nice/internal/models"
)

type fakeJobCompletionClient struct {
	response  openai.ChatResponse
	responses []openai.ChatResponse
	err       error
	model     string
	session   string
	request   openai.ChatRequest
	sessions  []string
	requests  []openai.ChatRequest
}

func TestJobAnalyzerReturnsVersionedEvidenceBackedAnalysis(t *testing.T) {
	client := &fakeJobCompletionClient{response: openai.ChatResponse{
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
	assert.Equal(t, "analyzer-test-model", client.model)
	assert.NotEmpty(t, client.session)
	assert.NotEmpty(t, analysis.InputSHA256)
	assert.NotEmpty(t, analysis.AnalyzedAt)
	assert.Equal(t, "go", analysis.Analysis.Requirements[0].Concept)
	assert.Equal(t, "full_stack_software_engineer", analysis.Analysis.Role.Family)
	assert.Contains(t, analysis.NormalizedDescription, "Must have 5 years of professional Golang experience.")
	assert.Contains(t, client.request.Messages[1].Content, "Supplementary provider metadata:")
	assert.Equal(t, "high", client.request.ReasoningEffort)
	assert.Equal(t, jobAnalysisMaxTokens, client.request.MaxTokens)
}

func TestJobAnalyzerRejectsClaimsWithoutPostingQuotes(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openai.ChatResponse{
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
	assert.ErrorContains(t, err, `quote "Must have Go experience." is not in source`)
}

func TestJobAnalyzerRetriesAnInvalidResponseWithCorrectionContext(t *testing.T) {
	invalid := `{
		"role":{"family":"backend_engineering","seniority":"senior","seniorityConfidence":"high"},
		"constraints":[],
		"requirements":[{"id":"go-experience","kind":"must_have","concept":"go","minimumYears":null,"screeningRisk":"high","quote":"Must have Go experience.","confidence":"high"}],
		"responsibilities":[],
		"preferences":[],
		"unknowns":[]
	}`
	valid := `{
		"role":{"family":"backend_engineering","seniority":"senior","seniorityConfidence":"high"},
		"constraints":[],
		"requirements":[{"id":"go-experience","kind":"must_have","concept":"go","minimumYears":null,"screeningRisk":"high","quote":"Build Go APIs.","confidence":"high"}],
		"responsibilities":[],
		"preferences":[],
		"unknowns":[]
	}`
	client := &fakeJobCompletionClient{responses: []openai.ChatResponse{{Content: invalid}, {Content: valid}}}

	analysis, err := NewJobAnalyzer(client, "analyzer-test-model", "high").Analyze(context.Background(), models.Job{Title: "Backend Engineer", BodyText: "Build Go APIs."})

	require.NoError(t, err)
	assert.Equal(t, "Build Go APIs.", analysis.Analysis.Requirements[0].Quote)
	require.Len(t, client.requests, 2)
	assert.Equal(t, client.sessions[0], client.sessions[1])
	assert.Equal(t, "assistant", client.requests[1].Messages[2].Role)
	assert.Equal(t, invalid, client.requests[1].Messages[2].Content)
	assert.Equal(t, "user", client.requests[1].Messages[3].Role)
	assert.Contains(t, client.requests[1].Messages[3].Content, `quote "Must have Go experience." is not in source`)
	assert.Equal(t, []LLMResponseRejection{{Attempt: 1, Reason: "quote_not_in_source"}}, analysis.RetryMetadata.Rejections)
}

func TestJobAnalyzerAcceptsLabeledAuthoritativeFieldQuote(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openai.ChatResponse{
		Model: "test-model",
		Content: `{
			"role":{"family":"backend_engineering","seniority":"unknown","seniorityConfidence":"low"},
			"constraints":[{"kind":"employment_type","value":"full-time","quote":"Employment type: Full-Time","confidence":"high"}],
			"requirements":[],
			"responsibilities":[],
			"preferences":[],
			"unknowns":[]
		}`,
	}}, "analyzer-test-model", "high")

	analysis, err := analyzer.Analyze(context.Background(), models.Job{
		Title:          "Backend Engineer",
		EmploymentType: "Full-Time",
		BodyText:       "Build APIs.",
	})
	require.NoError(t, err)
	assert.Equal(t, "employment_type", analysis.Analysis.Constraints[0].Kind)
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
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openai.ChatResponse{
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
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openai.ChatResponse{
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

func TestValidateJobAnalysisDraftCanonicalizesAndPreservesClaims(t *testing.T) {
	draft := models.JobAnalysisDraft{
		Role:         models.JobRole{Family: "Backend Engineering", Seniority: "Senior", SeniorityConfidence: "high"},
		Constraints:  []models.JobConstraint{{Kind: "workplace", Value: "remote", Quote: "This is remote.", Confidence: "high"}},
		Requirements: []models.JobRequirement{{ID: "Go Experience", Kind: "must_have", Concept: "Go", ScreeningRisk: "high", Quote: "Solid Go experience.", Confidence: "high"}},
		Preferences:  []models.JobEvidence{{Concept: "Kubernetes", Quote: "Experience with Kubernetes is a plus."}},
		Unknowns:     []string{"The posting does not state a salary."},
	}

	require.NoError(t, validateJobAnalysisDraft(&draft, "This is remote. Solid Go experience. Experience with Kubernetes is a plus."))
	assert.Equal(t, "backend_engineering", draft.Role.Family)
	assert.Equal(t, "senior", draft.Role.Seniority)
	assert.Equal(t, "go", draft.Requirements[0].Concept)
	assert.Equal(t, "go-experience", draft.Requirements[0].ID)
	assert.Equal(t, "strong_preference", draft.Requirements[0].Kind)
	assert.Equal(t, "kubernetes", draft.Preferences[0].Concept)
}

func TestValidateJobAnalysisDraftRejectsInvalidClaims(t *testing.T) {
	tests := []struct {
		name  string
		draft models.JobAnalysisDraft
		error string
	}{
		{
			name:  "invalid role",
			draft: models.JobAnalysisDraft{Role: models.JobRole{Family: "", Seniority: "senior", SeniorityConfidence: "high"}, Unknowns: []string{"unknown"}},
			error: "invalid role",
		},
		{
			name: "duplicate requirement",
			draft: models.JobAnalysisDraft{
				Role: models.JobRole{Family: "backend", Seniority: "senior", SeniorityConfidence: "high"},
				Requirements: []models.JobRequirement{
					{ID: "go", Kind: "must_have", Concept: "go", ScreeningRisk: "high", Quote: "Go", Confidence: "high"},
					{ID: "go", Kind: "must_have", Concept: "go", ScreeningRisk: "high", Quote: "Go", Confidence: "high"},
				},
			},
			error: "invalid requirement ID",
		},
		{
			name:  "blank unknown",
			draft: models.JobAnalysisDraft{Role: models.JobRole{Family: "backend", Seniority: "senior", SeniorityConfidence: "high"}, Unknowns: []string{""}},
			error: "blank unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateJobAnalysisDraft(&test.draft, "Go")
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.error)
		})
	}
}

func TestJobAnalyzerRejectsAnEmptyExtraction(t *testing.T) {
	analyzer := NewJobAnalyzer(&fakeJobCompletionClient{response: openai.ChatResponse{
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

func (client *fakeJobCompletionClient) Complete(_ context.Context, model, session string, request openai.ChatRequest) (openai.ChatResponse, error) {
	client.model = model
	client.session = session
	client.request = request
	client.sessions = append(client.sessions, session)
	client.requests = append(client.requests, request)
	if len(client.responses) == 0 {
		return client.response, client.err
	}

	response := client.responses[0]
	client.responses = client.responses[1:]
	return response, client.err
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

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return root
}
