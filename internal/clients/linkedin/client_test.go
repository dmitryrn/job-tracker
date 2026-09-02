package linkedin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchAndJobUsePublicLinkedInEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/seeMoreJobPostings/search":
			assert.Equal(t, "software engineer", request.URL.Query().Get("keywords"))
			assert.Equal(t, "Berlin", request.URL.Query().Get("location"))
			assert.Equal(t, "0", request.URL.Query().Get("start"))
			_, _ = writer.Write([]byte(`<li><a href="https://www.linkedin.com/jobs/view/platform-engineer-42?trackingId=abc"></a><h3 class="base-search-card__title">Platform Engineer</h3><h4 class="base-search-card__subtitle"><a>Example Co</a></h4><span class="job-search-card__location">Berlin, Germany</span><time datetime="2026-09-02T10:00:00Z"></time></li>`))
		case "/jobPosting/42":
			_, _ = writer.Write([]byte(`<div class="show-more-less-html__markup">Build reliable remote systems.</div>`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newClient(server.Client(), server.URL)
	results, err := client.Search(context.Background(), SearchFilter{Keywords: "software engineer", Location: "Berlin"})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "42", results[0].ID)
	assert.Equal(t, "Platform Engineer", results[0].Title)
	assert.Equal(t, "Example Co", results[0].Company)
	assert.Equal(t, "2026-09-02T10:00:00Z", results[0].PostedAt)

	job, err := client.Job(context.Background(), results[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "Build reliable remote systems.", job.Description)
}
