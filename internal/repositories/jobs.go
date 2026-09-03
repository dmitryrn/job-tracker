package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

var sqlBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)

type SQLite struct {
	db *sql.DB
}

func (repository *SQLite) DiscoverySettings(ctx context.Context) (models.DiscoverySettings, error) {
	var settings models.DiscoverySettings
	err := repository.db.QueryRowContext(ctx, `
		SELECT adzuna_enabled, adzuna_query, adzuna_country, adzuna_max_days_old, adzuna_max_pages, adzuna_results_per_page, adzuna_workplace,
		       remotive_enabled, remotive_query, remotive_category, jobicy_enabled, jobicy_count, jobicy_geo, jobicy_industry, jobicy_tag,
		       linkedin_enabled, linkedin_query, linkedin_location, linkedin_limit
		FROM discovery_settings WHERE id = 1`).Scan(
		&settings.Adzuna.Enabled, &settings.Adzuna.Query, &settings.Adzuna.Country, &settings.Adzuna.MaxDaysOld, &settings.Adzuna.MaxPages, &settings.Adzuna.ResultsPerPage, &settings.Adzuna.Workplace,
		&settings.Remotive.Enabled, &settings.Remotive.Query, &settings.Remotive.Category, &settings.Jobicy.Enabled, &settings.Jobicy.Count, &settings.Jobicy.Geo, &settings.Jobicy.Industry, &settings.Jobicy.Tag,
		&settings.LinkedIn.Enabled, &settings.LinkedIn.Query, &settings.LinkedIn.Location, &settings.LinkedIn.Limit,
	)
	return settings, err
}

func (repository *SQLite) SaveDiscoverySettings(ctx context.Context, settings models.DiscoverySettings) (models.DiscoverySettings, error) {
	_, err := repository.db.ExecContext(ctx, `
		UPDATE discovery_settings SET
			adzuna_enabled = ?, adzuna_query = ?, adzuna_country = ?, adzuna_max_days_old = ?, adzuna_max_pages = ?, adzuna_results_per_page = ?, adzuna_workplace = ?,
			remotive_enabled = ?, remotive_query = ?, remotive_category = ?, jobicy_enabled = ?, jobicy_count = ?, jobicy_geo = ?, jobicy_industry = ?, jobicy_tag = ?,
			linkedin_enabled = ?, linkedin_query = ?, linkedin_location = ?, linkedin_limit = ?
		WHERE id = 1`,
		settings.Adzuna.Enabled, settings.Adzuna.Query, settings.Adzuna.Country, settings.Adzuna.MaxDaysOld, settings.Adzuna.MaxPages, settings.Adzuna.ResultsPerPage, settings.Adzuna.Workplace,
		settings.Remotive.Enabled, settings.Remotive.Query, settings.Remotive.Category, settings.Jobicy.Enabled, settings.Jobicy.Count, settings.Jobicy.Geo, settings.Jobicy.Industry, settings.Jobicy.Tag,
		settings.LinkedIn.Enabled, settings.LinkedIn.Query, settings.LinkedIn.Location, settings.LinkedIn.Limit,
	)
	return settings, err
}

