package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/clients/openai"
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
	assert.False(t, result.Settings.LinkedIn.Enabled)

	response = requestWithBody(handler, http.MethodPut, "/api/discovery-settings", `{"adzuna":{"enabled":false,"query":"platform engineer","country":"de","maxDaysOld":14,"maxPages":2,"resultsPerPage":25,"workplace":"remote"},"remotive":{"enabled":true,"query":"platform engineer","category":"software-development"},"jobicy":{"enabled":true,"count":25,"geo":"europe","industry":"engineering","tag":"golang"},"linkedin":{"enabled":false,"query":"platform engineer","location":"Berlin","limit":25}}`)
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	assert.Equal(t, "platform engineer", result.Settings.Adzuna.Query)
	assert.False(t, result.Settings.Adzuna.Enabled)
	assert.Equal(t, "golang", result.Settings.Jobicy.Tag)
	assert.Equal(t, "Berlin", result.Settings.LinkedIn.Location)

	response = requestWithBody(handler, http.MethodPut, "/api/discovery-settings", `{"adzuna":{"enabled":false,"query":"","country":"de","maxDaysOld":14,"maxPages":2,"resultsPerPage":25,"workplace":"remote"},"remotive":{"enabled":true,"query":"platform engineer","category":"software-development"},"jobicy":{"enabled":true,"count":25,"geo":"europe","industry":"engineering","tag":"golang"},"linkedin":{"enabled":false,"query":"platform engineer","location":"Berlin","limit":25}}`)
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestEventsAPIListsFilteredPaginatedEvents(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	require.NoError(t, repository.RecordEvent(context.Background(), models.Event{Provider: "linkedin", RunID: "run-1", Type: "provider.run.started", Level: "info", Message: "LinkedIn job sync started"}))
	require.NoError(t, repository.RecordEvent(context.Background(), models.Event{Provider: "application", Type: "application.started", Level: "info", Message: "Application started"}))
	handler := newTestServer(repository).http.Handler

	response := request(handler, http.MethodGet, "/api/events?provider=linkedin&limit=1")

	require.Equal(t, http.StatusOK, response.Code)
	var page models.EventPage
	require.NoError(t, json.NewDecoder(response.Body).Decode(&page))
	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Events, 1)
	assert.Equal(t, "run-1", page.Events[0].RunID)

	response = request(handler, http.MethodGet, "/api/events?limit=0")
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

func TestJobMatchChatAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	require.NoError(t, db.Ping())
	require.NoError(t, migrations.Apply(db))
	require.NoError(t, enableForeignKeys(db))

	repository := repositories.NewSQLite(db)
	_, err = repository.SaveResume(context.Background(), models.Resume{FullName: "Ada Lovelace"})
	require.NoError(t, err)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "remotive", SourceID: "chat-job", SourceURL: "https://example.com/chat-job", Title: "Engineer", BodyText: "Build reliable services.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	require.NoError(t, repository.CreateJobMatch(context.Background(), 1, "Strong match"))
	handler := newTestServer(repository).http.Handler

	response := request(handler, http.MethodGet, "/api/jobs/1/match/chat")
	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"items":[]}`, response.Body.String())

	response = requestWithBody(handler, http.MethodPost, "/api/jobs/1/match/chat", `{"content":"How should I approach this role?","requestId":"chat-request"}`)
	require.Equal(t, http.StatusAccepted, response.Code)
	var accepted struct {
		Item models.JobMatchChatItem `json:"item"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&accepted))
	assert.Equal(t, "user_message", accepted.Item.Type)
	var userPayload struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.Unmarshal(accepted.Item.Payload, &userPayload))
	assert.Equal(t, "How should I approach this role?", userPayload.Content)
	waitForChatTurn(t, repository, 1)

	response = requestWithBody(handler, http.MethodPost, "/api/jobs/1/match/chat", `{"content":"How should I approach this role?","requestId":"chat-request"}`)
	require.Equal(t, http.StatusAccepted, response.Code)

	response = request(handler, http.MethodGet, "/api/jobs/1/match/chat")
	require.Equal(t, http.StatusOK, response.Code)
	var history struct {
		Items []models.JobMatchChatItem `json:"items"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&history))
	require.Len(t, history.Items, 9)
	assert.Equal(t, "user_message", history.Items[4].Type)
	assert.Equal(t, "assistant_message", history.Items[7].Type)

	response = request(handler, http.MethodDelete, "/api/jobs/1/match/chat/6")
	require.Equal(t, http.StatusNotFound, response.Code)

	response = request(handler, http.MethodDelete, "/api/jobs/1/match/chat/5")
	require.Equal(t, http.StatusNoContent, response.Code)
	response = request(handler, http.MethodGet, "/api/jobs/1/match/chat")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&history))
	require.Len(t, history.Items, 4)

	response = request(handler, http.MethodPost, "/api/jobs/1/match/redo")
	require.Equal(t, http.StatusAccepted, response.Code)
	response = request(handler, http.MethodGet, "/api/jobs/1/match/chat")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&history))
	require.Empty(t, history.Items)
}

func TestJobMatchChatAPIDoesNotDuplicateUserMessageAfterFailedReply(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	require.NoError(t, migrations.Apply(db))

	repository := repositories.NewSQLite(db)
	_, err = repository.SaveResume(context.Background(), models.Resume{FullName: "Ada Lovelace"})
	require.NoError(t, err)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{
		Source: "remotive", SourceID: "failed-chat-job", SourceURL: "https://example.com/failed-chat-job", Title: "Engineer", BodyText: "Build reliable services.", Workplace: "remote", MetadataJSON: "{}",
	}}))
	require.NoError(t, repository.CreateJobMatch(context.Background(), 1, "Strong match"))
	handler := newTestServerWithJobCompletionClient(repository, failingJobCompletionClient{}).http.Handler

	response := requestWithBody(handler, http.MethodPost, "/api/jobs/1/match/chat", `{"content":"How should I approach this role?","requestId":"failed-chat-request"}`)
	require.Equal(t, http.StatusAccepted, response.Code)
	waitForChatTurn(t, repository, 1)

	response = request(handler, http.MethodGet, "/api/jobs/1/match/chat")
	require.Equal(t, http.StatusOK, response.Code)
	var history struct {
		Items []models.JobMatchChatItem `json:"items"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&history))
	require.Len(t, history.Items, 8)
	assert.Equal(t, "user_message", history.Items[4].Type)

	response = requestWithBody(handler, http.MethodPost, "/api/jobs/1/match/chat", `{"content":"How should I approach this role?","requestId":"failed-chat-request"}`)
	require.Equal(t, http.StatusAccepted, response.Code)
	response = request(handler, http.MethodGet, "/api/jobs/1/match/chat")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&history))
	require.Len(t, history.Items, 8)
}

