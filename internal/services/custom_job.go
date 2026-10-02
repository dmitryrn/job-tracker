package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

var ErrInvalidCustomJobURL = errors.New("a valid HTTP or HTTPS job posting URL is required")

type CustomJobService struct {
	repository repositories.JobRepository
	importer   JobPostingImporter
	profiles   repositories.UserProfileRepository
	scorer     JobProfileScoreService
}

func NewCustomJobService(repository repositories.JobRepository, importer JobPostingImporter, profiles repositories.UserProfileRepository, scorer JobProfileScoreService) *CustomJobService {
	return &CustomJobService{repository: repository, importer: importer, profiles: profiles, scorer: scorer}
}

func (service *CustomJobService) Create(ctx context.Context, custom models.CustomJob) (models.BrowseJob, error) {
	custom.SourceURL = strings.TrimSpace(custom.SourceURL)
	parsed, err := url.ParseRequestURI(custom.SourceURL)
	if err != nil || validateFetchURL(parsed) != nil {
		return models.BrowseJob{}, ErrInvalidCustomJobURL
	}

	job, err := service.importer.Import(ctx, custom.SourceURL)
	if err != nil {
		return models.BrowseJob{}, err
	}

	if service.profiles != nil && service.scorer != nil {
		profile, err := service.profiles.UserProfile(ctx)
		if err != nil {
			return models.BrowseJob{}, fmt.Errorf("load profile for custom job score: %w", err)
		}

		if profile != nil {
			score, err := service.scorer.Score(ctx, job, *profile)
			if err != nil {
				return models.BrowseJob{}, fmt.Errorf("score custom job against profile: %w", err)
			}

			job.ProfileMatchScore = &score
		}
	}

	return service.repository.CreateCustomJob(ctx, job)
}
