package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openrouter"
	"nice/internal/config"
)

var updateJobResults = flag.Bool("update-job-results", false, "refresh job analyzer snapshots with OpenRouter")
var jobAnalyzerModelList = flag.String("job-analyzer-models", "", "comma-separated LLM models for job analyzer snapshots")

type jobFixtureResult struct {
	Fixture        string      `json:"fixture"`
	RequestedModel string      `json:"requestedModel"`
	GeneratedAt    string      `json:"generatedAt"`
	Analysis       JobAnalysis `json:"analysis"`
}

func TestAnalyzeJobFixtures(t *testing.T) {
	if !*updateJobResults {
		t.Skip("set -update-job-results to call OpenRouter and refresh job snapshots")
	}

	root := repositoryRoot(t)
	t.Chdir(root)
	configuration, err := config.Load()
	require.NoError(t, err)
	fixtures := jobFixtures(t, root)
	resultsDirectory := filepath.Join(root, "jobs", "results")
	require.NoError(t, os.MkdirAll(resultsDirectory, 0o755))

	for _, model := range configuredJobAnalyzerModels(t, configuration.LLM.Model) {
		analyzer := NewJobAnalyzer(fixedModelCompletionClient{client: openrouter.NewClient(configuration), model: model}, model)
		for _, fixture := range fixtures {
			t.Run(model+"/"+filepath.Base(fixture), func(t *testing.T) {
				job := loadJobFixture(t, filepath.Base(fixture))
				analysis, err := analyzer.Analyze(context.Background(), job)
				require.NoError(t, err)

				result := jobFixtureResult{
					Fixture:        filepath.Base(fixture),
					RequestedModel: model,
					GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
					Analysis:       analysis,
				}
				output, err := json.MarshalIndent(result, "", "  ")
				require.NoError(t, err)
				output = append(output, '\n')
				require.NoError(t, os.WriteFile(filepath.Join(resultsDirectory, jobFixtureResultName(fixture, model)), output, 0o600))
			})
		}
	}
}

func TestSavedJobAnalysesMatchFixtures(t *testing.T) {
	root := repositoryRoot(t)
	resultsDirectory := filepath.Join(root, "jobs", "results")
	entries, err := os.ReadDir(resultsDirectory)
	if os.IsNotExist(err) {
		t.Skip("no job analyzer snapshots have been generated")
	}
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".result.json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join(resultsDirectory, entry.Name()))
			require.NoError(t, err)
			var result jobFixtureResult
			require.NoError(t, json.Unmarshal(contents, &result))
			assert.Equal(t, JobAnalyzerVersion, result.Analysis.AnalyzerVersion)
			assert.Equal(t, JobPromptVersion, result.Analysis.PromptVersion)
			assert.NotEmpty(t, result.Analysis.Model)
			assert.NotEmpty(t, result.RequestedModel)

			job := loadJobFixture(t, result.Fixture)
			normalizedDescription := normalizeJobDescription(job.BodyText)
			require.NoError(t, validateJobAnalysisDraft(&result.Analysis.Analysis, jobQuoteSource(job, normalizedDescription)))
			hash := sha256.Sum256([]byte(canonicalJobInput(job, normalizedDescription)))
			assert.Equal(t, hex.EncodeToString(hash[:]), result.Analysis.InputSHA256)
			assert.Equal(t, normalizedDescription, result.Analysis.NormalizedDescription)
		})
	}
}

func jobFixtures(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "jobs"))
	require.NoError(t, err)
	fixtures := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		fixtures = append(fixtures, filepath.Join(root, "jobs", entry.Name()))
	}
	require.NotEmpty(t, fixtures)
	return fixtures
}

func jobFixtureResultName(fixture, model string) string {
	name := filepath.Base(fixture)
	model = strings.NewReplacer("/", "_", ":", "_").Replace(model)
	return fmt.Sprintf("%s.%s.result.json", strings.TrimSuffix(name, filepath.Ext(name)), model)
}

func configuredJobAnalyzerModels(t *testing.T, defaultModel string) []string {
	t.Helper()
	modelList := *jobAnalyzerModelList
	if strings.TrimSpace(modelList) == "" {
		modelList = defaultModel
	}
	values := strings.Split(modelList, ",")
	models := make([]string, 0, len(values))
	for _, value := range values {
		if model := strings.TrimSpace(value); model != "" {
			models = append(models, model)
		}
	}
	require.NotEmpty(t, models)
	return models
}

type fixedModelCompletionClient struct {
	client JobCompletionClient
	model  string
}

func (client fixedModelCompletionClient) Complete(ctx context.Context, request openrouter.ChatRequest) (openrouter.ChatResponse, error) {
	request.Model = client.model
	return client.client.Complete(ctx, request)
}
