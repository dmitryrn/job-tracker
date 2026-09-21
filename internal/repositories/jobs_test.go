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

func TestLinkedInWorkplaceClassificationRoundTrips(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	classification := `{"confidence":0.42,"probabilities":{"remote":0.04,"hybrid":0.61,"onsite":0.1,"unknown":0.25}}`
	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source:                      "linkedin",
		SourceID:                    "classification",
		SourceURL:                   "https://www.linkedin.com/jobs/view/classification",
		Title:                       "Engineer",
		Workplace:                   "hybrid",
		WorkplaceClassificationJSON: &classification,
		MetadataJSON:                "{}",
	}}))

	job, err := repository.JobBySourceID(context.Background(), "linkedin", "classification")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "hybrid", job.Workplace)
	require.NotNil(t, job.WorkplaceClassificationJSON)
	assert.JSONEq(t, classification, *job.WorkplaceClassificationJSON)
}

func TestMatchQueueDoesNotStoreDuplicateJobs(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "example", SourceID: "first", SourceURL: "https://example.com/first", Title: "First", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "example", SourceID: "second", SourceURL: "https://example.com/second", Title: "Second", Workplace: "remote", MetadataJSON: "{}"},
	}))

	_, err = repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	_, err = repository.QueueJobMatch(context.Background(), 2, false)
	require.NoError(t, err)
	_, err = repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)

	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, []int64{queue[0].ID, queue[1].ID})

	found, err := repository.ReplaceMatchQueue(context.Background(), []int64{2, 1, 2})
	require.NoError(t, err)
	require.True(t, found)
	queue, err = repository.MatchQueue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{2, 1}, []int64{queue[0].ID, queue[1].ID})
}

func TestSaveJobProfileMatchScoreStoresAndListsScore(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "score", SourceURL: "https://example.com/score", Title: "Engineer", Workplace: "remote", MetadataJSON: "{}",
	}}))
	require.NoError(t, repository.SaveJobProfileMatchScore(context.Background(), 1, 8))

	job, err := repository.Job(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, job.ProfileMatchScore)
	assert.Equal(t, 8, *job.ProfileMatchScore)

	page, err := repository.List(context.Background(), models.JobSearch{Limit: 10})
	require.NoError(t, err)
	require.Len(t, page.Jobs, 1)
	require.NotNil(t, page.Jobs[0].ProfileMatchScore)
	assert.Equal(t, 8, *page.Jobs[0].ProfileMatchScore)
}

func TestMarkJobViewedStoresLastViewedAt(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "viewed", SourceURL: "https://example.com/viewed", Title: "Engineer", Workplace: "remote", MetadataJSON: "{}",
	}}))

	var before sql.NullString
	require.NoError(t, db.QueryRow(`SELECT last_viewed_at FROM jobs WHERE id = 1`).Scan(&before))
	assert.False(t, before.Valid)

	require.NoError(t, repository.MarkJobViewed(context.Background(), 1))
	job, err := repository.Job(context.Background(), 1)
	require.NoError(t, err)
	assert.NotEmpty(t, job.LastViewedAt)
	_, err = time.Parse(time.RFC3339Nano, job.LastViewedAt)
	require.NoError(t, err)
}

func TestApplicationsCanBeCreatedListedAndRemoved(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "example", SourceID: "applied", SourceURL: "https://example.com/applied", Title: "Applied role", Workplace: "remote", MetadataJSON: "{}"},
	}))

	created, err := repository.CreateApplication(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), created.JobID)
	assert.NotEmpty(t, created.AppliedAt)

	duplicate, err := repository.CreateApplication(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, created.AppliedAt, duplicate.AppliedAt)

	page, err := repository.Applications(context.Background(), models.ApplicationSearch{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Applications, 1)
	assert.Equal(t, "Applied role", page.Applications[0].Job.Title)

	removed, err := repository.DeleteApplication(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, removed)
	removed, err = repository.DeleteApplication(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, removed)
}