func TestJobMatchChatCreatesApplicationResumeRevision(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	require.NoError(t, migrations.Apply(db))
	require.NoError(t, enableForeignKeys(db))

	repository := repositories.NewSQLite(db)
	_, err = repository.SaveResume(context.Background(), models.Resume{FullName: "Ada Lovelace", Headline: "Software engineer"})
	require.NoError(t, err)
	require.NoError(t, repository.Upsert(context.Background(), []models.Job{{Source: "remotive", SourceID: "revision-job", SourceURL: "https://example.com/revision-job", Title: "Engineer", BodyText: "Build reliable services.", Workplace: "remote", MetadataJSON: "{}"}}))
	require.NoError(t, repository.CreateJobMatch(context.Background(), 1, "Strong match"))
	handler := newTestServerWithJobCompletionClient(repository, resumePatchCompletionClient{}).http.Handler

	response := requestWithBody(handler, http.MethodPost, "/api/jobs/1/match/chat", `{"content":"Tailor my resume.","requestId":"revision-request"}`)
	require.Equal(t, http.StatusAccepted, response.Code)
	waitForChatTurn(t, repository, 1)
	items, err := repository.JobMatchChatItems(context.Background(), 1, 0)
	require.NoError(t, err)
	accepted := models.JobMatchChatItem{}
	for _, item := range items {
		if item.Type == "tool_result" {
			accepted = item
			break
		}
	}
	assert.Equal(t, "tool_result", accepted.Type)
	var revision struct {
		Resume   models.Resume `json:"resume"`
		Revision int           `json:"revision"`
	}
	require.NoError(t, json.Unmarshal(accepted.Payload, &revision))
	assert.Equal(t, 1, revision.Revision)
	assert.Equal(t, "Backend engineer", revision.Resume.Headline)
}

func TestResumeAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	handler := newTestServer(repositories.NewSQLite(db)).http.Handler
	response := request(handler, http.MethodGet, "/api/resume")
	require.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"resume":null}`, response.Body.String())

	response = requestWithBody(handler, http.MethodPut, "/api/resume", `{
		"fullName":" Ada Lovelace ","headline":" Backend engineer ","location":" Berlin ","email":" ada@example.com ","phone":" +49 123 ","summaryParagraphs":[{"content":" Builds systems. "},{"content":""},{"content":" Delivers reliable software. "}],
		"links":[{"label":" GitHub ","url":" https://github.com/ada "},{"label":"","url":""}],
		"skills":[{"name":" Go "},{"name":""}],
		"competencies":[{"title":" Backend systems ","bullets":[{"content":" Built APIs "},{"content":""}]}],
		"experience":[{"company":" Acme ","title":" Engineer ","location":" Berlin ","startDate":"2023-01","endDate":"","isCurrent":true,"stack":" Go, PostgreSQL ","bullets":[{"content":" Shipped a service "},{"content":""}]}],
		"education":[{"institution":" University ","location":" Berlin ","degree":" MSc ","fieldOfStudy":" Computer science ","startDate":"2019","endDate":"2021","details":" Distributed systems "},{"institution":"","location":"","degree":"","fieldOfStudy":"","startDate":"","endDate":"","details":""}]
	}`)
	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), `"level"`)

	var result struct {
		Resume models.Resume `json:"resume"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	assert.Equal(t, int64(1), result.Resume.ID)
	assert.Equal(t, "Ada Lovelace", result.Resume.FullName)
	assert.Equal(t, []string{"Builds systems.", "Delivers reliable software."}, []string{result.Resume.SummaryParagraphs[0].Content, result.Resume.SummaryParagraphs[1].Content})
	assert.Positive(t, result.Resume.SummaryParagraphs[0].ID)
	require.Len(t, result.Resume.Skills, 1)
	assert.Equal(t, "Go", result.Resume.Skills[0].Name)
	require.Len(t, result.Resume.Links, 1)
	assert.Equal(t, "GitHub", result.Resume.Links[0].Label)
	require.Len(t, result.Resume.Competencies, 1)
	assert.Equal(t, "Built APIs", result.Resume.Competencies[0].Bullets[0].Content)
	assert.Positive(t, result.Resume.Competencies[0].Bullets[0].ID)
	require.Len(t, result.Resume.Experience, 1)
	assert.True(t, result.Resume.Experience[0].IsCurrent)
	require.Len(t, result.Resume.Education, 1)
	assert.Equal(t, "University", result.Resume.Education[0].Institution)

	response = request(handler, http.MethodGet, "/api/resume")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	assert.Equal(t, "Backend engineer", result.Resume.Headline)
	assert.Equal(t, "Go, PostgreSQL", result.Resume.Experience[0].Stack)
	paragraphID := result.Resume.SummaryParagraphs[0].ID
	experienceID := result.Resume.Experience[0].ID
	bulletID := result.Resume.Experience[0].Bullets[0].ID
	result.Resume.Headline = "Platform engineer"
	result.Resume.Experience[0].Bullets[0].Content = "Shipped a resilient service"
	body, err := json.Marshal(result.Resume)
	require.NoError(t, err)
	response = requestWithBody(handler, http.MethodPut, "/api/resume", string(body))
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
	assert.Equal(t, paragraphID, result.Resume.SummaryParagraphs[0].ID)
	assert.Equal(t, experienceID, result.Resume.Experience[0].ID)
	assert.Equal(t, bulletID, result.Resume.Experience[0].Bullets[0].ID)
}

func TestResumePhotoAPI(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	handler := newTestServer(repositories.NewSQLite(db)).http.Handler
	response := requestWithBody(handler, http.MethodPut, "/api/resume", `{"fullName":"Ada Lovelace"}`)
	require.Equal(t, http.StatusOK, response.Code)

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("photo", "photo.png")
	require.NoError(t, err)
	require.NoError(t, png.Encode(file, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	require.NoError(t, form.Close())
	photoRequest := httptest.NewRequest(http.MethodPost, "/api/resume/photo", &body)
	photoRequest.Header.Set("Content-Type", form.FormDataContentType())
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, photoRequest)
	require.Equal(t, http.StatusOK, response.Code)

	response = request(handler, http.MethodGet, "/api/resume/photo")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "image/png", response.Header().Get("Content-Type"))
	assert.NotEmpty(t, response.Body.Bytes())
}

