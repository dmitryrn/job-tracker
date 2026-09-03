package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUsesProviderConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, "6h", "12h", "24h", "48h")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 6*time.Hour, cfg.Providers.Adzuna.SyncInterval)
	assert.Equal(t, 12*time.Hour, cfg.Providers.Remotive.SyncInterval)
	assert.Equal(t, 24*time.Hour, cfg.Providers.Jobicy.SyncInterval)
	assert.Equal(t, 48*time.Hour, cfg.Providers.LinkedIn.SyncInterval)
	assert.Equal(t, 30*time.Second, cfg.Providers.LinkedIn.RequestInterval)
	assert.Equal(t, "https://llm.example.com/v1/chat/completions", cfg.LLM.BaseURL)
	assert.Equal(t, "test-go-key", cfg.LLM.APIKey)
	assert.Equal(t, "job-analysis-model", cfg.LLM.JobAnalysis.Model)
	assert.Equal(t, "high", cfg.LLM.JobAnalysis.ReasoningEffort)
	assert.Equal(t, "profile-matcher-model", cfg.LLM.ProfileMatcher.Model)
	assert.Equal(t, "medium", cfg.LLM.ProfileMatcher.ReasoningEffort)
	assert.Equal(t, "job-chat-model", cfg.LLM.JobChat.Model)
	assert.Equal(t, "high", cfg.LLM.JobChat.ReasoningEffort)
	assert.Equal(t, 2*time.Minute, cfg.JobMatch.RunInterval)
	assert.Equal(t, 1000, cfg.Events.QueueSize)
	assert.Equal(t, 100, cfg.Events.BatchSize)
	assert.Equal(t, time.Second, cfg.Events.FlushInterval)
}

func TestLoadRejectsInvalidProviderDuration(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, "6h", "12h", "daily", "48h")

	_, err := Load()
	assert.ErrorContains(t, err, "duration")
}

func TestLoadRejectsJobicyIntervalBelowOneHour(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, "6h", "12h", "30m", "48h")

	_, err := Load()
	assert.ErrorContains(t, err, "at least 1h")
}

func TestLoadRejectsMissingRequiredSettings(t *testing.T) {
	config := testConfig("6h", "12h", "24h", "48h")
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Adzuna sync interval",
			content: strings.Replace(config, "sync_interval = \"6h\"\n", "", 1),
		},
		{
			name:    "server section",
			content: strings.Replace(config, "[server]\nhttp_address = \":8080\"\n\n", "", 1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeTestConfigContent(t, test.content)

			_, err := Load()
			assert.Error(t, err)
		})
	}
}

func writeTestConfig(t *testing.T, adzunaInterval, remotiveInterval, jobicyInterval, linkedInInterval string) {
	t.Helper()
	writeTestConfigContent(t, testConfig(adzunaInterval, remotiveInterval, jobicyInterval, linkedInInterval))
}

func testConfig(adzunaInterval, remotiveInterval, jobicyInterval, linkedInInterval string) string {
	return fmt.Sprintf(`[server]
http_address = ":8080"

[database]
path = "jobs.db"

[llm]
base_url = "https://llm.example.com/v1/chat/completions"

[llm.job_analysis]
model = "job-analysis-model"
reasoning_effort = "high"

[llm.profile_matcher]
model = "profile-matcher-model"
reasoning_effort = "medium"

[llm.job_chat]
model = "job-chat-model"
reasoning_effort = "high"

[job_match]
run_interval = "2m"

[events]
queue_size = 1000
batch_size = 100
flush_interval = "1s"

[providers.adzuna]
sync_interval = %q

[providers.remotive]
sync_interval = %q

[providers.jobicy]
sync_interval = %q

[providers.linkedin]
sync_interval = %q
request_interval = "30s"
`, adzunaInterval, remotiveInterval, jobicyInterval, linkedInInterval)
}

func writeTestConfigContent(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0o600))
	require.NoError(t, os.WriteFile(tokensEnvFile, []byte("ADZUNA_APP_ID=test\nADZUNA_API_KEY=test\nOPENROUTER_API_KEY=test\nOPENCODE_GO_KEY_4=test-go-key\n"), 0o600))
}
