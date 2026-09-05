package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
	"nice/internal/models"
	"nice/internal/repositories"
)

func TestJobMatchWorkerCreatesOnlyOneMatchPerJob(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	matcher := &recordingProfileJobMatcher{assessment: models.JobMatchAssessment{MatcherVersion: "test", Score: 84, Summary: "First assessment"}}
	analyzer := &recordingJobAnalyzer{analysis: testJobAnalysis()}
	events := &jobMatchEventRecorder{}
	worker := NewJobMatchWorker(repository, repository, repository, repository, repository, analyzer, matcher, events, zap.NewNop(), time.Minute)
	worked, err := worker.process(context.Background())
	require.NoError(t, err)
	require.False(t, worked)
	assert.Zero(t, matcher.calls)

	found, err := repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	require.True(t, found)

	worked, err = worker.process(context.Background())
	require.NoError(t, err)
	require.True(t, worked)
	worked, err = worker.process(context.Background())
	require.NoError(t, err)
	require.False(t, worked)

	match, err := repository.JobMatch(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, match)
	require.NotNil(t, match.Assessment)
	assert.Equal(t, "First assessment", match.Assessment.Summary)
	assert.Equal(t, "backend_engineering", matcher.analyses[0].Analysis.Role.Family)
	assert.Equal(t, 1, matcher.calls)
	assert.Equal(t, 1, analyzer.calls)
	storedAnalysis, err := repository.JobAnalysis(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, storedAnalysis)
	assert.Equal(t, "backend_engineering", storedAnalysis.Analysis.Role.Family)
	assert.Equal(t, []string{"job_match.started", "job_match.analysis.completed", "job_match.completed"}, events.types())
	assert.Equal(t, events.events[0].RunID, events.events[1].RunID)
	assert.Equal(t, events.events[0].RunID, events.events[2].RunID)

	found, err = repository.ReplaceMatchQueue(context.Background(), []int64{1})
	require.NoError(t, err)
	require.True(t, found)
	worked, err = worker.process(context.Background())
	require.NoError(t, err)
	require.True(t, worked)
	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	assert.Empty(t, queue)
	assert.Equal(t, []string{"job_match.started", "job_match.analysis.completed", "job_match.completed", "job_match.started", "job_match.analysis.reused", "job_match.skipped_existing"}, events.types())
}

func TestJobMatchWorkerWaitsForProfile(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	found, err := repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	require.True(t, found)

	matcher := &recordingProfileJobMatcher{assessment: models.JobMatchAssessment{MatcherVersion: "test", Score: 84, Summary: "unused"}}
	events := &jobMatchEventRecorder{}
	worker := NewJobMatchWorker(repository, repository, repository, repository, repository, &recordingJobAnalyzer{analysis: testJobAnalysis()}, matcher, events, zap.NewNop(), time.Minute)
	worked, err := worker.process(context.Background())
	require.NoError(t, err)
	require.False(t, worked)
	assert.Zero(t, matcher.calls)
	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	assert.Len(t, queue, 1)
	assert.Equal(t, []string{"job_match.waiting_for_profile"}, events.types())
}

func TestJobMatchWorkerKeepsFailedRequestInQueue(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	found, err := repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	require.True(t, found)

	matcher := &recordingProfileJobMatcher{err: errors.New("model unavailable")}
	events := &jobMatchEventRecorder{}
	worker := NewJobMatchWorker(repository, repository, repository, repository, repository, &recordingJobAnalyzer{analysis: testJobAnalysis()}, matcher, events, zap.NewNop(), time.Minute)
	worked, err := worker.process(context.Background())
	require.True(t, worked)
	require.EqualError(t, err, "model unavailable")
	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, int64(1), queue[0].ID)
	assert.Equal(t, []string{"job_match.started", "job_match.analysis.completed", "job_match.failed"}, events.types())
}

func TestJobMatchWorkerRecordsAnalysisFailure(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	_, err = repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)

	events := &jobMatchEventRecorder{}
	worker := NewJobMatchWorker(repository, repository, repository, repository, repository, &recordingJobAnalyzer{err: errors.New("model unavailable")}, &recordingProfileJobMatcher{}, events, zap.NewNop(), time.Minute)
	worked, err := worker.process(context.Background())
	require.True(t, worked)
	require.EqualError(t, err, "model unavailable")
	assert.Equal(t, []string{"job_match.started", "job_match.analysis.failed", "job_match.failed"}, events.types())
}

