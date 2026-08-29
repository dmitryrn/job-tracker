package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

type JobRepository interface {
	Upsert(context.Context, []models.Job) error
	StartProviderRun(context.Context, string, time.Duration, time.Time) (bool, error)
	CompleteProviderRun(context.Context, string, error, time.Time) error
	List(context.Context, models.JobSearch) ([]models.BrowseJob, error)
	Delete(context.Context, int64) (bool, error)
	Providers(context.Context) ([]string, error)
	Companies(context.Context, string) ([]models.BrowseCompany, error)
	UserProfile(context.Context) (*models.UserProfile, error)
	SaveUserProfile(context.Context, models.UserProfile) (models.UserProfile, error)
	JobsWithoutMatches(context.Context, int) ([]models.BrowseJob, error)
	JobMatchExists(context.Context, int64) (bool, error)
	CreateJobMatch(context.Context, int64, string) error
	JobMatch(context.Context, int64) (*models.JobMatchRecord, error)
}

var sqlBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)

type SQLite struct {
	db *sql.DB
}

func NewSQLite(db *sql.DB) JobRepository {
	return &SQLite{db: db}
}

func (repository *SQLite) Upsert(ctx context.Context, jobs []models.Job) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, job := range jobs {
		companyID, err := upsertCompany(ctx, transaction, job.Company, now)
		if err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO jobs (
				source, source_job_id, company_id, source_url, title, body_text, location,
				workplace, employment_type, salary_min, salary_max, posted_at,
				first_seen_at, last_seen_at, metadata_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(source, source_job_id) DO UPDATE SET
				company_id = excluded.company_id,
				source_url = excluded.source_url,
				title = excluded.title,
				body_text = excluded.body_text,
				location = excluded.location,
				workplace = excluded.workplace,
				employment_type = excluded.employment_type,
				salary_min = excluded.salary_min,
				salary_max = excluded.salary_max,
				posted_at = excluded.posted_at,
				last_seen_at = excluded.last_seen_at,
				metadata_json = excluded.metadata_json`,
			job.Source, job.SourceID, companyID, job.SourceURL, job.Title, job.BodyText,
			job.Location, job.Workplace, job.EmploymentType, job.SalaryMin, job.SalaryMax,
			job.PostedAt, now, now, job.MetadataJSON); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func (repository *SQLite) StartProviderRun(ctx context.Context, provider string, interval time.Duration, now time.Time) (bool, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer transaction.Rollback()

	var lastRun string
	err = transaction.QueryRowContext(ctx, `SELECT last_run_at FROM provider_runs WHERE provider = ?`, provider).Scan(&lastRun)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	if err == nil {
		lastRunAt, err := time.Parse(time.RFC3339Nano, lastRun)
		if err != nil {
			return false, fmt.Errorf("parse last run time for %s: %w", provider, err)
		}
		if now.Sub(lastRunAt) < interval {
			return false, transaction.Commit()
		}
	}

	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO provider_runs (provider, last_run_at, last_completed_at, status, last_error)
		VALUES (?, ?, NULL, 'running', NULL)
		ON CONFLICT(provider) DO UPDATE SET
			last_run_at = excluded.last_run_at,
			last_completed_at = NULL,
			status = excluded.status,
			last_error = NULL`, provider, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return false, err
	}
	if err := transaction.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (repository *SQLite) CompleteProviderRun(ctx context.Context, provider string, runError error, completedAt time.Time) error {
	status := "succeeded"
	var lastError any
	if runError != nil {
		status = "failed"
		lastError = runError.Error()
	}
	result, err := repository.db.ExecContext(ctx, `
		UPDATE provider_runs
		SET last_completed_at = ?, status = ?, last_error = ?
		WHERE provider = ?`, completedAt.UTC().Format(time.RFC3339Nano), status, lastError, provider)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return fmt.Errorf("provider run %s was not started", provider)
	}
	return nil
}