func NewSQLite(db *sql.DB) *SQLite {
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
		INSERT INTO provider_runs (provider, last_run_at)
		VALUES (?, ?)
		ON CONFLICT(provider) DO UPDATE SET last_run_at = excluded.last_run_at`, provider, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return false, err
	}
	if err := transaction.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (repository *SQLite) List(ctx context.Context, search models.JobSearch) ([]models.BrowseJob, error) {
	query := sqlBuilder.Select(
		"jobs.id", "jobs.source", "jobs.source_url", "jobs.title", "COALESCE(companies.name, '')",
		"COALESCE(jobs.location, '')", "jobs.workplace", "COALESCE(jobs.employment_type, '')",
		"jobs.salary_min", "jobs.salary_max", "COALESCE(jobs.posted_at, '')", "jobs.body_text",
		"EXISTS (SELECT 1 FROM job_matches WHERE job_matches.job_id = jobs.id)",
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
			&job.PostedAt, &job.BodyText, &job.HasMatch); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

func (repository *SQLite) Job(ctx context.Context, id int64) (*models.BrowseJob, error) {
	return repository.job(ctx, repository.db, id)
}

func (repository *SQLite) AnalysisJob(ctx context.Context, jobID int64) (*models.Job, error) {
	var job models.Job
	err := repository.db.QueryRowContext(ctx, `
		SELECT jobs.source, jobs.source_job_id, jobs.source_url, jobs.title, jobs.body_text,
			COALESCE(companies.name, ''), COALESCE(jobs.location, ''), jobs.workplace,
			COALESCE(jobs.employment_type, ''), jobs.salary_min, jobs.salary_max,
			COALESCE(jobs.posted_at, ''), jobs.metadata_json
		FROM jobs LEFT JOIN companies ON companies.id = jobs.company_id WHERE jobs.id = ?`, jobID,
	).Scan(&job.Source, &job.SourceID, &job.SourceURL, &job.Title, &job.BodyText,
		&job.Company, &job.Location, &job.Workplace, &job.EmploymentType, &job.SalaryMin,
		&job.SalaryMax, &job.PostedAt, &job.MetadataJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get job for analysis: %w", err)
	}
	return &job, nil
}

func (repository *SQLite) Delete(ctx context.Context, id int64) (bool, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin delete job transaction: %w", err)
	}
	defer transaction.Rollback()

	queue, err := readMatchQueue(ctx, transaction)
	if err != nil {
		return false, err
	}
	if updatedQueue, removed := removeAllMatchRequests(queue, id); removed {
		queue = updatedQueue
		if err := writeMatchQueue(ctx, transaction, queue); err != nil {
			return false, err
		}
	}

	result, err := transaction.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete job: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted jobs: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return false, fmt.Errorf("commit delete job: %w", err)
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
	var skills, workHistory, education string
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, headline, location, work_authorization, summary, skills_json, work_history_json, education_json, updated_at
		FROM user_profiles WHERE id = 1`,
	).Scan(&profile.ID, &profile.Headline, &profile.Location, &profile.WorkAuthorization, &profile.Summary, &skills, &workHistory, &education, &profile.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	if err := json.Unmarshal([]byte(skills), &profile.Skills); err != nil {
		return nil, fmt.Errorf("decode user profile skills: %w", err)
	}
	if err := json.Unmarshal([]byte(workHistory), &profile.WorkHistory); err != nil {
		return nil, fmt.Errorf("decode user profile work history: %w", err)
	}
	if err := json.Unmarshal([]byte(education), &profile.Education); err != nil {
		return nil, fmt.Errorf("decode user profile education: %w", err)
	}
	return &profile, nil
}

func (repository *SQLite) SaveUserProfile(ctx context.Context, profile models.UserProfile) (models.UserProfile, error) {
	skills, err := json.Marshal(profile.Skills)
	if err != nil {
		return models.UserProfile{}, fmt.Errorf("encode user profile skills: %w", err)
	}
	workHistory, err := json.Marshal(profile.WorkHistory)
	if err != nil {
		return models.UserProfile{}, fmt.Errorf("encode user profile work history: %w", err)
	}
	education, err := json.Marshal(profile.Education)
	if err != nil {
		return models.UserProfile{}, fmt.Errorf("encode user profile education: %w", err)
	}
	profile.ID = 1
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err = repository.db.ExecContext(ctx, `
		INSERT INTO user_profiles (id, headline, location, work_authorization, summary, skills_json, work_history_json, education_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			headline = excluded.headline,
			location = excluded.location,
			work_authorization = excluded.work_authorization,
			summary = excluded.summary,
			skills_json = excluded.skills_json,
			work_history_json = excluded.work_history_json,
			education_json = excluded.education_json,
			updated_at = excluded.updated_at`,
		profile.ID, profile.Headline, profile.Location, profile.WorkAuthorization, profile.Summary, string(skills), string(workHistory), string(education), profile.UpdatedAt,
	)
	if err != nil {
		return models.UserProfile{}, fmt.Errorf("save user profile: %w", err)
	}
	return profile, nil
}

