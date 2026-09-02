package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"nice/internal/models"
)

func TestJobSyncSkipsDisabledProviders(t *testing.T) {
	runs := &providerRunRecorder{}
	syncer := JobSync{
		providerRuns: runs,
		settings: discoverySettingsStub{settings: models.DiscoverySettings{
			Adzuna:   models.AdzunaSearchSettings{Enabled: false},
			Remotive: models.RemotiveSearchSettings{Enabled: false},
			Jobicy:   models.JobicySearchSettings{Enabled: false},
			LinkedIn: models.LinkedInSearchSettings{Enabled: false},
		}},
		logger: zap.NewNop(),
	}

	syncer.sync(context.Background())

	assert.Zero(t, runs.started)
}

func TestJobSyncRunsProvidersConcurrently(t *testing.T) {
	starts := make(chan string, 4)
	release := make(chan struct{})
	syncer := JobSync{
		adzuna:       adzunaBlockingFetcher{starts: starts, release: release},
		jobicy:       jobicyBlockingFetcher{starts: starts, release: release},
		linkedin:     linkedInBlockingFetcher{starts: starts, release: release},
		remotive:     remotiveBlockingFetcher{starts: starts, release: release},
		providerRuns: &providerRunRecorder{},
		settings: discoverySettingsStub{settings: models.DiscoverySettings{
			Adzuna:   models.AdzunaSearchSettings{Enabled: true},
			Remotive: models.RemotiveSearchSettings{Enabled: true},
			Jobicy:   models.JobicySearchSettings{Enabled: true},
			LinkedIn: models.LinkedInSearchSettings{Enabled: true},
		}},
		logger: zap.NewNop(),
	}
	done := make(chan struct{})
	go func() {
		syncer.sync(context.Background())
		close(done)
	}()

	started := make([]string, 0, 4)
	for range 4 {
		select {
		case provider := <-starts:
			started = append(started, provider)
		case <-time.After(time.Second):
			close(release)
			t.Fatal("providers did not begin concurrently")
		}
	}
	close(release)
	<-done

	assert.ElementsMatch(t, []string{"adzuna", "jobicy", "linkedin", "remotive"}, started)
}

func TestJobSyncPersistsPartialLinkedInResults(t *testing.T) {
	fetchErr := errors.New("detail request failed")
	runs := &providerRunRecorder{}
	jobs := &jobRepositoryRecorder{}
	events := &eventRepositoryRecorder{}
	syncer := JobSync{
		linkedin: linkedInFetcherStub{
			jobs: []models.Job{{Source: "linkedin", SourceID: "completed"}},
			err:  fetchErr,
		},
		jobs:         jobs,
		providerRuns: runs,
		events:       events,
		logger:       zap.NewNop(),
	}

	syncer.syncLinkedIn(context.Background(), models.LinkedInSearchSettings{Enabled: true})

	assert.Equal(t, []models.Job{{Source: "linkedin", SourceID: "completed"}}, jobs.upserted)
	assert.ErrorIs(t, runs.errorFor("linkedin"), fetchErr)
	require.Len(t, events.events, 2)
	assert.Equal(t, "provider.run.started", events.events[0].Type)
	assert.Equal(t, "provider.run.finished", events.events[1].Type)
	assert.Equal(t, events.events[0].RunID, events.events[1].RunID)
	assert.Equal(t, 1, events.events[1].Data["savedJobs"])
}

func TestJobSyncFinalizesLinkedInEventsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events := &eventRepositoryRecorder{}
	syncer := JobSync{events: events, logger: zap.NewNop()}

	syncer.finishLinkedInRun(ctx, "run-1", 25, LinkedInFetchResult{}, 0, context.Canceled)

	require.Len(t, events.events, 1)
	assert.False(t, events.contextCanceled[0])
	assert.Equal(t, "provider.run.finished", events.events[0].Type)
}

func TestJobSyncSavesLinkedInJobsIndividually(t *testing.T) {
	jobs := &jobRepositoryRecorder{}
	syncer := JobSync{
		linkedin:     linkedInFetcherStub{jobs: []models.Job{{SourceID: "first"}, {SourceID: "second"}}},
		jobs:         jobs,
		providerRuns: &providerRunRecorder{},
		logger:       zap.NewNop(),
	}

	syncer.syncLinkedIn(context.Background(), models.LinkedInSearchSettings{Enabled: true, Limit: 2})

	require.Len(t, jobs.batches, 2)
	assert.Equal(t, []models.Job{{SourceID: "first"}}, jobs.batches[0])
	assert.Equal(t, []models.Job{{SourceID: "second"}}, jobs.batches[1])
}

type discoverySettingsStub struct {
	settings models.DiscoverySettings
}

