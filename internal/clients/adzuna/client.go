package adzuna

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nice/internal/config"
	"nice/internal/models"
)

const baseURL = "https://api.adzuna.com/v1/api"

type Client struct {
	config config.AdzunaConfig
	http   *http.Client
}

type response struct {
	Results []job `json:"results"`
}

type job struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Created           string `json:"created"`
	RedirectURL       string `json:"redirect_url"`
	SalaryMin         *int64 `json:"salary_min"`
	SalaryMax         *int64 `json:"salary_max"`
	SalaryIsPredicted any    `json:"salary_is_predicted"`
	ContractTime      string `json:"contract_time"`
	ContractType      string `json:"contract_type"`
	Company           struct {
		DisplayName string `json:"display_name"`
	} `json:"company"`
	Location struct {
		DisplayName string   `json:"display_name"`
		Area        []string `json:"area"`
	} `json:"location"`
	Category struct {
		Label string `json:"label"`
		Tag   string `json:"tag"`
	} `json:"category"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		config: cfg.Providers.Adzuna,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Fetch(ctx context.Context, settings models.AdzunaSearchSettings) ([]models.Job, error) {
	var jobs []models.Job
	for page := 1; page <= settings.MaxPages; page++ {
		result, err := c.fetchPage(ctx, page, settings)
		if err != nil {
			return nil, err
		}

		for _, item := range result {
			if matchesWorkplace(item, settings.Workplace) {
				model, err := toModel(item)
				if err != nil {
					return nil, fmt.Errorf("encode Adzuna job metadata: %w", err)
				}

				jobs = append(jobs, model)
			}
		}

		if len(result) < settings.ResultsPerPage {
			break
		}
	}

	return jobs, nil
}

func (c *Client) fetchPage(ctx context.Context, page int, settings models.AdzunaSearchSettings) ([]job, error) {
	endpoint := fmt.Sprintf("%s/jobs/%s/search/%d", baseURL, url.PathEscape(settings.Country), page)
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	params := requestURL.Query()
	params.Set("app_id", c.config.AppID)
	params.Set("app_key", c.config.APIKey)
	params.Set("what", settings.Query)
	params.Set("max_days_old", fmt.Sprint(settings.MaxDaysOld))
	params.Set("results_per_page", fmt.Sprint(settings.ResultsPerPage))
	params.Set("content-type", "application/json")
	requestURL.RawQuery = params.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/json")
	httpResponse, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 4<<10))
		if readErr != nil {
			return nil, fmt.Errorf("adzuna returned %s", httpResponse.Status)
		}

		return nil, fmt.Errorf("adzuna returned %s: %s", httpResponse.Status, strings.TrimSpace(string(body)))
	}

	var result response
	if err := json.NewDecoder(httpResponse.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func matchesWorkplace(job job, workplace string) bool {
	if workplace == "any" {
		return true
	}

	text := strings.ToLower(strings.Join([]string{job.Title, job.Description, job.Location.DisplayName}, " "))
	if workplace == "remote" {
		return strings.Contains(text, "remote")
	}

	return strings.Contains(text, "remote") || strings.Contains(text, "hybrid")
}

func toModel(job job) (models.Job, error) {
	metadata, err := json.Marshal(map[string]any{
		"category":            job.Category,
		"location_area":       job.Location.Area,
		"salary_is_predicted": salaryIsPredicted(job.SalaryIsPredicted),
		"contract_type":       job.ContractType,
	})
	if err != nil {
		return models.Job{}, err
	}

	return models.Job{
		Source:         "adzuna",
		SourceID:       job.ID,
		SourceURL:      job.RedirectURL,
		Title:          job.Title,
		BodyText:       job.Description,
		Company:        strings.TrimSpace(job.Company.DisplayName),
		Location:       job.Location.DisplayName,
		Workplace:      workplace(job),
		EmploymentType: job.ContractTime,
		SalaryMin:      job.SalaryMin,
		SalaryMax:      job.SalaryMax,
		PostedAt:       parseTime(job.Created),
		MetadataJSON:   string(metadata),
	}, nil
}

func salaryIsPredicted(value any) bool {
	switch value := value.(type) {
	case float64:
		return value == 1
	case string:
		return value == "1"
	default:
		return false
	}
}

func workplace(job job) string {
	text := strings.ToLower(strings.Join([]string{job.Title, job.Description, job.Location.DisplayName}, " "))
	if strings.Contains(text, "remote") {
		return "remote"
	}

	if strings.Contains(text, "hybrid") {
		return "hybrid"
	}

	return "unknown"
}

func parseTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}

	return parsed.UTC().Format(time.RFC3339)
}
