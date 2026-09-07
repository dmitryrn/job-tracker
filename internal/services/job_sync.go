package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/clients/adzuna"
	"nice/internal/clients/ipinfo"
	"nice/internal/clients/jobicy"
	"nice/internal/clients/remotive"
	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/repositories"
)

type JobSync struct {
	adzuna                  adzunaFetcher
	ipInfo                  ipInfoLookup
	jobicy                  jobicyFetcher
	linkedin                linkedInSyncer
	remotive                remotiveFetcher
	jobs                    repositories.JobRepository
	providerRuns            repositories.ProviderRunRepository
	events                  repositories.EventRecorder
	settings                repositories.DiscoverySettingsRepository
	adzunaInterval          time.Duration
	jobicyInterval          time.Duration
	linkedinInterval        time.Duration
	linkedInRequestInterval time.Duration
	remotiveInterval        time.Duration
	logger                  *zap.Logger
	metrics                 linkedInRunMetrics
	cancel                  context.CancelFunc
	done                    chan struct{}
	trigger                 chan string
	running                 bool
	mutex                   sync.Mutex
}

type linkedInRunMetrics interface {
	Start(time.Time)
	Complete(time.Time, LinkedInFetchResult, bool)
}

type linkedInSyncer interface {
	Sync(context.Context, models.LinkedInSearchSettings, string, time.Duration) (LinkedInFetchResult, error)
}

func NewJobSync(cfg config.Config, adzunaClient *adzuna.Client, ipInfoClient *ipinfo.Client, jobicyClient *jobicy.Client, linkedInJobs *LinkedInJobs, remotiveClient *remotive.Client, jobs repositories.JobRepository, providerRuns repositories.ProviderRunRepository, events repositories.EventRecorder, settings repositories.DiscoverySettingsRepository, metrics *LinkedInMetrics, logger *zap.Logger) *JobSync {
	return &JobSync{
		adzuna:                  adzunaClient,
		ipInfo:                  ipInfoClient,
		jobicy:                  jobicyClient,
		linkedin:                linkedInJobs,
		remotive:                remotiveClient,
		jobs:                    jobs,
		providerRuns:            providerRuns,
		events:                  events,
		settings:                settings,
		metrics:                 metrics,
		adzunaInterval:          cfg.Providers.Adzuna.SyncInterval,
		jobicyInterval:          cfg.Providers.Jobicy.SyncInterval,
		linkedinInterval:        cfg.Providers.LinkedIn.SyncInterval,
		linkedInRequestInterval: cfg.Providers.LinkedIn.RequestInterval,
		remotiveInterval:        cfg.Providers.Remotive.SyncInterval,
		logger:                  logger,
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
			syncer.trigger = make(chan string, 1)
			syncer.logger.Info("job sync scheduler started", zap.Duration("check_interval", time.Minute))
			go syncer.run(ctx, syncer.trigger)
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
				syncer.mutex.Lock()
				syncer.cancel = nil
				syncer.done = nil
				syncer.trigger = nil
				syncer.mutex.Unlock()
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

// Trigger queues one forced provider run when the scheduler is idle.
func (syncer *JobSync) Trigger(provider string) bool {
	syncer.mutex.Lock()
	defer syncer.mutex.Unlock()

	if !isDiscoveryProvider(provider) {
		syncer.logger.Warn("job sync request ignored", zap.String("provider", provider), zap.String("reason", "unknown provider"))
		return false
	}
	if syncer.trigger == nil {
		syncer.logger.Info("job sync request ignored", zap.String("provider", provider), zap.String("reason", "scheduler not running"))
		return false
	}
	if syncer.running {
		syncer.logger.Info("job sync request ignored", zap.String("provider", provider), zap.String("reason", "sync already running"))
		return false
	}
	select {
	case syncer.trigger <- provider:
		syncer.logger.Info("job sync requested", zap.String("provider", provider))
		return true
	default:
		syncer.logger.Info("job sync request ignored", zap.String("provider", provider), zap.String("reason", "sync already requested"))
		return false
	}
}

func (syncer *JobSync) run(ctx context.Context, trigger <-chan string) {
	defer close(syncer.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	syncer.runSync(ctx, "")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncer.runSync(ctx, "")
		case provider := <-trigger:
			syncer.runSync(ctx, provider)
		}
	}
}

func (syncer *JobSync) sync(ctx context.Context) {
	syncer.syncProviders(ctx, false)
}

func (syncer *JobSync) runSync(ctx context.Context, provider string) {
	syncer.mutex.Lock()
	syncer.running = true
	syncer.mutex.Unlock()
	defer func() {
		syncer.mutex.Lock()
		syncer.running = false
		syncer.mutex.Unlock()
	}()
	if provider == "" {
		syncer.syncProviders(ctx, false)
		return
	}
	settings, err := syncer.settings.DiscoverySettings(ctx)
	if err != nil {
		syncer.logger.Error("load discovery settings failed", zap.String("provider", provider), zap.Error(err))
		return
	}
	syncer.syncProvider(ctx, settings, provider, true)
}

func (syncer *JobSync) syncProviders(ctx context.Context, force bool) {
	settings, err := syncer.settings.DiscoverySettings(ctx)
	if err != nil {
		syncer.logger.Error("load discovery settings failed", zap.Error(err))
		return
	}
	var group sync.WaitGroup
	group.Add(4)
	go func() {
		defer group.Done()
		syncer.syncProvider(ctx, settings, "adzuna", force)
	}()
	go func() {
		defer group.Done()
		syncer.syncProvider(ctx, settings, "jobicy", force)
	}()
	go func() {
		defer group.Done()
		syncer.syncProvider(ctx, settings, "linkedin", force)
	}()
	go func() {
		defer group.Done()
		syncer.syncProvider(ctx, settings, "remotive", force)
	}()
	group.Wait()
}

func (syncer *JobSync) syncProvider(ctx context.Context, settings models.DiscoverySettings, provider string, force bool) {
	switch provider {
	case "adzuna":
		syncer.syncAdzuna(ctx, settings.Adzuna, force)
	case "jobicy":
		syncer.syncJobicy(ctx, settings.Jobicy, force)
	case "linkedin":
		syncer.syncLinkedIn(ctx, settings.LinkedIn, force)
	case "remotive":
		syncer.syncRemotive(ctx, settings.Remotive, force)
	}
}

func isDiscoveryProvider(provider string) bool {
	switch provider {
	case "adzuna", "jobicy", "linkedin", "remotive":
		return true
	default:
		return false
	}
}

func (syncer *JobSync) syncLinkedIn(ctx context.Context, settings models.LinkedInSearchSettings, force bool) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "linkedin"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "linkedin", syncer.linkedinInterval, force) {
		return
	}
	runID := fmt.Sprintf("linkedin-%d", time.Now().UTC().UnixNano())
	if syncer.metrics != nil {
		syncer.metrics.Start(time.Now())
	}
	if !syncer.allowLinkedInSync(ctx, runID) {
		if syncer.metrics != nil {
			syncer.metrics.Complete(time.Now(), LinkedInFetchResult{}, false)
		}
		return
	}
	syncer.recordLinkedInEvent(ctx, runID, "provider.run.started", "info", "LinkedIn job sync started", map[string]any{
		"query": settings.Query, "location": settings.Location, "postedWithin": settings.PostedWithin, "workplace": settings.Workplace, "experienceLevel": settings.ExperienceLevel, "requestedLimit": settings.Limit,
	})
	syncer.logger.Info("syncing LinkedIn jobs", zap.String("run_id", runID))
	fetch, fetchErr := syncer.linkedin.Sync(ctx, settings, runID, syncer.linkedInRequestInterval)
	if fetchErr != nil {
		syncer.logger.Error("LinkedIn fetch incomplete", zap.String("run_id", runID), zap.Error(fetchErr), zap.Int("fetched_job_count", fetch.FetchedJobs))
	}
	if fetchErr != nil {
		if syncer.metrics != nil {
			syncer.metrics.Complete(time.Now(), fetch, false)
		}
		syncer.finishLinkedInRun(ctx, runID, settings.Limit, fetch, fetch.SavedJobs, fetchErr)
		return
	}
	if syncer.metrics != nil {
		syncer.metrics.Complete(time.Now(), fetch, true)
	}
	syncer.logger.Info("stored LinkedIn jobs", zap.String("run_id", runID), zap.Int("count", fetch.SavedJobs))
	syncer.finishLinkedInRun(ctx, runID, settings.Limit, fetch, fetch.SavedJobs, nil)
}

