package linkedin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const baseURL = "https://www.linkedin.com/jobs-guest/jobs/api"

var jobIDPattern = regexp.MustCompile(`/jobs/view/(?:[^/?#]*-)?(\d+)(?:[/?#]|$)`)

type Client struct {
	http    *http.Client
	baseURL string
}

type SearchFilter struct {
	Keywords string
	Location string
	Start    int
}

type SearchResult struct {
	ID       string
	URL      string
	Title    string
	Company  string
	Location string
	PostedAt string
}

type Job struct {
	Description    string
	EmploymentType string
}

func NewClient() *Client {
	return newClient(&http.Client{Timeout: 30 * time.Second}, baseURL)
}

func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{http: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

func (c *Client) Search(ctx context.Context, filter SearchFilter) ([]SearchResult, error) {
	requestURL, err := url.Parse(c.baseURL + "/seeMoreJobPostings/search")
	if err != nil {
		return nil, err
	}
	params := requestURL.Query()
	params.Set("keywords", filter.Keywords)
	if filter.Location != "" {
		params.Set("location", filter.Location)
	}
	params.Set("start", fmt.Sprint(filter.Start))
	requestURL.RawQuery = params.Encode()

	body, err := c.get(ctx, requestURL.String())
	if err != nil {
		return nil, err
	}
	return parseListings(body)
}

func (c *Client) Job(ctx context.Context, id string) (Job, error) {
	body, err := c.get(ctx, c.baseURL+"/jobPosting/"+url.PathEscape(id))
	if err != nil {
		return Job{}, err
	}
	return parseJob(body), nil
}

func (c *Client) get(ctx context.Context, requestURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/131.0.0.0 Safari/537.36")
	httpResponse, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 4<<10))
		if readErr != nil {
			return nil, fmt.Errorf("LinkedIn returned %s", httpResponse.Status)
		}
		return nil, fmt.Errorf("LinkedIn returned %s: %s", httpResponse.Status, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(httpResponse.Body)
}

func parseListings(body []byte) ([]SearchResult, error) {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	listings := make([]SearchResult, 0)
	seen := make(map[string]struct{})
	visit(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "li" {
			return
		}
		link := first(node, func(candidate *html.Node) bool {
			return candidate.Type == html.ElementNode && candidate.Data == "a" && jobID(candidateAttr(candidate, "href")) != ""
		})
		if link == nil {
			return
		}
		jobURL := candidateAttr(link, "href")
		id := jobID(jobURL)
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		listing := SearchResult{ID: id, URL: jobURL}
		if title := first(node, hasClass("h3", "base-search-card__title")); title != nil {
			listing.Title = text(title)
		}
		if company := first(node, hasClass("h4", "base-search-card__subtitle")); company != nil {
			listing.Company = text(company)
		}
		if location := first(node, hasClass("span", "job-search-card__location")); location != nil {
			listing.Location = text(location)
		}
		if posted := first(node, func(candidate *html.Node) bool {
			return candidate.Type == html.ElementNode && candidate.Data == "time"
		}); posted != nil {
			listing.PostedAt = candidateAttr(posted, "datetime")
		}
		listings = append(listings, listing)
	})
	return listings, nil
}

func parseJob(body []byte) Job {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return Job{}
	}
	job := Job{EmploymentType: jobCriteria(document, "Employment type")}
	if node := first(document, hasClass("div", "show-more-less-html__markup")); node != nil {
		job.Description = text(node)
		return job
	}
	if node := first(document, hasClass("div", "description__text")); node != nil {
		job.Description = text(node)
	}
	return job
}

func jobCriteria(document *html.Node, label string) string {
	var value string
	visit(document, func(node *html.Node) {
		if value != "" || !hasClass("li", "description__job-criteria-item")(node) {
			return
		}
		header := first(node, hasClass("h3", "description__job-criteria-subheader"))
		if header == nil || !strings.EqualFold(text(header), label) {
			return
		}
		if criteria := first(node, hasClass("span", "description__job-criteria-text--criteria")); criteria != nil {
			value = text(criteria)
		}
	})
	return value
}

func jobID(jobURL string) string {
	matches := jobIDPattern.FindStringSubmatch(jobURL)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func hasClass(element, class string) func(*html.Node) bool {
	return func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == element && strings.Contains(" "+candidateAttr(node, "class")+" ", " "+class+" ")
	}
}

func candidateAttr(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func first(node *html.Node, match func(*html.Node) bool) *html.Node {
	if match(node) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := first(child, match); found != nil {
			return found
		}
	}
	return nil
}

func visit(node *html.Node, fn func(*html.Node)) {
	fn(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		visit(child, fn)
	}
}

func text(node *html.Node) string {
	var builder strings.Builder
	visit(node, func(candidate *html.Node) {
		if candidate.Type == html.TextNode {
			builder.WriteString(candidate.Data)
			builder.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(builder.String()), " ")
}
