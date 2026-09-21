package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"nice/internal/clients/linkedin"
	"nice/internal/clients/typesafe"
	"nice/internal/models"
	"nice/internal/repositories"
)

type linkedInClient interface {
	Search(context.Context, linkedin.SearchFilter) ([]linkedin.SearchResult, error)
	Job(context.Context, string) (linkedin.Job, error)
}

type linkedInJobRepository interface {
	JobExists(context.Context, string, string) (bool, error)
	JobBySourceID(context.Context, string, string) (*models.Job, error)
	Upsert(context.Context, []models.Job) error
}

type LinkedInJobs struct {
	client   linkedInClient
	jobs     linkedInJobRepository
	events   repositories.EventRecorder
	logger   *zap.Logger
	typeSafe TypeSafeSystemOneClient
}

type LinkedInFetchResult struct {
	Jobs           []models.Job
	SearchResults  int
	DetailRequests int
	FetchedJobs    int
	SavedJobs      int
	SkippedJobs    int
	InvalidJobs    int
}

type linkedInFetchOptions struct {
	runID           string
	requestInterval time.Duration
	save            bool
	onJob           func(models.Job) error
}

type linkedInClientError struct {
	cause error
}

func (err *linkedInClientError) Error() string {
	return err.cause.Error()
}

func (err *linkedInClientError) Unwrap() error {
	return err.cause
}

func NewLinkedInJobs(client *linkedin.Client, jobs repositories.JobRepository, events repositories.EventRecorder, logger *zap.Logger, typeSafe *typesafe.Client) *LinkedInJobs {
	return &LinkedInJobs{
		client:   client,
		jobs:     jobs,
		events:   events,
		logger:   logger,
		typeSafe: typeSafe,
	}
}

func (service *LinkedInJobs) Preview(ctx context.Context, settings models.LinkedInSearchSettings, requestInterval time.Duration) (LinkedInFetchResult, error) {
	fetch, err := service.fetch(ctx, settings, linkedInFetchOptions{
		requestInterval: requestInterval,
	})
	return service.previewResult(fetch, err)
}

func (service *LinkedInJobs) PreviewStream(ctx context.Context, settings models.LinkedInSearchSettings, requestInterval time.Duration, onJob func(models.Job) error) (LinkedInFetchResult, error) {
	fetch, err := service.fetch(ctx, settings, linkedInFetchOptions{
		requestInterval: requestInterval,
		onJob:           onJob,
	})
	return service.previewResult(fetch, err)
}

func (service *LinkedInJobs) previewResult(fetch LinkedInFetchResult, err error) (LinkedInFetchResult, error) {
	var clientErr *linkedInClientError
	if len(fetch.Jobs) > 0 && errors.As(err, &clientErr) {
		if service.logger != nil {
			service.logger.Warn("LinkedIn preview incomplete; returning fetched jobs", zap.Int("job_count", len(fetch.Jobs)), zap.Error(err))
		}

		return fetch, nil
	}

	return fetch, err
}

func (service *LinkedInJobs) Sync(ctx context.Context, settings models.LinkedInSearchSettings, runID string, requestInterval time.Duration) (LinkedInFetchResult, error) {
	return service.fetch(ctx, settings, linkedInFetchOptions{
		runID:           runID,
		requestInterval: requestInterval,
		save:            true,
	})
}

