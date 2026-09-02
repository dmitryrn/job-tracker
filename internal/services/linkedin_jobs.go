package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"nice/internal/clients/linkedin"
	"nice/internal/config"
	"nice/internal/models"
)

const linkedInPageSize = 25

type linkedInClient interface {
	Search(context.Context, linkedin.SearchFilter) ([]linkedin.SearchResult, error)
	Job(context.Context, string) (linkedin.Job, error)
}

type LinkedInJobs struct {
	client          linkedInClient
	requestInterval time.Duration
}

func NewLinkedInJobs(cfg config.Config, client *linkedin.Client) *LinkedInJobs {
	return &LinkedInJobs{
		client:          client,
		requestInterval: cfg.Providers.LinkedIn.RequestInterval,
	}
}

func (service *LinkedInJobs) Fetch(ctx context.Context, settings models.LinkedInSearchSettings) ([]models.Job, error) {
	jobs := make([]models.Job, 0, settings.Limit)
	seen := make(map[string]struct{}, settings.Limit)
	requested := false
	for start := 0; len(jobs) < settings.Limit; start += linkedInPageSize {
		if requested {
			if err := service.wait(ctx); err != nil {
				return jobs, err
			}
		}
		requested = true
		results, err := service.client.Search(ctx, linkedin.SearchFilter{
			Keywords: settings.Query,
			Location: settings.Location,
			Start:    start,
		})
		if err != nil {
			return jobs, err
		}
		if len(results) == 0 {
			break
		}
		for _, result := range results {
			if len(jobs) == settings.Limit {
				break
			}
			if !validLinkedInSearchResult(result) {
				continue
			}
			if _, exists := seen[result.ID]; exists {
				continue
			}
			seen[result.ID] = struct{}{}
			if err := service.wait(ctx); err != nil {
				return jobs, err
			}
			details, err := service.client.Job(ctx, result.ID)
			if err != nil {
				return jobs, err
			}
			if !validLinkedInJob(details) {
				continue
			}
			jobs = append(jobs, toLinkedInJob(result, details))
		}
		if len(results) < linkedInPageSize {
			break
		}
	}
	return jobs, nil
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
