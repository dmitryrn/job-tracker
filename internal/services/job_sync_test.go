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

	"nice/internal/clients/ipinfo"
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

func TestJobSyncTriggerQueuesOneProviderOnly(t *testing.T) {
	syncer := JobSync{trigger: make(chan string, 1), logger: zap.NewNop()}

	assert.True(t, syncer.Trigger("linkedin"))
	assert.False(t, syncer.Trigger("adzuna"))
	assert.Equal(t, "linkedin", <-syncer.trigger)

	syncer.running = true
	assert.False(t, syncer.Trigger("linkedin"))
	syncer.running = false
	assert.False(t, syncer.Trigger("unknown"))
}

func TestJobSyncRunsProvidersConcurrently(t *testing.T) {
	starts := make(chan string, 4)
	release := make(chan struct{})
	syncer := JobSync{
		adzuna:       adzunaBlockingFetcher{starts: starts, release: release},
		ipInfo:       ipInfoLookupStub{info: ipinfo.Info{Country: "DE"}},
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

func TestJobSyncRecordsPartialLinkedInResults(t *testing.T) {
	fetchErr := errors.New("detail request failed")
	runs := &providerRunRecorder{}
	events := &eventRepositoryRecorder{}
	syncer := JobSync{
		linkedin: linkedInFetcherStub{
			jobs: []models.Job{{Source: "linkedin", SourceID: "completed"}},
			err:  fetchErr,
		},
		ipInfo:       ipInfoLookupStub{info: ipinfo.Info{Country: "DE"}},
		providerRuns: runs,
		events:       events,
		logger:       zap.NewNop(),
	}

	syncer.syncLinkedIn(context.Background(), models.LinkedInSearchSettings{Enabled: true}, false)

	require.Len(t, events.events, 3)
	assert.Equal(t, "linkedin.ip_info.resolved", events.events[0].Type)
	assert.Equal(t, "DE", events.events[0].Data["country"])
	assert.Equal(t, "provider.run.started", events.events[1].Type)
	assert.Equal(t, "provider.run.finished", events.events[2].Type)
	assert.Equal(t, events.events[1].RunID, events.events[2].RunID)
	assert.Equal(t, 1, events.events[2].Data["savedJobs"])
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

func TestJobSyncBlocksLinkedInWhenEgressIsInSerbia(t *testing.T) {
	fetcher := &linkedInFetcherRecorder{}
	events := &eventRepositoryRecorder{}
	syncer := JobSync{
		linkedin:     fetcher,
		ipInfo:       ipInfoLookupStub{info: ipinfo.Info{Country: "rs"}},
		providerRuns: &providerRunRecorder{},
		events:       events,
		logger:       zap.NewNop(),
	}

	syncer.syncLinkedIn(context.Background(), models.LinkedInSearchSettings{Enabled: true}, false)

	assert.Zero(t, fetcher.calls)
	require.Len(t, events.events, 2)
	assert.Equal(t, "linkedin.ip_info.resolved", events.events[0].Type)
	assert.Equal(t, "RS", events.events[0].Data["country"])
	assert.Equal(t, "linkedin.ip_info.blocked", events.events[1].Type)
}

func TestJobSyncBlocksLinkedInWhenIPInfoLookupFails(t *testing.T) {
	fetcher := &linkedInFetcherRecorder{}
	events := &eventRepositoryRecorder{}
	syncer := JobSync{
		linkedin:     fetcher,
		ipInfo:       ipInfoLookupStub{err: errors.New("IPinfo unavailable")},
		providerRuns: &providerRunRecorder{},
		events:       events,
		logger:       zap.NewNop(),
	}

	syncer.syncLinkedIn(context.Background(), models.LinkedInSearchSettings{Enabled: true}, false)

	assert.Zero(t, fetcher.calls)
	require.Len(t, events.events, 1)
	assert.Equal(t, "linkedin.ip_info.failed", events.events[0].Type)
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
}

func (recorder *providerRunRecorder) StartProviderRun(context.Context, string, time.Duration, time.Time) (bool, error) {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.started++
	return true, nil
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

func (fetcher linkedInBlockingFetcher) Sync(context.Context, models.LinkedInSearchSettings, string, time.Duration) (LinkedInFetchResult, error) {
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

type ipInfoLookupStub struct {
	info ipinfo.Info
	err  error
}

func (stub ipInfoLookupStub) Lookup(context.Context) (ipinfo.Info, error) {
	return stub.info, stub.err
}

type linkedInFetcherRecorder struct {
	calls int
}

func (fetcher *linkedInFetcherRecorder) Sync(context.Context, models.LinkedInSearchSettings, string, time.Duration) (LinkedInFetchResult, error) {
	fetcher.calls++
	return LinkedInFetchResult{}, nil
}

func (stub linkedInFetcherStub) Sync(context.Context, models.LinkedInSearchSettings, string, time.Duration) (LinkedInFetchResult, error) {
	return LinkedInFetchResult{Jobs: stub.jobs, FetchedJobs: len(stub.jobs), SavedJobs: len(stub.jobs)}, stub.err
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
