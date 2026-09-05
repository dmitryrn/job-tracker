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
	"nice/internal/models"
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

func TestJobExistsFindsJobBySourceAndSourceID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "linkedin", SourceID: "123", SourceURL: "https://www.linkedin.com/jobs/view/123", Title: "Engineer", Company: "Example Co", Workplace: "remote", BodyText: "Build systems", MetadataJSON: "{}",
	}}))

	exists, err := repository.JobExists(context.Background(), "linkedin", "123")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repository.JobExists(context.Background(), "linkedin", "456")
	require.NoError(t, err)
	assert.False(t, exists)
}
