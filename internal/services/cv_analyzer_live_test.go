package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openrouter"
	"nice/internal/config"
)

var updateCVResults = flag.Bool("update-cv-results", false, "refresh CV analyzer snapshots with OpenRouter")

type cvFixtureResult struct {
	Fixture     string     `json:"fixture"`
	GeneratedAt string     `json:"generatedAt"`
	Analysis    CVAnalysis `json:"analysis"`
}

func TestAnalyzeCVFixtures(t *testing.T) {
	if !*updateCVResults {
		t.Skip("set -update-cv-results to call OpenRouter and refresh CV snapshots")
	}

	root := repositoryRoot(t)
	t.Chdir(root)
	configuration, err := config.Load()
	require.NoError(t, err)
	analyzer := NewCVAnalyzer(openrouter.NewClient(configuration), configuration.LLM.JobAnalysis.Model)
	fixtures := cvFixtures(t, root)
	resultsDirectory := filepath.Join(root, "profiles", "results")
	require.NoError(t, os.MkdirAll(resultsDirectory, 0o755))

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			input, err := os.ReadFile(fixture)
			require.NoError(t, err)
			analysis, err := analyzer.Analyze(context.Background(), string(input))
			require.NoError(t, err)

			result := cvFixtureResult{
				Fixture:     filepath.Base(fixture),
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Analysis:    analysis,
			}
			output, err := json.MarshalIndent(result, "", "  ")
			require.NoError(t, err)
			output = append(output, '\n')
			require.NoError(t, os.WriteFile(filepath.Join(resultsDirectory, fixtureResultName(fixture)), output, 0o600))
		})
	}
}

func TestSavedCVAnalysesMatchFixtures(t *testing.T) {
	root := repositoryRoot(t)
	resultsDirectory := filepath.Join(root, "profiles", "results")
	entries, err := os.ReadDir(resultsDirectory)
	if os.IsNotExist(err) {
		t.Skip("no CV analyzer snapshots have been generated")
	}
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".result.json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join(resultsDirectory, entry.Name()))
			require.NoError(t, err)
			var result cvFixtureResult
			require.NoError(t, json.Unmarshal(contents, &result))
			if result.Analysis.AnalyzerVersion != CVAnalyzerVersion || result.Analysis.PromptVersion != CVPromptVersion {
				t.Skip("snapshot uses an older CV analysis contract")
			}
			assert.NotEmpty(t, result.Analysis.Model)

			input, err := os.ReadFile(filepath.Join(root, "profiles", result.Fixture))
			require.NoError(t, err)
			require.NoError(t, validateProfileDraft(&result.Analysis.Profile, string(input)))
			hash := sha256.Sum256([]byte(strings.TrimSpace(string(input))))
			assert.Equal(t, hex.EncodeToString(hash[:]), result.Analysis.InputSHA256)
		})
	}
}

func cvFixtures(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "profiles"))
	require.NoError(t, err)
	fixtures := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !(strings.HasSuffix(name, ".txt") || strings.HasPrefix(name, "source-")) {
			continue
		}
		fixtures = append(fixtures, filepath.Join(root, "profiles", name))
	}
	require.NotEmpty(t, fixtures)
	return fixtures
}

func fixtureResultName(fixture string) string {
	name := filepath.Base(fixture)
	extension := filepath.Ext(name)
	return strings.TrimSuffix(name, extension) + ".result.json"
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
