package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBytes = 50 << 20

type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
	Score         float64            `json:"score"`
	Noul          float64            `json:"noul"`
}

type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
}

type Client struct {
	apiKey  string
	http    *http.Client
	baseURL string
	model   string
}

func NewClient(apiKey, baseURL, model string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	baseURL = strings.TrimSpace(baseURL)
	model = strings.TrimSpace(model)
	if apiKey == "" {
		return nil, fmt.Errorf("TypeSafe API key is required")
	}

	if baseURL == "" {
		return nil, fmt.Errorf("TypeSafe API URL is required")
	}

	if model == "" {
		return nil, fmt.Errorf("TypeSafe model is required")
	}

	return &Client{
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		model:   model,
	}, nil
}

func (client *Client) SystemOne(ctx context.Context, state any, questions map[string]Question) (Response, error) {
	payload, err := json.Marshal(struct {
		State     any                 `json:"state"`
		Model     string              `json:"model"`
		Questions map[string]Question `json:"questions"`
	}{State: state, Model: client.model, Questions: questions})
	if err != nil {
		return Response{}, fmt.Errorf("encode TypeSafe request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("create TypeSafe request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return Response{}, fmt.Errorf("call TypeSafe: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return Response{}, fmt.Errorf("read TypeSafe response: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if message := strings.TrimSpace(string(body)); message != "" {
			return Response{}, fmt.Errorf("TypeSafe returned %s: %s", response.Status, message)
		}

		return Response{}, fmt.Errorf("TypeSafe returned %s", response.Status)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return Response{}, fmt.Errorf("decode TypeSafe response: %w", err)
	}

	return result, nil
}
