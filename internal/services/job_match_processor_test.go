package services

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
	"nice/internal/models"
	"nice/internal/repositories"
)

func TestJobMatchProcessorCreatesOnlyOneMatchPerJob(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	profile, err := repository.SaveUserProfile(context.Background(), models.UserProfile{Name: "Riley", Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	jobs, err := repository.JobsWithoutMatches(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	matcher := &recordingProfileJobMatcher{content: "First assessment"}
	processor := NewJobMatchProcessor(repository, matcher, zap.NewNop())
	require.NoError(t, processor.processJob(context.Background(), jobs[0], profile))
	require.NoError(t, processor.processJob(context.Background(), jobs[0], profile))

	match, err := repository.JobMatch(context.Background(), jobs[0].ID)
	require.NoError(t, err)
	require.NotNil(t, match)
	assert.Equal(t, "First assessment", match.Content)
	assert.Equal(t, 1, matcher.calls)
}

type recordingProfileJobMatcher struct {
	calls   int
	content string
}

func (matcher *recordingProfileJobMatcher) Match(_ context.Context, _ models.BrowseJob, _ models.UserProfile) (string, error) {
	matcher.calls++
	return matcher.content, nil
}
