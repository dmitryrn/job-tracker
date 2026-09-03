package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nice/internal/clients/openai"
	"nice/internal/models"
	"nice/internal/repositories"
)

var (
	ErrJobMatchChatUnavailable         = errors.New("a current match is required to start a chat")
	ErrEmptyJobMatchChatMessage        = errors.New("message must not be empty")
	ErrMissingJobMatchChatRequestID    = errors.New("message request ID is required")
	ErrJobMatchChatUserMessageNotFound = errors.New("user chat message not found")
)

const jobMatchChatInstructions = `You are a thoughtful job-search assistant. Help the candidate discuss this specific job, its current match assessment, their profile, and their base resume. Be candid, practical, and concise. Do not claim the candidate has experience or qualifications that are not in the supplied context. Ask clarifying questions when useful.`

type JobMatchChat struct {
	jobs      repositories.JobRepository
	matches   repositories.JobMatchRepository
	messages  repositories.JobMatchChatRepository
	profiles  repositories.UserProfileRepository
	resumes   repositories.ResumeRepository
	client    JobCompletionClient
	model     string
	reasoning string
}

func NewJobMatchChat(jobs repositories.JobRepository, matches repositories.JobMatchRepository, messages repositories.JobMatchChatRepository, profiles repositories.UserProfileRepository, resumes repositories.ResumeRepository, client JobCompletionClient, model, reasoning string) *JobMatchChat {
	return &JobMatchChat{jobs: jobs, matches: matches, messages: messages, profiles: profiles, resumes: resumes, client: client, model: model, reasoning: reasoning}
}

func (service *JobMatchChat) Messages(ctx context.Context, jobID int64) ([]models.JobMatchChatMessage, error) {
	return service.messages.JobMatchChatMessages(ctx, jobID)
}

func (service *JobMatchChat) Revert(ctx context.Context, jobID, messageID int64) error {
	message, err := service.messages.JobMatchChatMessage(ctx, jobID, messageID)
	if err != nil {
		return fmt.Errorf("load user chat message to revert: %w", err)
	}
	if message == nil || message.Role != "user" {
		return ErrJobMatchChatUserMessageNotFound
	}
	deleted, err := service.messages.DeleteJobMatchChatMessagesFrom(ctx, jobID, messageID)
	if err != nil {
		return fmt.Errorf("delete chat messages to revert: %w", err)
	}
	if !deleted {
		return ErrJobMatchChatUserMessageNotFound
	}
	return nil
}

func (service *JobMatchChat) Reply(ctx context.Context, jobID int64, content, requestID string) (models.JobMatchChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.JobMatchChatMessage{}, ErrEmptyJobMatchChatMessage
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return models.JobMatchChatMessage{}, ErrMissingJobMatchChatRequestID
	}

	match, err := service.matches.JobMatch(ctx, jobID)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load current job match: %w", err)
	}
	if match == nil {
		return models.JobMatchChatMessage{}, ErrJobMatchChatUnavailable
	}
	existingAssistantMessage, err := service.messages.JobMatchChatMessageByRequestID(ctx, jobID, requestID, "assistant")
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load existing assistant chat message: %w", err)
	}
	if existingAssistantMessage != nil {
		return *existingAssistantMessage, nil
	}

	_, err = service.messages.CreateJobMatchChatMessage(ctx, models.JobMatchChatMessage{
		JobID:     jobID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
	})
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("save user chat message: %w", err)
	}

	job, err := service.jobs.Job(ctx, jobID)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load job for chat: %w", err)
	}
	if job == nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load job for chat: job %d not found", jobID)
	}
	profile, err := service.profiles.UserProfile(ctx)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load profile for chat: %w", err)
	}
	resume, err := service.resumes.Resume(ctx)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load base resume for chat: %w", err)
	}
	history, err := service.messages.JobMatchChatMessages(ctx, jobID)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load job match chat history: %w", err)
	}

	contextContent, err := json.Marshal(struct {
		Job     *models.BrowseJob      `json:"job"`
		Match   *models.JobMatchRecord `json:"match"`
		Profile *models.UserProfile    `json:"profile"`
		Resume  *models.Resume         `json:"resume"`
	}{job, match, profile, resume})
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("encode job match chat context: %w", err)
	}
	request := openai.ChatRequest{
		Messages:        []openai.Message{{Role: "system", Content: jobMatchChatInstructions + "\n\nCurrent context:\n" + string(contextContent)}},
		ReasoningEffort: service.reasoning,
	}
	for _, message := range history {
		request.Messages = append(request.Messages, openai.Message{Role: message.Role, Content: message.Content})
	}
	response, err := service.client.Complete(ctx, service.model, newLLMSessionID(), request)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("complete job match chat: %w", err)
	}

	assistantMessage, err := service.messages.CreateJobMatchChatMessage(ctx, models.JobMatchChatMessage{
		JobID:     jobID,
		Role:      "assistant",
		Content:   strings.TrimSpace(response.Content),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		RequestID: requestID,
	})
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("save assistant chat message: %w", err)
	}
	return assistantMessage, nil
}
