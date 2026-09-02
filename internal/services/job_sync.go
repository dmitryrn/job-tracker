package services

import (
	"context"
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
	adzuna           *adzuna.Client
	jobicy           *jobicy.Client
	linkedin         *LinkedInJobs
	remotive         *remotive.Client
	jobs             repositories.JobRepository
	providerRuns     repositories.ProviderRunRepository
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

func NewJobSync(cfg config.Config, adzunaClient *adzuna.Client, jobicyClient *jobicy.Client, linkedInJobs *LinkedInJobs, remotiveClient *remotive.Client, jobs repositories.JobRepository, providerRuns repositories.ProviderRunRepository, settings repositories.DiscoverySettingsRepository, logger *zap.Logger) *JobSync {
	return &JobSync{
		adzuna:           adzunaClient,
		jobicy:           jobicyClient,
		linkedin:         linkedInJobs,
		remotive:         remotiveClient,
		jobs:             jobs,
		providerRuns:     providerRuns,
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
	syncer.syncAdzuna(ctx, settings.Adzuna)
	syncer.syncJobicy(ctx, settings.Jobicy)
	syncer.syncLinkedIn(ctx, settings.LinkedIn)
	syncer.syncRemotive(ctx, settings.Remotive)
}

func (syncer *JobSync) syncLinkedIn(ctx context.Context, settings models.LinkedInSearchSettings) {
	if !settings.Enabled {
		syncer.logger.Info("provider sync skipped", zap.String("provider", "linkedin"), zap.String("reason", "disabled"))
		return
	}
	if !syncer.startProviderRun(ctx, "linkedin", syncer.linkedinInterval) {
		return
	}
	syncer.logger.Info("syncing LinkedIn jobs")
	jobs, err := syncer.linkedin.Fetch(ctx, settings)
	if err != nil {
		syncer.logger.Error("LinkedIn sync failed", zap.Error(err))
		syncer.completeProviderRun(ctx, "linkedin", err)
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist LinkedIn jobs failed", zap.Error(err))
		syncer.completeProviderRun(ctx, "linkedin", err)
		return
	}
	syncer.completeProviderRun(ctx, "linkedin", nil)
	syncer.logger.Info("stored LinkedIn jobs", zap.Int("count", len(jobs)))
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
		syncer.completeProviderRun(ctx, "adzuna", err)
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Adzuna jobs failed", zap.Error(err))
		syncer.completeProviderRun(ctx, "adzuna", err)
		return
	}
	syncer.completeProviderRun(ctx, "adzuna", nil)
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
		syncer.completeProviderRun(ctx, "remotive", err)
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Remotive jobs failed", zap.Error(err))
		syncer.completeProviderRun(ctx, "remotive", err)
		return
	}
	syncer.completeProviderRun(ctx, "remotive", nil)
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
		syncer.completeProviderRun(ctx, "jobicy", err)
		return
	}
	if err := syncer.jobs.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Jobicy jobs failed", zap.Error(err))
		syncer.completeProviderRun(ctx, "jobicy", err)
		return
	}
	syncer.completeProviderRun(ctx, "jobicy", nil)
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

func (syncer *JobSync) completeProviderRun(ctx context.Context, provider string, runError error) {
	if err := syncer.providerRuns.CompleteProviderRun(ctx, provider, runError, time.Now()); err != nil {
		syncer.logger.Error("complete provider sync failed", zap.String("provider", provider), zap.Error(err))
	}
}