func (repository *SQLite) Resume(ctx context.Context) (*models.Resume, error) {
	var resume models.Resume
	err := repository.db.QueryRowContext(ctx, `
		SELECT id, full_name, headline, location, email, phone, summary, updated_at
		FROM resumes WHERE base_resume = 1`,
	).Scan(&resume.ID, &resume.FullName, &resume.Headline, &resume.Location, &resume.Email, &resume.Phone, &resume.Summary, &resume.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get base resume: %w", err)
	}

	if resume.Links, err = readResumeLinks(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Skills, err = readResumeSkills(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Competencies, err = readResumeCompetencies(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Experience, err = readResumeExperience(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	if resume.Education, err = readResumeEducation(ctx, repository.db, resume.ID); err != nil {
		return nil, err
	}
	return &resume, nil
}

func (repository *SQLite) SaveResume(ctx context.Context, resume models.Resume) (models.Resume, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Resume{}, fmt.Errorf("begin save base resume transaction: %w", err)
	}
	defer transaction.Rollback()

	resume.ID = 1
	resume.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO resumes (id, title, base_resume, full_name, headline, location, email, phone, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			base_resume = excluded.base_resume,
			full_name = excluded.full_name,
			headline = excluded.headline,
			location = excluded.location,
			email = excluded.email,
			phone = excluded.phone,
			summary = excluded.summary,
			updated_at = excluded.updated_at`,
		resume.ID, "Base resume", true, resume.FullName, resume.Headline, resume.Location, resume.Email, resume.Phone, resume.Summary, resume.UpdatedAt, resume.UpdatedAt,
	); err != nil {
		return models.Resume{}, fmt.Errorf("save base resume: %w", err)
	}
	if err := deleteResumeSections(ctx, transaction, resume.ID); err != nil {
		return models.Resume{}, err
	}
	if err := insertResumeSections(ctx, transaction, resume); err != nil {
		return models.Resume{}, err
	}
	if err := transaction.Commit(); err != nil {
		return models.Resume{}, fmt.Errorf("commit base resume: %w", err)
	}
	return resume, nil
}

func deleteResumeSections(ctx context.Context, transaction *sql.Tx, resumeID int64) error {
	statements := []string{
		`DELETE FROM resume_competency_bullets WHERE competency_id IN (SELECT id FROM resume_competencies WHERE resume_id = ?)`,
		`DELETE FROM resume_experience_bullets WHERE experience_id IN (SELECT id FROM resume_experience WHERE resume_id = ?)`,
		`DELETE FROM resume_competencies WHERE resume_id = ?`,
		`DELETE FROM resume_experience WHERE resume_id = ?`,
		`DELETE FROM resume_links WHERE resume_id = ?`,
		`DELETE FROM resume_skills WHERE resume_id = ?`,
		`DELETE FROM resume_education WHERE resume_id = ?`,
	}
	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement, resumeID); err != nil {
			return fmt.Errorf("clear base resume sections: %w", err)
		}
	}
	return nil
}

func insertResumeSections(ctx context.Context, transaction *sql.Tx, resume models.Resume) error {
	for index, link := range resume.Links {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_links (resume_id, label, url, sort_order) VALUES (?, ?, ?, ?)`, resume.ID, link.Label, link.URL, index); err != nil {
			return fmt.Errorf("save base resume link: %w", err)
		}
	}
	for index, skill := range resume.Skills {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_skills (resume_id, name, level, sort_order) VALUES (?, ?, ?, ?)`, resume.ID, skill.Name, skill.Level, index); err != nil {
			return fmt.Errorf("save base resume skill: %w", err)
		}
	}
	for index, competency := range resume.Competencies {
		result, err := transaction.ExecContext(ctx, `INSERT INTO resume_competencies (resume_id, title, sort_order) VALUES (?, ?, ?)`, resume.ID, competency.Title, index)
		if err != nil {
			return fmt.Errorf("save base resume competency: %w", err)
		}
		competencyID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get base resume competency ID: %w", err)
		}
		for bulletIndex, bullet := range competency.Bullets {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_competency_bullets (competency_id, content, sort_order) VALUES (?, ?, ?)`, competencyID, bullet, bulletIndex); err != nil {
				return fmt.Errorf("save base resume competency bullet: %w", err)
			}
		}
	}
	for index, experience := range resume.Experience {
		result, err := transaction.ExecContext(ctx, `
			INSERT INTO resume_experience (resume_id, company, title, location, start_date, end_date, is_current, stack, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			resume.ID, experience.Company, experience.Title, experience.Location, experience.StartDate, experience.EndDate, experience.IsCurrent, experience.Stack, index,
		)
		if err != nil {
			return fmt.Errorf("save base resume experience: %w", err)
		}
		experienceID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get base resume experience ID: %w", err)
		}
		for bulletIndex, bullet := range experience.Bullets {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO resume_experience_bullets (experience_id, content, sort_order) VALUES (?, ?, ?)`, experienceID, bullet, bulletIndex); err != nil {
				return fmt.Errorf("save base resume experience bullet: %w", err)
			}
		}
	}
	for index, education := range resume.Education {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO resume_education (resume_id, institution, location, degree, field_of_study, start_date, end_date, details, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			resume.ID, education.Institution, education.Location, education.Degree, education.FieldOfStudy, education.StartDate, education.EndDate, education.Details, index,
		); err != nil {
			return fmt.Errorf("save base resume education: %w", err)
		}
	}
	return nil
}