func (stub discoverySettingsStub) DiscoverySettings(context.Context) (models.DiscoverySettings, error) {
	return stub.settings, nil
}

func (discoverySettingsStub) SaveDiscoverySettings(context.Context, models.DiscoverySettings) (models.DiscoverySettings, error) {
	return models.DiscoverySettings{}, nil
}

type providerRunRecorder struct {
	mutex   sync.Mutex
	started int
	errors  map[string]error
}

func (recorder *providerRunRecorder) StartProviderRun(context.Context, string, time.Duration, time.Time) (bool, error) {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.started++
	return true, nil
}

func (recorder *providerRunRecorder) CompleteProviderRun(_ context.Context, provider string, runError error, _ time.Time) error {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	if recorder.errors == nil {
		recorder.errors = make(map[string]error)
	}
	recorder.errors[provider] = runError
	return nil
}

func (recorder *providerRunRecorder) errorFor(provider string) error {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	return recorder.errors[provider]
}

type adzunaBlockingFetcher struct {
	starts  chan<- string
	release <-chan struct{}
}

func (fetcher adzunaBlockingFetcher) Fetch(context.Context, models.AdzunaSearchSettings) ([]models.Job, error) {
	fetcher.starts <- "adzuna"
	<-fetcher.release
	return nil, errors.New("failed")
}

type jobicyBlockingFetcher struct {
	starts  chan<- string
	release <-chan struct{}
}

func (fetcher jobicyBlockingFetcher) Fetch(context.Context, models.JobicySearchSettings) ([]models.Job, error) {
	fetcher.starts <- "jobicy"
	<-fetcher.release
	return nil, errors.New("failed")
}

type linkedInBlockingFetcher struct {
	starts  chan<- string
	release <-chan struct{}
}

func (fetcher linkedInBlockingFetcher) Fetch(context.Context, models.LinkedInSearchSettings, string, linkedInJobSaver) (LinkedInFetchResult, error) {
	fetcher.starts <- "linkedin"
	<-fetcher.release
	return LinkedInFetchResult{}, errors.New("failed")
}

type remotiveBlockingFetcher struct {
	starts  chan<- string
	release <-chan struct{}
}

func (fetcher remotiveBlockingFetcher) Fetch(context.Context, models.RemotiveSearchSettings) ([]models.Job, error) {
	fetcher.starts <- "remotive"
	<-fetcher.release
	return nil, errors.New("failed")
}

type linkedInFetcherStub struct {
	jobs []models.Job
	err  error
}

func (stub linkedInFetcherStub) Fetch(ctx context.Context, _ models.LinkedInSearchSettings, _ string, save linkedInJobSaver) (LinkedInFetchResult, error) {
	fetch := LinkedInFetchResult{Jobs: make([]models.Job, 0, len(stub.jobs))}
	for _, job := range stub.jobs {
		fetch.FetchedJobs++
		if err := save(ctx, job); err != nil {
			return fetch, err
		}
		fetch.SavedJobs++
		fetch.Jobs = append(fetch.Jobs, job)
	}
	return fetch, stub.err
}

type jobRepositoryRecorder struct {
	upserted []models.Job
	batches  [][]models.Job
}

type eventRepositoryRecorder struct {
	events          []models.Event
	contextCanceled []bool
}

func (recorder *eventRepositoryRecorder) RecordEvent(ctx context.Context, event models.Event) error {
	recorder.events = append(recorder.events, event)
	recorder.contextCanceled = append(recorder.contextCanceled, ctx.Err() != nil)
	return nil
}

func (*eventRepositoryRecorder) Events(context.Context, models.EventSearch) (models.EventPage, error) {
	return models.EventPage{}, nil
}

func (recorder *jobRepositoryRecorder) Upsert(_ context.Context, jobs []models.Job) error {
	batch := append([]models.Job(nil), jobs...)
	recorder.batches = append(recorder.batches, batch)
	recorder.upserted = append(recorder.upserted, jobs...)
	return nil
}

func (*jobRepositoryRecorder) List(context.Context, models.JobSearch) ([]models.BrowseJob, error) {
	return nil, nil
}

func (*jobRepositoryRecorder) Job(context.Context, int64) (*models.BrowseJob, error) {
	return nil, nil
}

func (*jobRepositoryRecorder) Delete(context.Context, int64) (bool, error) {
	return false, nil
}

func (*jobRepositoryRecorder) Providers(context.Context) ([]string, error) {
	return nil, nil
}

func (*jobRepositoryRecorder) Companies(context.Context, string) ([]models.BrowseCompany, error) {
	return nil, nil
}

func (*jobRepositoryRecorder) AnalysisJob(context.Context, int64) (*models.Job, error) {
	return nil, nil
}
