package ipinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://ipinfo.io/json"

type Client struct {
	http    *http.Client
	baseURL string
}

type Info struct {
	IP       string `json:"ip"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Location string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

func NewClient() *Client {
	return newClient(&http.Client{Timeout: 30 * time.Second}, baseURL)
}

func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{http: httpClient, baseURL: baseURL}
}

func (c *Client) Lookup(ctx context.Context) (Info, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return Info{}, err
	}

	request.Header.Set("Accept", "application/json")
	httpResponse, err := c.http.Do(request)
	if err != nil {
		return Info{}, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 4<<10))
		if readErr != nil {
			return Info{}, fmt.Errorf("IPinfo returned %s", httpResponse.Status)
		}

		return Info{}, fmt.Errorf("IPinfo returned %s: %s", httpResponse.Status, strings.TrimSpace(string(body)))
	}

	var info Info
	if err := json.NewDecoder(httpResponse.Body).Decode(&info); err != nil {
		return Info{}, err
	}

	info.Country = strings.ToUpper(strings.TrimSpace(info.Country))
	if info.Country == "" {
		return Info{}, fmt.Errorf("IPinfo response did not include a country code")
	}

	return info, nil
}