func (repository *SQLite) List(ctx context.Context, search models.JobSearch) ([]models.BrowseJob, error) {
	query := sqlBuilder.Select(
		"jobs.id", "jobs.source", "jobs.source_url", "jobs.title", "COALESCE(companies.name, '')",
		"COALESCE(jobs.location, '')", "jobs.workplace", "COALESCE(jobs.employment_type, '')",
		"jobs.salary_min", "jobs.salary_max", "COALESCE(jobs.posted_at, '')", "jobs.body_text",
	).From("jobs").LeftJoin("companies ON companies.id = jobs.company_id")
	if search.Search != "" && len(search.Fields) > 0 {
		matches := squirrel.Or{}
		for _, field := range search.Fields {
			matches = append(matches, squirrel.Expr(field+" LIKE ?", "%"+search.Search+"%"))
		}
		query = query.Where(matches)
	}
	if search.Provider != "" {
		query = query.Where(squirrel.Eq{"jobs.source": search.Provider})
	}
	statement, args, err := query.OrderBy("jobs.posted_at DESC", "jobs.id DESC").Limit(100).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build jobs query: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]models.BrowseJob, 0)
	for rows.Next() {
		var job models.BrowseJob
		if err := rows.Scan(&job.ID, &job.Source, &job.SourceURL, &job.Title, &job.Company,
			&job.Location, &job.Workplace, &job.EmploymentType, &job.SalaryMin, &job.SalaryMax,
			&job.PostedAt, &job.BodyText); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

func (repository *SQLite) Delete(ctx context.Context, id int64) (bool, error) {
	statement, args, err := sqlBuilder.Delete("jobs").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return false, fmt.Errorf("build delete job query: %w", err)
	}
	result, err := repository.db.ExecContext(ctx, statement, args...)
	if err != nil {
		return false, fmt.Errorf("delete job: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted jobs: %w", err)
	}
	return deleted > 0, nil
}

func (repository *SQLite) Providers(ctx context.Context) ([]string, error) {
	statement, args, err := sqlBuilder.Select("source").Distinct().From("jobs").Where(squirrel.NotEq{"source": ""}).OrderBy("source").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build providers query: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	providers := make([]string, 0)
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}
	return providers, nil
}

func (repository *SQLite) Companies(ctx context.Context, search string) ([]models.BrowseCompany, error) {
	query := sqlBuilder.Select("companies.id", "companies.name", "COUNT(jobs.id)", "companies.last_seen_at").
		From("companies").LeftJoin("jobs ON jobs.company_id = companies.id").
		GroupBy("companies.id").OrderBy("COUNT(jobs.id) DESC", "companies.name ASC").Limit(100)
	if search != "" {
		query = query.Where(squirrel.Expr("companies.name LIKE ?", "%"+search+"%"))
	}
	statement, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build companies query: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("query companies: %w", err)
	}
	defer rows.Close()

	companies := make([]models.BrowseCompany, 0)
	for rows.Next() {
		var company models.BrowseCompany
		if err := rows.Scan(&company.ID, &company.Name, &company.JobCount, &company.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan company: %w", err)
		}
		companies = append(companies, company)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate companies: %w", err)
	}
	return companies, nil
}

