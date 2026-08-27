package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadUsesSeparateProviderIntervals(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(envFile, []byte(`HTTP_ADDRESS=:8080
DATABASE_PATH=jobs.db
ADZUNA_COUNTRY=de
JOB_QUERY=software engineer
ADZUNA_MAX_DAYS_OLD=30
ADZUNA_MAX_PAGES=5
ADZUNA_RESULTS_PER_PAGE=50
ADZUNA_WORKPLACE=remote-hybrid
ADZUNA_SYNC_INTERVAL=6h
REMOTIVE_SYNC_INTERVAL=12h
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokensEnvFile, []byte("ADZUNA_APP_ID=test\nADZUNA_API_KEY=test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sync.AdzunaInterval != 6*time.Hour {
		t.Errorf("Adzuna interval = %s, want 6h", cfg.Sync.AdzunaInterval)
	}
	if cfg.Sync.RemotiveInterval != 12*time.Hour {
		t.Errorf("Remotive interval = %s, want 12h", cfg.Sync.RemotiveInterval)
	}
}
