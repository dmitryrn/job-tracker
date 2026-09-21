package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteOmitsProviderPreferencesForNonOpenRouterEndpoint(t *testing.T) {
	const model = "test-model"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
		assert.Empty(t, request.Header.Get("X-Opencode-Session"))

		var body struct {
			Model string `json:"model"`
			ChatRequest
		}
		assert.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		assert.Equal(t, model, body.Model)
		assert.Equal(t, []Message{{Role: "user", Content: "Extract this CV"}}, body.Messages)
		assert.Nil(t, body.Provider)
		assert.NotNil(t, body.Temperature)
		assert.Zero(t, *body.Temperature)
		assert.Equal(t, "high", body.ReasoningEffort)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"provider/model","choices":[{"message":{"content":"{\"name\":\"Riley\"}"}}]}`))
	}))
	defer server.Close()

	client := newClient("test-key", server.Client(), server.URL)
	temperature := 0.0
	response, err := client.Complete(context.Background(), model, "session-123", ChatRequest{
		Messages:        []Message{{Role: "user", Content: "Extract this CV"}},
		Temperature:     &temperature,
		ReasoningEffort: "high",
		Provider:        &ProviderPreferences{RequireParameters: true},
	})
	require.NoError(t, err)
	assert.Equal(t, "provider/model", response.Model)
	assert.JSONEq(t, `{"name":"Riley"}`, response.Content)
}

func TestRequestForEndpointSendsProviderPreferencesToOpenRouter(t *testing.T) {
	client := newClient("test-key", http.DefaultClient, "https://openrouter.ai/api/v1/chat/completions")
	request := client.requestForEndpoint(ChatRequest{
		Provider: &ProviderPreferences{RequireParameters: true},
	})

	require.NotNil(t, request.Provider)
	assert.True(t, request.Provider.RequireParameters)
}

func TestNewClient(t *testing.T) {
	client := NewClient("proxy-key", "http://127.0.0.1:8080/v1/chat/completions")

	assert.Equal(t, "proxy-key", client.apiKey)
	assert.Equal(t, "http://127.0.0.1:8080/v1/chat/completions", client.baseURL)
}

func TestCompleteRejectsUnsuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"code":429,"message":"Daily free model quota exhausted"}}`))
	}))
	defer server.Close()

	client := newClient("test-key", server.Client(), server.URL)
	_, err := client.Complete(context.Background(), "test-model", "session-456", ChatRequest{Messages: []Message{{Role: "user", Content: "CV"}}})
	require.ErrorContains(t, err, "429 Too Many Requests")
	require.ErrorContains(t, err, `{"error":{"code":429,"message":"Daily free model quota exhausted"}}`)
}

func TestCompleteIdentifiesModelWhenCompletionHasNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"provider/model","choices":[{"finish_reason":"length","message":{"content":null}}]}`))
	}))
	defer server.Close()

	client := newClient("test-key", server.Client(), server.URL)
	_, err := client.Complete(context.Background(), "test-model", "session-789", ChatRequest{Messages: []Message{{Role: "user", Content: "CV"}}})
	assert.EqualError(t, err, "LLM returned no completion content (length) from provider/model")
}

func TestCompleteSupportsFunctionCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct{ ChatRequest }
		assert.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		assert.Len(t, body.Tools, 1)
		assert.Equal(t, "revise_resume", body.Tools[0].Function.Name)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"provider/model","choices":[{"finish_reason":"tool_calls","message":{"content":null,"tool_calls":[{"id":"call-1","type":"function","function":{"name":"revise_resume","arguments":"{\"revision\":1}"}}]}}]}`))
	}))
	defer server.Close()

	client := newClient("test-key", server.Client(), server.URL)
	response, err := client.Complete(context.Background(), "test-model", "session", ChatRequest{
		Tools: []Tool{{
			Type:     "function",
			Function: ToolFunction{Name: "revise_resume", Parameters: json.RawMessage(`{"type":"object"}`)},
		}},
	})

	require.NoError(t, err)
	assert.Empty(t, response.Content)
	assert.Equal(t, "tool_calls", response.FinishReason)
	require.Len(t, response.ToolCalls, 1)
	assert.Equal(t, `{"revision":1}`, response.ToolCalls[0].Function.Arguments)
}

func TestCompleteRejectsMissingSessionID(t *testing.T) {
	client := newClient("test-key", http.DefaultClient, "https://opencode.ai/zen/go/v1/chat/completions")

	_, err := client.Complete(context.Background(), "test-model", "", ChatRequest{})

	assert.EqualError(t, err, "LLM session ID is required")
}

func TestCompleteSendsSessionIDToOpenCode(t *testing.T) {
	client := newClient("test-key", &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "session-123", request.Header.Get("X-Opencode-Session"))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"model":"provider/model","choices":[{"message":{"content":"response"}}]}`)),
		}, nil
	})}, "https://opencode.ai/zen/go/v1/chat/completions")

	_, err := client.Complete(context.Background(), "test-model", "session-123", ChatRequest{})

	require.NoError(t, err)
}

func TestIsOpenCodeURL(t *testing.T) {
	assert.True(t, isOpenCodeURL("https://opencode.ai/zen/go/v1/chat/completions"))
	assert.True(t, isOpenCodeURL("https://api.opencode.ai/v1/chat/completions"))
	assert.False(t, isOpenCodeURL("https://openrouter.ai/api/v1/chat/completions"))
	assert.False(t, isOpenCodeURL("https://llm.example.com/v1/chat/completions"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
