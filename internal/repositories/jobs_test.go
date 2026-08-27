package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"nice/internal/migrations"
)

func TestProviderRunHonorsIntervalAndRecordsFailure(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(db); err != nil {
		t.Fatal(err)
	}

	repository := NewSQLite(db)
	startedAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	run, err := repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !run {
		t.Fatal("first provider run was not started")
	}

	fetchErr := errors.New("upstream unavailable")
	if err := repository.CompleteProviderRun(context.Background(), "adzuna", fetchErr, startedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	run, err = repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt.Add(59*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if run {
		t.Fatal("provider run started before its interval elapsed")
	}

	var status, lastError string
	if err := db.QueryRow(`SELECT status, last_error FROM provider_runs WHERE provider = 'adzuna'`).Scan(&status, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || lastError != fetchErr.Error() {
		t.Errorf("status/error = %q/%q, want failed/%q", status, lastError, fetchErr)
	}

	run, err = repository.StartProviderRun(context.Background(), "adzuna", time.Hour, startedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !run {
		t.Fatal("provider run did not start after its interval elapsed")
	}

	if err := db.QueryRow(`SELECT status, last_completed_at, last_error FROM provider_runs WHERE provider = 'adzuna'`).Scan(&status, new(sql.NullString), new(sql.NullString)); err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}
