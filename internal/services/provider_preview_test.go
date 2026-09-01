package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/models"
)

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
	assert.ErrorIs(t, err, ErrInvalidDiscoverySettings)

	_, err = service.Preview(context.Background(), "unknown", models.DiscoverySettings{})
	assert.True(t, errors.Is(err, ErrUnknownDiscoveryProvider))
}

type adzunaPreviewStub struct {
	jobs     []models.Job
	settings models.AdzunaSearchSettings
}

func (stub *adzunaPreviewStub) Fetch(_ context.Context, settings models.AdzunaSearchSettings) ([]models.Job, error) {
	stub.settings = settings
	return stub.jobs, nil
}