func (service *LinkedInJobs) fetch(ctx context.Context, settings models.LinkedInSearchSettings, options linkedInFetchOptions) (LinkedInFetchResult, error) {
	runID := options.runID
	requestInterval := options.requestInterval
	save := options.save
	onJob := options.onJob
	fetch := LinkedInFetchResult{Jobs: make([]models.Job, 0, settings.Limit)}
	seen := make(map[string]struct{}, settings.Limit)
	requested := false
	for start := 0; len(fetch.Jobs) < settings.Limit; {
		if requested {
			if err := service.wait(ctx, requestInterval); err != nil {
				return fetch, err
			}
		}

		requested = true
		service.recordEvent(ctx, runID, "linkedin.search.started", "info", "LinkedIn search started", map[string]any{
			"query": settings.Query, "location": settings.Location, "postedWithin": settings.PostedWithin, "workplace": settings.Workplace, "experienceLevel": settings.ExperienceLevel, "start": start, "requestedLimit": settings.Limit,
		})
		results, err := service.client.Search(ctx, linkedin.SearchFilter{
			Keywords:        settings.Query,
			Location:        settings.Location,
			PostedWithin:    settings.PostedWithin,
			Workplace:       settings.Workplace,
			ExperienceLevel: settings.ExperienceLevel,
			Start:           start,
		})
		if err != nil {
			service.recordEvent(ctx, runID, "linkedin.search.failed", "error", "LinkedIn search failed", map[string]any{
				"start": start, "error": err.Error(),
			})
			return fetch, &linkedInClientError{cause: err}
		}

		fetch.SearchResults += len(results)
		service.recordEvent(ctx, runID, "linkedin.search.succeeded", "info", "LinkedIn search succeeded", map[string]any{
			"start": start, "resultCount": len(results), "searchResultsFetched": fetch.SearchResults, "requestedLimit": settings.Limit,
		})
		if len(results) == 0 {
			break
		}

		for _, candidate := range results {
			if len(fetch.Jobs) == settings.Limit {
				break
			}

			if !validLinkedInSearchResult(candidate) {
				fetch.InvalidJobs++
				continue
			}

			if _, exists := seen[candidate.ID]; exists {
				continue
			}

			seen[candidate.ID] = struct{}{}
			exists, err := service.jobs.JobExists(ctx, "linkedin", candidate.ID)
			if err != nil {
				service.recordEvent(ctx, runID, "linkedin.job_lookup.failed", "error", "LinkedIn job lookup failed", map[string]any{
					"jobID": candidate.ID, "error": err.Error(),
				})
				if service.logger != nil {
					service.logger.Error("check LinkedIn job existence failed", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.Error(err))
				}

				return fetch, err
			}

			if exists {
				if service.typeSafe != nil {
					service.classifyExistingJob(ctx, runID, candidate)
				}

				fetch.SkippedJobs++
				service.recordEvent(ctx, runID, "linkedin.job_fetch.skipped", "info", "LinkedIn job already exists", map[string]any{
					"jobID": candidate.ID, "reason": "already_exists",
				})
				continue
			}

			if err := service.wait(ctx, requestInterval); err != nil {
				return fetch, err
			}

			fetch.DetailRequests++
			service.recordEvent(ctx, runID, "linkedin.job_fetch.started", "info", "LinkedIn job fetch started", map[string]any{
				"jobID": candidate.ID, "title": candidate.Title, "detailRequests": fetch.DetailRequests,
			})
			details, err := service.client.Job(ctx, candidate.ID)
			if err != nil {
				service.recordEvent(ctx, runID, "linkedin.job_fetch.failed", "error", "LinkedIn job fetch failed", map[string]any{
					"jobID": candidate.ID, "error": err.Error(),
				})
				return fetch, &linkedInClientError{cause: err}
			}

			if !validLinkedInJob(details) {
				fetch.InvalidJobs++
				service.recordEvent(ctx, runID, "linkedin.job_fetch.succeeded", "info", "LinkedIn job fetch succeeded but listing was incomplete", map[string]any{
					"jobID": candidate.ID, "accepted": false, "reason": "missing_description",
				})
				continue
			}

			job := toLinkedInJob(candidate, details)
			if !validLinkedInResult(job) {
				fetch.InvalidJobs++
				service.recordEvent(ctx, runID, "linkedin.job_fetch.succeeded", "info", "LinkedIn job fetch succeeded but listing was incomplete", map[string]any{
					"jobID": candidate.ID, "accepted": false, "reason": "missing_posted_at_or_employment_type",
				})
				continue
			}

			if service.typeSafe != nil {
				workplace, classificationJSON, err := service.classifyWorkplace(ctx, candidate.Title, details.Description)
				if err != nil {
					service.recordEvent(ctx, runID, "linkedin.workplace_classification.failed", "error", "LinkedIn workplace classification failed", map[string]any{
						"jobID": candidate.ID, "error": err.Error(),
					})
					if service.logger != nil {
						service.logger.Error("classify LinkedIn workplace failed", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.Error(err))
					}
				} else {
					job.Workplace = workplace
					job.WorkplaceClassificationJSON = classificationJSON
					service.recordEvent(ctx, runID, "linkedin.workplace_classification.succeeded", "info", "LinkedIn workplace classified", map[string]any{
						"jobID": candidate.ID, "workplace": workplace,
					})
					if service.logger != nil {
						service.logger.Info("LinkedIn workplace classified", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.String("workplace", workplace))
					}
				}
			}

			fetch.FetchedJobs++
			service.recordEvent(ctx, runID, "linkedin.job_fetch.succeeded", "info", "LinkedIn job fetch succeeded", map[string]any{
				"jobID": candidate.ID, "accepted": true, "fetchedJobCount": fetch.FetchedJobs,
			})
			if save {
				if err := service.jobs.Upsert(ctx, []models.Job{job}); err != nil {
					service.recordEvent(ctx, runID, "linkedin.job_save.failed", "error", "LinkedIn job save failed", map[string]any{
						"jobID": candidate.ID, "error": err.Error(),
					})
					if service.logger != nil {
						service.logger.Error("save LinkedIn job failed", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.Error(err))
					}

					return fetch, err
				}

				fetch.SavedJobs++
				service.recordEvent(ctx, runID, "linkedin.job_save.succeeded", "info", "LinkedIn job saved", map[string]any{
					"jobID": candidate.ID, "savedJobCount": fetch.SavedJobs,
				})
			}

			fetch.Jobs = append(fetch.Jobs, job)
			if onJob != nil {
				if err := onJob(job); err != nil {
					return fetch, err
				}
			}
		}

		start += len(results)
	}

	return fetch, nil
}

