package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/clients/adzuna"
	"nice/internal/clients/jobicy"
	"nice/internal/clients/remotive"
	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/repositories"
)

type JobSync struct {
	adzuna           adzunaFetcher
	jobicy           jobicyFetcher
	linkedin         linkedInFetcher
	remotive         remotiveFetcher
	jobs             repositories.JobRepository
	providerRuns     repositories.ProviderRunRepository
	events           repositories.EventRecorder
	settings         repositories.DiscoverySettingsRepository
	adzunaInterval   time.Duration
	jobicyInterval   time.Duration
	linkedinInterval time.Duration
	remotiveInterval time.Duration
	logger           *zap.Logger
	cancel           context.CancelFunc
	done             chan struct{}
	mutex            sync.Mutex
}

func NewJobSync(cfg config.Config, adzunaClient *adzuna.Client, jobicyClient *jobicy.Client, linkedInJobs *LinkedInJobs, remotiveClient *remotive.Client, jobs repositories.JobRepository, providerRuns repositories.ProviderRunRepository, events repositories.EventRecorder, settings repositories.DiscoverySettingsRepository, logger *zap.Logger) *JobSync {
	return &JobSync{
		adzuna:           adzunaClient,
		jobicy:           jobicyClient,
		linkedin:         linkedInJobs,
		remotive:         remotiveClient,
		jobs:             jobs,
		providerRuns:     providerRuns,
		events:           events,
		settings:         settings,
		adzunaInterval:   cfg.Providers.Adzuna.SyncInterval,
		jobicyInterval:   cfg.Providers.Jobicy.SyncInterval,
		linkedinInterval: cfg.Providers.LinkedIn.SyncInterval,
		remotiveInterval: cfg.Providers.Remotive.SyncInterval,
		logger:           logger,
	}
}

