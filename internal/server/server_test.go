package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/config"
	"nice/internal/migrations"
	"nice/internal/models"
	"nice/internal/repositories"
	"nice/internal/services"
)

func TestJobAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "remotive", SourceID: "body", SourceURL: "https://example.com/body", Title: "Designer", BodyText: "Searchable description", Company: "Studio North", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "adzuna", SourceID: "title", SourceURL: "https://example.com/title", Title: "Searchable title", BodyText: "Other text", Company: "Other Co", Workplace: "remote", MetadataJSON: "{}"},
	}))

	handler := newTestServer(repository).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?search=searchable&fields=body")
	require.Equal(t, http.StatusOK, response.Code)
	var jobs struct {
		Jobs []models.BrowseJob `json:"jobs"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&jobs))
	require.Len(t, jobs.Jobs, 1)
	assert.Equal(t, "Designer", jobs.Jobs[0].Title)
	assert.False(t, jobs.Jobs[0].HasMatch)
	require.NoError(t, repository.CreateJobMatch(context.Background(), jobs.Jobs[0].ID, "match"))

	response = request(handler, http.MethodGet, "/api/jobs?search=searchable&fields=body")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&jobs))
	require.Len(t, jobs.Jobs, 1)
	assert.True(t, jobs.Jobs[0].HasMatch)

	response = request(handler, http.MethodGet, "/api/providers")
	require.Equal(t, http.StatusOK, response.Code)
	var providers struct {
		Providers []string `json:"providers"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&providers))
	assert.Equal(t, []string{"adzuna", "remotive"}, providers.Providers)

	response = request(handler, http.MethodDelete, "/api/jobs/1")
	assert.Equal(t, http.StatusNoContent, response.Code)
	response = request(handler, http.MethodDelete, "/api/jobs/1")
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestJobAPIRejectsUnknownSearchField(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))
	handler := newTestServer(repositories.NewSQLite(db)).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs?fields=invalid")
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestDiscoverySettingsAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	handler := newTestServer(repositories.NewSQLite(db)).http.Handler
	response := request(handler, http.MethodGet, "/api/discovery-settings")
	require.Equal(t, http.StatusOK, response.Code)
	var result struct {
		Settings models.DiscoverySettings `json:"settings"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	assert.Equal(t, "software engineer", result.Settings.Adzuna.Query)
	assert.True(t, result.Settings.Adzuna.Enabled)
	assert.Equal(t, "software-development", result.Settings.Remotive.Category)

	response = requestWithBody(handler, http.MethodPut, "/api/discovery-settings", `{"adzuna":{"enabled":false,"query":"platform engineer","country":"de","maxDaysOld":14,"maxPages":2,"resultsPerPage":25,"workplace":"remote"},"remotive":{"enabled":true,"query":"platform engineer","category":"software-development"},"jobicy":{"enabled":true,"count":25,"geo":"europe","industry":"engineering","tag":"golang"}}`)
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	assert.Equal(t, "platform engineer", result.Settings.Adzuna.Query)
	assert.False(t, result.Settings.Adzuna.Enabled)
	assert.Equal(t, "golang", result.Settings.Jobicy.Tag)

	response = requestWithBody(handler, http.MethodPut, "/api/discovery-settings", `{"adzuna":{"enabled":false,"query":"","country":"de","maxDaysOld":14,"maxPages":2,"resultsPerPage":25,"workplace":"remote"},"remotive":{"enabled":true,"query":"platform engineer","category":"software-development"},"jobicy":{"enabled":true,"count":25,"geo":"europe","industry":"engineering","tag":"golang"}}`)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestDiscoveryPreviewAPIRejectsUnknownProvider(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	response := requestWithBody(newTestServer(repositories.NewSQLite(db)).http.Handler, http.MethodPost, "/api/discovery-preview/unknown", `{}`)
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.JSONEq(t, `{"error":"unknown discovery provider"}`, response.Body.String())
}

func TestDiscoveryPreviewAPIEncodesNoJobsAsArray(t *testing.T) {
	response := httptest.NewRecorder()
	discoveryPreviewHandler(discoveryPreviewStub{}, zap.NewNop()).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/discovery-preview/adzuna", strings.NewReader(`{}`)))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"jobs":[]}`, response.Body.String())
}

type discoveryPreviewStub struct{}

func (discoveryPreviewStub) Preview(context.Context, string, models.DiscoverySettings) ([]models.Job, error) {
	return nil, nil
}

func TestQueueUnmatchedJobsAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "example", SourceID: "matched", SourceURL: "https://example.com/matched", Title: "Matched", BodyText: "", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "example", SourceID: "unmatched", SourceURL: "https://example.com/unmatched", Title: "Unmatched", BodyText: "", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "example", SourceID: "queued", SourceURL: "https://example.com/queued", Title: "Queued", BodyText: "", Workplace: "remote", MetadataJSON: "{}"},
	}))
	require.NoError(t, repository.CreateJobMatch(context.Background(), 1, "Existing match"))
	_, err = repository.QueueJobMatch(context.Background(), 3, false)
	require.NoError(t, err)

	response := requestWithBody(newTestServer(repository).http.Handler, http.MethodPost, "/api/match-queue", `{"jobIds":[1,2,3]}`)
	require.Equal(t, http.StatusAccepted, response.Code)
	assert.JSONEq(t, `{"queued":1}`, response.Body.String())

	match, err := repository.JobMatch(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, match)
	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{2, 3}, []int64{queue[0].ID, queue[1].ID})
}

func TestRemoveMatchQueueItemAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "example", SourceID: "first", SourceURL: "https://example.com/first", Title: "First", BodyText: "", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "example", SourceID: "second", SourceURL: "https://example.com/second", Title: "Second", BodyText: "", Workplace: "remote", MetadataJSON: "{}"},
	}))
	_, err = repository.QueueJobMatch(context.Background(), 2, false)
	require.NoError(t, err)
	_, err = repository.QueueJobMatch(context.Background(), 1, false)
	require.NoError(t, err)

	response := request(newTestServer(repository).http.Handler, http.MethodDelete, "/api/match-queue/1")
	require.Equal(t, http.StatusNoContent, response.Code)
	queue, err := repository.MatchQueue(context.Background())
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, int64(2), queue[0].ID)
}

func TestProfileAndJobMatchAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{
		{Source: "remotive", SourceID: "job", SourceURL: "https://example.com/job", Title: "Engineer", BodyText: "Build services.", Workplace: "remote", MetadataJSON: "{}"},
		{Source: "remotive", SourceID: "job-2", SourceURL: "https://example.com/job-2", Title: "Designer", BodyText: "Design systems.", Workplace: "remote", MetadataJSON: "{}"},
	}))
	handler := newTestServer(repository).http.Handler

	response := request(handler, http.MethodGet, "/api/profile")
	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"profile":null}`, response.Body.String())

	response = requestWithBody(handler, http.MethodPut, "/api/profile", `{"headline":"Backend engineer","location":"Berlin","workAuthorization":"EU","summary":"APIs and systems","skills":[{"name":" Go ","level":"expert","notes":"Production services"},{"name":"","level":"","notes":""}],"workHistory":[{"company":" Acme ","title":"Engineer","startDate":"2020","endDate":"2022","body":"Built APIs"},{"company":"","title":"","startDate":"","endDate":"","body":""}],"education":[{"institution":" University ","degree":"BSc","startDate":"2016","endDate":"2020","body":"Computer science"},{"institution":"","degree":"","startDate":"","endDate":"","body":""}]}`)
	require.Equal(t, http.StatusOK, response.Code)
	var profileResponse struct {
		Profile models.UserProfile `json:"profile"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&profileResponse))
	assert.Equal(t, "Backend engineer", profileResponse.Profile.Headline)
	require.Len(t, profileResponse.Profile.Skills, 1)
	assert.Equal(t, "Go", profileResponse.Profile.Skills[0].Name)
	require.Len(t, profileResponse.Profile.WorkHistory, 1)
	assert.Equal(t, "Acme", profileResponse.Profile.WorkHistory[0].Company)
	require.Len(t, profileResponse.Profile.Education, 1)
	assert.Equal(t, "University", profileResponse.Profile.Education[0].Institution)

	require.NoError(t, repository.CreateJobMatch(context.Background(), 1, "No-op match"))
	require.NoError(t, repository.SaveJobAnalysis(context.Background(), models.JobAnalysisRecord{
		JobID:           1,
		AnalyzerVersion: "v2",
		PromptVersion:   "test",
		InputSHA256:     "input",
		Model:           "test-model",
		AnalyzedAt:      "2026-08-31T12:00:00Z",
		Analysis: models.JobAnalysisDraft{
			Role:     models.JobRole{Family: "backend_engineering", Seniority: "senior", SeniorityConfidence: "high"},
			Unknowns: []string{"Salary is not listed."},
		},
	}))
	response = request(handler, http.MethodGet, "/api/jobs/1/match")
	require.Equal(t, http.StatusOK, response.Code)
	var matchResponse struct {
		Match    *models.JobMatchRecord    `json:"match"`
		Analysis *models.JobAnalysisRecord `json:"analysis"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&matchResponse))
	require.NotNil(t, matchResponse.Match)
	assert.Equal(t, int64(1), matchResponse.Match.JobID)
	assert.Equal(t, "No-op match", matchResponse.Match.Content)
	assert.NotEmpty(t, matchResponse.Match.CreatedAt)
	require.NotNil(t, matchResponse.Analysis)
	assert.Equal(t, "backend_engineering", matchResponse.Analysis.Analysis.Role.Family)
	require.NoError(t, repository.CreateJobMatch(context.Background(), 2, "Second match"))
	assessmentContent, err := json.Marshal(models.JobMatchAssessment{MatcherVersion: "test", Score: 91, Label: "Strong match"})
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE job_matches SET content = ? WHERE job_id = 1`, string(assessmentContent))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE job_matches SET created_at = CASE job_id WHEN 1 THEN '2026-08-27T12:00:00Z' WHEN 2 THEN '2026-08-28T12:00:00Z' END`)
	require.NoError(t, err)
	response = request(handler, http.MethodGet, "/api/matches")
	require.Equal(t, http.StatusOK, response.Code)
	var matchesResponse struct {
		Matches []models.JobMatchSummary `json:"matches"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&matchesResponse))
	require.Len(t, matchesResponse.Matches, 2)
	assert.Equal(t, int64(2), matchesResponse.Matches[0].Job.ID)
	assert.Equal(t, "2026-08-28T12:00:00Z", matchesResponse.Matches[0].CreatedAt)
	assert.Equal(t, "Strong match", matchesResponse.Matches[1].Label)
	assert.Equal(t, 91, matchesResponse.Matches[1].Score)

	response = request(handler, http.MethodGet, "/api/jobs/1")
	require.Equal(t, http.StatusOK, response.Code)
	var jobResponse struct {
		Job models.BrowseJob `json:"job"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&jobResponse))
	assert.Equal(t, "Engineer", jobResponse.Job.Title)

	response = request(handler, http.MethodPost, "/api/jobs/1/match/redo")
	require.Equal(t, http.StatusAccepted, response.Code)
	response = request(handler, http.MethodPost, "/api/jobs/2/match")
	require.Equal(t, http.StatusAccepted, response.Code)
	response = request(handler, http.MethodGet, "/api/match-queue")
	require.Equal(t, http.StatusOK, response.Code)
	var queueResponse struct {
		Jobs []models.BrowseJob `json:"jobs"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&queueResponse))
	require.Equal(t, []int64{2, 1}, []int64{queueResponse.Jobs[0].ID, queueResponse.Jobs[1].ID})

	response = requestWithBody(handler, http.MethodPut, "/api/match-queue", `{"jobIds":[1,2]}`)
	require.Equal(t, http.StatusOK, response.Code)
	response = request(handler, http.MethodGet, "/api/match-queue")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&queueResponse))
	require.Equal(t, []int64{1, 2}, []int64{queueResponse.Jobs[0].ID, queueResponse.Jobs[1].ID})
}

func newTestServer(repository *repositories.SQLite) *Server {
	worker := services.NewJobMatchWorker(repository, repository, repository, repository, repository, noOpJobAnalysisService{}, noOpProfileJobMatcher{}, zap.NewNop(), time.Minute)
	return New(
		config.Config{},
		zap.NewNop(),
		services.NewJobBrowse(repository),
		services.NewDiscoverySettingsService(repository),
		services.NewProviderPreviewService(nil, nil, nil),
		services.NewUserProfileService(repository),
		services.NewJobMatches(repository, repository),
		services.NewJobMatchRequests(repository, worker),
	)
}

type noOpJobAnalysisService struct{}

func (noOpJobAnalysisService) Analyze(context.Context, models.Job) (services.JobAnalysis, error) {
	return services.JobAnalysis{}, nil
}

type noOpProfileJobMatcher struct{}

func (noOpProfileJobMatcher) Match(context.Context, models.BrowseJob, models.JobAnalysisRecord, models.UserProfile) (models.JobMatchAssessment, error) {
	return models.JobMatchAssessment{}, nil
}

func request(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}

func requestWithBody(handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, strings.NewReader(body)))
	return response
}
