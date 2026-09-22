package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

const maxLinkedInResults = 1000

var ErrInvalidDiscoverySettings = errors.New("invalid discovery settings")

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
	settings = normalizeDiscoverySettings(settings)
	if !discoverySettingsValid(settings) {
		return models.DiscoverySettings{}, fmt.Errorf("%w: check required fields and numeric limits", ErrInvalidDiscoverySettings)
	}

	return service.repository.SaveDiscoverySettings(ctx, settings)
}

func normalizeDiscoverySettings(settings models.DiscoverySettings) models.DiscoverySettings {
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
	return settings
}

func discoverySettingsValid(settings models.DiscoverySettings) bool {
	return validAdzunaSettings(settings) && validRemotiveSettings(settings) && validJobicySettings(settings) && validLinkedInSettings(settings)
}

func validAdzunaSettings(settings models.DiscoverySettings) bool {
	value := settings.Adzuna
	return value.Query != "" && value.Country != "" && value.MaxDaysOld >= 1 && value.MaxPages >= 1 && value.ResultsPerPage >= 1 && validWorkplace(value.Workplace)
}

func validRemotiveSettings(settings models.DiscoverySettings) bool {
	return settings.Remotive.Query != "" && settings.Remotive.Category != ""
}

func validJobicySettings(settings models.DiscoverySettings) bool {
	return settings.Jobicy.Count >= 1 && settings.Jobicy.Count <= 200
}

func validLinkedInSettings(settings models.DiscoverySettings) bool {
	value := settings.LinkedIn
	return value.Query != "" && value.Location != "" && value.Limit >= 1 && value.Limit <= maxLinkedInResults && validLinkedInPostedWithin(value.PostedWithin) && validLinkedInWorkplace(value.Workplace) && validLinkedInExperienceLevel(value.ExperienceLevel)
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
