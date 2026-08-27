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

	for _, table := range []string{"companies", "jobs", "provider_runs"} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count))
		assert.Equal(t, 1, count, "table %s was not created", table)
	}

	var ftsTables int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'jobs_fts%'`).Scan(&ftsTables))
	assert.Zero(t, ftsTables)
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
