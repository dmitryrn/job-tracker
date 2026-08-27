package services

import (
	"context"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/clients/adzuna"
	"nice/internal/config"
	"nice/internal/repositories"
)

type JobSync struct {
	client     *adzuna.Client
	repository repositories.JobRepository
	interval   time.Duration
	logger     *zap.Logger
	cancel     context.CancelFunc
	done       chan struct{}
	mutex      sync.Mutex
}

func NewJobSync(cfg config.Config, client *adzuna.Client, repository repositories.JobRepository, logger *zap.Logger) *JobSync {
	return &JobSync{
		client:     client,
		repository: repository,
		interval:   cfg.Sync.Interval,
		logger:     logger,
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
			syncer.logger.Info("job sync scheduler started", zap.Duration("interval", syncer.interval))
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
	ticker := time.NewTicker(syncer.interval)
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
	syncer.logger.Info("syncing Adzuna jobs")
	jobs, err := syncer.client.Fetch(ctx)
	if err != nil {
		syncer.logger.Error("Adzuna sync failed", zap.Error(err))
		return
	}
	if err := syncer.repository.Upsert(ctx, jobs); err != nil {
		syncer.logger.Error("persist Adzuna jobs failed", zap.Error(err))
		return
	}
	syncer.logger.Info("stored Adzuna jobs", zap.Int("count", len(jobs)))
}
