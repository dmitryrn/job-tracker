package remotive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"nice/internal/models"
)

const baseURL = "https://remotive.com/api/remote-jobs"

type Client struct {
	http    *http.Client
	baseURL string
}

type response struct {
	Jobs []job `json:"jobs"`
}

type job struct {
	ID                        int64    `json:"id"`
	URL                       string   `json:"url"`
	Title                     string   `json:"title"`
	CompanyName               string   `json:"company_name"`
	Category                  string   `json:"category"`
	Tags                      []string `json:"tags"`
	JobType                   string   `json:"job_type"`
	PublicationDate           string   `json:"publication_date"`
	CandidateRequiredLocation string   `json:"candidate_required_location"`
	Salary                    string   `json:"salary"`
	Description               string   `json:"description"`
	CompanyLogoURL            string   `json:"company_logo_url"`
}

func NewClient() *Client {
	return newClient(&http.Client{Timeout: 30 * time.Second}, baseURL)
}

func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{http: httpClient, baseURL: baseURL}
}

func (c *Client) Fetch(ctx context.Context, settings models.RemotiveSearchSettings) ([]models.Job, error) {
	requestURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	params := requestURL.Query()
	params.Set("search", settings.Query)
	params.Set("category", settings.Category)
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
			return nil, fmt.Errorf("remotive returned %s", httpResponse.Status)
		}

		return nil, fmt.Errorf("remotive returned %s: %s", httpResponse.Status, string(body))
	}

	var result response
	if err := json.NewDecoder(httpResponse.Body).Decode(&result); err != nil {
		return nil, err
	}

	jobs := make([]models.Job, 0, len(result.Jobs))
	for _, item := range result.Jobs {
		if !matchesCategory(item.Category, settings.Category) {
			continue
		}

		model, err := toModel(item)
		if err != nil {
			return nil, fmt.Errorf("encode Remotive job metadata: %w", err)
		}

		jobs = append(jobs, model)
	}

	return jobs, nil
}

func matchesCategory(category, filter string) bool {
	return strings.EqualFold(category, filter) || strings.EqualFold(category, strings.ReplaceAll(filter, "-", " "))
}

func toModel(job job) (models.Job, error) {
	metadata, err := json.Marshal(map[string]any{
		"category":         job.Category,
		"tags":             job.Tags,
		"salary":           job.Salary,
		"company_logo_url": job.CompanyLogoURL,
	})
	if err != nil {
		return models.Job{}, err
	}

	return models.Job{
		Source:         "remotive",
		SourceID:       strconv.FormatInt(job.ID, 10),
		SourceURL:      job.URL,
		Title:          job.Title,
		BodyText:       job.Description,
		Company:        job.CompanyName,
		Location:       job.CandidateRequiredLocation,
		Workplace:      "remote",
		EmploymentType: job.JobType,
		PostedAt:       parseTime(job.PublicationDate),
		MetadataJSON:   string(metadata),
	}, nil
}

func parseTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		parsed, err = time.Parse("2006-01-02T15:04:05", value)
	}

	if err != nil {
		return ""
	}

	return parsed.UTC().Format(time.RFC3339)
}
