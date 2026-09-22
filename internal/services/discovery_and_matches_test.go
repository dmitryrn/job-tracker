package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/models"
)

type discoverySettingsRepositoryStub struct {
	saved models.DiscoverySettings
	err   error
	calls int
}

type jobMatchRepositoryStub struct {
	search models.JobMatchSearch
	page   models.JobMatchPage
	err    error
	calls  int
}

func TestDiscoverySettingsSaveTrimsValuesBeforePersisting(t *testing.T) {
	repository := &discoverySettingsRepositoryStub{}
	service := NewDiscoverySettingsService(repository)
	settings := validDiscoverySettings()
	settings.Adzuna.Query = "  backend  "
	settings.Adzuna.Country = " de "
	settings.Adzuna.Workplace = " remote "
	settings.Remotive.Query = " go "
	settings.Remotive.Category = " backend "
	settings.Jobicy.Geo = " europe "
	settings.Jobicy.Industry = " tech "
	settings.Jobicy.Tag = " go "
	settings.LinkedIn.Query = " engineer "
	settings.LinkedIn.Location = " Berlin "
	settings.LinkedIn.PostedWithin = " r86400 "
	settings.LinkedIn.Workplace = " 1 "
	settings.LinkedIn.ExperienceLevel = " 4 "

	saved, err := service.Save(context.Background(), settings)

	require.NoError(t, err)
	assert.Equal(t, repository.saved, saved)
	assert.Equal(t, "backend", saved.Adzuna.Query)
	assert.Equal(t, "de", saved.Adzuna.Country)
	assert.Equal(t, "remote", saved.Adzuna.Workplace)
	assert.Equal(t, "go", saved.Remotive.Query)
	assert.Equal(t, "backend", saved.Remotive.Category)
	assert.Equal(t, "europe", saved.Jobicy.Geo)
	assert.Equal(t, "tech", saved.Jobicy.Industry)
	assert.Equal(t, "go", saved.Jobicy.Tag)
	assert.Equal(t, "engineer", saved.LinkedIn.Query)
	assert.Equal(t, "Berlin", saved.LinkedIn.Location)
	assert.Equal(t, "r86400", saved.LinkedIn.PostedWithin)
	assert.Equal(t, "1", saved.LinkedIn.Workplace)
	assert.Equal(t, "4", saved.LinkedIn.ExperienceLevel)
	assert.Equal(t, 1, repository.calls)
}

func TestDiscoverySettingsSaveRejectsInvalidValuesWithoutPersisting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*models.DiscoverySettings)
	}{
		{name: "adzuna query", mutate: func(settings *models.DiscoverySettings) { settings.Adzuna.Query = "" }},
		{name: "adzuna country", mutate: func(settings *models.DiscoverySettings) { settings.Adzuna.Country = "" }},
		{name: "adzuna max days", mutate: func(settings *models.DiscoverySettings) { settings.Adzuna.MaxDaysOld = 0 }},
		{name: "adzuna max pages", mutate: func(settings *models.DiscoverySettings) { settings.Adzuna.MaxPages = 0 }},
		{name: "adzuna page size", mutate: func(settings *models.DiscoverySettings) { settings.Adzuna.ResultsPerPage = 0 }},
		{name: "adzuna workplace", mutate: func(settings *models.DiscoverySettings) { settings.Adzuna.Workplace = "onsite" }},
		{name: "remotive query", mutate: func(settings *models.DiscoverySettings) { settings.Remotive.Query = "" }},
		{name: "remotive category", mutate: func(settings *models.DiscoverySettings) { settings.Remotive.Category = "" }},
		{name: "jobicy count zero", mutate: func(settings *models.DiscoverySettings) { settings.Jobicy.Count = 0 }},
		{name: "jobicy count too large", mutate: func(settings *models.DiscoverySettings) { settings.Jobicy.Count = 201 }},
		{name: "linkedin query", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.Query = "" }},
		{name: "linkedin location", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.Location = "" }},
		{name: "linkedin limit zero", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.Limit = 0 }},
		{name: "linkedin limit too large", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.Limit = maxLinkedInResults + 1 }},
		{name: "linkedin posted within", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.PostedWithin = "r3600" }},
		{name: "linkedin workplace", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.Workplace = "4" }},
		{name: "linkedin experience", mutate: func(settings *models.DiscoverySettings) { settings.LinkedIn.ExperienceLevel = "7" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &discoverySettingsRepositoryStub{}
			settings := validDiscoverySettings()
			test.mutate(&settings)

			_, err := NewDiscoverySettingsService(repository).Save(context.Background(), settings)

			require.ErrorIs(t, err, ErrInvalidDiscoverySettings)
			assert.Zero(t, repository.calls)
		})
	}
}

