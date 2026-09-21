package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

func (repository *SQLite) CreateApplication(ctx context.Context, jobID int64) (*models.Application, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create application transaction: %w", err)
	}

	defer func() { _ = transaction.Rollback() }()

	jobStatement, jobArguments, err := sqlBuilder.Select("id").From("jobs").Where(squirrel.Eq{"id": jobID}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build application job lookup: %w", err)
	}

	var existingJobID int64
	if err := transaction.QueryRowContext(ctx, jobStatement, jobArguments...).Scan(&existingJobID); err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	} else if err != nil {
		return nil, fmt.Errorf("lookup application job: %w", err)
	}

	appliedAt := time.Now().UTC().Format(time.RFC3339Nano)
	statement, arguments, err := sqlBuilder.Insert("applications").
		Columns("job_id", "applied_at").
		Values(jobID, appliedAt).
		Suffix("ON CONFLICT(job_id) DO NOTHING").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build create application query: %w", err)
	}

	if _, err := transaction.ExecContext(ctx, statement, arguments...); err != nil {
		return nil, fmt.Errorf("create application: %w", err)
	}

	application, err := application(ctx, transaction, jobID)
	if err != nil {
		return nil, fmt.Errorf("load created application: %w", err)
	}

	if err := transaction.Commit(); err != nil {
		return nil, fmt.Errorf("commit create application: %w", err)
	}

	return application, nil
}

func (repository *SQLite) DeleteApplication(ctx context.Context, jobID int64) (bool, error) {
	statement, arguments, err := sqlBuilder.Delete("applications").Where(squirrel.Eq{"job_id": jobID}).ToSql()
	if err != nil {
		return false, fmt.Errorf("build delete application query: %w", err)
	}

	result, err := repository.db.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return false, fmt.Errorf("delete application: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted applications: %w", err)
	}

	return deleted > 0, nil
}

func (repository *SQLite) Application(ctx context.Context, jobID int64) (*models.Application, error) {
	return application(ctx, repository.db, jobID)
}

func application(ctx context.Context, query queryRower, jobID int64) (*models.Application, error) {
	statement, arguments, err := sqlBuilder.Select("job_id", "applied_at").From("applications").Where(squirrel.Eq{"job_id": jobID}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build application query: %w", err)
	}

	var result models.Application
	if err := query.QueryRowContext(ctx, statement, arguments...).Scan(&result.JobID, &result.AppliedAt); err == sql.ErrNoRows {
		return nil, nil //nolint:nilnil // nil represents an application that has not been created.
	} else if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}

	return &result, nil
}

func (repository *SQLite) Applications(ctx context.Context, search models.ApplicationSearch) (models.ApplicationPage, error) {
	columns := []string{
		"jobs.id",
		"jobs.source",
		"jobs.source_url",
		"jobs.title",
		"COALESCE(companies.name, '')",
		"COALESCE(jobs.location, '')",
		"jobs.workplace",
		"jobs.workplace_classification_json",
		"COALESCE(jobs.employment_type, '')",
		"jobs.salary_min",
		"jobs.salary_max",
		"COALESCE(jobs.posted_at, '')",
		"jobs.body_text",
		"COALESCE(jobs.last_viewed_at, '')",
		"EXISTS (SELECT 1 FROM job_matches WHERE job_matches.job_id = jobs.id)",
		"jobs.profile_match_score",
		"applications.applied_at",
	}
	query := sqlBuilder.Select(columns...).From("applications").Join("jobs ON jobs.id = applications.job_id").LeftJoin("companies ON companies.id = jobs.company_id")
	statement, arguments, err := query.OrderBy("applications.applied_at DESC", "applications.job_id DESC").Limit(paginationValue(search.Limit)).Offset(paginationValue(search.Offset)).ToSql()
	if err != nil {
		return models.ApplicationPage{}, fmt.Errorf("build applications query: %w", err)
	}

	rows, err := repository.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return models.ApplicationPage{}, fmt.Errorf("query applications: %w", err)
	}
	defer rows.Close()

	page := models.ApplicationPage{Applications: make([]models.ApplicationSummary, 0)}
	for rows.Next() {
		var item models.ApplicationSummary
		if err := rows.Scan(
			&item.Job.ID,
			&item.Job.Source,
			&item.Job.SourceURL,
			&item.Job.Title,
			&item.Job.Company,
			&item.Job.Location,
			&item.Job.Workplace,
			&item.Job.WorkplaceClassificationJSON,
			&item.Job.EmploymentType,
			&item.Job.SalaryMin,
			&item.Job.SalaryMax,
			&item.Job.PostedAt,
			&item.Job.BodyText,
			&item.Job.LastViewedAt,
			&item.Job.HasMatch,
			&item.Job.ProfileMatchScore,
			&item.AppliedAt,
		); err != nil {
			return models.ApplicationPage{}, fmt.Errorf("scan application: %w", err)
		}

		page.Applications = append(page.Applications, item)
	}

	if err := rows.Err(); err != nil {
		return models.ApplicationPage{}, fmt.Errorf("iterate applications: %w", err)
	}

	countStatement, countArguments, err := sqlBuilder.Select("COUNT(*)").From("applications").ToSql()
	if err != nil {
		return models.ApplicationPage{}, fmt.Errorf("build applications count query: %w", err)
	}

	if err := repository.db.QueryRowContext(ctx, countStatement, countArguments...).Scan(&page.Total); err != nil {
		return models.ApplicationPage{}, fmt.Errorf("count applications: %w", err)
	}

	return page, nil
}
