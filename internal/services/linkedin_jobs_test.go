package services

import (
	"context"
	"errors"
	"testing"
	"time"

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
		results[index] = linkedin.SearchResult{ID: id, URL: "https://www.linkedin.com/jobs/view/" + id, Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"}
		details[id] = linkedin.Job{Description: "Remote work", EmploymentType: "Full-time"}
	}
	details["last"] = linkedin.Job{Description: "Hybrid work", EmploymentType: "Full-time"}
	client := &linkedInClientStub{
		results: map[int][]linkedin.SearchResult{
			0:  results,
			25: {{ID: "last", URL: "https://www.linkedin.com/jobs/view/last", Title: "Staff Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"}},
		},
		details: details,
	}
	service := LinkedInJobs{client: client, jobs: &linkedInJobRepositoryStub{}}

	fetch, err := service.Preview(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Location: "Berlin", Limit: linkedInPageSize + 1}, 0)

	require.NoError(t, err)
	jobs := fetch.Jobs
	require.Len(t, jobs, linkedInPageSize+1)
	assert.Equal(t, []int{0, 25}, client.starts)
	assert.Equal(t, "linkedin", jobs[0].Source)
	assert.Equal(t, "remote", jobs[0].Workplace)
	assert.Equal(t, "hybrid", jobs[linkedInPageSize].Workplace)
	assert.Equal(t, "2026-09-02T00:00:00Z", jobs[linkedInPageSize].PostedAt)
	assert.Equal(t, "Full-time", jobs[0].EmploymentType)
}

