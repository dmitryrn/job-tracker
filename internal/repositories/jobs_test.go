package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
)

func TestProviderRunHonorsIntervalAndRecordsFailure(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	startedAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	run, err := repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt)
	require.NoError(t, err)
	require.True(t, run)

	fetchErr := errors.New("upstream unavailable")
	require.NoError(t, repository.CompleteProviderRun(context.Background(), "adzuna", fetchErr, startedAt.Add(time.Second)))

	run, err = repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt.Add(59*time.Minute))
	require.NoError(t, err)
	assert.False(t, run)

	var status, lastError string
	require.NoError(t, db.QueryRow(`SELECT status, last_error FROM provider_runs WHERE provider = 'adzuna'`).Scan(&status, &lastError))
	assert.Equal(t, "failed", status)
	assert.Equal(t, fetchErr.Error(), lastError)

	run, err = repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, run)

	require.NoError(t, db.QueryRow(`SELECT status, last_completed_at, last_error FROM provider_runs WHERE provider = 'adzuna'`).Scan(&status, new(sql.NullString), new(sql.NullString)))
	assert.Equal(t, "running", status)
}
