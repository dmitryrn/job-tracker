package migrations

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyCreatesInitialSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"companies", "jobs", "provider_runs"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %s was not created", table)
		}
	}

	var ftsTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'jobs_fts%'`).Scan(&ftsTables); err != nil {
		t.Fatal(err)
	}
	if ftsTables != 0 {
		t.Errorf("unexpected FTS tables: %d", ftsTables)
	}
}
