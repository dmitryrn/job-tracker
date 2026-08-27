package remotive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchMapsFullDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("search"); got != "software engineer" {
			t.Errorf("search query = %q, want software engineer", got)
		}
		if got := request.URL.Query().Get("category"); got != "software-development" {
			t.Errorf("category query = %q, want software-development", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jobs":[{"id":42,"url":"https://remotive.com/remote-jobs/software-engineer-42","title":"Software Engineer","company_name":"Example Co","category":"Software Development","tags":["Go"],"job_type":"full_time","publication_date":"2026-08-27T10:00:00","candidate_required_location":"Europe","salary":"€80,000","description":"<p>Build useful things.</p>","company_logo_url":"https://remotive.com/logo.png"}]}`))
	}))
	defer server.Close()

	jobs, err := newClient(server.Client(), server.URL, "software engineer").Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("job count = %d, want 1", len(jobs))
	}
	job := jobs[0]
	if job.Source != "remotive" || job.SourceID != "42" {
		t.Errorf("source = %q/%q, want remotive/42", job.Source, job.SourceID)
	}
	if job.BodyText != "<p>Build useful things.</p>" {
		t.Errorf("body text = %q", job.BodyText)
	}
	if job.Workplace != "remote" || job.Location != "Europe" {
		t.Errorf("workplace/location = %q/%q", job.Workplace, job.Location)
	}
	if job.PostedAt != "2026-08-27T10:00:00Z" {
		t.Errorf("posted at = %q", job.PostedAt)
	}
}

func TestFetchFiltersOtherCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jobs":[{"id":1,"category":"Software Development"},{"id":2,"category":"Marketing"}]}`))
	}))
	defer server.Close()

	jobs, err := newClient(server.Client(), server.URL, "engineer").Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].SourceID != "1" {
		t.Errorf("jobs = %#v, want only Software Development job", jobs)
	}
}
