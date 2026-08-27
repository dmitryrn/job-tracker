package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(db); err != nil {
		t.Fatal(err)
	}

	repository := repositories.NewSQLite(db)
	if err := repository.Upsert(context.Background(), []models.Job{
		{Source: "remotive", SourceID: "body", SourceURL: "https://example.com/body", Title: "Designer", BodyText: "Searchable description", Company: "Studio North", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "adzuna", SourceID: "title", SourceURL: "https://example.com/title", Title: "Searchable title", BodyText: "Other text", Company: "Other Co", Workplace: "remote", MetadataJSON: "{}"},
	}); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{}, zap.NewNop(), services.NewJobBrowse(repository)).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?search=searchable&fields=body")
	if response.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d", response.Code, http.StatusOK)
	}
	var jobs struct {
		Jobs []models.BrowseJob `json:"jobs"`
	}
	if err := json.NewDecoder(response.Body).Decode(&jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Jobs) != 1 || jobs.Jobs[0].Title != "Designer" {
		t.Fatalf("body search jobs = %#v, want Designer only", jobs.Jobs)
	}

	response = request(handler, http.MethodGet, "/api/providers")
	if response.Code != http.StatusOK {
		t.Fatalf("providers status = %d, want %d", response.Code, http.StatusOK)
	}
	var providers struct {
		Providers []string `json:"providers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&providers); err != nil {
		t.Fatal(err)
	}
	if len(providers.Providers) != 2 || providers.Providers[0] != "adzuna" || providers.Providers[1] != "remotive" {
		t.Fatalf("providers = %#v", providers.Providers)
	}

	response = request(handler, http.MethodDelete, "/api/jobs/1")
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", response.Code, http.StatusNoContent)
	}
	response = request(handler, http.MethodDelete, "/api/jobs/1")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing delete status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestJobAPIRejectsUnknownSearchField(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(db); err != nil {
		t.Fatal(err)
	}
	handler := New(config.Config{}, zap.NewNop(), services.NewJobBrowse(repositories.NewSQLite(db))).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?fields=invalid")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func request(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}
