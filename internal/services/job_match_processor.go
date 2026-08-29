package services

import (
	"context"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/models"
	"nice/internal/repositories"
)

const noOpMatchContent = "Match analysis has not been implemented yet."

type ProfileJobMatcher interface {
	Match(context.Context, models.BrowseJob, models.UserProfile) (string, error)
}

type noOpProfileJobMatcher struct{}

func NewNoOpProfileJobMatcher() ProfileJobMatcher {
	return noOpProfileJobMatcher{}
}

func (noOpProfileJobMatcher) Match(_ context.Context, _ models.BrowseJob, _ models.UserProfile) (string, error) {
	return noOpMatchContent, nil
}

type JobMatchProcessor struct {
	repository repositories.JobRepository
	matcher    ProfileJobMatcher
	logger     *zap.Logger
	cancel     context.CancelFunc
	done       chan struct{}
	mutex      sync.Mutex
}

func NewJobMatchProcessor(repository repositories.JobRepository, matcher ProfileJobMatcher, logger *zap.Logger) *JobMatchProcessor {
	return &JobMatchProcessor{repository: repository, matcher: matcher, logger: logger}
}

func (processor *JobMatchProcessor) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			processor.mutex.Lock()
			defer processor.mutex.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			processor.cancel = cancel
			processor.done = make(chan struct{})
			processor.logger.Info("job match processor started", zap.Duration("check_interval", time.Minute))
			go processor.run(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			processor.mutex.Lock()
			cancel := processor.cancel
			done := processor.done
			processor.mutex.Unlock()
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

func (processor *JobMatchProcessor) run(ctx context.Context) {
	defer close(processor.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		processor.process(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (processor *JobMatchProcessor) process(ctx context.Context) {
	profile, err := processor.repository.UserProfile(ctx)
	if err != nil {
		processor.logger.Error("load user profile for matching failed", zap.Error(err))
		return
	}
	if profile == nil {
		return
	}

	jobs, err := processor.repository.JobsWithoutMatches(ctx, 20)
	if err != nil {
		processor.logger.Error("load jobs awaiting matches failed", zap.Error(err))
		return
	}
	for _, job := range jobs {
		if err := processor.processJob(ctx, job, *profile); err != nil {
			processor.logger.Error("create job match failed", zap.Int64("job_id", job.ID), zap.Error(err))
		}
	}
}

func (processor *JobMatchProcessor) processJob(ctx context.Context, job models.BrowseJob, profile models.UserProfile) error {
	exists, err := processor.repository.JobMatchExists(ctx, job.ID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	content, err := processor.matcher.Match(ctx, job, profile)
	if err != nil {
		return err
	}
	return processor.repository.CreateJobMatch(ctx, job.ID, content)
}

type JobMatches struct {
	repository repositories.JobRepository
}

func NewJobMatches(repository repositories.JobRepository) *JobMatches {
	return &JobMatches{repository: repository}
}

func (matches *JobMatches) Match(ctx context.Context, jobID int64) (*models.JobMatchRecord, error) {
	return matches.repository.JobMatch(ctx, jobID)
}
