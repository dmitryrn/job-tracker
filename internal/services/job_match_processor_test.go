package services

import (
	"context"
	"database/sql"
	"errors"
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
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	matcher := &recordingProfileJobMatcher{content: "First assessment"}
	processor := NewJobMatchProcessor(repository, matcher, zap.NewNop())
	worked, err := processor.process(context.Background())
	require.NoError(t, err)
	require.False(t, worked)
	assert.Zero(t, matcher.calls)

	found, err := repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	require.True(t, found)

	worked, err = processor.process(context.Background())
	require.NoError(t, err)
	require.True(t, worked)
	worked, err = processor.process(context.Background())
	require.NoError(t, err)
	require.False(t, worked)

	match, err := repository.JobMatch(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, match)
	assert.Equal(t, "First assessment", match.Content)
	assert.Equal(t, 1, matcher.calls)
}

func TestJobMatchProcessorKeepsFailedRequestInQueue(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	found, err := repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	require.True(t, found)

	matcher := &recordingProfileJobMatcher{err: errors.New("model unavailable")}
	processor := NewJobMatchProcessor(repository, matcher, zap.NewNop())
	worked, err := processor.process(context.Background())
	require.True(t, worked)
	require.EqualError(t, err, "model unavailable")
	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, int64(1), queue[0].ID)
}

type recordingProfileJobMatcher struct {
	calls   int
	content string
	err     error
}

func (matcher *recordingProfileJobMatcher) Match(_ context.Context, _ models.BrowseJob, _ models.UserProfile) (string, error) {
	matcher.calls++
	return matcher.content, matcher.err
}
