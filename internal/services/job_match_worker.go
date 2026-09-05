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

// JobMatchWorker schedules queued matching work; repositories only persist its state.
type JobMatchWorker struct {
	jobs        repositories.JobRepository
	analyses    repositories.JobAnalysisRepository
	matches     repositories.JobMatchRepository
	queue       repositories.MatchQueueRepository
	profiles    repositories.UserProfileRepository
	analyzer    JobAnalysisService
	matcher     ProfileJobMatcher
	events      repositories.EventRecorder
	logger      *zap.Logger
	runInterval time.Duration
	cancel      context.CancelFunc
	done        chan struct{}
	wake        chan struct{}
	mutex       sync.Mutex
}

func NewJobMatchWorker(jobs repositories.JobRepository, analyses repositories.JobAnalysisRepository, matches repositories.JobMatchRepository, queue repositories.MatchQueueRepository, profiles repositories.UserProfileRepository, analyzer JobAnalysisService, matcher ProfileJobMatcher, events repositories.EventRecorder, logger *zap.Logger, runInterval time.Duration) *JobMatchWorker {
	return &JobMatchWorker{jobs: jobs, analyses: analyses, matches: matches, queue: queue, profiles: profiles, analyzer: analyzer, matcher: matcher, events: events, logger: logger, runInterval: runInterval, wake: make(chan struct{}, 1)}
}

func (worker *JobMatchWorker) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			worker.mutex.Lock()
			defer worker.mutex.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			worker.cancel = cancel
			worker.done = make(chan struct{})
			worker.logger.Info("job match worker started", zap.Duration("idle_retry_interval", jobMatchRetryInterval))
			go worker.run(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			worker.mutex.Lock()
			cancel := worker.cancel
			done := worker.done
			worker.mutex.Unlock()
			if cancel == nil {
				return nil
			}
			cancel()
			select {
			case <-done:
				return nil
			case <-ctx.Done():
				worker.logger.Error("job match worker shutdown timed out", zap.Error(ctx.Err()))
				return ctx.Err()
			}
		},
	})
}