func TestLinkedInJobsFetchSkipsIncompleteJobs(t *testing.T) {
	client := &linkedInClientStub{
		results: map[int][]linkedin.SearchResult{
			0: {
				{URL: "https://www.linkedin.com/jobs/view/missing-id", Title: "Engineer", Company: "Example Co", Location: "Berlin"},
				{ID: "missing-title", URL: "https://www.linkedin.com/jobs/view/missing-title", Company: "Example Co", Location: "Berlin"},
				{ID: "missing-description", URL: "https://www.linkedin.com/jobs/view/missing-description", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
				{ID: "missing-employment", URL: "https://www.linkedin.com/jobs/view/missing-employment", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
				{ID: "missing-posted-at", URL: "https://www.linkedin.com/jobs/view/missing-posted-at", Title: "Engineer", Company: "Example Co", Location: "Berlin"},
				{ID: "invalid-posted-at", URL: "https://www.linkedin.com/jobs/view/invalid-posted-at", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "not-a-date"},
				{ID: "accepted", URL: "https://www.linkedin.com/jobs/view/accepted", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
			},
		},
		details: map[string]linkedin.Job{
			"missing-description": {EmploymentType: "Full-time"},
			"missing-employment":  {Description: "Build systems"},
			"invalid-posted-at":   {Description: "Build systems", EmploymentType: "Full-time"},
			"accepted":            {Description: "Build systems", EmploymentType: "Full-time"},
		},
	}
	service := LinkedInJobs{client: client, jobs: &linkedInJobRepositoryStub{}}

	fetch, err := service.Preview(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Limit: 10}, 0)

	require.NoError(t, err)
	jobs := fetch.Jobs
	require.Len(t, jobs, 1)
	assert.Equal(t, "accepted", jobs[0].SourceID)
	assert.Equal(t, []string{"missing-description", "missing-employment", "invalid-posted-at", "accepted"}, client.jobIDs)
}

func TestLinkedInPostedAt(t *testing.T) {
	assert.Equal(t, "2026-09-02T00:00:00Z", linkedInPostedAt("2026-09-02"))
	assert.Equal(t, "2026-09-02T10:00:00Z", linkedInPostedAt("2026-09-02T10:00:00Z"))
	assert.Empty(t, linkedInPostedAt("not-a-date"))
}

func TestLinkedInJobsFetchReturnsCompletedJobsWhenLaterRequestFails(t *testing.T) {
	fetchErr := errors.New("LinkedIn unavailable")
	client := &linkedInClientStub{
		results: map[int][]linkedin.SearchResult{
			0: {
				{ID: "completed", URL: "https://www.linkedin.com/jobs/view/completed", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
				{ID: "failed", URL: "https://www.linkedin.com/jobs/view/failed", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
			},
		},
		details: map[string]linkedin.Job{
			"completed": {Description: "Build systems", EmploymentType: "Full-time"},
		},
		jobErrors: map[string]error{"failed": fetchErr},
	}
	service := LinkedInJobs{client: client, jobs: &linkedInJobRepositoryStub{}}

	fetch, err := service.Preview(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Limit: 10}, 0)

	require.ErrorIs(t, err, fetchErr)
	jobs := fetch.Jobs
	require.Len(t, jobs, 1)
	assert.Equal(t, "completed", jobs[0].SourceID)
}

func TestLinkedInJobsFetchWaitsBetweenRequests(t *testing.T) {
	requestInterval := 10 * time.Millisecond
	client := &linkedInClientStub{
		results: map[int][]linkedin.SearchResult{
			0: {{ID: "accepted", URL: "https://www.linkedin.com/jobs/view/accepted", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"}},
		},
		details: map[string]linkedin.Job{"accepted": {Description: "Build systems", EmploymentType: "Full-time"}},
	}
	service := LinkedInJobs{client: client, jobs: &linkedInJobRepositoryStub{}}

	_, err := service.Preview(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Limit: 1}, requestInterval)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, client.jobAt.Sub(client.searchAt), requestInterval)
}

func TestLinkedInJobsSyncRecordsRequestEventsWithoutInterruptingFetch(t *testing.T) {
	events := &linkedInEventRecorder{err: errors.New("event storage unavailable")}
	service := LinkedInJobs{
		client: &linkedInClientStub{
			results: map[int][]linkedin.SearchResult{
				0: {{ID: "accepted", URL: "https://www.linkedin.com/jobs/view/accepted", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"}},
			},
			details: map[string]linkedin.Job{"accepted": {Description: "Build systems", EmploymentType: "Full-time"}},
		},
		events: events,
		jobs:   &linkedInJobRepositoryStub{},
	}

	fetch, err := service.Sync(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Limit: 1}, "run-1", 0)

	require.NoError(t, err)
	require.Len(t, fetch.Jobs, 1)
	assert.Equal(t, 1, fetch.SavedJobs)
	assert.Equal(t, []string{"linkedin.search.started", "linkedin.search.succeeded", "linkedin.job_fetch.started", "linkedin.job_fetch.succeeded", "linkedin.job_save.succeeded"}, events.types())
	assert.Equal(t, "engineer", events.events[0].Data["query"])
}

func TestLinkedInJobsPreviewDoesNotRecordEvents(t *testing.T) {
	events := &linkedInEventRecorder{}
	jobs := &linkedInJobRepositoryStub{existing: map[string]bool{"accepted": true}}
	service := LinkedInJobs{
		client: &linkedInClientStub{
			results: map[int][]linkedin.SearchResult{
				0: {{ID: "accepted", URL: "https://www.linkedin.com/jobs/view/accepted", Title: "Engineer", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"}},
			},
			details: map[string]linkedin.Job{"accepted": {Description: "Build systems", EmploymentType: "Full-time"}},
		},
		events: events,
		jobs:   jobs,
	}

	_, err := service.Preview(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Limit: 1}, 0)

	require.NoError(t, err)
	assert.Empty(t, events.events)
	assert.Equal(t, []string{"accepted"}, jobs.checked)
	assert.Empty(t, service.client.(*linkedInClientStub).jobIDs)
}

func TestLinkedInJobsSyncSkipsExistingJobs(t *testing.T) {
	client := &linkedInClientStub{
		results: map[int][]linkedin.SearchResult{
			0: {
				{ID: "first", URL: "https://www.linkedin.com/jobs/view/first", Title: "First", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
				{ID: "existing", URL: "https://www.linkedin.com/jobs/view/existing", Title: "Existing", Company: "Example Co", Location: "Berlin", PostedAt: "2026-09-02"},
			},
		},
		details: map[string]linkedin.Job{"first": {Description: "First description", EmploymentType: "Full-time"}},
	}
	jobs := &linkedInJobRepositoryStub{existing: map[string]bool{"existing": true}}
	service := LinkedInJobs{client: client, jobs: jobs}

	fetch, err := service.Sync(context.Background(), models.LinkedInSearchSettings{Query: "engineer", Limit: 2}, "run-1", 0)

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "existing"}, jobs.checked)
	assert.Equal(t, []string{"first"}, client.jobIDs)
	require.Len(t, jobs.upserted, 1)
	assert.Equal(t, "first", jobs.upserted[0].SourceID)
	assert.Equal(t, 1, fetch.FetchedJobs)
	assert.Equal(t, 1, fetch.SavedJobs)
}

type linkedInClientStub struct {
	results   map[int][]linkedin.SearchResult
	details   map[string]linkedin.Job
	jobErrors map[string]error
	jobIDs    []string
	starts    []int
	searchAt  time.Time
	jobAt     time.Time
}

func (stub *linkedInClientStub) Search(_ context.Context, filter linkedin.SearchFilter) ([]linkedin.SearchResult, error) {
	stub.searchAt = time.Now()
	stub.starts = append(stub.starts, filter.Start)
	return stub.results[filter.Start], nil
}

func (stub *linkedInClientStub) Job(_ context.Context, id string) (linkedin.Job, error) {
	stub.jobAt = time.Now()
	stub.jobIDs = append(stub.jobIDs, id)
	return stub.details[id], stub.jobErrors[id]
}

type linkedInEventRecorder struct {
	events []models.Event
	err    error
}

type linkedInJobRepositoryStub struct {
	existing map[string]bool
	checked  []string
	upserted []models.Job
}

func (stub *linkedInJobRepositoryStub) JobExists(_ context.Context, source, sourceID string) (bool, error) {
	if source != "linkedin" {
		return false, errors.New("unexpected source")
	}
	stub.checked = append(stub.checked, sourceID)
	return stub.existing[sourceID], nil
}

func (stub *linkedInJobRepositoryStub) Upsert(_ context.Context, jobs []models.Job) error {
	stub.upserted = append(stub.upserted, jobs...)
	return nil
}

func (recorder *linkedInEventRecorder) RecordEvent(_ context.Context, event models.Event) error {
	recorder.events = append(recorder.events, event)
	return recorder.err
}

func (*linkedInEventRecorder) Events(context.Context, models.EventSearch) (models.EventPage, error) {
	return models.EventPage{}, nil
}

func (recorder *linkedInEventRecorder) types() []string {
	types := make([]string, 0, len(recorder.events))
	for _, event := range recorder.events {
		types = append(types, event.Type)
	}
	return types
}
