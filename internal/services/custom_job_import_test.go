package services

import (
	"context"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/linkedin"
	"nice/internal/clients/openai"
)

type customJobPageFetcherStub struct {
	page CustomJobPage
	err  error
}

type customJobImportCompletionStub struct {
	response openai.ChatResponse
	model    string
	session  string
	request  openai.ChatRequest
}

type customLinkedInJobFetcherStub struct {
	job linkedin.Job
	err error
}

func TestCustomJobImporterStoresPageMarkdownAndExtractedFields(t *testing.T) {
	client := &customJobImportCompletionStub{response: openai.ChatResponse{Model: "import-model", Content: `{
		"title":"Senior Platform Engineer",
		"company":"Example Labs",
		"location":"London, United Kingdom",
		"workplace":"hybrid",
		"employmentType":"full-time",
		"salaryMin":90000,
		"salaryMax":120000,
		"postedAt":"2026-09-08T00:00:00Z"
	}`}}
	importer := NewCustomJobImporter(customJobPageFetcherStub{page: CustomJobPage{
		Title:    "Example Labs careers",
		Markdown: "# Senior Platform Engineer\n\nBuild reliable systems.",
	}}, nil, client, "configured-import-model", "high")

	job, err := importer.Import(context.Background(), "https://careers.example.com/platform-engineer")

	require.NoError(t, err)
	assert.Equal(t, "custom", job.Source)
	assert.Equal(t, "https://careers.example.com/platform-engineer", job.SourceID)
	assert.Equal(t, "Senior Platform Engineer", job.Title)
	assert.Equal(t, "# Senior Platform Engineer\n\nBuild reliable systems.", job.BodyText)
	assert.Equal(t, "Example Labs", job.Company)
	assert.Equal(t, "hybrid", job.Workplace)
	assert.Equal(t, "configured-import-model", client.model)
	assert.NotEmpty(t, client.session)
	assert.Contains(t, client.request.Messages[1].Content, "<job_posting_markdown>")
}

func TestCustomJobImporterUsesPageTitleWhenNoTitleIsExtracted(t *testing.T) {
	importer := NewCustomJobImporter(customJobPageFetcherStub{page: CustomJobPage{Title: "Careers | Example", Markdown: "Job description"}}, nil, &customJobImportCompletionStub{response: openai.ChatResponse{Content: `{
		"title":"", "company":"", "location":"", "workplace":"unknown", "employmentType":"", "salaryMin":null, "salaryMax":null, "postedAt":""
	}`}}, "configured-import-model", "low")

	job, err := importer.Import(context.Background(), "https://careers.example.com/job")

	require.NoError(t, err)
	assert.Equal(t, "Careers | Example", job.Title)
}

func TestCustomJobImporterUsesLinkedInParserForLinkedInURL(t *testing.T) {
	completion := &customJobImportCompletionStub{response: openai.ChatResponse{Content: `{
		"title":"Senior Golang Engineer", "company":"K2 Partnering Solutions", "location":"Spain", "workplace":"unknown", "employmentType":"", "salaryMin":null, "salaryMax":null, "postedAt":""
	}`}}
	linkedinFetcher := customLinkedInJobFetcherStub{job: linkedin.Job{
		Title:          "Senior Golang Engineer",
		Company:        "K2 Partnering Solutions",
		Location:       "Spain",
		Description:    "Senior Software Engineer - Golang/Kubernetes. Fully remote role.",
		EmploymentType: "Full-time",
		WorkplaceType:  "Remote",
	}}
	importer := NewCustomJobImporter(customJobPageFetcherStub{err: assert.AnError}, linkedinFetcher, completion, "model", "low")

	job, err := importer.Import(context.Background(), "https://www.linkedin.com/jobs/view/4470002318")

	require.NoError(t, err)
	assert.Equal(t, "Senior Golang Engineer", job.Title)
	assert.Equal(t, "Senior Software Engineer - Golang/Kubernetes. Fully remote role.", job.BodyText)
	assert.Equal(t, "K2 Partnering Solutions", job.Company)
	assert.Equal(t, "Spain", job.Location)
	assert.Equal(t, "Full-time", job.EmploymentType)
	assert.Equal(t, "remote", job.Workplace)
	assert.NotContains(t, completion.request.Messages[1].Content, "LinkedIn respects your privacy")
}

func TestCustomJobPageFromHTMLRendersReadableMarkdown(t *testing.T) {
	baseURL, err := url.Parse("https://careers.example.com/jobs/platform-engineer")
	require.NoError(t, err)
	page, err := customJobPageFromHTML(`<!doctype html><html><head><title>Platform Engineer</title><style>hidden</style></head><body><nav>Navigation</nav><main><h1>Platform Engineer</h1><p>Build <strong>reliable</strong> systems.</p><ul><li>Go</li><li><a href="/about">Learn more</a></li></ul></main><script>ignored</script></body></html>`, baseURL)

	require.NoError(t, err)
	assert.Equal(t, "Platform Engineer", page.Title)
	assert.Equal(t, "# Platform Engineer\n\nBuild reliable systems.\n\n- Go\n\n- [Learn more](https://careers.example.com/about)", page.Markdown)
}

func TestCustomJobPageFromHTMLHandlesBreaksAndInvalidLinks(t *testing.T) {
	baseURL, err := url.Parse("https://careers.example.com/jobs/platform-engineer")
	require.NoError(t, err)

	page, err := customJobPageFromHTML(`<main>First<br>Second<a href="%zz">broken link</a><a href=""> </a><svg>ignored</svg><footer>ignored</footer></main>`, baseURL)

	require.NoError(t, err)
	assert.Equal(t, "First\nSecond broken link", page.Markdown)
	assert.NotContains(t, page.Markdown, "ignored")
}

func TestValidateCustomJobFieldsRejectsInvalidDates(t *testing.T) {
	err := validateCustomJobFields(&customJobFields{Workplace: "remote", PostedAt: "2026-09-08"})
	assert.ErrorContains(t, err, "posted date must use RFC3339")
}

func TestIsPublicAddressRejectsPrivateNetworks(t *testing.T) {
	assert.False(t, isPublicAddress(netip.MustParseAddr("127.0.0.1")))
	assert.False(t, isPublicAddress(netip.MustParseAddr("10.0.0.1")))
	assert.True(t, isPublicAddress(netip.MustParseAddr("1.1.1.1")))
}

func (stub customJobPageFetcherStub) Fetch(context.Context, string) (CustomJobPage, error) {
	return stub.page, stub.err
}

func (stub customLinkedInJobFetcherStub) Job(context.Context, string) (linkedin.Job, error) {
	return stub.job, stub.err
}

func (stub *customJobImportCompletionStub) Complete(_ context.Context, model, session string, request openai.ChatRequest) (openai.ChatResponse, error) {
	stub.model = model
	stub.session = session
	stub.request = request
	return stub.response, nil
}