func (worker *JobMatchWorker) run(ctx context.Context) {
	defer close(worker.done)
	for {
		worked, err := worker.process(ctx)
		interval, cooldown := jobMatchRunInterval(worked, err, worker.runInterval)
		if cooldown {
			if err != nil {
				worker.logger.Info("job match worker cooling down after failed run", zap.Duration("run_interval", interval), zap.Error(err))
			} else {
				worker.logger.Info("job match worker cooling down after successful run", zap.Duration("run_interval", interval))
			}
		}

		timer := time.NewTimer(interval)
		wake := worker.wake
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

func (worker *JobMatchWorker) Wake() {
	select {
	case worker.wake <- struct{}{}:
	default:
	}
}

func (worker *JobMatchWorker) process(ctx context.Context) (bool, error) {
	queue, err := worker.queue.MatchQueue(ctx)
	if err != nil {
		worker.logger.Error("load job match queue failed", zap.Error(err))
		return false, err
	}
	if len(queue) == 0 {
		return false, nil
	}
	profile, err := worker.profiles.UserProfile(ctx)
	if err != nil {
		worker.logger.Error("load user profile for job matching failed", zap.Error(err))
		return false, err
	}
	if profile == nil {
		worker.logger.Warn("job match worker waiting for user profile")
		worker.recordEvent(ctx, "", "job_match.waiting_for_profile", "warn", "Job match worker waiting for user profile", nil)
		return false, nil
	}
	return true, worker.processJob(ctx, queue[0], *profile)
}

func (worker *JobMatchWorker) processJob(ctx context.Context, job models.BrowseJob, profile models.UserProfile) (err error) {
	runID := fmt.Sprintf("job-match-%d-%d", job.ID, time.Now().UTC().UnixNano())
	matchCompleted := false
	worker.recordEvent(ctx, runID, "job_match.started", "info", "Job match started", map[string]any{"jobId": job.ID})
	worker.logger.Info("job match worker executing", zap.Int64("job_id", job.ID))
	defer func() {
		if err != nil {
			worker.logger.Error("job match worker failed", zap.Int64("job_id", job.ID), zap.Error(err))
			worker.recordEvent(ctx, runID, "job_match.failed", "error", "Job match failed", map[string]any{"jobId": job.ID, "error": err.Error()})
			return
		}
		worker.logger.Info("job match worker finished successfully", zap.Int64("job_id", job.ID))
		if matchCompleted {
			worker.recordEvent(ctx, runID, "job_match.completed", "info", "Job match completed", map[string]any{"jobId": job.ID})
		}
	}()
	if err := worker.ensureJobAnalysis(ctx, runID, job.ID); err != nil {
		return err
	}

	exists, err := worker.matches.JobMatchExists(ctx, job.ID)
	if err != nil {
		return err
	}
	if exists {
		if err := worker.queue.RemoveMatchRequest(ctx, job.ID); err != nil {
			return err
		}
		worker.recordEvent(ctx, runID, "job_match.skipped_existing", "info", "Job match skipped because an assessment already exists", map[string]any{"jobId": job.ID})
		return nil
	}

	analysis, err := worker.analyses.JobAnalysis(ctx, job.ID)
	if err != nil {
		return err
	}
	if analysis == nil {
		return errors.New("job analysis was not saved")
	}
	assessment, err := worker.matcher.Match(ctx, job, *analysis, profile)
	if err != nil {
		return err
	}
	content, err := json.Marshal(assessment)
	if err != nil {
		return fmt.Errorf("encode job match assessment: %w", err)
	}
	if err := worker.queue.CompleteMatchRequest(ctx, job.ID, string(content)); err != nil {
		return err
	}
	matchCompleted = true
	return nil
}

func (worker *JobMatchWorker) ensureJobAnalysis(ctx context.Context, runID string, jobID int64) (err error) {
	stage := "load_job"
	worker.logger.Info("job analysis worker executing", zap.Int64("job_id", jobID))
	defer func() {
		if err != nil {
			worker.logger.Error("job analysis worker failed", zap.Int64("job_id", jobID), zap.Error(err))
			worker.recordEvent(ctx, runID, "job_match.analysis.failed", "error", "Job match analysis failed", map[string]any{"jobId": jobID, "stage": stage, "error": err.Error()})
			return
		}
		worker.logger.Info("job analysis worker finished successfully", zap.Int64("job_id", jobID))
	}()

	job, err := worker.jobs.AnalysisJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return ErrMatchJobNotFound
	}

	stage = "load_analysis"
	existing, err := worker.analyses.JobAnalysis(ctx, jobID)
	if err != nil {
		return err
	}
	if jobAnalysisCurrent(existing, *job) {
		worker.logger.Info("job analysis worker reused current analysis", zap.Int64("job_id", jobID))
		worker.recordEvent(ctx, runID, "job_match.analysis.reused", "info", "Job match analysis reused", map[string]any{"jobId": jobID})
		return nil
	}

	stage = "analyze"
	analysis, err := worker.analyzer.Analyze(ctx, *job)
	if err != nil {
		return err
	}
	stage = "save_analysis"
	if err := worker.analyses.SaveJobAnalysis(ctx, models.JobAnalysisRecord{
		JobID:                 jobID,
		AnalyzerVersion:       analysis.AnalyzerVersion,
		PromptVersion:         analysis.PromptVersion,
		InputSHA256:           analysis.InputSHA256,
		Model:                 analysis.Model,
		AnalyzedAt:            analysis.AnalyzedAt,
		NormalizedDescription: analysis.NormalizedDescription,
		Analysis:              analysis.Analysis,
	}); err != nil {
		return err
	}
	worker.recordEvent(ctx, runID, "job_match.analysis.completed", "info", "Job match analysis completed", map[string]any{"jobId": jobID})
	return nil
}

func (worker *JobMatchWorker) recordEvent(ctx context.Context, runID, eventType, level, message string, data map[string]any) {
	if worker.events == nil {
		return
	}
	if err := worker.events.RecordEvent(ctx, models.Event{Provider: "job_match", RunID: runID, Type: eventType, Level: level, Message: message, Data: data}); err != nil {
		worker.logger.Error("record job match event failed", zap.String("run_id", runID), zap.String("event_type", eventType), zap.Error(err))
	}
}

func jobAnalysisCurrent(analysis *models.JobAnalysisRecord, job models.Job) bool {
	return analysis != nil &&
		analysis.AnalyzerVersion == JobAnalyzerVersion &&
		analysis.PromptVersion == JobPromptVersion &&
		analysis.InputSHA256 == jobAnalysisInputSHA256(job)
}

type JobMatches struct {
	analyses repositories.JobAnalysisRepository
	matches  repositories.JobMatchRepository
}

func NewJobMatches(analyses repositories.JobAnalysisRepository, matches repositories.JobMatchRepository) *JobMatches {
	return &JobMatches{analyses: analyses, matches: matches}
}

func (matches *JobMatches) Match(ctx context.Context, jobID int64) (*models.JobMatchRecord, error) {
	return matches.matches.JobMatch(ctx, jobID)
}

func (matches *JobMatches) Analysis(ctx context.Context, jobID int64) (*models.JobAnalysisRecord, error) {
	return matches.analyses.JobAnalysis(ctx, jobID)
}

func (matches *JobMatches) List(ctx context.Context) ([]models.JobMatchSummary, error) {
	return matches.matches.JobMatches(ctx)
}

type JobMatchRequests struct {
	queue  repositories.MatchQueueRepository
	worker *JobMatchWorker
}

func NewJobMatchRequests(queue repositories.MatchQueueRepository, worker *JobMatchWorker) *JobMatchRequests {
	return &JobMatchRequests{queue: queue, worker: worker}
}

func (requests *JobMatchRequests) Queue(ctx context.Context, jobID int64, redo bool) error {
	found, err := requests.queue.QueueJobMatch(ctx, jobID, redo)
	if err != nil {
		return err
	}
	if !found {
		return ErrMatchJobNotFound
	}
	requests.worker.Wake()
	return nil
}

func (requests *JobMatchRequests) QueueUnmatched(ctx context.Context, jobIDs []int64) (int, error) {
	queued, err := requests.queue.QueueJobsWithoutMatches(ctx, jobIDs)
	if err != nil {
		return 0, err
	}
	if queued > 0 {
		requests.worker.Wake()
	}
	return queued, nil
}

func (requests *JobMatchRequests) List(ctx context.Context) ([]models.BrowseJob, error) {
	return requests.queue.MatchQueue(ctx)
}

func (requests *JobMatchRequests) Reorder(ctx context.Context, jobIDs []int64) error {
	found, err := requests.queue.ReplaceMatchQueue(ctx, jobIDs)
	if err != nil {
		return err
	}
	if !found {
		return ErrMatchJobNotFound
	}
	requests.worker.Wake()
	return nil
}

func (requests *JobMatchRequests) Remove(ctx context.Context, jobID int64) error {
	return requests.queue.RemoveMatchRequest(ctx, jobID)
}
