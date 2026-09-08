package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

var (
	ErrInvalidSearchField      = errors.New("fields must contain only title, company, location, or body")
	ErrInvalidMatchFilter      = errors.New("match must be all, has, or none")
	ErrEmptyJobRejectionReason = errors.New("a reason is required")
)

type JobBrowse struct {
	repository repositories.JobRepository
}

func NewJobBrowse(repository repositories.JobRepository) *JobBrowse {
	return &JobBrowse{repository: repository}
}

func (browse *JobBrowse) Jobs(ctx context.Context, search models.JobSearch) (models.JobPage, error) {
	fields, err := normalizeSearchFields(search.Fields)
	if err != nil {
		return models.JobPage{}, err
	}
	search.Search = strings.TrimSpace(search.Search)
	search.Provider = strings.TrimSpace(search.Provider)
	search.Match = strings.TrimSpace(strings.ToLower(search.Match))
	if search.Match == "all" {
		search.Match = ""
	}
	if search.Match != "" && search.Match != "has" && search.Match != "none" {
		return models.JobPage{}, ErrInvalidMatchFilter
	}
	search.Fields = fields
	return browse.repository.List(ctx, search)
}

func (browse *JobBrowse) Job(ctx context.Context, id int64) (*models.BrowseJob, error) {
	return browse.repository.Job(ctx, id)
}

func (browse *JobBrowse) DeleteJob(ctx context.Context, id int64) (bool, error) {
	return browse.repository.Delete(ctx, id)
}

func (browse *JobBrowse) RejectJob(ctx context.Context, id int64, reason string) (bool, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return false, ErrEmptyJobRejectionReason
	}
	return browse.repository.Reject(ctx, id, reason)
}

func (browse *JobBrowse) Providers(ctx context.Context) ([]string, error) {
	return browse.repository.Providers(ctx)
}

func (browse *JobBrowse) Companies(ctx context.Context, search string) ([]models.BrowseCompany, error) {
	return browse.repository.Companies(ctx, strings.TrimSpace(search))
}

func normalizeSearchFields(values []string) ([]string, error) {
	allowed := map[string]string{
		"title":    "jobs.title",
		"company":  "companies.name",
		"location": "jobs.location",
		"body":     "jobs.body_text",
	}
	if len(values) == 0 {
		return []string{"jobs.title", "companies.name", "jobs.location", "jobs.body_text"}, nil
	}

	fields := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		for _, name := range strings.Split(value, ",") {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			field, found := allowed[name]
			if !found {
				return nil, fmt.Errorf("%w: %s", ErrInvalidSearchField, name)
			}
			seen[name] = true
			fields = append(fields, field)
		}
	}
	return fields, nil
}