func (syncer *JobSync) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			syncer.mutex.Lock()
			defer syncer.mutex.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			syncer.cancel = cancel
			syncer.done = make(chan struct{})
			syncer.logger.Info("job sync scheduler started", zap.Duration("check_interval", time.Minute))
			go syncer.run(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			syncer.mutex.Lock()
			cancel := syncer.cancel
			done := syncer.done
			syncer.mutex.Unlock()
			if cancel == nil {
				return nil
			}
			cancel()
			select {
			case <-done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

func (syncer *JobSync) run(ctx context.Context) {
	defer close(syncer.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		syncer.sync(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (syncer *JobSync) sync(ctx context.Context) {
	settings, err := syncer.settings.DiscoverySettings(ctx)
	if err != nil {
		syncer.logger.Error("load discovery settings failed", zap.Error(err))
		return
	}
	var group sync.WaitGroup
	group.Add(4)
	go func() {
		defer group.Done()
		syncer.syncAdzuna(ctx, settings.Adzuna)
	}()
	go func() {
		defer group.Done()
		syncer.syncJobicy(ctx, settings.Jobicy)
	}()
	go func() {
		defer group.Done()
		syncer.syncLinkedIn(ctx, settings.LinkedIn)
	}()
	go func() {
		defer group.Done()
		syncer.syncRemotive(ctx, settings.Remotive)
	}()
	group.Wait()
}

func (syncer *JobSync) syncLinkedIn(ctx context.Context, settings models.LinkedInSearchSettings) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "linkedin"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "linkedin", syncer.linkedinInterval) {
		return
	}
	runID := fmt.Sprintf("linkedin-%d", time.Now().UTC().UnixNano())
	syncer.recordLinkedInEvent(ctx, runID, "provider.run.started", "info", "LinkedIn job sync started", map[string]any{
		"query": settings.Query, "location": settings.Location, "requestedLimit": settings.Limit,
	})
	syncer.logger.Info("syncing LinkedIn jobs", zap.String("run_id", runID))
	fetch, fetchErr := syncer.linkedin.Fetch(ctx, settings, runID, func(ctx context.Context, job models.Job) error {
		return syncer.jobs.Upsert(ctx, []models.Job{job})
	})
	if fetchErr != nil {
		syncer.logger.Error("LinkedIn fetch incomplete", zap.String("run_id", runID), zap.Error(fetchErr), zap.Int("fetched_job_count", fetch.FetchedJobs))
	}
	if fetchErr != nil {
		syncer.finishLinkedInRun(ctx, runID, settings.Limit, fetch, fetch.SavedJobs, fetchErr)
		return
	}
	syncer.logger.Info("stored LinkedIn jobs", zap.String("run_id", runID), zap.Int("count", fetch.SavedJobs))
	syncer.finishLinkedInRun(ctx, runID, settings.Limit, fetch, fetch.SavedJobs, nil)
}

func (syncer *JobSync) finishLinkedInRun(ctx context.Context, runID string, limit int, fetch LinkedInFetchResult, saved int, runError error) {
	ctx, cancel := finalizationContext(ctx)
	defer cancel()
	level := "info"
	message := "LinkedIn job sync finished"
	data := map[string]any{
		"requestedLimit": limit, "searchResultsFetched": fetch.SearchResults, "detailRequests": fetch.DetailRequests,
		"jobsFetched": fetch.FetchedJobs, "savedJobs": saved,
	}
	if runError != nil {
		level = "error"
		message = "LinkedIn job sync failed"
		data["error"] = runError.Error()
	}
	syncer.recordLinkedInEvent(ctx, runID, "provider.run.finished", level, message, data)
}

func (syncer *JobSync) recordLinkedInEvent(ctx context.Context, runID, eventType, level, message string, data map[string]any) {
	if syncer.events == nil {
		return
	}
	if err := syncer.events.RecordEvent(ctx, models.Event{Provider: "linkedin", RunID: runID, Type: eventType, Level: level, Message: message, Data: data}); err != nil {
		syncer.logger.Error("record LinkedIn event failed", zap.String("run_id", runID), zap.String("event_type", eventType), zap.Error(err))
	}
}

func (syncer *JobSync) syncAdzuna(ctx context.Context, settings models.AdzunaSearchSettings) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "adzuna"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "adzuna", syncer.adzunaInterval) {
		return
	}
	syncer.logger.Info("syncing Adzuna jobs")
	jobs, err := syncer.adzuna.Fetch(ctx, settings)
	if err != nil {
		syncer.logger.Error("Adzuna sync failed", zap.Error(err))
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Adzuna jobs failed", zap.Error(err))
		return
	}
	syncer.logger.Info("stored Adzuna jobs", zap.Int("count", len(jobs)))
}

func (syncer *JobSync) syncRemotive(ctx context.Context, settings models.RemotiveSearchSettings) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "remotive"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "remotive", syncer.remotiveInterval) {
		return
	}
	syncer.logger.Info("syncing Remotive jobs")
	jobs, err := syncer.remotive.Fetch(ctx, settings)
	if err != nil {
		syncer.logger.Error("Remotive sync failed", zap.Error(err))
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Remotive jobs failed", zap.Error(err))
		return
	}
	syncer.logger.Info("stored Remotive jobs", zap.Int("count", len(jobs)))
}

func (syncer *JobSync) syncJobicy(ctx context.Context, settings models.JobicySearchSettings) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "jobicy"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "jobicy", syncer.jobicyInterval) {
		return
	}
	syncer.logger.Info("syncing Jobicy jobs")
	jobs, err := syncer.jobicy.Fetch(ctx, settings)
	if err != nil {
		syncer.logger.Error("Jobicy sync failed", zap.Error(err))
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Jobicy jobs failed", zap.Error(err))
		return
	}
	syncer.logger.Info("stored Jobicy jobs", zap.Int("count", len(jobs)))
}

func (syncer *JobSync) startProviderRun(ctx context.Context, provider string, interval time.Duration) bool {
	run, err := syncer.providerRuns.StartProviderRun(ctx, provider, interval, time.Now())
	if err != nil {
		syncer.logger.Error("start provider sync failed", zap.String("provider", provider), zap.Error(err))
		return false
	}
	if !run {
		syncer.logger.Debug("provider sync not due", zap.String("provider", provider))
	}
	return run
}

func finalizationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.Background(), 5*time.Second)
}
