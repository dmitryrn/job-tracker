package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"nice/internal/clients/openai"
	"nice/internal/models"
	"nice/internal/repositories"
)

type jobMatchChatRepositoryStub struct {
	items   []models.JobMatchChatItem
	created []models.JobMatchChatItem
}

type resumeRepositoryStub struct {
	resume *models.Resume
}

func TestExecuteTurnAppendsSuccessfulAssistantReply(t *testing.T) {
	items := &jobMatchChatRepositoryStub{items: resumeRevisionItems(t)}
	resumeRepository := &resumeRepositoryStub{resume: &models.Resume{Country: "Canada"}}
	client := &fakeJobCompletionClient{response: openai.ChatResponse{Model: "chat-model", Content: "Here is the updated application."}}
	service := newExecuteTurnService(items, resumeRepository, client)

	err := service.executeTurn(context.Background(), 1, "request-1")

	require.NoError(t, err)
	assert.Equal(t, []string{"provider_request", "provider_response", "assistant_message", "turn_completed"}, createdChatItemTypes(items.created))
	assert.Equal(t, "request-1", items.created[0].RequestID)
	assert.Equal(t, "Here is the updated application.", chatItemContent(t, items.created[2]))
	assert.Equal(t, "chat-model", client.model)
	assert.NotEmpty(t, client.session)
}

func TestExecuteTurnRecordsProviderFailure(t *testing.T) {
	items := &jobMatchChatRepositoryStub{items: resumeRevisionItems(t)}
	client := &fakeJobCompletionClient{err: errors.New("provider unavailable")}
	service := newExecuteTurnService(items, &resumeRepositoryStub{resume: &models.Resume{Country: "Canada"}}, client)

	err := service.executeTurn(context.Background(), 1, "request-1")

	require.EqualError(t, err, "complete job match chat: provider unavailable")
	assert.Equal(t, []string{"provider_request", "turn_error", "turn_halted"}, createdChatItemTypes(items.created))
}

func TestExecuteTurnRecordsCancellationBeforeProviderRequest(t *testing.T) {
	items := &jobMatchChatRepositoryStub{items: resumeRevisionItems(t)}
	service := newExecuteTurnService(items, &resumeRepositoryStub{resume: &models.Resume{Country: "Canada"}}, &fakeJobCompletionClient{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.executeTurn(ctx, 1, "request-1")

	require.NoError(t, err)
	assert.Equal(t, []string{"turn_stopped"}, createdChatItemTypes(items.created))
}

func TestExecuteTurnAcceptsResumePatchAndContinues(t *testing.T) {
	items := &jobMatchChatRepositoryStub{items: resumeRevisionItemsWithResume(t, models.Resume{Country: "Canada", Headline: "Old headline"})}
	client := &fakeJobCompletionClient{responses: []openai.ChatResponse{
		{Model: "chat-model", ToolCalls: []openai.ToolCall{{
			ID: "call-1", Type: "function", Function: openai.ToolFunction{Name: resumePatchTool.Function.Name, Arguments: `{"baseRevision":0,"operations":[{"op":"replace","section":"headline","expected":"Old headline","value":"New headline"}]}`},
		}}},
		{Model: "chat-model", Content: "The application is ready."},
	}}
	service := newExecuteTurnService(items, &resumeRepositoryStub{resume: &models.Resume{Country: "Canada"}}, client)

	err := service.executeTurn(context.Background(), 1, "request-1")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"provider_request", "provider_response", "assistant_tool_call", "tool_result",
		"provider_request", "provider_response", "assistant_message", "turn_completed",
	}, createdChatItemTypes(items.created))
	assert.Contains(t, string(items.created[3].Payload), `"status":"accepted"`)
	assert.Len(t, client.requests, 2)
}

func TestExecuteTurnRejectsResumePatchThenRetries(t *testing.T) {
	items := &jobMatchChatRepositoryStub{items: resumeRevisionItemsWithResume(t, models.Resume{Country: "Canada"})}
	client := &fakeJobCompletionClient{responses: []openai.ChatResponse{
		{Model: "chat-model", ToolCalls: []openai.ToolCall{{
			ID: "call-1", Type: "function", Function: openai.ToolFunction{Name: resumePatchTool.Function.Name, Arguments: `{"baseRevision":0,"operations":[]}`},
		}}},
		{Model: "chat-model", Content: "Please review the application."},
	}}
	service := newExecuteTurnService(items, &resumeRepositoryStub{resume: &models.Resume{Country: "Canada"}}, client)

	err := service.executeTurn(context.Background(), 1, "request-1")

	require.NoError(t, err)
	assert.Contains(t, createdChatItemTypes(items.created), "patch_retrying")
	assert.Contains(t, string(items.created[3].Payload), `"status":"rejected"`)
}

