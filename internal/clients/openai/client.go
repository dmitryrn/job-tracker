package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxErrorResponseBytes = 64 << 10
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ProviderPreferences struct {
	RequireParameters bool `json:"require_parameters"`
}

type ChatRequest struct {
	Messages        []Message            `json:"messages"`
	ResponseFormat  json.RawMessage      `json:"response_format,omitempty"`
	MaxTokens       int                  `json:"max_tokens,omitempty"`
	Temperature     *float64             `json:"temperature,omitempty"`
	ReasoningEffort string               `json:"reasoning_effort,omitempty"`
	Provider        *ProviderPreferences `json:"provider,omitempty"`
	Tools           []Tool               `json:"tools,omitempty"`
	ToolChoice      any                  `json:"tool_choice,omitempty"`
}

type chatCompletionPayload struct {
	Model string `json:"model"`
	ChatRequest
}

type ChatResponse struct {
	Model            string
	Content          string
	ToolCalls        []ToolCall
	FinishReason     string
	Refusal          string
	ReasoningSummary string
	Reasoning        json.RawMessage
	Usage            json.RawMessage
	ProviderMetadata json.RawMessage
}

type chatCompletionResponse struct {
	Model    string                 `json:"model"`
	Choices  []chatCompletionChoice `json:"choices"`
	Usage    json.RawMessage        `json:"usage"`
	Provider json.RawMessage        `json:"provider"`
	Metadata json.RawMessage        `json:"metadata"`
}

type chatCompletionChoice struct {
	FinishReason       string                `json:"finish_reason"`
	NativeFinishReason string                `json:"native_finish_reason"`
	Message            chatCompletionMessage `json:"message"`
}

type chatCompletionMessage struct {
	Content          *string         `json:"content"`
	Refusal          *string         `json:"refusal"`
	ToolCalls        []ToolCall      `json:"tool_calls"`
	Reasoning        json.RawMessage `json:"reasoning"`
	ReasoningDetails json.RawMessage `json:"reasoning_details"`
}

type Client struct {
	apiKey                      string
	http                        *http.Client
	baseURL                     string
	supportsProviderPreferences bool
	usesOpenCode                bool
}

func NewClient(apiKey, baseURL string) *Client {
	return newClient(apiKey, &http.Client{Timeout: 120 * time.Second}, baseURL)
}

func newClient(apiKey string, httpClient *http.Client, baseURL string) *Client {
	return &Client{
		apiKey:                      apiKey,
		http:                        httpClient,
		baseURL:                     baseURL,
		supportsProviderPreferences: isOpenRouterURL(baseURL),
		usesOpenCode:                isOpenCodeURL(baseURL),
	}
}

func (client *Client) Complete(ctx context.Context, model, sessionID string, input ChatRequest) (ChatResponse, error) {
	if strings.TrimSpace(client.apiKey) == "" {
		return ChatResponse{}, fmt.Errorf("LLM API key is required")
	}

	model = strings.TrimSpace(model)
	if model == "" {
		return ChatResponse{}, fmt.Errorf("LLM model is required")
	}

	sessionID = strings.TrimSpace(sessionID)
	if client.usesOpenCode && sessionID == "" {
		return ChatResponse{}, fmt.Errorf("LLM session ID is required")
	}

	payload, err := json.Marshal(chatCompletionPayload{Model: model, ChatRequest: client.requestForEndpoint(input)})
	if err != nil {
		return ChatResponse{}, fmt.Errorf("encode LLM request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL, bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("create LLM request: %w", err)
	}

	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Content-Type", "application/json")
	if client.usesOpenCode {
		request.Header.Set("X-Opencode-Session", sessionID)
	}

	response, err := client.http.Do(request)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("call LLM: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ChatResponse{}, unsuccessfulResponseError(response)
	}

	return decodeChatCompletionResponse(response.Body)
}

func unsuccessfulResponseError(response *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(response.Body, maxErrorResponseBytes))
	if err != nil {
		return fmt.Errorf("read unsuccessful LLM response: %w", err)
	}

	if body := strings.TrimSpace(string(body)); body != "" {
		return fmt.Errorf("LLM returned %s: %s", response.Status, body)
	}

	return fmt.Errorf("LLM returned %s", response.Status)
}

func decodeChatCompletionResponse(reader io.Reader) (ChatResponse, error) {
	var body chatCompletionResponse

	decoder := json.NewDecoder(io.LimitReader(reader, 2<<20))
	if err := decoder.Decode(&body); err != nil {
		return ChatResponse{}, fmt.Errorf("decode LLM response: %w", err)
	}

	if len(body.Choices) == 0 {
		return ChatResponse{}, noCompletionContentError(body.Model, "")
	}

	choice := body.Choices[0]
	content := ""
	if choice.Message.Content != nil {
		content = *choice.Message.Content
	}

	if strings.TrimSpace(content) == "" && len(choice.Message.ToolCalls) == 0 {
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

	reasoning := choice.Message.ReasoningDetails
	if len(reasoning) == 0 {
		reasoning = choice.Message.Reasoning
	}

	reasoningSummary := ""
	if json.Unmarshal(choice.Message.Reasoning, &reasoningSummary) != nil {
		_ = json.Unmarshal(choice.Message.ReasoningDetails, &reasoningSummary)
	}

	refusal := ""
	if choice.Message.Refusal != nil {
		refusal = *choice.Message.Refusal
	}

	providerMetadata := body.Provider
	if len(providerMetadata) == 0 {
		providerMetadata = body.Metadata
	}

	return ChatResponse{Model: body.Model, Content: content, ToolCalls: choice.Message.ToolCalls, FinishReason: choice.FinishReason, Refusal: refusal, ReasoningSummary: reasoningSummary, Reasoning: reasoning, Usage: body.Usage, ProviderMetadata: providerMetadata}, nil
}

func (client *Client) requestForEndpoint(input ChatRequest) ChatRequest {
	if !client.supportsProviderPreferences {
		input.Provider = nil
	}

	return input
}

func isOpenRouterURL(baseURL string) bool {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	hostname := strings.ToLower(endpoint.Hostname())
	return hostname == "openrouter.ai" || strings.HasSuffix(hostname, ".openrouter.ai")
}

func isOpenCodeURL(baseURL string) bool {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return false
	}

	hostname := strings.ToLower(endpoint.Hostname())
	return hostname == "opencode.ai" || strings.HasSuffix(hostname, ".opencode.ai")
}

func noCompletionContentError(model, reason string) error {
	message := "LLM returned no completion content"

	if reason != "" {
		message += " (" + reason + ")"
	}

	if strings.TrimSpace(model) != "" {
		message += " from " + model
	}

	return fmt.Errorf("%s", message)
}
