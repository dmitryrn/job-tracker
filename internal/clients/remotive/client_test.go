package remotive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/models"
)

func TestFetchMapsFullDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "software engineer", request.URL.Query().Get("search"))
		assert.Equal(t, "software-development", request.URL.Query().Get("category"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jobs":[{"id":42,"url":"https://remotive.com/remote-jobs/software-engineer-42","title":"Software Engineer","company_name":"Example Co","category":"Software Development","tags":["Go"],"job_type":"full_time","publication_date":"2026-08-27T10:00:00","candidate_required_location":"Europe","salary":"€80,000","description":"<p>Build useful things.</p>","company_logo_url":"https://remotive.com/logo.png"}]}`))
	}))
	defer server.Close()

	jobs, err := newClient(server.Client(), server.URL).Fetch(context.Background(), models.RemotiveSearchSettings{Query: "software engineer", Category: "software-development"})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	job := jobs[0]
	assert.Equal(t, "remotive", job.Source)
	assert.Equal(t, "42", job.SourceID)
	assert.Equal(t, "<p>Build useful things.</p>", job.BodyText)
	assert.Equal(t, "remote", job.Workplace)
	assert.Equal(t, "Europe", job.Location)
	assert.Equal(t, "2026-08-27T10:00:00Z", job.PostedAt)
}

func TestFetchFiltersOtherCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jobs":[{"id":1,"category":"Software Development"},{"id":2,"category":"Marketing"}]}`))
	}))
	defer server.Close()

	jobs, err := newClient(server.Client(), server.URL).Fetch(context.Background(), models.RemotiveSearchSettings{Query: "engineer", Category: "software-development"})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, "1", jobs[0].SourceID)
}