func TestJobMatchRunInterval(t *testing.T) {
	runInterval := 2 * time.Minute
	interval, cooldown := jobMatchRunInterval(true, nil, runInterval)
	assert.Equal(t, runInterval, interval)
	assert.True(t, cooldown)

	interval, cooldown = jobMatchRunInterval(true, errors.New("match failed"), runInterval)
	assert.Equal(t, runInterval, interval)
	assert.True(t, cooldown)

	interval, cooldown = jobMatchRunInterval(false, errors.New("queue failed"), runInterval)
	assert.Equal(t, runInterval, interval)
	assert.True(t, cooldown)

	interval, cooldown = jobMatchRunInterval(false, nil, runInterval)
	assert.Equal(t, jobMatchRetryInterval, interval)
	assert.False(t, cooldown)
}

type recordingProfileJobMatcher struct {
	calls      int
	analyses   []models.JobAnalysisRecord
	assessment models.JobMatchAssessment
	err        error
}

func (matcher *recordingProfileJobMatcher) Match(_ context.Context, _ models.BrowseJob, analysis models.JobAnalysisRecord, _ models.UserProfile) (models.JobMatchAssessment, error) {
	matcher.calls++
	matcher.analyses = append(matcher.analyses, analysis)
	return matcher.assessment, matcher.err
}

func TestJobMatchWorkerReusesCurrentJobAnalysis(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "example", SourceID: "job-1", SourceURL: "https://example.com/jobs/1", Title: "Backend Engineer", BodyText: "Build APIs.", Workplace: "remote", MetadataJSON: `{"source":"test"}`,
	}}))
	_, err = repository.SaveUserProfile(context.Background(), models.UserProfile{Skills: []models.UserProfileSkill{}})
	require.NoError(t, err)
	analyzer := &recordingJobAnalyzer{analysis: testJobAnalysis()}
	events := &jobMatchEventRecorder{}
	worker := NewJobMatchWorker(repository, repository, repository, repository, repository, analyzer, &recordingProfileJobMatcher{assessment: models.JobMatchAssessment{MatcherVersion: "test", Score: 84, Summary: "assessment"}}, events, zap.NewNop(), time.Minute)

	_, err = repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)
	_, err = worker.process(context.Background())
	require.NoError(t, err)

	_, err = repository.QueueJobMatch(context.Background(), 1, true)
	require.NoError(t, err)
	_, err = worker.process(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, analyzer.calls)
	assert.Equal(t, "job-1", analyzer.jobs[0].SourceID)
	assert.Equal(t, `{"source":"test"}`, analyzer.jobs[0].MetadataJSON)
	assert.Equal(t, []string{"job_match.started", "job_match.analysis.completed", "job_match.completed", "job_match.started", "job_match.analysis.reused", "job_match.completed"}, events.types())
}

type recordingJobAnalyzer struct {
	calls    int
	jobs     []models.Job
	analysis JobAnalysis
	err      error
}

func (analyzer *recordingJobAnalyzer) Analyze(_ context.Context, job models.Job) (JobAnalysis, error) {
	analyzer.calls++
	analyzer.jobs = append(analyzer.jobs, job)
	analysis := analyzer.analysis
	if analysis.InputSHA256 == "" {
		analysis.InputSHA256 = jobAnalysisInputSHA256(job)
	}
	return analysis, analyzer.err
}

func testJobAnalysis() JobAnalysis {
	return JobAnalysis{
		AnalyzerVersion:       JobAnalyzerVersion,
		PromptVersion:         JobPromptVersion,
		Model:                 "test-model",
		AnalyzedAt:            time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		NormalizedDescription: "Build APIs.",
		Analysis: models.JobAnalysisDraft{
			Role:     models.JobRole{Family: "backend_engineering", Seniority: "unknown", SeniorityConfidence: "low"},
			Unknowns: []string{"The posting does not state requirements."},
		},
	}
}

type jobMatchEventRecorder struct {
	events []models.Event
}

func (recorder *jobMatchEventRecorder) RecordEvent(_ context.Context, event models.Event) error {
	recorder.events = append(recorder.events, event)
	return nil
}

func (recorder *jobMatchEventRecorder) types() []string {
	types := make([]string, 0, len(recorder.events))
	for _, event := range recorder.events {
		types = append(types, event.Type)
	}
	return types
}