func readResumeLinks(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeLink, error) {
	rows, err := database.QueryContext(ctx, `SELECT label, url FROM resume_links WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume links: %w", err)
	}
	defer rows.Close()
	links := make([]models.ResumeLink, 0)
	for rows.Next() {
		var link models.ResumeLink
		if err := rows.Scan(&link.Label, &link.URL); err != nil {
			return nil, fmt.Errorf("scan base resume link: %w", err)
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func readResumeSkills(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeSkill, error) {
	rows, err := database.QueryContext(ctx, `SELECT name, level FROM resume_skills WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume skills: %w", err)
	}
	defer rows.Close()
	skills := make([]models.ResumeSkill, 0)
	for rows.Next() {
		var skill models.ResumeSkill
		if err := rows.Scan(&skill.Name, &skill.Level); err != nil {
			return nil, fmt.Errorf("scan base resume skill: %w", err)
		}
		skills = append(skills, skill)
	}
	return skills, rows.Err()
}

func readResumeCompetencies(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeCompetency, error) {
	rows, err := database.QueryContext(ctx, `SELECT id, title FROM resume_competencies WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume competencies: %w", err)
	}
	type competencyRow struct {
		id    int64
		title string
	}
	stored := make([]competencyRow, 0)
	for rows.Next() {
		var row competencyRow
		if err := rows.Scan(&row.id, &row.title); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan base resume competency: %w", err)
		}
		stored = append(stored, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate base resume competencies: %w", err)
	}
	rows.Close()

	competencies := make([]models.ResumeCompetency, 0, len(stored))
	for _, row := range stored {
		bulletRows, err := database.QueryContext(ctx, `SELECT content FROM resume_competency_bullets WHERE competency_id = ? ORDER BY sort_order, id`, row.id)
		if err != nil {
			return nil, fmt.Errorf("get base resume competency bullets: %w", err)
		}
		competency := models.ResumeCompetency{Title: row.title, Bullets: make([]string, 0)}
		for bulletRows.Next() {
			var bullet string
			if err := bulletRows.Scan(&bullet); err != nil {
				bulletRows.Close()
				return nil, fmt.Errorf("scan base resume competency bullet: %w", err)
			}
			competency.Bullets = append(competency.Bullets, bullet)
		}
		if err := bulletRows.Err(); err != nil {
			bulletRows.Close()
			return nil, fmt.Errorf("iterate base resume competency bullets: %w", err)
		}
		bulletRows.Close()
		competencies = append(competencies, competency)
	}
	return competencies, nil
}

func readResumeExperience(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeExperience, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, company, title, location, start_date, end_date, is_current, stack
		FROM resume_experience WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume experience: %w", err)
	}
	type experienceRow struct {
		id    int64
		entry models.ResumeExperience
	}
	stored := make([]experienceRow, 0)
	for rows.Next() {
		var row experienceRow
		if err := rows.Scan(&row.id, &row.entry.Company, &row.entry.Title, &row.entry.Location, &row.entry.StartDate, &row.entry.EndDate, &row.entry.IsCurrent, &row.entry.Stack); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan base resume experience: %w", err)
		}
		stored = append(stored, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate base resume experience: %w", err)
	}
	rows.Close()

	experience := make([]models.ResumeExperience, 0, len(stored))
	for _, row := range stored {
		bulletRows, err := database.QueryContext(ctx, `SELECT content FROM resume_experience_bullets WHERE experience_id = ? ORDER BY sort_order, id`, row.id)
		if err != nil {
			return nil, fmt.Errorf("get base resume experience bullets: %w", err)
		}
		entry := row.entry
		entry.Bullets = make([]string, 0)
		for bulletRows.Next() {
			var bullet string
			if err := bulletRows.Scan(&bullet); err != nil {
				bulletRows.Close()
				return nil, fmt.Errorf("scan base resume experience bullet: %w", err)
			}
			entry.Bullets = append(entry.Bullets, bullet)
		}
		if err := bulletRows.Err(); err != nil {
			bulletRows.Close()
			return nil, fmt.Errorf("iterate base resume experience bullets: %w", err)
		}
		bulletRows.Close()
		experience = append(experience, entry)
	}
	return experience, nil
}

func readResumeEducation(ctx context.Context, database *sql.DB, resumeID int64) ([]models.ResumeEducation, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT institution, location, degree, field_of_study, start_date, end_date, details
		FROM resume_education WHERE resume_id = ? ORDER BY sort_order, id`, resumeID)
	if err != nil {
		return nil, fmt.Errorf("get base resume education: %w", err)
	}
	defer rows.Close()
	education := make([]models.ResumeEducation, 0)
	for rows.Next() {
		var entry models.ResumeEducation
		if err := rows.Scan(&entry.Institution, &entry.Location, &entry.Degree, &entry.FieldOfStudy, &entry.StartDate, &entry.EndDate, &entry.Details); err != nil {
			return nil, fmt.Errorf("scan base resume education: %w", err)
		}
		education = append(education, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate base resume education: %w", err)
	}
	return education, nil
}

func (repository *SQLite) JobMatchExists(ctx context.Context, jobID int64) (bool, error) {
	var exists bool
	if err := repository.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM job_matches WHERE job_id = ?)`, jobID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check job match: %w", err)
	}
	return exists, nil
}

func (repository *SQLite) JobAnalysis(ctx context.Context, jobID int64) (*models.JobAnalysisRecord, error) {
	var analysis models.JobAnalysisRecord
	var raw string
	err := repository.db.QueryRowContext(ctx, `
		SELECT job_id, analyzer_version, prompt_version, input_sha256, model, analyzed_at,
			normalized_description, analysis_json
		FROM job_analyses WHERE job_id = ?`, jobID,
	).Scan(&analysis.JobID, &analysis.AnalyzerVersion, &analysis.PromptVersion, &analysis.InputSHA256,
		&analysis.Model, &analysis.AnalyzedAt, &analysis.NormalizedDescription, &raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get job analysis: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), &analysis.Analysis); err != nil {
		return nil, fmt.Errorf("decode job analysis: %w", err)
	}
	return &analysis, nil
}

func (repository *SQLite) SaveJobAnalysis(ctx context.Context, analysis models.JobAnalysisRecord) error {
	raw, err := json.Marshal(analysis.Analysis)
	if err != nil {
		return fmt.Errorf("encode job analysis: %w", err)
	}
	_, err = repository.db.ExecContext(ctx, `
		INSERT INTO job_analyses (
			job_id, analyzer_version, prompt_version, input_sha256, model, analyzed_at,
			normalized_description, analysis_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			analyzer_version = excluded.analyzer_version,
			prompt_version = excluded.prompt_version,
			input_sha256 = excluded.input_sha256,
			model = excluded.model,
			analyzed_at = excluded.analyzed_at,
			normalized_description = excluded.normalized_description,
			analysis_json = excluded.analysis_json`,
		analysis.JobID, analysis.AnalyzerVersion, analysis.PromptVersion, analysis.InputSHA256,
		analysis.Model, analysis.AnalyzedAt, analysis.NormalizedDescription, string(raw),
	)
	if err != nil {
		return fmt.Errorf("save job analysis: %w", err)
	}
	return nil
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
	var assessment models.JobMatchAssessment
	if err := json.Unmarshal([]byte(match.Content), &assessment); err == nil && assessment.MatcherVersion != "" {
		match.Assessment = &assessment
	}
	return &match, nil
}

func (repository *SQLite) JobMatches(ctx context.Context) ([]models.JobMatchSummary, error) {
	rows, err := repository.db.QueryContext(ctx, `
		SELECT jobs.id, jobs.source, jobs.source_url, jobs.title, COALESCE(companies.name, ''),
			COALESCE(jobs.location, ''), jobs.workplace, COALESCE(jobs.employment_type, ''),
			jobs.salary_min, jobs.salary_max, COALESCE(jobs.posted_at, ''), jobs.body_text, job_matches.created_at, job_matches.content
		FROM job_matches
		JOIN jobs ON jobs.id = job_matches.job_id
		LEFT JOIN companies ON companies.id = jobs.company_id
		ORDER BY job_matches.created_at DESC, job_matches.job_id DESC
		LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("query job matches: %w", err)
	}
	defer rows.Close()

	matches := make([]models.JobMatchSummary, 0)
	for rows.Next() {
		var match models.JobMatchSummary
		var content string
		if err := rows.Scan(&match.Job.ID, &match.Job.Source, &match.Job.SourceURL, &match.Job.Title, &match.Job.Company,
			&match.Job.Location, &match.Job.Workplace, &match.Job.EmploymentType, &match.Job.SalaryMin, &match.Job.SalaryMax,
			&match.Job.PostedAt, &match.Job.BodyText, &match.CreatedAt, &content); err != nil {
			return nil, fmt.Errorf("scan job match: %w", err)
		}
		var assessment models.JobMatchAssessment
		if err := json.Unmarshal([]byte(content), &assessment); err == nil && assessment.MatcherVersion != "" {
			match.Score = assessment.Score
			match.Label = assessment.Label
		}
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job matches: %w", err)
	}
	return matches, nil
}

