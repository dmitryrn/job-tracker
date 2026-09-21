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

func TestUpsertSkipsExactBodyDuplicatesWithinProvider(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "linkedin", SourceID: "first", SourceURL: "https://www.linkedin.com/jobs/view/first", Title: "First title", Company: "First Co", Workplace: "remote", BodyText: "Identical raw job description.", MetadataJSON: "{}"},
		{Source: "linkedin", SourceID: "duplicate", SourceURL: "https://www.linkedin.com/jobs/view/duplicate", Title: "Different title", Company: "Different Co", Workplace: "remote", BodyText: "Identical raw job description.", MetadataJSON: "{}"},
		{Source: "remotive", SourceID: "same-body", SourceURL: "https://remotive.com/jobs/same-body", Title: "Different provider", Company: "Remotive Co", Workplace: "remote", BodyText: "Identical raw job description.", MetadataJSON: "{}"},
	}))

	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "linkedin", SourceID: "first", SourceURL: "https://www.linkedin.com/jobs/view/first", Title: "Updated title", Company: "First Co", Workplace: "remote", BodyText: "Identical raw job description.", MetadataJSON: "{}",
	}}))

	var jobCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&jobCount))
	assert.Equal(t, 2, jobCount)

	job, err := repository.JobBySourceID(context.Background(), "linkedin", "first")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "Updated title", job.Title)

	duplicate, err := repository.JobBySourceID(context.Background(), "linkedin", "duplicate")
	require.NoError(t, err)
	assert.Nil(t, duplicate)

	otherProvider, err := repository.JobBySourceID(context.Background(), "remotive", "same-body")
	require.NoError(t, err)
	assert.NotNil(t, otherProvider)
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

func TestJobMatchesFiltersSortsAndPaginates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	ctx := context.Background()
	repository := NewSQLite(db)
	require.NoError(t, repository.Upsert(ctx, []models.Job{
		{Source: "example", SourceID: "first", SourceURL: "https://example.com/first", Title: "First", Company: "Alpha", Location: "Remote", Workplace: "remote", BodyText: "First body", MetadataJSON: "{}"},
		{Source: "example", SourceID: "second", SourceURL: "https://example.com/second", Title: "Second", Company: "Beta", Location: "New York", Workplace: "hybrid", BodyText: "Second body", MetadataJSON: "{}"},
		{Source: "example", SourceID: "third", SourceURL: "https://example.com/third", Title: "Third", Company: "Gamma", Location: "London", Workplace: "onsite", BodyText: "Third body", MetadataJSON: "{}"},
		{Source: "example", SourceID: "rejected", SourceURL: "https://example.com/rejected", Title: "Rejected", Company: "Delta", Workplace: "remote", BodyText: "Rejected body", MetadataJSON: "{}"},
	}))

	require.NoError(t, repository.CreateJobMatch(ctx, 1, `{"matcherVersion":"v1","score":90,"label":"strong"}`))
	require.NoError(t, repository.CreateJobMatch(ctx, 2, `{"matcherVersion":"v1","score":70,"label":"good"}`))
	require.NoError(t, repository.CreateJobMatch(ctx, 3, `{"matcherVersion":"v1","score":40,"label":"weak"}`))
	require.NoError(t, repository.CreateJobMatch(ctx, 4, `{"matcherVersion":"v1","score":99,"label":"strong"}`))
	require.NoError(t, repository.MarkJobViewed(ctx, 2))
	_, err = repository.CreateApplication(ctx, 2)
	require.NoError(t, err)
	rejected, err := repository.Reject(ctx, 4, "not relevant")
	require.NoError(t, err)
	require.True(t, rejected)

	page, err := repository.JobMatches(ctx, models.JobMatchSearch{Sort: "score-desc", Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, page.Total)
	require.Len(t, page.Matches, 2)
	assert.Equal(t, []int64{1, 2}, []int64{page.Matches[0].Job.ID, page.Matches[1].Job.ID})
	assert.Equal(t, []int{90, 70}, []int{page.Matches[0].Score, page.Matches[1].Score})
	assert.Equal(t, []string{"strong", "good"}, []string{page.Matches[0].Label, page.Matches[1].Label})
	assert.Equal(t, "Alpha", page.Matches[0].Job.Company)
	assert.Equal(t, "First body", page.Matches[0].Job.BodyText)
	assert.False(t, page.Matches[0].Applied)
	assert.True(t, page.Matches[1].Applied)

	page, err = repository.JobMatches(ctx, models.JobMatchSearch{Sort: "score-desc", Limit: 1, Offset: 1})
	require.NoError(t, err)
	assert.Equal(t, 3, page.Total)
	require.Len(t, page.Matches, 1)
	assert.Equal(t, int64(2), page.Matches[0].Job.ID)

	minimumScore := 80
	page, err = repository.JobMatches(ctx, models.JobMatchSearch{
		MinimumScore: &minimumScore,
		Viewed:       "unseen",
		Applied:      "not-applied",
		Limit:        10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Matches, 1)
	assert.Equal(t, int64(1), page.Matches[0].Job.ID)

	page, err = repository.JobMatches(ctx, models.JobMatchSearch{Viewed: "seen", Applied: "applied", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Matches, 1)
	assert.Equal(t, int64(2), page.Matches[0].Job.ID)
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
