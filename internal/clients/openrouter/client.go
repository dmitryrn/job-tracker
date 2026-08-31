package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nice/internal/config"
)

const (
	baseURL      = "https://openrouter.ai/api/v1/chat/completions"
	DefaultModel = "openrouter/free"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ProviderPreferences struct {
	RequireParameters bool `json:"require_parameters"`
}

type ChatRequest struct {
	Model          string              `json:"model"`
	Messages       []Message           `json:"messages"`
	ResponseFormat json.RawMessage     `json:"response_format,omitempty"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
	Temperature    *float64            `json:"temperature,omitempty"`
	Provider       ProviderPreferences `json:"provider"`
}

type ChatResponse struct {
	Model   string
	Content string
}

type Client struct {
	apiKey  string
	model   string
	http    *http.Client
	baseURL string
}

func NewClient(cfg config.Config) *Client {
	return newClient(cfg.OpenRouter.APIKey, DefaultModel, &http.Client{Timeout: 120 * time.Second}, baseURL)
}

func newClient(apiKey, model string, httpClient *http.Client, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		http:    httpClient,
		baseURL: baseURL,
	}
}

func (client *Client) Complete(ctx context.Context, input ChatRequest) (ChatResponse, error) {
	if strings.TrimSpace(client.apiKey) == "" {
		return ChatResponse{}, fmt.Errorf("OpenRouter API key is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		input.Model = client.model
	}
	if strings.TrimSpace(input.Model) == "" {
		return ChatResponse{}, fmt.Errorf("OpenRouter model is required")
	}

	payload, err := json.Marshal(input)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("encode OpenRouter request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL, bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("create OpenRouter request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.http.Do(request)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("call OpenRouter: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ChatResponse{}, fmt.Errorf("OpenRouter returned %s", response.Status)
	}

	var body struct {
		Model   string `json:"model"`
		Choices []struct {
			FinishReason       string `json:"finish_reason"`
			NativeFinishReason string `json:"native_finish_reason"`
			Message            struct {
				Content *string `json:"content"`
				Refusal *string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(&body); err != nil {
		return ChatResponse{}, fmt.Errorf("decode OpenRouter response: %w", err)
	}
	if len(body.Choices) == 0 {
		return ChatResponse{}, noCompletionContentError(body.Model, "")
	}
	choice := body.Choices[0]
	if choice.Message.Content == nil || strings.TrimSpace(*choice.Message.Content) == "" {
		reason := choice.NativeFinishReason
		if reason == "" {
			reason = choice.FinishReason
		}
		if choice.Message.Refusal != nil && strings.TrimSpace(*choice.Message.Refusal) != "" {
			reason = "refusal"
		}
		if reason == "" {
			return ChatResponse{}, noCompletionContentError(body.Model, "")
		}
		return ChatResponse{}, noCompletionContentError(body.Model, reason)
	}
	return ChatResponse{Model: body.Model, Content: *choice.Message.Content}, nil
}

func noCompletionContentError(model, reason string) error {
	message := "OpenRouter returned no completion content"
	if reason != "" {
		message += " (" + reason + ")"
	}
	if strings.TrimSpace(model) != "" {
		message += " from " + model
	}
	return fmt.Errorf("%s", message)
}
