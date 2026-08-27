package jobicy

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

	"nice/internal/config"
	"nice/internal/models"
)

const baseURL = "https://jobicy.com/api/v2/remote-jobs"

type Client struct {
	config  config.JobicyConfig
	http    *http.Client
	baseURL string
}

type response struct {
	Jobs []job `json:"jobs"`
}

type industriesResponse struct {
	Industries []industry `json:"industries"`
}

type industry struct {
	Name string `json:"industryName"`
	Slug string `json:"industrySlug"`
}

type job struct {
	ID              int64    `json:"id"`
	URL             string   `json:"url"`
	Title           string   `json:"jobTitle"`
	CompanyName     string   `json:"companyName"`
	CompanyLogo     string   `json:"companyLogo"`
	Industry        []string `json:"jobIndustry"`
	Type            []string `json:"jobType"`
	Geo             string   `json:"jobGeo"`
	Level           string   `json:"jobLevel"`
	Excerpt         string   `json:"jobExcerpt"`
	Description     string   `json:"jobDescription"`
	PublicationDate string   `json:"pubDate"`
	SalaryMin       *int64   `json:"salaryMin"`
	SalaryMax       *int64   `json:"salaryMax"`
	SalaryCurrency  string   `json:"salaryCurrency"`
	SalaryPeriod    string   `json:"salaryPeriod"`
}

func NewClient(cfg config.Config) *Client {
	return newClient(&http.Client{Timeout: 30 * time.Second}, baseURL, cfg.Providers.Jobicy)
}

func newClient(httpClient *http.Client, baseURL string, cfg config.JobicyConfig) *Client {
	return &Client{config: cfg, http: httpClient, baseURL: baseURL}
}

func (c *Client) Fetch(ctx context.Context) ([]models.Job, error) {
	industryName, err := c.industryName(ctx)
	if err != nil {
		return nil, err
	}

	requestURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	params := requestURL.Query()
	params.Set("count", strconv.Itoa(c.config.Count))
	if c.config.Geo != "" {
		params.Set("geo", c.config.Geo)
	}
	if c.config.Industry != "" {
		params.Set("industry", c.config.Industry)
	}
	if c.config.Tag != "" {
		params.Set("tag", c.config.Tag)
	}
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
			return nil, fmt.Errorf("Jobicy returned %s", httpResponse.Status)
		}
		return nil, fmt.Errorf("Jobicy returned %s: %s", httpResponse.Status, strings.TrimSpace(string(body)))
	}

	var result response
	if err := json.NewDecoder(httpResponse.Body).Decode(&result); err != nil {
		return nil, err
	}
	jobs := make([]models.Job, 0, len(result.Jobs))
	for _, item := range result.Jobs {
		if industryName != "" && !matchesIndustry(item, industryName) {
			continue
		}
		jobs = append(jobs, toModel(item))
	}
	return jobs, nil
}

func (c *Client) industryName(ctx context.Context) (string, error) {
	if c.config.Industry == "" {
		return "", nil
	}

	requestURL, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	params := requestURL.Query()
	params.Set("get", "industries")
	requestURL.RawQuery = params.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	httpResponse, err := c.http.Do(request)
	if err != nil {
		return "", err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 4<<10))
		if readErr != nil {
			return "", fmt.Errorf("Jobicy returned %s", httpResponse.Status)
		}
		return "", fmt.Errorf("Jobicy returned %s: %s", httpResponse.Status, strings.TrimSpace(string(body)))
	}

	var result industriesResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&result); err != nil {
		return "", err
	}
	for _, industry := range result.Industries {
		if industry.Slug == c.config.Industry {
			return industry.Name, nil
		}
	}
	return "", fmt.Errorf("Jobicy industry %q was not found", c.config.Industry)
}

func matchesIndustry(job job, industry string) bool {
	for _, value := range job.Industry {
		if value == industry {
			return true
		}
	}
	return false
}

func toModel(job job) models.Job {
	metadata, _ := json.Marshal(map[string]any{
		"company_logo_url": job.CompanyLogo,
		"industry":         job.Industry,
		"level":            job.Level,
		"excerpt":          job.Excerpt,
		"salary_currency":  job.SalaryCurrency,
		"salary_period":    job.SalaryPeriod,
	})
	return models.Job{
		Source:         "jobicy",
		SourceID:       strconv.FormatInt(job.ID, 10),
		SourceURL:      job.URL,
		Title:          job.Title,
		BodyText:       job.Description,
		Company:        job.CompanyName,
		Location:       job.Geo,
		Workplace:      "remote",
		EmploymentType: strings.Join(job.Type, ", "),
		SalaryMin:      job.SalaryMin,
		SalaryMax:      job.SalaryMax,
		PostedAt:       parseTime(job.PublicationDate),
		MetadataJSON:   string(metadata),
	}
}

func parseTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}
