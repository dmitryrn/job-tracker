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
	writeTestConfig(t, "6h", "12h")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 6*time.Hour, cfg.Providers.Adzuna.SyncInterval)
	assert.Equal(t, 12*time.Hour, cfg.Providers.Remotive.SyncInterval)
	assert.Equal(t, "software engineer", cfg.Providers.Adzuna.Query)
	assert.Equal(t, "backend engineer", cfg.Providers.Remotive.Query)
}

func TestLoadRejectsInvalidProviderDuration(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestConfig(t, "daily", "12h")

	_, err := Load()
	assert.ErrorContains(t, err, "duration")
}

func TestLoadRejectsMissingRequiredSettings(t *testing.T) {
	config := testConfig("6h", "12h")
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Adzuna query",
			content: strings.Replace(config, "query = \"software engineer\"\n", "", 1),
		},
		{
			name:    "Adzuna max days old",
			content: strings.Replace(config, "max_days_old = 30\n", "", 1),
		},
		{
			name:    "Adzuna workplace",
			content: strings.Replace(config, "workplace = \"remote-hybrid\"\n", "", 1),
		},
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

func writeTestConfig(t *testing.T, adzunaInterval, remotiveInterval string) {
	t.Helper()
	writeTestConfigContent(t, testConfig(adzunaInterval, remotiveInterval))
}

func testConfig(adzunaInterval, remotiveInterval string) string {
	return fmt.Sprintf(`[server]
http_address = ":8080"

[database]
path = "jobs.db"

[providers.adzuna]
query = "software engineer"
country = "de"
max_days_old = 30
max_pages = 5
results_per_page = 50
workplace = "remote-hybrid"
sync_interval = %q

[providers.remotive]
query = "backend engineer"
sync_interval = %q
`, adzunaInterval, remotiveInterval)
}

func writeTestConfigContent(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0o600))
	require.NoError(t, os.WriteFile(tokensEnvFile, []byte("ADZUNA_APP_ID=test\nADZUNA_API_KEY=test\n"), 0o600))
}
