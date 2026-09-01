package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/models"
	"nice/internal/repositories"
)

const jobMatchRetryInterval = 30 * time.Second

var ErrMatchJobNotFound = errors.New("job not found")

type ProfileJobMatcher interface {
	Match(context.Context, models.BrowseJob, models.JobAnalysisRecord, models.UserProfile) (models.JobMatchAssessment, error)
}

type JobAnalysisService interface {
	Analyze(context.Context, models.Job) (JobAnalysis, error)
}

type JobMatchProcessor struct {
	repository  repositories.JobRepository
	analyzer    JobAnalysisService
	matcher     ProfileJobMatcher
	logger      *zap.Logger
	runInterval time.Duration
	cancel      context.CancelFunc
	done        chan struct{}
	wake        chan struct{}
	mutex       sync.Mutex
}

func NewJobMatchProcessor(repository repositories.JobRepository, analyzer JobAnalysisService, matcher ProfileJobMatcher, logger *zap.Logger, runInterval time.Duration) *JobMatchProcessor {
	return &JobMatchProcessor{repository: repository, analyzer: analyzer, matcher: matcher, logger: logger, runInterval: runInterval, wake: make(chan struct{}, 1)}
}

func (processor *JobMatchProcessor) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			processor.mutex.Lock()
			defer processor.mutex.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			processor.cancel = cancel
			processor.done = make(chan struct{})
			processor.logger.Info("job match processor started", zap.Duration("idle_retry_interval", jobMatchRetryInterval))
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
				processor.logger.Error("job match processor shutdown timed out", zap.Error(ctx.Err()))
				return ctx.Err()
			}
		},
	})
}

func (processor *JobMatchProcessor) run(ctx context.Context) {
	defer close(processor.done)
	for {
		worked, err := processor.process(ctx)
		interval, cooldown := jobMatchRunInterval(worked, err, processor.runInterval)
		if cooldown {
			if err != nil {
				processor.logger.Info("job match worker cooling down after failed run", zap.Duration("run_interval", interval), zap.Error(err))
			} else {
				processor.logger.Info("job match worker cooling down after successful run", zap.Duration("run_interval", interval))
			}
		}

		timer := time.NewTimer(interval)
		wake := processor.wake
		if cooldown {
			// Queue changes cannot skip the configured run cooldown.
			wake = nil
		}
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func jobMatchRunInterval(worked bool, err error, runInterval time.Duration) (time.Duration, bool) {
	if worked || err != nil {
		return runInterval, true
	}
	return jobMatchRetryInterval, false
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
		processor.logger.Error("load user profile for job matching failed", zap.Error(err))
		return false, err
	}
	if profile == nil {
		processor.logger.Warn("job match worker waiting for user profile")
		return false, nil
	}

	queue, err := processor.repository.MatchQueue(ctx)
	if err != nil {
		processor.logger.Error("load job match queue failed", zap.Error(err))
		return false, err
	}
	if len(queue) == 0 {
		return false, nil
	}
	return true, processor.processJob(ctx, queue[0], *profile)
}

func (processor *JobMatchProcessor) processJob(ctx context.Context, job models.BrowseJob, profile models.UserProfile) (err error) {
	processor.logger.Info("job match worker executing", zap.Int64("job_id", job.ID))
	defer func() {
		if err != nil {
			processor.logger.Error("job match worker failed", zap.Int64("job_id", job.ID), zap.Error(err))
			return
		}
		processor.logger.Info("job match worker finished successfully", zap.Int64("job_id", job.ID))
	}()
	if err := processor.ensureJobAnalysis(ctx, job.ID); err != nil {
		return err
	}

	exists, err := processor.repository.JobMatchExists(ctx, job.ID)
	if err != nil {
		return err
	}
	if exists {
		return processor.repository.RemoveMatchRequest(ctx, job.ID)
	}

	analysis, err := processor.repository.JobAnalysis(ctx, job.ID)
	if err != nil {
		return err
	}
	if analysis == nil {
		return errors.New("job analysis was not saved")
	}
	assessment, err := processor.matcher.Match(ctx, job, *analysis, profile)
	if err != nil {
		return err
	}
	content, err := json.Marshal(assessment)
	if err != nil {
		return fmt.Errorf("encode job match assessment: %w", err)
	}
	return processor.repository.CompleteMatchRequest(ctx, job.ID, string(content))
}

func (processor *JobMatchProcessor) ensureJobAnalysis(ctx context.Context, jobID int64) (err error) {
	processor.logger.Info("job analysis worker executing", zap.Int64("job_id", jobID))
	defer func() {
		if err != nil {
			processor.logger.Error("job analysis worker failed", zap.Int64("job_id", jobID), zap.Error(err))
			return
		}
		processor.logger.Info("job analysis worker finished successfully", zap.Int64("job_id", jobID))
	}()

	job, err := processor.repository.AnalysisJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return ErrMatchJobNotFound
	}

	existing, err := processor.repository.JobAnalysis(ctx, jobID)
	if err != nil {
		return err
	}
	if jobAnalysisCurrent(existing, *job) {
		processor.logger.Info("job analysis worker reused current analysis", zap.Int64("job_id", jobID))
		return nil
	}

	analysis, err := processor.analyzer.Analyze(ctx, *job)
	if err != nil {
		return err
	}
	return processor.repository.SaveJobAnalysis(ctx, models.JobAnalysisRecord{
		JobID:                 jobID,
		AnalyzerVersion:       analysis.AnalyzerVersion,
		PromptVersion:         analysis.PromptVersion,
		InputSHA256:           analysis.InputSHA256,
		Model:                 analysis.Model,
		AnalyzedAt:            analysis.AnalyzedAt,
		NormalizedDescription: analysis.NormalizedDescription,
		Analysis:              analysis.Analysis,
	})
}

func jobAnalysisCurrent(analysis *models.JobAnalysisRecord, job models.Job) bool {
	return analysis != nil &&
		analysis.AnalyzerVersion == JobAnalyzerVersion &&
		analysis.PromptVersion == JobPromptVersion &&
		analysis.InputSHA256 == jobAnalysisInputSHA256(job)
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

func (matches *JobMatches) Analysis(ctx context.Context, jobID int64) (*models.JobAnalysisRecord, error) {
	return matches.repository.JobAnalysis(ctx, jobID)
}

func (matches *JobMatches) List(ctx context.Context) ([]models.JobMatchSummary, error) {
	return matches.repository.JobMatches(ctx)
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
