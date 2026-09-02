package services

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"nice/internal/clients/linkedin"
	"nice/internal/models"
)

const linkedInPageSize = 25

type linkedInClient interface {
	Search(context.Context, linkedin.SearchFilter) ([]linkedin.SearchResult, error)
	Job(context.Context, string) (linkedin.Job, error)
}

type LinkedInJobs struct {
	client linkedInClient
}

func NewLinkedInJobs(client *linkedin.Client) *LinkedInJobs {
	return &LinkedInJobs{client: client}
}

func (service *LinkedInJobs) Fetch(ctx context.Context, settings models.LinkedInSearchSettings) ([]models.Job, error) {
	jobs := make([]models.Job, 0, settings.Limit)
	seen := make(map[string]struct{}, settings.Limit)
	for start := 0; len(jobs) < settings.Limit; start += linkedInPageSize {
		results, err := service.client.Search(ctx, linkedin.SearchFilter{
			Keywords: settings.Query,
			Location: settings.Location,
			Start:    start,
		})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			break
		}
		for _, result := range results {
			if len(jobs) == settings.Limit {
				break
			}
			if _, exists := seen[result.ID]; exists {
				continue
			}
			seen[result.ID] = struct{}{}
			details, err := service.client.Job(ctx, result.ID)
			if err != nil {
				return nil, err
			}
			jobs = append(jobs, toLinkedInJob(result, details))
		}
		if len(results) < linkedInPageSize {
			break
		}
	}
	return jobs, nil
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