func TestResumePDFHandler(t *testing.T) {
	handler := resumePDFHandler(resumePDFStub{content: []byte("pdf")}, zap.NewNop())
	response := request(handler, http.MethodGet, "/api/resume.pdf")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/pdf", response.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="resume.pdf"`, response.Header().Get("Content-Disposition"))
	assert.Equal(t, []byte("pdf"), response.Body.Bytes())

	response = request(resumePDFHandler(resumePDFStub{err: services.ErrResumeNotFound}, zap.NewNop()), http.MethodGet, "/api/resume.pdf")
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestJobApplicationResumePDFHandler(t *testing.T) {
	renderer := &applicationResumePDFStub{content: []byte("application pdf")}
	handler := jobApplicationResumePDFHandler(applicationResumeStub{resume: &models.Resume{Headline: "Backend engineer"}}, renderer, zap.NewNop())
	request := httptest.NewRequest(http.MethodGet, "/api/jobs/1/match/resume.pdf", nil)
	request.SetPathValue("id", "1")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/pdf", response.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="application-resume.pdf"`, response.Header().Get("Content-Disposition"))
	assert.Equal(t, []byte("application pdf"), response.Body.Bytes())
	assert.Equal(t, "Backend engineer", renderer.resume.Headline)
}

type resumePDFStub struct {
	content []byte
	err     error
}

func (stub resumePDFStub) Generate(context.Context) ([]byte, error) {
	return stub.content, stub.err
}

type applicationResumeStub struct {
	resume *models.Resume
	err    error
}

func (stub applicationResumeStub) LatestResume(context.Context, int64) (*models.Resume, error) {
	return stub.resume, stub.err
}

type applicationResumePDFStub struct {
	content []byte
	err     error
	resume  models.Resume
}

func (stub *applicationResumePDFStub) GenerateResume(_ context.Context, resume models.Resume) ([]byte, error) {
	stub.resume = resume
	return stub.content, stub.err
}

func newTestServer(repository *repositories.SQLite) *Server {
	return newTestServerWithJobCompletionClient(repository, noOpJobCompletionClient{})
}

func newTestServerWithJobCompletionClient(repository *repositories.SQLite, client services.JobCompletionClient) *Server {
	worker := services.NewJobMatchWorker(repository, repository, repository, repository, repository, noOpJobAnalysisService{}, noOpProfileJobMatcher{}, zap.NewNop(), time.Minute)
	return New(
		config.Config{},
		zap.NewNop(),
		services.NewJobBrowse(repository),
		services.NewEventLog(repository),
		services.NewDiscoverySettingsService(repository),
		services.NewProviderPreviewService(nil, nil, nil, nil),
		services.NewUserProfileService(repository),
		services.NewResumeService(repository),
		services.NewResumePDFService(services.NewResumeService(repository)),
		services.NewJobMatches(repository, repository),
		services.NewJobMatchRequests(repository, worker),
		services.NewJobMatchChat(repository, repository, repository, repository, repository, client, "test-model", "low", zap.NewNop()),
	)
}

func waitForChatTurn(t *testing.T, repository *repositories.SQLite, jobID int64) {
	t.Helper()
	var types []string
	completed := assert.Eventually(t, func() bool {
		items, err := repository.JobMatchChatItems(context.Background(), jobID, 0)
		if err != nil {
			return false
		}
		types = types[:0]
		for _, item := range items {
			types = append(types, item.Type)
		}
		return len(items) > 0 && (items[len(items)-1].Type == "turn_completed" || items[len(items)-1].Type == "turn_halted" || items[len(items)-1].Type == "turn_stopped")
	}, time.Second, 10*time.Millisecond)
	require.True(t, completed, "chat item types: %v", types)
}

func enableForeignKeys(db *sql.DB) error {
	_, err := db.Exec("PRAGMA foreign_keys = ON")
	return err
}

type noOpJobAnalysisService struct{}

func (noOpJobAnalysisService) Analyze(context.Context, models.Job) (services.JobAnalysis, error) {
	return services.JobAnalysis{}, nil
}

type noOpProfileJobMatcher struct{}

func (noOpProfileJobMatcher) Match(context.Context, models.BrowseJob, models.JobAnalysisRecord, models.UserProfile) (models.JobMatchAssessment, error) {
	return models.JobMatchAssessment{}, nil
}

type noOpJobCompletionClient struct{}

func (noOpJobCompletionClient) Complete(context.Context, string, string, openai.ChatRequest) (openai.ChatResponse, error) {
	return openai.ChatResponse{Model: "test-model", Content: "Test chat reply."}, nil
}

type failingJobCompletionClient struct{}

func (failingJobCompletionClient) Complete(context.Context, string, string, openai.ChatRequest) (openai.ChatResponse, error) {
	return openai.ChatResponse{}, errors.New("LLM unavailable")
}

type resumePatchCompletionClient struct{}

func (resumePatchCompletionClient) Complete(_ context.Context, _ string, _ string, request openai.ChatRequest) (openai.ChatResponse, error) {
	for _, message := range request.Messages {
		if message.Role == "tool" {
			return openai.ChatResponse{Model: "test-model", Content: "The resume has been tailored."}, nil
		}
	}
	return openai.ChatResponse{Model: "test-model", ToolCalls: []openai.ToolCall{{ID: "patch-call", Type: "function", Function: openai.ToolFunction{Name: "revise_application_resume", Arguments: `{"baseRevision":0,"operations":[{"op":"replace","section":"headline","id":0,"parentId":0,"expected":"Software engineer","value":"Backend engineer"}]}`}}}}, nil
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
