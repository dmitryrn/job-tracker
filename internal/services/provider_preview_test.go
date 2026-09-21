package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/models"
)

type adzunaPreviewStub struct {
	jobs     []models.Job
	settings models.AdzunaSearchSettings
}

type linkedInPreviewStub struct {
	jobs            []models.Job
	settings        models.LinkedInSearchSettings
	requestInterval time.Duration
}

func TestProviderPreviewUsesSelectedProviderSettings(t *testing.T) {
	adzuna := &adzunaPreviewStub{jobs: []models.Job{{Title: "Platform engineer"}}}
	service := ProviderPreviewService{adzuna: adzuna}
	settings := models.DiscoverySettings{
		Adzuna: models.AdzunaSearchSettings{
			Query:          "  platform engineer ",
			Country:        " de ",
			MaxDaysOld:     14,
			MaxPages:       5,
			ResultsPerPage: 25,
			Workplace:      "remote",
		},
	}

	jobs, err := service.Preview(context.Background(), "adzuna", settings)

	require.NoError(t, err)
	assert.Equal(t, []models.Job{{Title: "Platform engineer"}}, jobs)
	assert.Equal(t, "platform engineer", adzuna.settings.Query)
	assert.Equal(t, "de", adzuna.settings.Country)
	assert.Equal(t, 5, adzuna.settings.MaxPages)
}

func TestProviderPreviewRejectsInvalidOrUnknownProvider(t *testing.T) {
	service := ProviderPreviewService{}

	_, err := service.Preview(context.Background(), "adzuna", models.DiscoverySettings{})
	require.ErrorIs(t, err, ErrInvalidDiscoverySettings)

	_, err = service.Preview(context.Background(), "unknown", models.DiscoverySettings{})
	require.ErrorIs(t, err, ErrUnknownDiscoveryProvider)

	_, err = service.Preview(context.Background(), "linkedin", models.DiscoverySettings{LinkedIn: models.LinkedInSearchSettings{Query: "engineer", Workplace: "remote", Limit: 25}})
	require.ErrorIs(t, err, ErrInvalidDiscoverySettings)
}

func TestProviderPreviewUsesLinkedInFilters(t *testing.T) {
	linkedIn := &linkedInPreviewStub{jobs: []models.Job{{Title: "Platform engineer"}}}
	service := ProviderPreviewService{linkedin: linkedIn, linkedInPreviewRequestInterval: 5 * time.Second}

	jobs, err := service.Preview(context.Background(), "linkedin", models.DiscoverySettings{LinkedIn: models.LinkedInSearchSettings{
		Query:           "  platform engineer ",
		Location:        " Europe ",
		PostedWithin:    " r604800 ",
		Workplace:       " 2 ",
		ExperienceLevel: " 4 ",
		Limit:           1000,
	}})

	require.NoError(t, err)
	assert.Equal(t, []models.Job{{Title: "Platform engineer"}}, jobs)
	assert.Equal(t, "platform engineer", linkedIn.settings.Query)
	assert.Equal(t, "Europe", linkedIn.settings.Location)
	assert.Equal(t, "r604800", linkedIn.settings.PostedWithin)
	assert.Equal(t, "2", linkedIn.settings.Workplace)
	assert.Equal(t, "4", linkedIn.settings.ExperienceLevel)
	assert.Equal(t, 1000, linkedIn.settings.Limit)
	assert.Equal(t, 5*time.Second, linkedIn.requestInterval)
}

func (stub *adzunaPreviewStub) Fetch(_ context.Context, settings models.AdzunaSearchSettings) ([]models.Job, error) {
	stub.settings = settings
	return stub.jobs, nil
}

func (stub *linkedInPreviewStub) Preview(_ context.Context, settings models.LinkedInSearchSettings, requestInterval time.Duration) (LinkedInFetchResult, error) {
	stub.settings = settings
	stub.requestInterval = requestInterval
	return LinkedInFetchResult{Jobs: stub.jobs}, nil
}

func (stub *linkedInPreviewStub) PreviewStream(_ context.Context, settings models.LinkedInSearchSettings, requestInterval time.Duration, onJob func(models.Job) error) (LinkedInFetchResult, error) {
	stub.settings = settings
	stub.requestInterval = requestInterval
	for _, job := range stub.jobs {
		if err := onJob(job); err != nil {
			return LinkedInFetchResult{}, err
		}
	}

	return LinkedInFetchResult{Jobs: stub.jobs}, nil
}
