package services

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
	"nice/internal/models"
	"nice/internal/repositories"
)

type customJobImportServiceStub struct {
	job models.Job
}

type jobProfileScorerStub struct {
	job     models.Job
	profile models.UserProfile
	score   int
}

func (stub customJobImportServiceStub) Import(context.Context, string) (models.Job, error) {
	return stub.job, nil
}

func (scorer *jobProfileScorerStub) Score(_ context.Context, job models.Job, profile models.UserProfile) (int, error) {
	scorer.job = job
	scorer.profile = profile
	return scorer.score, nil
}

func TestCreateCustomJobScoresAndPersistsProfileFitSynchronously(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Headline: "Senior Go engineer"})
	require.NoError(t, err)
	scorer := &jobProfileScorerStub{score: 8}
	customJobs := NewCustomJobService(repository, customJobImportServiceStub{job: models.Job{
		Source: "custom", SourceID: "https://example.com/job", SourceURL: "https://example.com/job",
		Title: "Go Engineer", BodyText: "Build Go services", Workplace: "unknown", MetadataJSON: "{}",
	}}, repository, scorer)

	job, err := customJobs.Create(context.Background(), models.CustomJob{SourceURL: "https://example.com/job"})

	require.NoError(t, err)
	require.NotNil(t, job.ProfileMatchScore)
	assert.Equal(t, 8, *job.ProfileMatchScore)
	assert.Equal(t, "Go Engineer", scorer.job.Title)
	assert.Equal(t, "Senior Go engineer", scorer.profile.Headline)

	saved, err := repository.Job(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, saved.ProfileMatchScore)
	assert.Equal(t, 8, *saved.ProfileMatchScore)
}
