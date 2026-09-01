package jobicy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/models"
)

func TestFetchMapsJobicyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("get") == "industries" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"industries":[{"industryName":"Software Engineering","industrySlug":"engineering"}]}`))
			return
		}

		assert.Equal(t, "50", request.URL.Query().Get("count"))
		assert.Equal(t, "europe", request.URL.Query().Get("geo"))
		assert.Equal(t, "engineering", request.URL.Query().Get("industry"))
		assert.Empty(t, request.URL.Query().Get("tag"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jobs":[{"id":42,"url":"https://jobicy.com/jobs/42-example","jobTitle":"Software Engineer","companyName":"Example Co","companyLogo":"https://jobicy.com/logo.png","jobIndustry":["Software Engineering"],"jobType":["Full-Time","Contract"],"jobGeo":"Europe","jobLevel":"Senior","jobExcerpt":"Build useful things.","jobDescription":"<p>Build useful things.</p>","pubDate":"2026-08-27T10:00:00+02:00","salaryMin":80000,"salaryMax":100000,"salaryCurrency":"EUR","salaryPeriod":"yearly"},{"id":43,"jobTitle":"QA Tester","jobIndustry":["QA & Testing"]}]}`))
	}))
	defer server.Close()

	jobs, err := newClient(server.Client(), server.URL).Fetch(context.Background(), models.JobicySearchSettings{
		Count:    50,
		Geo:      "europe",
		Industry: "engineering",
	})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	job := jobs[0]
	assert.Equal(t, "jobicy", job.Source)
	assert.Equal(t, "42", job.SourceID)
	assert.Equal(t, "https://jobicy.com/jobs/42-example", job.SourceURL)
	assert.Equal(t, "<p>Build useful things.</p>", job.BodyText)
	assert.Equal(t, "Europe", job.Location)
	assert.Equal(t, "remote", job.Workplace)
	assert.Equal(t, "Full-Time, Contract", job.EmploymentType)
	assert.EqualValues(t, 80000, *job.SalaryMin)
	assert.EqualValues(t, 100000, *job.SalaryMax)
	assert.Equal(t, "2026-08-27T08:00:00Z", job.PostedAt)
}
