package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/models"
	"nice/internal/repositories"
)

const noOpMatchContent = "Match analysis has not been implemented yet."

var ErrMatchJobNotFound = errors.New("job not found")

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
	wake       chan struct{}
	mutex      sync.Mutex
}

func NewJobMatchProcessor(repository repositories.JobRepository, matcher ProfileJobMatcher, logger *zap.Logger) *JobMatchProcessor {
	return &JobMatchProcessor{repository: repository, matcher: matcher, logger: logger, wake: make(chan struct{}, 1)}
}

func (processor *JobMatchProcessor) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			processor.mutex.Lock()
			defer processor.mutex.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			processor.cancel = cancel
			processor.done = make(chan struct{})
			processor.logger.Info("job match processor started", zap.Duration("idle_retry_interval", time.Minute))
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
	for {
		worked, err := processor.process(ctx)
		if err != nil {
			processor.logger.Error("process job match request failed", zap.Error(err))
		}
		if worked && err == nil {
			continue
		}

		timer := time.NewTimer(time.Minute)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-processor.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (processor *JobMatchProcessor) Wake() {
	select {
	case processor.wake <- struct{}{}:
	default:
	}
}

func (processor *JobMatchProcessor) process(ctx context.Context) (bool, error) {
	profile, err := processor.repository.UserProfile(ctx)
	if err != nil {
		return false, err
	}
	if profile == nil {
		return false, nil
	}

	queue, err := processor.repository.MatchQueue(ctx)
	if err != nil {
		return false, err
	}
	if len(queue) == 0 {
		return false, nil
	}
	return true, processor.processJob(ctx, queue[0], *profile)
}

func (processor *JobMatchProcessor) processJob(ctx context.Context, job models.BrowseJob, profile models.UserProfile) error {
	exists, err := processor.repository.JobMatchExists(ctx, job.ID)
	if err != nil {
		return err
	}
	if exists {
		return processor.repository.RemoveMatchRequest(ctx, job.ID)
	}

	content, err := processor.matcher.Match(ctx, job, profile)
	if err != nil {
		return err
	}
	return processor.repository.CompleteMatchRequest(ctx, job.ID, content)
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

type JobMatchRequests struct {
	repository repositories.JobRepository
	processor  *JobMatchProcessor
}

func NewJobMatchRequests(repository repositories.JobRepository, processor *JobMatchProcessor) *JobMatchRequests {
	return &JobMatchRequests{repository: repository, processor: processor}
}

func (requests *JobMatchRequests) Queue(ctx context.Context, jobID int64, redo bool) error {
	found, err := requests.repository.QueueJobMatch(ctx, jobID, redo)
	if err != nil {
		return err
	}
	if !found {
		return ErrMatchJobNotFound
	}
	requests.processor.Wake()
	return nil
}

func (requests *JobMatchRequests) List(ctx context.Context) ([]models.BrowseJob, error) {
	return requests.repository.MatchQueue(ctx)
}

func (requests *JobMatchRequests) Reorder(ctx context.Context, jobIDs []int64) error {
	found, err := requests.repository.ReplaceMatchQueue(ctx, jobIDs)
	if err != nil {
		return err
	}
	if !found {
		return ErrMatchJobNotFound
	}
	requests.processor.Wake()
	return nil
}