func (repository *SQLite) UserProfile(ctx context.Context) (*models.UserProfile, error) {
	var profile models.UserProfile
	var skills string
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, name, headline, location, work_authorization, summary, skills_json, updated_at
		FROM user_profiles WHERE id = 1`,
	).Scan(&profile.ID, &profile.Name, &profile.Headline, &profile.Location, &profile.WorkAuthorization, &profile.Summary, &skills, &profile.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	if err := json.Unmarshal([]byte(skills), &profile.Skills); err != nil {
		return nil, fmt.Errorf("decode user profile skills: %w", err)
	}
	return &profile, nil
}

func (repository *SQLite) SaveUserProfile(ctx context.Context, profile models.UserProfile) (models.UserProfile, error) {
	skills, err := json.Marshal(profile.Skills)
	if err != nil {
		return models.UserProfile{}, fmt.Errorf("encode user profile skills: %w", err)
	}
	profile.ID = 1
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err = repository.db.ExecContext(ctx, `
		INSERT INTO user_profiles (id, name, headline, location, work_authorization, summary, skills_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			headline = excluded.headline,
			location = excluded.location,
			work_authorization = excluded.work_authorization,
			summary = excluded.summary,
			skills_json = excluded.skills_json,
			updated_at = excluded.updated_at`,
		profile.ID, profile.Name, profile.Headline, profile.Location, profile.WorkAuthorization, profile.Summary, string(skills), profile.UpdatedAt,
	)
	if err != nil {
		return models.UserProfile{}, fmt.Errorf("save user profile: %w", err)
	}
	return profile, nil
}

func (repository *SQLite) JobsWithoutMatches(ctx context.Context, limit int) ([]models.BrowseJob, error) {
	if limit < 1 {
		return []models.BrowseJob{}, nil
	}
	rows, err := repository.db.QueryContext(ctx, `
		SELECT jobs.id, jobs.source, jobs.source_url, jobs.title, COALESCE(companies.name, ''),
			COALESCE(jobs.location, ''), jobs.workplace, COALESCE(jobs.employment_type, ''),
			jobs.salary_min, jobs.salary_max, COALESCE(jobs.posted_at, ''), jobs.body_text
		FROM jobs
		LEFT JOIN companies ON companies.id = jobs.company_id
		LEFT JOIN job_matches ON job_matches.job_id = jobs.id
		WHERE job_matches.job_id IS NULL
		ORDER BY jobs.posted_at DESC, jobs.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query jobs without matches: %w", err)
	}
	defer rows.Close()

	jobs := make([]models.BrowseJob, 0)
	for rows.Next() {
		var job models.BrowseJob
		if err := rows.Scan(&job.ID, &job.Source, &job.SourceURL, &job.Title, &job.Company,
			&job.Location, &job.Workplace, &job.EmploymentType, &job.SalaryMin, &job.SalaryMax,
			&job.PostedAt, &job.BodyText); err != nil {
			return nil, fmt.Errorf("scan job without match: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs without matches: %w", err)
	}
	return jobs, nil
}

func (repository *SQLite) JobMatchExists(ctx context.Context, jobID int64) (bool, error) {
	var exists bool
	if err := repository.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM job_matches WHERE job_id = ?)`, jobID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check job match: %w", err)
	}
	return exists, nil
}

func (repository *SQLite) CreateJobMatch(ctx context.Context, jobID int64, content string) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO job_matches (job_id, content, created_at) VALUES (?, ?, ?)`,
		jobID, content, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create job match: %w", err)
	}
	return nil
}

func (repository *SQLite) JobMatch(ctx context.Context, jobID int64) (*models.JobMatchRecord, error) {
	var match models.JobMatchRecord
	err := repository.db.QueryRowContext(ctx, `
		SELECT job_id, content, created_at FROM job_matches WHERE job_id = ?`, jobID,
	).Scan(&match.JobID, &match.Content, &match.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get job match: %w", err)
	}
	return &match, nil
}

func upsertCompany(ctx context.Context, transaction *sql.Tx, name, now string) (any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	normalized := strings.Join(strings.Fields(strings.ToLower(name)), " ")
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO companies (name, normalized_name, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(normalized_name) DO UPDATE SET name = excluded.name, last_seen_at = excluded.last_seen_at`,
		name, normalized, now, now); err != nil {
		return nil, err
	}
	var id int64
	if err := transaction.QueryRowContext(ctx, "SELECT id FROM companies WHERE normalized_name = ?", normalized).Scan(&id); err != nil {
		return nil, fmt.Errorf("get company ID: %w", err)
	}
	return id, nil
}
