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
		"adzuna_enabled",
		"adzuna_query",
		"adzuna_country",
		"adzuna_max_days_old",
		"adzuna_max_pages",
		"adzuna_results_per_page",
		"adzuna_workplace",
		"remotive_enabled",
		"remotive_query",
		"remotive_category",
		"jobicy_enabled",
		"jobicy_count",
		"jobicy_geo",
		"jobicy_industry",
		"jobicy_tag",
	).From("discovery_settings").Where(squirrel.Eq{"id": 1}).ToSql()
	if err != nil {
		return models.DiscoverySettings{}, err
	}

	var settings models.DiscoverySettings
	err = repository.db.QueryRowContext(ctx, statement, args...).Scan(
		&settings.Adzuna.Enabled,
		&settings.Adzuna.Query,
		&settings.Adzuna.Country,
		&settings.Adzuna.MaxDaysOld,
		&settings.Adzuna.MaxPages,
		&settings.Adzuna.ResultsPerPage,
		&settings.Adzuna.Workplace,
		&settings.Remotive.Enabled,
		&settings.Remotive.Query,
		&settings.Remotive.Category,
		&settings.Jobicy.Enabled,
		&settings.Jobicy.Count,
		&settings.Jobicy.Geo,
		&settings.Jobicy.Industry,
		&settings.Jobicy.Tag,
	)
	if err != nil {
		return settings, err
	}

	settings.LinkedIn, err = repository.linkedinSearches(ctx)
	return settings, err
}

func (repository *SQLite) SaveDiscoverySettings(ctx context.Context, settings models.DiscoverySettings) (models.DiscoverySettings, error) {
	statement, args, err := squirrel.Update("discovery_settings").SetMap(map[string]any{
		"adzuna_enabled":          settings.Adzuna.Enabled,
		"adzuna_query":            settings.Adzuna.Query,
		"adzuna_country":          settings.Adzuna.Country,
		"adzuna_max_days_old":     settings.Adzuna.MaxDaysOld,
		"adzuna_max_pages":        settings.Adzuna.MaxPages,
		"adzuna_results_per_page": settings.Adzuna.ResultsPerPage,
		"adzuna_workplace":        settings.Adzuna.Workplace,
		"remotive_enabled":        settings.Remotive.Enabled,
		"remotive_query":          settings.Remotive.Query,
		"remotive_category":       settings.Remotive.Category,
		"jobicy_enabled":          settings.Jobicy.Enabled,
		"jobicy_count":            settings.Jobicy.Count,
		"jobicy_geo":              settings.Jobicy.Geo,
		"jobicy_industry":         settings.Jobicy.Industry,
		"jobicy_tag":              settings.Jobicy.Tag,
	}).Where(squirrel.Eq{"id": 1}).ToSql()
	if err != nil {
		return settings, err
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return settings, err
	}

	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, statement, args...); err != nil {
		return settings, err
	}

	deleteStatement, deleteArgs, err := squirrel.Delete("linkedin_searches").ToSql()
	if err != nil {
		return settings, err
	}

	if _, err = tx.ExecContext(ctx, deleteStatement, deleteArgs...); err != nil {
		return settings, err
	}

	for _, search := range settings.LinkedIn {
		insertStatement, insertArgs, buildErr := squirrel.Insert("linkedin_searches").Columns(
			"id",
			"name",
			"enabled",
			"sort_order",
			"query",
			"location",
			"posted_within",
			"workplace",
			"experience_level",
			"result_limit",
		).Values(
			search.ID,
			search.Name,
			search.Enabled,
			search.SortOrder,
			search.Query,
			search.Location,
			search.PostedWithin,
			search.Workplace,
			search.ExperienceLevel,
			search.Limit,
		).ToSql()
		if buildErr != nil {
			return settings, buildErr
		}

		if _, err = tx.ExecContext(ctx, insertStatement, insertArgs...); err != nil {
			return settings, err
		}
	}

	if err = tx.Commit(); err != nil {
		return settings, err
	}

	return settings, nil
}

func (repository *SQLite) linkedinSearches(ctx context.Context) ([]models.LinkedInSearchSettings, error) {
	statement, args, err := squirrel.Select(
		"id",
		"name",
		"enabled",
		"sort_order",
		"query",
		"location",
		"posted_within",
		"workplace",
		"experience_level",
		"result_limit",
	).From("linkedin_searches").OrderBy("sort_order ASC").ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	searches := make([]models.LinkedInSearchSettings, 0)
	for rows.Next() {
		var search models.LinkedInSearchSettings
		if err := rows.Scan(
			&search.ID,
			&search.Name,
			&search.Enabled,
			&search.SortOrder,
			&search.Query,
			&search.Location,
			&search.PostedWithin,
			&search.Workplace,
			&search.ExperienceLevel,
			&search.Limit,
		); err != nil {
			return nil, err
		}

		searches = append(searches, search)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return searches, nil
}