func TestExecuteTurnAcceptsCoverLetterCreation(t *testing.T) {
	items := &jobMatchChatRepositoryStub{items: resumeRevisionItems(t)}
	client := &fakeJobCompletionClient{responses: []openai.ChatResponse{
		{Model: "chat-model", ToolCalls: []openai.ToolCall{{
			ID: "call-1", Type: "function", Function: openai.ToolFunction{Name: createCoverLetterTool.Function.Name, Arguments: `{"content":"Dear hiring team"}`},
		}}},
		{Model: "chat-model", Content: "The cover letter is ready."},
	}}
	service := newExecuteTurnService(items, &resumeRepositoryStub{resume: &models.Resume{Country: "Canada"}}, client)

	err := service.executeTurn(context.Background(), 1, "request-1")

	require.NoError(t, err)
	assert.Contains(t, createdChatItemTypes(items.created), "tool_result")
	assert.Contains(t, string(items.created[3].Payload), `"status":"accepted"`)
}

func TestProviderRequestIncludesCoverLetterAndAssistantMessages(t *testing.T) {
	request, err := providerRequest([]models.JobMatchChatItem{
		{Type: "cover_letter_revision", Payload: payload(coverLetterRevisionPayload{Revision: 2, Content: "Dear hiring team"})},
		{Type: "user_message", Payload: payload(map[string]string{"content": "Please revise this."})},
		{Type: "assistant_message", Payload: payload(map[string]string{"content": "I will revise it."})},
		{Type: "unknown_item", Payload: payload(map[string]string{"content": "ignored"})},
	}, "medium", "Canada")

	require.NoError(t, err)
	require.Len(t, request.Messages, 3)
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Contains(t, request.Messages[0].Content, "Application cover letter revision 2:")
	assert.Equal(t, "user", request.Messages[1].Role)
	assert.Equal(t, "assistant", request.Messages[2].Role)
	assert.Equal(t, "medium", request.ReasoningEffort)
}

func TestProviderRequestReturnsMessagePayloadErrors(t *testing.T) {
	tests := []struct {
		name string
		item models.JobMatchChatItem
	}{
		{name: "initial instructions", item: models.JobMatchChatItem{Type: "initial_instructions", Payload: []byte("{")}},
		{name: "assistant tool call", item: models.JobMatchChatItem{Type: "assistant_tool_call", Payload: []byte("{")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := providerRequest([]models.JobMatchChatItem{test.item}, "low", "Canada")

			assert.Error(t, err)
		})
	}
}

func newExecuteTurnService(items repositories.JobMatchChatRepository, resumes repositories.ResumeRepository, client JobCompletionClient) *JobMatchChat {
	return NewJobMatchChat(nil, nil, items, nil, resumes, client, "chat-model", "low", zap.NewNop())
}

func resumeRevisionItems(t *testing.T) []models.JobMatchChatItem {
	t.Helper()
	return resumeRevisionItemsWithResume(t, models.Resume{Country: "Canada"})
}

func resumeRevisionItemsWithResume(t *testing.T, resume models.Resume) []models.JobMatchChatItem {
	t.Helper()
	return []models.JobMatchChatItem{{
		JobID:   1,
		Type:    "resume_revision",
		Payload: payload(resumeRevisionPayload{Revision: 0, Resume: resume}),
	}}
}

func createdChatItemTypes(items []models.JobMatchChatItem) []string {
	types := make([]string, 0, len(items))
	for _, item := range items {
		types = append(types, item.Type)
	}

	return types
}

func (repository *jobMatchChatRepositoryStub) JobMatchChatItems(context.Context, int64, int64) ([]models.JobMatchChatItem, error) {
	return append([]models.JobMatchChatItem(nil), repository.items...), nil
}

func (repository *jobMatchChatRepositoryStub) JobMatchChatItemByRequestID(context.Context, int64, string, string) (*models.JobMatchChatItem, error) {
	return nil, errors.New("chat item lookup method not used")
}

func (repository *jobMatchChatRepositoryStub) CreateJobMatchChatItem(_ context.Context, item models.JobMatchChatItem) (models.JobMatchChatItem, error) {
	item.Sequence = int64(len(repository.items) + len(repository.created) + 1)
	repository.created = append(repository.created, item)
	return item, nil
}

func (repository *jobMatchChatRepositoryStub) DeleteJobMatchChatItemsFrom(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (repository *resumeRepositoryStub) Resume(context.Context) (*models.Resume, error) {
	return repository.resume, nil
}

func (repository *resumeRepositoryStub) SaveResume(_ context.Context, resume models.Resume) (models.Resume, error) {
	return resume, nil
}

func (repository *resumeRepositoryStub) ResumePhoto(context.Context) (*models.ResumePhoto, error) {
	return nil, errors.New("resume photo method not used")
}

func (repository *resumeRepositoryStub) SaveResumePhoto(context.Context, models.ResumePhoto) error {
	return nil
}

func chatItemContent(t *testing.T, item models.JobMatchChatItem) string {
	t.Helper()
	var value struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.Unmarshal(item.Payload, &value))
	return value.Content
}
