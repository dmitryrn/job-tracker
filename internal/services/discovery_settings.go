package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

var ErrInvalidDiscoverySettings = errors.New("invalid discovery settings")

const maxLinkedInResults = 1000

type DiscoverySettingsService struct {
	repository repositories.DiscoverySettingsRepository
}

func NewDiscoverySettingsService(repository repositories.DiscoverySettingsRepository) *DiscoverySettingsService {
	return &DiscoverySettingsService{repository: repository}
}

func (service *DiscoverySettingsService) Settings(ctx context.Context) (models.DiscoverySettings, error) {
	return service.repository.DiscoverySettings(ctx)
}

func (service *DiscoverySettingsService) Save(ctx context.Context, settings models.DiscoverySettings) (models.DiscoverySettings, error) {
	settings.Adzuna.Query = strings.TrimSpace(settings.Adzuna.Query)
	settings.Adzuna.Country = strings.TrimSpace(settings.Adzuna.Country)
	settings.Adzuna.Workplace = strings.TrimSpace(settings.Adzuna.Workplace)
	settings.Remotive.Query = strings.TrimSpace(settings.Remotive.Query)
	settings.Remotive.Category = strings.TrimSpace(settings.Remotive.Category)
	settings.Jobicy.Geo = strings.TrimSpace(settings.Jobicy.Geo)
	settings.Jobicy.Industry = strings.TrimSpace(settings.Jobicy.Industry)
	settings.Jobicy.Tag = strings.TrimSpace(settings.Jobicy.Tag)
	settings.LinkedIn.Query = strings.TrimSpace(settings.LinkedIn.Query)
	settings.LinkedIn.Location = strings.TrimSpace(settings.LinkedIn.Location)
	settings.LinkedIn.PostedWithin = strings.TrimSpace(settings.LinkedIn.PostedWithin)
	settings.LinkedIn.Workplace = strings.TrimSpace(settings.LinkedIn.Workplace)
	settings.LinkedIn.ExperienceLevel = strings.TrimSpace(settings.LinkedIn.ExperienceLevel)

	if settings.Adzuna.Query == "" || settings.Adzuna.Country == "" || settings.Adzuna.MaxDaysOld < 1 || settings.Adzuna.MaxPages < 1 || settings.Adzuna.ResultsPerPage < 1 || !validWorkplace(settings.Adzuna.Workplace) || settings.Remotive.Query == "" || settings.Remotive.Category == "" || settings.Jobicy.Count < 1 || settings.Jobicy.Count > 200 || settings.LinkedIn.Query == "" || settings.LinkedIn.Location == "" || settings.LinkedIn.Limit < 1 || settings.LinkedIn.Limit > maxLinkedInResults || !validLinkedInPostedWithin(settings.LinkedIn.PostedWithin) || !validLinkedInWorkplace(settings.LinkedIn.Workplace) || !validLinkedInExperienceLevel(settings.LinkedIn.ExperienceLevel) {
		return models.DiscoverySettings{}, fmt.Errorf("%w: check required fields and numeric limits", ErrInvalidDiscoverySettings)
	}
	return service.repository.SaveDiscoverySettings(ctx, settings)
}

func validWorkplace(value string) bool {
	return value == "any" || value == "remote" || value == "remote-hybrid"
}

func validLinkedInPostedWithin(value string) bool {
	return value == "" || value == "r86400" || value == "r604800" || value == "r2592000"
}

func validLinkedInWorkplace(value string) bool {
	return value == "" || value == "1" || value == "2" || value == "3"
}

func validLinkedInExperienceLevel(value string) bool {
	return value == "" || value == "1" || value == "2" || value == "3" || value == "4" || value == "5" || value == "6"
}