type ipInfoLookup interface {
	Lookup(context.Context) (ipinfo.Info, error)
}

func (syncer *JobSync) allowLinkedInSync(ctx context.Context, runID string) bool {
	info, err := syncer.ipInfo.Lookup(ctx)
	if err != nil {
		syncer.recordLinkedInEvent(ctx, runID, "linkedin.ip_info.failed", "error", "LinkedIn IP info lookup failed; sync blocked", map[string]any{"error": err.Error()})
		syncer.logger.Error("LinkedIn IP info lookup failed; sync blocked", zap.String("run_id", runID), zap.Error(err))
		return false
	}
	country := strings.ToUpper(strings.TrimSpace(info.Country))
	syncer.recordLinkedInEvent(ctx, runID, "linkedin.ip_info.resolved", "info", "LinkedIn IP info resolved", map[string]any{"country": country})
	syncer.logger.Info("LinkedIn IP info resolved", zap.String("run_id", runID), zap.String("country", country))
	if country == "RS" {
		syncer.recordLinkedInEvent(ctx, runID, "linkedin.ip_info.blocked", "error", "LinkedIn job sync blocked: egress country is Serbia", map[string]any{"country": country})
		syncer.logger.Error("LinkedIn job sync blocked: egress country is Serbia", zap.String("run_id", runID), zap.String("country", country))
		return false
	}
	return true
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

func (syncer *JobSync) syncAdzuna(ctx context.Context, settings models.AdzunaSearchSettings, force bool) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "adzuna"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "adzuna", syncer.adzunaInterval, force) {
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

func (syncer *JobSync) syncRemotive(ctx context.Context, settings models.RemotiveSearchSettings, force bool) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "remotive"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "remotive", syncer.remotiveInterval, force) {
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

func (syncer *JobSync) syncJobicy(ctx context.Context, settings models.JobicySearchSettings, force bool) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "jobicy"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "jobicy", syncer.jobicyInterval, force) {
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

func (syncer *JobSync) startProviderRun(ctx context.Context, provider string, interval time.Duration, force bool) bool {
	if force {
		interval = 0
	}
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