func (service *LinkedInJobs) classifyExistingJob(ctx context.Context, runID string, candidate linkedin.SearchResult) {
	existing, err := service.jobs.JobBySourceID(ctx, "linkedin", candidate.ID)
	if err != nil {
		service.recordEvent(ctx, runID, "linkedin.workplace_classification.failed", "error", "LinkedIn stored workplace lookup failed", map[string]any{
			"jobID": candidate.ID, "error": err.Error(),
		})
		if service.logger != nil {
			service.logger.Error("look up stored LinkedIn job for workplace classification failed", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.Error(err))
		}

		return
	}

	if existing == nil || existing.WorkplaceClassificationJSON != nil {
		return
	}

	workplace, classificationJSON, err := service.classifyWorkplace(ctx, existing.Title, existing.BodyText)
	if err != nil {
		service.recordEvent(ctx, runID, "linkedin.workplace_classification.failed", "error", "LinkedIn workplace classification failed", map[string]any{
			"jobID": candidate.ID, "error": err.Error(),
		})
		if service.logger != nil {
			service.logger.Error("classify stored LinkedIn workplace failed", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.Error(err))
		}

		return
	}

	existing.Workplace = workplace
	existing.WorkplaceClassificationJSON = classificationJSON
	if err := service.jobs.Upsert(ctx, []models.Job{*existing}); err != nil {
		service.recordEvent(ctx, runID, "linkedin.workplace_classification.failed", "error", "LinkedIn workplace classification save failed", map[string]any{
			"jobID": candidate.ID, "error": err.Error(),
		})
		if service.logger != nil {
			service.logger.Error("save stored LinkedIn workplace classification failed", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.Error(err))
		}

		return
	}

	service.recordEvent(ctx, runID, "linkedin.workplace_classification.succeeded", "info", "Stored LinkedIn workplace classified", map[string]any{
		"jobID": candidate.ID, "workplace": workplace,
	})
	if service.logger != nil {
		service.logger.Info("stored LinkedIn workplace classified", zap.String("run_id", runID), zap.String("job_id", candidate.ID), zap.String("workplace", workplace))
	}
}

func (service *LinkedInJobs) recordEvent(ctx context.Context, runID, eventType, level, message string, data map[string]any) {
	if runID == "" || service.events == nil {
		return
	}

	if err := service.events.RecordEvent(ctx, models.Event{Provider: "linkedin", RunID: runID, Type: eventType, Level: level, Message: message, Data: data}); err != nil && service.logger != nil {
		service.logger.Error("record LinkedIn event failed", zap.String("run_id", runID), zap.String("event_type", eventType), zap.Error(err))
	}
}

