package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
)

func TestProviderRunHonorsIntervalAndRecordsStartTime(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	startedAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	run, err := repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt)
	require.NoError(t, err)
	require.True(t, run)

	run, err = repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt.Add(59*time.Minute))
	require.NoError(t, err)
	assert.False(t, run)

	var lastRunAt string
	require.NoError(t, db.QueryRow(`SELECT last_run_at FROM provider_runs WHERE provider = 'adzuna'`).Scan(&lastRunAt))
	assert.Equal(t, startedAt.Format(time.RFC3339Nano), lastRunAt)

	run, err = repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, run)

	require.NoError(t, db.QueryRow(`SELECT last_run_at FROM provider_runs WHERE provider = 'adzuna'`).Scan(&lastRunAt))
	assert.Equal(t, startedAt.Add(time.Hour).Format(time.RFC3339Nano), lastRunAt)
}
