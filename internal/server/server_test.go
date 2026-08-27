package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/config"
	"nice/internal/migrations"
	"nice/internal/models"
	"nice/internal/repositories"
	"nice/internal/services"
)

func TestJobAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "remotive", SourceID: "body", SourceURL: "https://example.com/body", Title: "Designer", BodyText: "Searchable description", Company: "Studio North", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "adzuna", SourceID: "title", SourceURL: "https://example.com/title", Title: "Searchable title", BodyText: "Other text", Company: "Other Co", Workplace: "remote", MetadataJSON: "{}"},
	}))

	handler := New(config.Config{}, zap.NewNop(), services.NewJobBrowse(repository)).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?search=searchable&fields=body")
	require.Equal(t, http.StatusOK, response.Code)
	var jobs struct {
		Jobs []models.BrowseJob `json:"jobs"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&jobs))
	require.Len(t, jobs.Jobs, 1)
	assert.Equal(t, "Designer", jobs.Jobs[0].Title)

	response = request(handler, http.MethodGet, "/api/providers")
	require.Equal(t, http.StatusOK, response.Code)
	var providers struct {
		Providers []string `json:"providers"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&providers))
	assert.Equal(t, []string{"adzuna", "remotive"}, providers.Providers)

	response = request(handler, http.MethodDelete, "/api/jobs/1")
	assert.Equal(t, http.StatusNoContent, response.Code)
	response = request(handler, http.MethodDelete, "/api/jobs/1")
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestJobAPIRejectsUnknownSearchField(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))
	handler := New(config.Config{}, zap.NewNop(), services.NewJobBrowse(repositories.NewSQLite(db))).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?fields=invalid")
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func request(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}
