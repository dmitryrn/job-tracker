package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteSendsOpenAICompatibleRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))

		var body ChatRequest
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		assert.Equal(t, DefaultModel, body.Model)
		assert.Equal(t, []Message{{Role: "user", Content: "Extract this CV"}}, body.Messages)
		assert.True(t, body.Provider.RequireParameters)
		require.NotNil(t, body.Temperature)
		assert.Zero(t, *body.Temperature)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"provider/model","choices":[{"message":{"content":"{\"name\":\"Riley\"}"}}]}`))
	}))
	defer server.Close()

	client := newClient("test-key", DefaultModel, server.Client(), server.URL)
	temperature := 0.0
	response, err := client.Complete(context.Background(), ChatRequest{
		Messages:    []Message{{Role: "user", Content: "Extract this CV"}},
		Temperature: &temperature,
		Provider:    ProviderPreferences{RequireParameters: true},
	})
	require.NoError(t, err)
	assert.Equal(t, "provider/model", response.Model)
	assert.Equal(t, `{"name":"Riley"}`, response.Content)
}

func TestCompleteRejectsUnsuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := newClient("test-key", DefaultModel, server.Client(), server.URL)
	_, err := client.Complete(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "CV"}}})
	assert.ErrorContains(t, err, "429 Too Many Requests")
}

func TestCompleteIdentifiesModelWhenCompletionHasNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"provider/model","choices":[{"finish_reason":"length","message":{"content":null}}]}`))
	}))
	defer server.Close()

	client := newClient("test-key", DefaultModel, server.Client(), server.URL)
	_, err := client.Complete(context.Background(), ChatRequest{Messages: []Message{{Role: "user", Content: "CV"}}})
	assert.EqualError(t, err, "OpenRouter returned no completion content (length) from provider/model")
}