func (repository *SQLite) MatchQueue(ctx context.Context) ([]models.BrowseJob, error) {
	queue, err := readMatchQueue(ctx, repository.db)
	if err != nil {
		return nil, err
	}
	jobs := make([]models.BrowseJob, 0, len(queue))
	for _, jobID := range queue {
		job, err := repository.job(ctx, repository.db, jobID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func (repository *SQLite) QueueJobMatch(ctx context.Context, jobID int64, redo bool) (bool, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin queue match request: %w", err)
	}
	defer transaction.Rollback()

	if _, err := repository.job(ctx, transaction, jobID); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if redo {
		if _, err := transaction.ExecContext(ctx, `DELETE FROM job_matches WHERE job_id = ?`, jobID); err != nil {
			return false, fmt.Errorf("delete job match for redo: %w", err)
		}
	}
	queue, err := readMatchQueue(ctx, transaction)
	if err != nil {
		return false, err
	}
	queue = append([]int64{jobID}, queue...)
	if err := writeMatchQueue(ctx, transaction, queue); err != nil {
		return false, err
	}
	if err := transaction.Commit(); err != nil {
		return false, fmt.Errorf("commit queue match request: %w", err)
	}
	return true, nil
}

func (repository *SQLite) QueueJobsWithoutMatches(ctx context.Context, jobIDs []int64) (int, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin queue unmatched job matches: %w", err)
	}
	defer transaction.Rollback()

	queue, err := readMatchQueue(ctx, transaction)
	if err != nil {
		return 0, err
	}
	alreadyQueued := make(map[int64]struct{}, len(queue))
	for _, jobID := range queue {
		alreadyQueued[jobID] = struct{}{}
	}

	requested := make([]int64, 0, len(jobIDs))
	seen := make(map[int64]struct{}, len(jobIDs))
	for _, jobID := range jobIDs {
		if _, duplicate := seen[jobID]; duplicate {
			continue
		}
		seen[jobID] = struct{}{}
		if _, queued := alreadyQueued[jobID]; queued {
			continue
		}

		var exists, hasMatch bool
		if err := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = ?)`, jobID).Scan(&exists); err != nil {
			return 0, fmt.Errorf("check job for match queue: %w", err)
		}
		if !exists {
			continue
		}
		if err := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM job_matches WHERE job_id = ?)`, jobID).Scan(&hasMatch); err != nil {
			return 0, fmt.Errorf("check job match for match queue: %w", err)
		}
		if hasMatch {
			continue
		}
		requested = append(requested, jobID)
	}

	if len(requested) > 0 {
		queue = append(requested, queue...)
		if err := writeMatchQueue(ctx, transaction, queue); err != nil {
			return 0, err
		}
	}
	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("commit queue unmatched job matches: %w", err)
	}
	return len(requested), nil
}

