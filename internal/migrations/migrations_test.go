package migrations

import (
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestApplyCreatesInitialSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, Apply(db))
	require.NoError(t, Apply(db))

	for _, table := range []string{
		"companies", "jobs", "provider_runs", "user_profiles", "job_matches", "job_match_queue", "job_analyses",
		"resumes", "resume_links", "resume_skills",
		"resume_experience", "resume_experience_bullets", "resume_education", "resume_summary_paragraphs", "job_match_chat_items",
		"applications",
	} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count))
		assert.Equal(t, 1, count, "table %s was not created", table)
	}

	var levelColumns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('resume_skills') WHERE name = 'level'`).Scan(&levelColumns))
	assert.Zero(t, levelColumns)

	var itemColumns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('job_match_chat_items') WHERE name = 'sequence'`).Scan(&itemColumns))
	assert.Equal(t, 1, itemColumns)

	var profileLocationColumns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('user_profiles') WHERE name = 'location'`).Scan(&profileLocationColumns))
	assert.Zero(t, profileLocationColumns)

	var profileLinkColumns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('user_profiles') WHERE name IN ('github_url', 'linkedin_url')`).Scan(&profileLinkColumns))
	assert.Equal(t, 2, profileLinkColumns)

	var resumeLocationColumns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('resumes') WHERE name = 'location'`).Scan(&resumeLocationColumns))
	assert.Zero(t, resumeLocationColumns)

	var resumeLocationParts int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('resumes') WHERE name IN ('town', 'country')`).Scan(&resumeLocationParts))
	assert.Equal(t, 2, resumeLocationParts)

	var ftsTables int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'jobs_fts%'`).Scan(&ftsTables))
	assert.Zero(t, ftsTables)
}

func TestRemoveResumeCompetenciesMigrationDropsTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	goose.SetBaseFS(files)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.UpTo(db, "sql", 28))

	for _, table := range []string{"resume_competencies", "resume_competency_bullets"} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count))
		assert.Equal(t, 1, count)
	}

	require.NoError(t, goose.Up(db, "sql"))
	for _, table := range []string{"resume_competencies", "resume_competency_bullets"} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count))
		assert.Zero(t, count)
	}
}

func TestResumeLocationMigrationSplitsTownAndCountry(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	goose.SetBaseFS(files)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.UpTo(db, "sql", 23))
	_, err = db.Exec(`
		INSERT INTO resumes (id, title, base_resume, full_name, headline, location, email, phone, created_at, updated_at)
		VALUES
			(1, 'Base resume', 1, '', '', 'Berlin, Germany', '', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
			(2, 'Other resume', 0, '', '', 'London', '', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)

	require.NoError(t, goose.Up(db, "sql"))
	var town, country string
	require.NoError(t, db.QueryRow(`SELECT town, country FROM resumes WHERE id = 1`).Scan(&town, &country))
	assert.Equal(t, "Berlin", town)
	assert.Equal(t, "Germany", country)
	require.NoError(t, db.QueryRow(`SELECT town, country FROM resumes WHERE id = 2`).Scan(&town, &country))
	assert.Equal(t, "London", town)
	assert.Empty(t, country)
}

func TestSummaryParagraphMigrationPreservesExistingParagraphs(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	goose.SetBaseFS(files)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.UpTo(db, "sql", 16))
	_, err = db.Exec(`
		INSERT INTO resumes (id, title, base_resume, full_name, headline, location, email, phone, summary, created_at, updated_at)
		VALUES (1, 'Base resume', 1, 'Ada Lovelace', '', '', '', '', 'First paragraph.' || CHAR(13) || CHAR(10) || CHAR(13) || CHAR(10) || 'Second paragraph.', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)

	require.NoError(t, goose.Up(db, "sql"))
	rows, err := db.Query(`SELECT content FROM resume_summary_paragraphs WHERE resume_id = 1 ORDER BY sort_order`)
	require.NoError(t, err)
	defer rows.Close()
	paragraphs := make([]string, 0)
	for rows.Next() {
		var paragraph string
		require.NoError(t, rows.Scan(&paragraph))
		paragraphs = append(paragraphs, paragraph)
	}

	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"First paragraph.", "Second paragraph."}, paragraphs)

	var summaryColumns int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('resumes') WHERE name = 'summary'`).Scan(&summaryColumns))
	assert.Zero(t, summaryColumns)
}

func TestMigrationsCanBeAppliedAndRolledBackRepeatedly(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	goose.SetBaseFS(files)
	require.NoError(t, goose.SetDialect("sqlite3"))

	for range 2 {
		require.NoError(t, goose.Up(db, "sql"))
		require.NoError(t, goose.DownTo(db, "sql", 0))
	}
}

func TestEventLevelMigrationRemovesConstraintAndPreservesExistingEvents(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	goose.SetBaseFS(files)
	require.NoError(t, goose.SetDialect("sqlite3"))
	require.NoError(t, goose.UpTo(db, "sql", 26))
	_, err = db.Exec(`INSERT INTO app_events (occurred_at, type, level, message) VALUES ('2026-09-05T13:00:00Z', 'existing', 'info', 'Existing event')`)
	require.NoError(t, err)

	require.NoError(t, goose.Up(db, "sql"))
	_, err = db.Exec(`INSERT INTO app_events (occurred_at, type, level, message) VALUES ('2026-09-05T13:01:00Z', 'debug', 'debug', 'Debug event')`)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM app_events`).Scan(&count))
	assert.Equal(t, 2, count)
}
