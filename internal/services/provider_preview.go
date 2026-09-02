package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nice/internal/clients/adzuna"
	"nice/internal/clients/jobicy"
	"nice/internal/clients/remotive"
	"nice/internal/models"
)

var ErrUnknownDiscoveryProvider = errors.New("unknown discovery provider")

type adzunaFetcher interface {
	Fetch(context.Context, models.AdzunaSearchSettings) ([]models.Job, error)
}

type jobicyFetcher interface {
	Fetch(context.Context, models.JobicySearchSettings) ([]models.Job, error)
}

type linkedInFetcher interface {
	Fetch(context.Context, models.LinkedInSearchSettings, string, linkedInJobSaver) (LinkedInFetchResult, error)
}

type remotiveFetcher interface {
	Fetch(context.Context, models.RemotiveSearchSettings) ([]models.Job, error)
}

type ProviderPreviewService struct {
	adzuna   adzunaFetcher
	jobicy   jobicyFetcher
	linkedin linkedInFetcher
	remotive remotiveFetcher
}

func NewProviderPreviewService(adzunaClient *adzuna.Client, jobicyClient *jobicy.Client, linkedInJobs *LinkedInJobs, remotiveClient *remotive.Client) *ProviderPreviewService {
	return &ProviderPreviewService{
		adzuna:   adzunaClient,
		jobicy:   jobicyClient,
		linkedin: linkedInJobs,
		remotive: remotiveClient,
	}
}

// Preview fetches with unsaved settings and deliberately does not persist the results.
func (service *ProviderPreviewService) Preview(ctx context.Context, provider string, settings models.DiscoverySettings) ([]models.Job, error) {
	switch provider {
	case "adzuna":
		settings.Adzuna.Query = strings.TrimSpace(settings.Adzuna.Query)
		settings.Adzuna.Country = strings.TrimSpace(settings.Adzuna.Country)
		settings.Adzuna.Workplace = strings.TrimSpace(settings.Adzuna.Workplace)
		if settings.Adzuna.Query == "" || settings.Adzuna.Country == "" || settings.Adzuna.MaxDaysOld < 1 || settings.Adzuna.MaxPages < 1 || settings.Adzuna.ResultsPerPage < 1 || !validWorkplace(settings.Adzuna.Workplace) {
			return nil, fmt.Errorf("%w: check Adzuna required fields and numeric limits", ErrInvalidDiscoverySettings)
		}
		return service.adzuna.Fetch(ctx, settings.Adzuna)
	case "remotive":
		settings.Remotive.Query = strings.TrimSpace(settings.Remotive.Query)
		settings.Remotive.Category = strings.TrimSpace(settings.Remotive.Category)
		if settings.Remotive.Query == "" || settings.Remotive.Category == "" {
			return nil, fmt.Errorf("%w: check Remotive required fields", ErrInvalidDiscoverySettings)
		}
		return service.remotive.Fetch(ctx, settings.Remotive)
	case "jobicy":
		settings.Jobicy.Geo = strings.TrimSpace(settings.Jobicy.Geo)
		settings.Jobicy.Industry = strings.TrimSpace(settings.Jobicy.Industry)
		settings.Jobicy.Tag = strings.TrimSpace(settings.Jobicy.Tag)
		if settings.Jobicy.Count < 1 || settings.Jobicy.Count > 200 {
			return nil, fmt.Errorf("%w: Jobicy results must be between 1 and 200", ErrInvalidDiscoverySettings)
		}
		return service.jobicy.Fetch(ctx, settings.Jobicy)
	case "linkedin":
		settings.LinkedIn.Query = strings.TrimSpace(settings.LinkedIn.Query)
		settings.LinkedIn.Location = strings.TrimSpace(settings.LinkedIn.Location)
		if settings.LinkedIn.Query == "" || settings.LinkedIn.Limit < 1 || settings.LinkedIn.Limit > 100 {
			return nil, fmt.Errorf("%w: LinkedIn query is required and results must be between 1 and 100", ErrInvalidDiscoverySettings)
		}
		result, err := service.linkedin.Fetch(ctx, settings.LinkedIn, "", nil)
		return result.Jobs, err
	default:
		return nil, ErrUnknownDiscoveryProvider
	}
}
