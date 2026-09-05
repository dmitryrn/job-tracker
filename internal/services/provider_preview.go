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
		settings.Adzuna.Query = strings.TrimSpace(settings.Adzuna.Query)
		settings.Adzuna.Country = strings.TrimSpace(settings.Adzuna.Country)
		settings.Adzuna.Workplace = strings.TrimSpace(settings.Adzuna.Workplace)
		if settings.Adzuna.Query == "" || settings.Adzuna.Country == "" || settings.Adzuna.MaxDaysOld < 1 || settings.Adzuna.MaxPages < 1 || settings.Adzuna.ResultsPerPage < 1 || !validWorkplace(settings.Adzuna.Workplace) {
			return fmt.Errorf("%w: check Adzuna required fields and numeric limits", ErrInvalidDiscoverySettings)
		}
		jobs, err := service.adzuna.Fetch(ctx, settings.Adzuna)
		return emitPreviewJobs(jobs, err, onJob)
	case "remotive":
		settings.Remotive.Query = strings.TrimSpace(settings.Remotive.Query)
		settings.Remotive.Category = strings.TrimSpace(settings.Remotive.Category)
		if settings.Remotive.Query == "" || settings.Remotive.Category == "" {
			return fmt.Errorf("%w: check Remotive required fields", ErrInvalidDiscoverySettings)
		}
		jobs, err := service.remotive.Fetch(ctx, settings.Remotive)
		return emitPreviewJobs(jobs, err, onJob)
	case "jobicy":
		settings.Jobicy.Geo = strings.TrimSpace(settings.Jobicy.Geo)
		settings.Jobicy.Industry = strings.TrimSpace(settings.Jobicy.Industry)
		settings.Jobicy.Tag = strings.TrimSpace(settings.Jobicy.Tag)
		if settings.Jobicy.Count < 1 || settings.Jobicy.Count > 200 {
			return fmt.Errorf("%w: Jobicy results must be between 1 and 200", ErrInvalidDiscoverySettings)
		}
		jobs, err := service.jobicy.Fetch(ctx, settings.Jobicy)
		return emitPreviewJobs(jobs, err, onJob)
	case "linkedin":
		settings.LinkedIn.Query = strings.TrimSpace(settings.LinkedIn.Query)
		settings.LinkedIn.Location = strings.TrimSpace(settings.LinkedIn.Location)
		settings.LinkedIn.PostedWithin = strings.TrimSpace(settings.LinkedIn.PostedWithin)
		settings.LinkedIn.Workplace = strings.TrimSpace(settings.LinkedIn.Workplace)
		settings.LinkedIn.ExperienceLevel = strings.TrimSpace(settings.LinkedIn.ExperienceLevel)
		if settings.LinkedIn.Query == "" || settings.LinkedIn.Location == "" || settings.LinkedIn.Limit < 1 || settings.LinkedIn.Limit > 100 || !validLinkedInPostedWithin(settings.LinkedIn.PostedWithin) || !validLinkedInWorkplace(settings.LinkedIn.Workplace) || !validLinkedInExperienceLevel(settings.LinkedIn.ExperienceLevel) {
			return fmt.Errorf("%w: LinkedIn query is required and results must be between 1 and 100", ErrInvalidDiscoverySettings)
		}
		_, err := service.linkedin.PreviewStream(ctx, settings.LinkedIn, service.linkedInPreviewRequestInterval, onJob)
		return err
	default:
		return ErrUnknownDiscoveryProvider
	}
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
