package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

type DiscoverySettingsRepository interface {
	DiscoverySettings(context.Context) (models.DiscoverySettings, error)
	SaveDiscoverySettings(context.Context, models.DiscoverySettings) (models.DiscoverySettings, error)
}

func NewDiscoverySettingsRepository(sqlite *SQLite) DiscoverySettingsRepository {
	return sqlite
}

func (repository *SQLite) DiscoverySettings(ctx context.Context) (models.DiscoverySettings, error) {
	statement, args, err := squirrel.Select(
		"adzuna_enabled", "adzuna_query", "adzuna_country", "adzuna_max_days_old", "adzuna_max_pages", "adzuna_results_per_page", "adzuna_workplace",
		"remotive_enabled", "remotive_query", "remotive_category", "jobicy_enabled", "jobicy_count", "jobicy_geo", "jobicy_industry", "jobicy_tag",
		"linkedin_enabled", "linkedin_query", "linkedin_location", "linkedin_posted_within", "linkedin_workplace", "linkedin_experience_level", "linkedin_limit",
	).From("discovery_settings").Where(squirrel.Eq{"id": 1}).ToSql()
	if err != nil {
		return models.DiscoverySettings{}, err
	}

	var settings models.DiscoverySettings
	err = repository.db.QueryRowContext(ctx, statement, args...).Scan(
		&settings.Adzuna.Enabled, &settings.Adzuna.Query, &settings.Adzuna.Country, &settings.Adzuna.MaxDaysOld, &settings.Adzuna.MaxPages, &settings.Adzuna.ResultsPerPage, &settings.Adzuna.Workplace,
		&settings.Remotive.Enabled, &settings.Remotive.Query, &settings.Remotive.Category, &settings.Jobicy.Enabled, &settings.Jobicy.Count, &settings.Jobicy.Geo, &settings.Jobicy.Industry, &settings.Jobicy.Tag,
		&settings.LinkedIn.Enabled, &settings.LinkedIn.Query, &settings.LinkedIn.Location, &settings.LinkedIn.PostedWithin, &settings.LinkedIn.Workplace, &settings.LinkedIn.ExperienceLevel, &settings.LinkedIn.Limit,
	)
	return settings, err
}

func (repository *SQLite) SaveDiscoverySettings(ctx context.Context, settings models.DiscoverySettings) (models.DiscoverySettings, error) {
	statement, args, err := squirrel.Update("discovery_settings").SetMap(map[string]any{
		"adzuna_enabled":            settings.Adzuna.Enabled,
		"adzuna_query":              settings.Adzuna.Query,
		"adzuna_country":            settings.Adzuna.Country,
		"adzuna_max_days_old":       settings.Adzuna.MaxDaysOld,
		"adzuna_max_pages":          settings.Adzuna.MaxPages,
		"adzuna_results_per_page":   settings.Adzuna.ResultsPerPage,
		"adzuna_workplace":          settings.Adzuna.Workplace,
		"remotive_enabled":          settings.Remotive.Enabled,
		"remotive_query":            settings.Remotive.Query,
		"remotive_category":         settings.Remotive.Category,
		"jobicy_enabled":            settings.Jobicy.Enabled,
		"jobicy_count":              settings.Jobicy.Count,
		"jobicy_geo":                settings.Jobicy.Geo,
		"jobicy_industry":           settings.Jobicy.Industry,
		"jobicy_tag":                settings.Jobicy.Tag,
		"linkedin_enabled":          settings.LinkedIn.Enabled,
		"linkedin_query":            settings.LinkedIn.Query,
		"linkedin_location":         settings.LinkedIn.Location,
		"linkedin_posted_within":    settings.LinkedIn.PostedWithin,
		"linkedin_workplace":        settings.LinkedIn.Workplace,
		"linkedin_experience_level": settings.LinkedIn.ExperienceLevel,
		"linkedin_limit":            settings.LinkedIn.Limit,
	}).Where(squirrel.Eq{"id": 1}).ToSql()
	if err != nil {
		return settings, err
	}
	_, err = repository.db.ExecContext(ctx, statement, args...)
	return settings, err
}