func (service *LinkedInJobs) wait(ctx context.Context, requestInterval time.Duration) error {
	if requestInterval <= 0 {
		return nil
	}

	timer := time.NewTimer(requestInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validLinkedInSearchResult(result linkedin.SearchResult) bool {
	return strings.TrimSpace(result.ID) != "" &&
		strings.TrimSpace(result.URL) != "" &&
		strings.TrimSpace(result.Title) != "" &&
		strings.TrimSpace(result.Company) != "" &&
		strings.TrimSpace(result.Location) != "" &&
		strings.TrimSpace(result.PostedAt) != ""
}

func validLinkedInJob(details linkedin.Job) bool {
	return strings.TrimSpace(details.Description) != ""
}

func validLinkedInResult(job models.Job) bool {
	return strings.TrimSpace(job.PostedAt) != "" && strings.TrimSpace(job.EmploymentType) != ""
}

func toLinkedInJob(result linkedin.SearchResult, details linkedin.Job) models.Job {
	metadata, _ := json.Marshal(map[string]string{"posted_text": result.PostedAt})
	workplace := strings.ToLower(strings.TrimSpace(details.WorkplaceType))
	workplace = strings.ReplaceAll(workplace, "-", "")
	workplace = strings.ReplaceAll(workplace, " ", "")
	if workplace == "" {
		workplace = "unknown"
	}

	return models.Job{
		Source:         "linkedin",
		SourceID:       result.ID,
		SourceURL:      result.URL,
		Title:          result.Title,
		BodyText:       details.Description,
		Company:        result.Company,
		Location:       result.Location,
		Workplace:      workplace,
		EmploymentType: details.EmploymentType,
		PostedAt:       linkedInPostedAt(result.PostedAt),
		MetadataJSON:   string(metadata),
	}
}

const linkedInWorkplaceQuestionID = "workplace"

var linkedInWorkplaceOptions = map[string]string{
	"remote":  "The role is fully remote or does not require regular in-person work.",
	"hybrid":  "The role intentionally combines remote work with regular in-person work.",
	"onsite":  "The role is primarily or obligatorily performed at a physical workplace.",
	"unknown": "The available job context does not provide enough reliable evidence to determine the work arrangement.",
}

type linkedInWorkplaceState struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type linkedInWorkplaceClassification struct {
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (service *LinkedInJobs) classifyWorkplace(ctx context.Context, title, description string) (string, *string, error) {
	response, err := service.typeSafe.SystemOne(ctx, linkedInWorkplaceState{
		Title:       title,
		Description: description,
	}, map[string]typesafe.Question{
		linkedInWorkplaceQuestionID: {
			Type:         "choice",
			Instructions: "What work arrangement best describes this LinkedIn job? Use only explicit evidence in the job context and do not infer an arrangement from generic claims about flexibility.",
			Criteria:     linkedInWorkplaceOptions,
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("call TypeSafe workplace choice: %w", err)
	}

	answer, found := response.Answers[linkedInWorkplaceQuestionID]
	if !found || answer.Type != "choice" {
		return "", nil, fmt.Errorf("TypeSafe response did not contain a workplace choice")
	}

	if _, valid := linkedInWorkplaceOptions[answer.Choice]; !valid {
		return "", nil, fmt.Errorf("TypeSafe workplace choice %q is not supported", answer.Choice)
	}

	classificationJSON, err := json.Marshal(linkedInWorkplaceClassification{
		Confidence:    answer.Confidence,
		Probabilities: answer.Probabilities,
	})
	if err != nil {
		return "", nil, fmt.Errorf("encode TypeSafe workplace classification: %w", err)
	}

	classification := string(classificationJSON)
	return answer.Choice, &classification, nil
}

func linkedInPostedAt(value string) string {
	parsed, err := time.Parse(time.DateOnly, value)
	if err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}

	parsed, err = time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}

	return parsed.UTC().Format(time.RFC3339)
}
