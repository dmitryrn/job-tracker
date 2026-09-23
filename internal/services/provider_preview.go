package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nice/internal/clients/adzuna"
	"nice/internal/clients/jobicy"
	"nice/internal/clients/remotive"
	"nice/internal/config"
	"nice/internal/models"
)

var ErrUnknownDiscoveryProvider = errors.New("unknown discovery provider")

type adzunaFetcher interface {
	Fetch(context.Context, models.AdzunaSearchSettings) ([]models.Job, error)
}

type jobicyFetcher interface {
	Fetch(context.Context, models.JobicySearchSettings) ([]models.Job, error)
}

type linkedInPreviewer interface {
	PreviewStream(context.Context, models.LinkedInSearchSettings, time.Duration, func(models.Job) error) (LinkedInFetchResult, error)
}

type remotiveFetcher interface {
	Fetch(context.Context, models.RemotiveSearchSettings) ([]models.Job, error)
}

type ProviderPreviewService struct {
	adzuna                         adzunaFetcher
	jobicy                         jobicyFetcher
	linkedin                       linkedInPreviewer
	linkedInPreviewRequestInterval time.Duration
	remotive                       remotiveFetcher
}

func NewProviderPreviewService(cfg config.Config, adzunaClient *adzuna.Client, jobicyClient *jobicy.Client, linkedInJobs *LinkedInJobs, remotiveClient *remotive.Client) *ProviderPreviewService {
	return &ProviderPreviewService{
		adzuna:                         adzunaClient,
		jobicy:                         jobicyClient,
		linkedin:                       linkedInJobs,
		linkedInPreviewRequestInterval: cfg.Providers.LinkedIn.PreviewRequestInterval,
		remotive:                       remotiveClient,
	}
}

// Preview fetches with unsaved settings and deliberately does not persist the results.
func (service *ProviderPreviewService) Preview(ctx context.Context, provider string, settings models.DiscoverySettings) ([]models.Job, error) {
	jobs := make([]models.Job, 0)
	err := service.StreamPreview(ctx, provider, settings, func(job models.Job) error {
		jobs = append(jobs, job)
		return nil
	})
	return jobs, err
}

// StreamPreview fetches with unsaved settings and calls onJob for each result.
func (service *ProviderPreviewService) StreamPreview(ctx context.Context, provider string, settings models.DiscoverySettings, onJob func(models.Job) error) error {
	switch provider {
	case "adzuna":
		return service.previewAdzuna(ctx, settings.Adzuna, onJob)
	case "remotive":
		return service.previewRemotive(ctx, settings.Remotive, onJob)
	case "jobicy":
		return service.previewJobicy(ctx, settings.Jobicy, onJob)
	case "linkedin":
		return service.previewLinkedIn(ctx, settings.LinkedIn, onJob)
	default:
		return ErrUnknownDiscoveryProvider
	}
}

func (service *ProviderPreviewService) previewAdzuna(ctx context.Context, settings models.AdzunaSearchSettings, onJob func(models.Job) error) error {
	settings.Query = strings.TrimSpace(settings.Query)
	settings.Country = strings.TrimSpace(settings.Country)
	settings.Workplace = strings.TrimSpace(settings.Workplace)
	if settings.Query == "" || settings.Country == "" || settings.MaxDaysOld < 1 || settings.MaxPages < 1 || settings.ResultsPerPage < 1 || !validWorkplace(settings.Workplace) {
		return fmt.Errorf("%w: check Adzuna required fields and numeric limits", ErrInvalidDiscoverySettings)
	}

	jobs, err := service.adzuna.Fetch(ctx, settings)
	return emitPreviewJobs(jobs, err, onJob)
}

func (service *ProviderPreviewService) previewRemotive(ctx context.Context, settings models.RemotiveSearchSettings, onJob func(models.Job) error) error {
	settings.Query = strings.TrimSpace(settings.Query)
	settings.Category = strings.TrimSpace(settings.Category)
	if settings.Query == "" || settings.Category == "" {
		return fmt.Errorf("%w: check Remotive required fields", ErrInvalidDiscoverySettings)
	}

	jobs, err := service.remotive.Fetch(ctx, settings)
	return emitPreviewJobs(jobs, err, onJob)
}

func (service *ProviderPreviewService) previewJobicy(ctx context.Context, settings models.JobicySearchSettings, onJob func(models.Job) error) error {
	settings.Geo = strings.TrimSpace(settings.Geo)
	settings.Industry = strings.TrimSpace(settings.Industry)
	settings.Tag = strings.TrimSpace(settings.Tag)
	if settings.Count < 1 || settings.Count > 200 {
		return fmt.Errorf("%w: Jobicy results must be between 1 and 200", ErrInvalidDiscoverySettings)
	}

	jobs, err := service.jobicy.Fetch(ctx, settings)
	return emitPreviewJobs(jobs, err, onJob)
}

func (service *ProviderPreviewService) previewLinkedIn(ctx context.Context, searches []models.LinkedInSearchSettings, onJob func(models.Job) error) error {
	if len(searches) == 0 {
		return fmt.Errorf("%w: at least one LinkedIn search is required", ErrInvalidDiscoverySettings)
	}

	for _, settings := range searches {
		settings.Name = strings.TrimSpace(settings.Name)
		settings.Query = strings.TrimSpace(settings.Query)
		settings.Location = strings.TrimSpace(settings.Location)
		settings.PostedWithin = strings.TrimSpace(settings.PostedWithin)
		settings.Workplace = strings.TrimSpace(settings.Workplace)
		settings.ExperienceLevel = strings.TrimSpace(settings.ExperienceLevel)
		if settings.Name == "" || settings.Query == "" || settings.Location == "" || settings.Limit < 1 || settings.Limit > maxLinkedInResults || !validLinkedInPostedWithin(settings.PostedWithin) || !validLinkedInWorkplace(settings.Workplace) || !validLinkedInExperienceLevel(settings.ExperienceLevel) {
			return fmt.Errorf("%w: LinkedIn search query is required and results must be between 1 and %d", ErrInvalidDiscoverySettings, maxLinkedInResults)
		}

		if _, err := service.linkedin.PreviewStream(ctx, settings, service.linkedInPreviewRequestInterval, onJob); err != nil {
			return err
		}
	}

	return nil
}

func emitPreviewJobs(jobs []models.Job, fetchErr error, onJob func(models.Job) error) error {
	if fetchErr != nil {
		return fetchErr
	}

	for _, job := range jobs {
		if err := onJob(job); err != nil {
			return err
		}
	}

	return nil
}