func TestDiscoverySettingsSaveReturnsRepositoryError(t *testing.T) {
	repository := &discoverySettingsRepositoryStub{err: errors.New("settings unavailable")}

	_, err := NewDiscoverySettingsService(repository).Save(context.Background(), validDiscoverySettings())

	assert.EqualError(t, err, "settings unavailable")
}

func (repository *discoverySettingsRepositoryStub) DiscoverySettings(context.Context) (models.DiscoverySettings, error) {
	return repository.saved, repository.err
}

func (repository *discoverySettingsRepositoryStub) SaveDiscoverySettings(_ context.Context, settings models.DiscoverySettings) (models.DiscoverySettings, error) {
	repository.calls++
	repository.saved = settings
	return settings, repository.err
}

func TestJobMatchesListNormalizesFiltersBeforeQuerying(t *testing.T) {
	repository := &jobMatchRepositoryStub{page: models.JobMatchPage{Total: 1}}
	service := NewJobMatches(nil, repository)
	minimumScore := 70

	page, err := service.List(context.Background(), models.JobMatchSearch{
		MinimumScore: &minimumScore,
		Sort:         " SCORE-DESC ",
		Viewed:       " SEEN ",
		Applied:      " NOT-APPLIED ",
		Limit:        20,
		Offset:       5,
	})

	require.NoError(t, err)
	assert.Equal(t, repository.page, page)
	assert.Equal(t, models.JobMatchSearch{MinimumScore: &minimumScore, Sort: "score-desc", Viewed: "seen", Applied: "not-applied", Limit: 20, Offset: 5}, repository.search)
	assert.Equal(t, 1, repository.calls)
}

func TestJobMatchesListUsesDefaultFilters(t *testing.T) {
	repository := &jobMatchRepositoryStub{}

	_, err := NewJobMatches(nil, repository).List(context.Background(), models.JobMatchSearch{})

	require.NoError(t, err)
	assert.Equal(t, "created-desc", repository.search.Sort)
	assert.Equal(t, "all", repository.search.Viewed)
	assert.Equal(t, "all", repository.search.Applied)
}

func TestJobMatchesListRejectsInvalidFilters(t *testing.T) {
	tests := []struct {
		name   string
		search models.JobMatchSearch
		err    error
	}{
		{name: "sort", search: models.JobMatchSearch{Sort: "random"}, err: ErrInvalidJobMatchSort},
		{name: "viewed", search: models.JobMatchSearch{Viewed: "maybe"}, err: ErrInvalidJobMatchViewed},
		{name: "applied", search: models.JobMatchSearch{Applied: "maybe"}, err: ErrInvalidJobMatchApplied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &jobMatchRepositoryStub{}

			_, err := NewJobMatches(nil, repository).List(context.Background(), test.search)

			require.ErrorIs(t, err, test.err)
			assert.Zero(t, repository.calls)
		})
	}
}

func TestJobMatchesListReturnsRepositoryError(t *testing.T) {
	repository := &jobMatchRepositoryStub{err: errors.New("matches unavailable")}

	_, err := NewJobMatches(nil, repository).List(context.Background(), models.JobMatchSearch{})

	assert.EqualError(t, err, "matches unavailable")
}

func (repository *jobMatchRepositoryStub) JobMatchExists(context.Context, int64) (bool, error) {
	return false, nil
}

func (repository *jobMatchRepositoryStub) CreateJobMatch(context.Context, int64, string) error {
	return nil
}

func (repository *jobMatchRepositoryStub) JobMatch(context.Context, int64) (*models.JobMatchRecord, error) {
	return nil, errors.New("job match method not used")
}

func (repository *jobMatchRepositoryStub) JobMatches(_ context.Context, search models.JobMatchSearch) (models.JobMatchPage, error) {
	repository.calls++
	repository.search = search
	return repository.page, repository.err
}

func validDiscoverySettings() models.DiscoverySettings {
	return models.DiscoverySettings{
		Adzuna:   models.AdzunaSearchSettings{Query: "backend", Country: "de", MaxDaysOld: 7, MaxPages: 1, ResultsPerPage: 20, Workplace: "remote"},
		Remotive: models.RemotiveSearchSettings{Query: "go", Category: "software-dev"},
		Jobicy:   models.JobicySearchSettings{Count: 20},
		LinkedIn: models.LinkedInSearchSettings{Query: "engineer", Location: "Berlin", PostedWithin: "r86400", Workplace: "1", ExperienceLevel: "4", Limit: 20},
	}
}
