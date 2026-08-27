package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"nice/internal/config"
	"nice/internal/models"
)

type JobRepository interface {
	Upsert(context.Context, []models.Job) error
	Close() error
}

type SQLite struct {
	db *sql.DB
}

func NewSQLite(cfg config.Config) (JobRepository, error) {
	db, err := sql.Open("sqlite", cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := initialize(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLite{db: db}, nil
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

func (repository *SQLite) Close() error {
	return repository.db.Close()
}

func initialize(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS companies (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL UNIQUE,
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY,
			source TEXT NOT NULL,
			source_job_id TEXT NOT NULL,
			company_id INTEGER REFERENCES companies(id),
			source_url TEXT NOT NULL,
			title TEXT NOT NULL,
			body_text TEXT NOT NULL,
			location TEXT,
			workplace TEXT NOT NULL,
			employment_type TEXT,
			salary_min INTEGER,
			salary_max INTEGER,
			posted_at TEXT,
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			metadata_json TEXT NOT NULL,
			UNIQUE(source, source_job_id)
		);
		CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(
			title, body_text, content='jobs', content_rowid='id'
		);
		CREATE TRIGGER IF NOT EXISTS jobs_ai AFTER INSERT ON jobs BEGIN
			INSERT INTO jobs_fts(rowid, title, body_text) VALUES (new.id, new.title, new.body_text);
		END;
		CREATE TRIGGER IF NOT EXISTS jobs_ad AFTER DELETE ON jobs BEGIN
			INSERT INTO jobs_fts(jobs_fts, rowid, title, body_text) VALUES ('delete', old.id, old.title, old.body_text);
		END;
		CREATE TRIGGER IF NOT EXISTS jobs_au AFTER UPDATE OF title, body_text ON jobs BEGIN
			INSERT INTO jobs_fts(jobs_fts, rowid, title, body_text) VALUES ('delete', old.id, old.title, old.body_text);
			INSERT INTO jobs_fts(rowid, title, body_text) VALUES (new.id, new.title, new.body_text);
		END;
	`)
	return err
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
