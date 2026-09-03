package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"

	"nice/internal/clients/linkedin"
	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/repositories"
)

const linkedInPageSize = 25

type linkedInClient interface {
	Search(context.Context, linkedin.SearchFilter) ([]linkedin.SearchResult, error)
	Job(context.Context, string) (linkedin.Job, error)
}

type LinkedInJobs struct {
	client          linkedInClient
	requestInterval time.Duration
	events          repositories.EventRecorder
	logger          *zap.Logger
}

type LinkedInFetchResult struct {
	Jobs           []models.Job
	SearchResults  int
	DetailRequests int
	FetchedJobs    int
	SavedJobs      int
}

type linkedInJobSaver func(context.Context, models.Job) error

func NewLinkedInJobs(cfg config.Config, client *linkedin.Client, events repositories.EventRecorder, logger *zap.Logger) *LinkedInJobs {
	return &LinkedInJobs{
		client:          client,
		requestInterval: cfg.Providers.LinkedIn.RequestInterval,
		events:          events,
		logger:          logger,
	}
}

func (service *LinkedInJobs) Fetch(ctx context.Context, settings models.LinkedInSearchSettings, runID string, save linkedInJobSaver) (LinkedInFetchResult, error) {
	fetch := LinkedInFetchResult{Jobs: make([]models.Job, 0, settings.Limit)}
	seen := make(map[string]struct{}, settings.Limit)
	requested := false
	for start := 0; len(fetch.Jobs) < settings.Limit; start += linkedInPageSize {
		if requested {
			if err := service.wait(ctx); err != nil {
				return fetch, err
			}
		}
		requested = true
		service.recordEvent(ctx, runID, "linkedin.search.started", "info", "LinkedIn search started", map[string]any{
			"query": settings.Query, "location": settings.Location, "start": start, "requestedLimit": settings.Limit,
		})
		results, err := service.client.Search(ctx, linkedin.SearchFilter{
			Keywords: settings.Query,
			Location: settings.Location,
			Start:    start,
		})
		if err != nil {
			service.recordEvent(ctx, runID, "linkedin.search.failed", "error", "LinkedIn search failed", map[string]any{
				"start": start, "error": err.Error(),
			})
			return fetch, err
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
				continue
			}
			if _, exists := seen[candidate.ID]; exists {
				continue
			}
			seen[candidate.ID] = struct{}{}
			if err := service.wait(ctx); err != nil {
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
				return fetch, err
			}
			if !validLinkedInJob(details) {
				service.recordEvent(ctx, runID, "linkedin.job_fetch.succeeded", "info", "LinkedIn job fetch succeeded but listing was incomplete", map[string]any{
					"jobID": candidate.ID, "accepted": false, "reason": "missing_description",
				})
				continue
			}
			job := toLinkedInJob(candidate, details)
			fetch.FetchedJobs++
			service.recordEvent(ctx, runID, "linkedin.job_fetch.succeeded", "info", "LinkedIn job fetch succeeded", map[string]any{
				"jobID": candidate.ID, "accepted": true, "fetchedJobCount": fetch.FetchedJobs,
			})
			if save != nil {
				if err := save(ctx, job); err != nil {
					service.recordEvent(ctx, runID, "linkedin.job_save.failed", "error", "LinkedIn job save failed", map[string]any{
						"jobID": candidate.ID, "error": err.Error(),
					})
					return fetch, err
				}
				fetch.SavedJobs++
				service.recordEvent(ctx, runID, "linkedin.job_save.succeeded", "info", "LinkedIn job saved", map[string]any{
					"jobID": candidate.ID, "savedJobCount": fetch.SavedJobs,
				})
			}
			fetch.Jobs = append(fetch.Jobs, job)
		}
		if len(results) < linkedInPageSize {
			break
		}
	}
	return fetch, nil
}

func (service *LinkedInJobs) recordEvent(ctx context.Context, runID, eventType, level, message string, data map[string]any) {
	if runID == "" || service.events == nil {
		return
	}
	if err := service.events.RecordEvent(ctx, models.Event{Provider: "linkedin", RunID: runID, Type: eventType, Level: level, Message: message, Data: data}); err != nil && service.logger != nil {
		service.logger.Error("record LinkedIn event failed", zap.String("run_id", runID), zap.String("event_type", eventType), zap.Error(err))
	}
}

func (service *LinkedInJobs) wait(ctx context.Context) error {
	if service.requestInterval <= 0 {
		return nil
	}
	timer := time.NewTimer(service.requestInterval)
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
		strings.TrimSpace(result.Location) != ""
}

func validLinkedInJob(details linkedin.Job) bool {
	return strings.TrimSpace(details.Description) != ""
}

func toLinkedInJob(result linkedin.SearchResult, details linkedin.Job) models.Job {
	metadata, _ := json.Marshal(map[string]string{"posted_text": result.PostedAt})
	return models.Job{
		Source:       "linkedin",
		SourceID:     result.ID,
		SourceURL:    result.URL,
		Title:        result.Title,
		BodyText:     details.Description,
		Company:      result.Company,
		Location:     result.Location,
		Workplace:    linkedInWorkplace(result.Title, details.Description, result.Location),
		PostedAt:     linkedInPostedAt(result.PostedAt),
		MetadataJSON: string(metadata),
	}
}

func linkedInPostedAt(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func linkedInWorkplace(values ...string) string {
	text := strings.ToLower(strings.Join(values, " "))
	if strings.Contains(text, "remote") {
		return "remote"
	}
	if strings.Contains(text, "hybrid") {
		return "hybrid"
	}
	return "unknown"
}
