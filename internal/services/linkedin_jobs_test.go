package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/linkedin"
	"nice/internal/models"
)

func TestLinkedInJobsFetchPaginatesAndMapsResults(t *testing.T) {
	results := make([]linkedin.SearchResult, linkedInPageSize)
	details := make(map[string]linkedin.Job, linkedInPageSize+1)
	for index := range results {
		id := string(rune('a' + index))
		results[index] = linkedin.SearchResult{ID: id, URL: "https://www.linkedin.com/jobs/view/" + id, Title: "Engineer", Company: "Example Co", Location: "Berlin"}
		details[id] = linkedin.Job{Description: "Remote work"}
	}
	details["last"] = linkedin.Job{Description: "Hybrid work"}
	client := &linkedInClientStub{
		results: map[int][]linkedin.SearchResult{
			0:  results,
			25: {{ID: "last", URL: "https://www.linkedin.com/jobs/view/last", Title: "Staff Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02T10:00:00Z"}},
		},
		details: details,
	}
	service := LinkedInJobs{client: client}

	jobs, err := service.Fetch(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Location: "Berlin", Limit: linkedInPageSize + 1})

	require.NoError(t, err)
	require.Len(t, jobs, linkedInPageSize+1)
	assert.Equal(t, []int{0, 25}, client.starts)
	assert.Equal(t, "linkedin", jobs[0].Source)
	assert.Equal(t, "remote", jobs[0].Workplace)
	assert.Equal(t, "hybrid", jobs[linkedInPageSize].Workplace)
	assert.Equal(t, "2026-09-02T10:00:00Z", jobs[linkedInPageSize].PostedAt)
}

type linkedInClientStub struct {
	results map[int][]linkedin.SearchResult
	details map[string]linkedin.Job
	starts  []int
}

func (stub *linkedInClientStub) Search(_ context.Context, filter linkedin.SearchFilter) ([]linkedin.SearchResult, error) {
	stub.starts = append(stub.starts, filter.Start)
	return stub.results[filter.Start], nil
}

func (stub *linkedInClientStub) Job(_ context.Context, id string) (linkedin.Job, error) {
	return stub.details[id], nil
}
