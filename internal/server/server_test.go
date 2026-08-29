package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	handler := newTestServer(repository).http.Handler

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
	handler := newTestServer(repositories.NewSQLite(db)).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?fields=invalid")
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestProfileAndJobMatchAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "remotive", SourceID: "job", SourceURL: "https://example.com/job", Title: "Engineer", BodyText: "Build services.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	handler := newTestServer(repository).http.Handler

	response := request(handler, http.MethodGet, "/api/profile")
	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"profile":null}`, response.Body.String())

	response = requestWithBody(handler, http.MethodPut, "/api/profile", `{"name":"  Riley  ","headline":"Backend engineer","location":"Berlin","workAuthorization":"EU","summary":"APIs and systems","skills":[{"name":" Go ","level":"expert","notes":"Production services"},{"name":"","level":"","notes":""}]}`)
	require.Equal(t, http.StatusOK, response.Code)
	var profileResponse struct {
		Profile models.UserProfile `json:"profile"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&profileResponse))
	assert.Equal(t, "Riley", profileResponse.Profile.Name)
	require.Len(t, profileResponse.Profile.Skills, 1)
	assert.Equal(t, "Go", profileResponse.Profile.Skills[0].Name)

	require.NoError(t, repository.CreateJobMatch(context.Background(), 1, "No-op match"))
	response = request(handler, http.MethodGet, "/api/jobs/1/match")
	require.Equal(t, http.StatusOK, response.Code)
	var matchResponse struct {
		Match *models.JobMatchRecord `json:"match"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&matchResponse))
	require.NotNil(t, matchResponse.Match)
	assert.Equal(t, int64(1), matchResponse.Match.JobID)
	assert.Equal(t, "No-op match", matchResponse.Match.Content)
	assert.NotEmpty(t, matchResponse.Match.CreatedAt)
}

func newTestServer(repository repositories.JobRepository) *Server {
	return New(
		config.Config{},
		zap.NewNop(),
		services.NewJobBrowse(repository),
		services.NewUserProfileService(repository),
		services.NewJobMatches(repository),
	)
}

func request(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}

func requestWithBody(handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, strings.NewReader(body)))
	return response
}