func (repository *SQLite) ReplaceMatchQueue(ctx context.Context, jobIDs []int64) (bool, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin replace match queue: %w", err)
	}
	defer transaction.Rollback()
	for _, jobID := range jobIDs {
		if _, err := repository.job(ctx, transaction, jobID); errors.Is(err, sql.ErrNoRows) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	if err := writeMatchQueue(ctx, transaction, jobIDs); err != nil {
		return false, err
	}
	if err := transaction.Commit(); err != nil {
		return false, fmt.Errorf("commit replace match queue: %w", err)
	}
	return true, nil
}

func (repository *SQLite) RemoveMatchRequest(ctx context.Context, jobID int64) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove match request: %w", err)
	}
	defer transaction.Rollback()
	queue, err := readMatchQueue(ctx, transaction)
	if err != nil {
		return err
	}
	if updatedQueue, removed := removeFirstMatchRequest(queue, jobID); removed {
		queue = updatedQueue
		if err := writeMatchQueue(ctx, transaction, queue); err != nil {
			return err
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit remove match request: %w", err)
	}
	return nil
}

func (repository *SQLite) CompleteMatchRequest(ctx context.Context, jobID int64, content string) error {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete match request: %w", err)
	}
	defer transaction.Rollback()
	queue, err := readMatchQueue(ctx, transaction)
	if err != nil {
		return err
	}
	updatedQueue, removed := removeFirstMatchRequest(queue, jobID)
	if !removed {
		return transaction.Commit()
	}
	queue = updatedQueue
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO job_matches (job_id, content, created_at) VALUES (?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET content = excluded.content, created_at = excluded.created_at`,
		jobID, content, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("save completed job match: %w", err)
	}
	if err := writeMatchQueue(ctx, transaction, queue); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit completed match request: %w", err)
	}
	return nil
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type matchQueueQuerier interface {
	queryRower
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (repository *SQLite) job(ctx context.Context, query queryRower, jobID int64) (*models.BrowseJob, error) {
	var job models.BrowseJob
	err := query.QueryRowContext(ctx, `
		SELECT jobs.id, jobs.source, jobs.source_url, jobs.title, COALESCE(companies.name, ''),
			COALESCE(jobs.location, ''), jobs.workplace, COALESCE(jobs.employment_type, ''),
			jobs.salary_min, jobs.salary_max, COALESCE(jobs.posted_at, ''), jobs.body_text
		FROM jobs LEFT JOIN companies ON companies.id = jobs.company_id WHERE jobs.id = ?`, jobID,
	).Scan(&job.ID, &job.Source, &job.SourceURL, &job.Title, &job.Company, &job.Location,
		&job.Workplace, &job.EmploymentType, &job.SalaryMin, &job.SalaryMax, &job.PostedAt, &job.BodyText)
	if err != nil {
		return nil, fmt.Errorf("get queued job: %w", err)
	}
	return &job, nil
}

func readMatchQueue(ctx context.Context, query queryRower) ([]int64, error) {
	var raw string
	if err := query.QueryRowContext(ctx, `SELECT job_ids_json FROM job_match_queue WHERE id = 1`).Scan(&raw); err != nil {
		return nil, fmt.Errorf("read job match queue: %w", err)
	}
	queue := make([]int64, 0)
	if err := json.Unmarshal([]byte(raw), &queue); err != nil {
		return nil, fmt.Errorf("decode job match queue: %w", err)
	}
	return queue, nil
}

func writeMatchQueue(ctx context.Context, query matchQueueQuerier, queue []int64) error {
	encoded, err := json.Marshal(queue)
	if err != nil {
		return fmt.Errorf("encode job match queue: %w", err)
	}
	if _, err := query.ExecContext(ctx, `UPDATE job_match_queue SET job_ids_json = ? WHERE id = 1`, string(encoded)); err != nil {
		return fmt.Errorf("write job match queue: %w", err)
	}
	return nil
}

func removeFirstMatchRequest(queue []int64, jobID int64) ([]int64, bool) {
	for index, queuedJobID := range queue {
		if queuedJobID == jobID {
			copy(queue[index:], queue[index+1:])
			return queue[:len(queue)-1], true
		}
	}
	return queue, false
}

func removeAllMatchRequests(queue []int64, jobID int64) ([]int64, bool) {
	filtered := queue[:0]
	removed := false
	for _, queuedJobID := range queue {
		if queuedJobID == jobID {
			removed = true
			continue
		}
		filtered = append(filtered, queuedJobID)
	}
	return filtered, removed
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
