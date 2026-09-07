package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkedInMetricsRecordsLatestRun(t *testing.T) {
	metrics := NewLinkedInMetrics()
	started := time.Unix(1_700_000_000, 0)
	finished := started.Add(time.Minute)

	metrics.Start(started)
	metrics.Complete(finished, LinkedInFetchResult{SavedJobs: 3, SkippedJobs: 2, InvalidJobs: 1}, true)

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `linkedin_sync_jobs{outcome="added"} 3`)
	assert.Contains(t, body, `linkedin_sync_jobs{outcome="skipped"} 2`)
	assert.Contains(t, body, `linkedin_sync_jobs{outcome="invalid"} 1`)
	assert.Contains(t, body, `linkedin_sync_jobs{outcome="errors"} 0`)
	assert.Contains(t, body, "linkedin_sync_last_run_started_timestamp_seconds 1.7e+09")
	assert.Contains(t, body, "linkedin_sync_last_run_finished_timestamp_seconds 1.70000006e+09")
	assert.Contains(t, body, "linkedin_sync_last_success_timestamp_seconds 1.70000006e+09")
	assert.NotContains(t, body, "go_goroutines")
	assert.NotContains(t, body, "process_cpu_seconds")
}

func TestLinkedInMetricsRecordsFailedRun(t *testing.T) {
	metrics := NewLinkedInMetrics()
	started := time.Unix(1_700_000_000, 0)
	finished := started.Add(time.Minute)

	metrics.Start(started)
	metrics.Complete(finished, LinkedInFetchResult{SavedJobs: 1}, false)

	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `linkedin_sync_jobs{outcome="errors"} 1`)
	assert.Contains(t, body, `linkedin_sync_jobs{outcome="added"} 1`)
	assert.Contains(t, body, "linkedin_sync_last_success_timestamp_seconds 0")
}
