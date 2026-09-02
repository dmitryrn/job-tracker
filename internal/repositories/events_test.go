package repositories

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
	"nice/internal/models"
)

func TestEventsFiltersAndPaginates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	require.NoError(t, repository.RecordEvent(context.Background(), models.Event{Provider: "linkedin", RunID: "run-1", Type: "linkedin.search.started", Level: "info", Message: "Search started", Data: map[string]any{"query": "engineer"}}))
	require.NoError(t, repository.RecordEvent(context.Background(), models.Event{Provider: "application", Type: "application.started", Level: "info", Message: "Application started"}))
	require.NoError(t, repository.RecordEvent(context.Background(), models.Event{Provider: "linkedin", RunID: "run-2", Type: "linkedin.search.succeeded", Level: "info", Message: "Search succeeded"}))

	page, err := repository.Events(context.Background(), models.EventSearch{Provider: "linkedin", Limit: 1, Offset: 0})

	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	require.Len(t, page.Events, 1)
	assert.Equal(t, "run-2", page.Events[0].RunID)
	assert.NotEmpty(t, page.Events[0].OccurredAt)

	page, err = repository.Events(context.Background(), models.EventSearch{RunID: "run-1", Limit: 50, Offset: 0})

	require.NoError(t, err)
	require.Len(t, page.Events, 1)
	assert.Equal(t, "engineer", page.Events[0].Data["query"])
}
