package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
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

const jobMatchChatInstructions = `You are a thoughtful job-search assistant. Help the candidate discuss this specific job, its current match assessment, their profile, and their application resume. Be candid, practical, and concise. Do not claim the candidate has experience or qualifications that are not in the supplied context. When the user asks to edit or tailor the resume, call revise_application_resume with narrow, factual changes instead of describing hypothetical edits. Ask clarifying questions when useful.`

const resumePatchAttempts = 3

var resumePatchTool = openai.Tool{
	Type: "function",
	Function: openai.ToolFunction{
		Name:        "revise_application_resume",
		Description: "Apply narrowly targeted, factual resume tailoring changes. Use exact expected text from the application resume and never invent experience, credentials, employers, or dates.",
		Parameters:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["baseRevision","summary","operations"],"properties":{"baseRevision":{"type":"integer","minimum":0},"summary":{"type":"string","minLength":1},"operations":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"object","additionalProperties":false,"required":["op","section","id","parentId","expected","value"],"properties":{"op":{"type":"string","enum":["replace","add","remove"]},"section":{"type":"string","enum":["headline","summary","skill","competencyBullet","experienceBullet"]},"id":{"type":"integer","minimum":0},"parentId":{"type":"integer","minimum":0},"expected":{"type":"string"},"value":{"type":"string"}}}}}}`),
	},
}

type resumePatch struct {
	BaseRevision int                    `json:"baseRevision"`
	Summary      string                 `json:"summary"`
	Operations   []resumePatchOperation `json:"operations"`
}

type resumePatchOperation struct {
	Op       string `json:"op"`
	Section  string `json:"section"`
	ID       int64  `json:"id"`
	ParentID int64  `json:"parentId"`
	Expected string `json:"expected"`
	Value    string `json:"value"`
}

type JobMatchChat struct {
	jobs         repositories.JobRepository
	matches      repositories.JobMatchRepository
	messages     repositories.JobMatchChatRepository
	profiles     repositories.UserProfileRepository
	resumes      repositories.ResumeRepository
	applications repositories.ApplicationResumeRepository
	client       JobCompletionClient
	model        string
	reasoning    string
	jobLocks     sync.Map
}

func NewJobMatchChat(jobs repositories.JobRepository, matches repositories.JobMatchRepository, messages repositories.JobMatchChatRepository, profiles repositories.UserProfileRepository, resumes repositories.ResumeRepository, applications repositories.ApplicationResumeRepository, client JobCompletionClient, model, reasoning string) *JobMatchChat {
	return &JobMatchChat{jobs: jobs, matches: matches, messages: messages, profiles: profiles, resumes: resumes, applications: applications, client: client, model: model, reasoning: reasoning}
}

func (service *JobMatchChat) Messages(ctx context.Context, jobID int64) ([]models.JobMatchChatMessage, error) {
	return service.messages.JobMatchChatMessages(ctx, jobID)
}

func (service *JobMatchChat) Revert(ctx context.Context, jobID, messageID int64) error {
	unlock := service.lockJob(jobID)
	defer unlock()
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

func (service *JobMatchChat) ApplicationResume(ctx context.Context, jobID int64) (*models.ApplicationResume, []models.ApplicationResumeAgentEvent, error) {
	application, err := service.applications.ApplicationResume(ctx, jobID)
	if err != nil || application == nil {
		return application, nil, err
	}
	events, err := service.applications.ApplicationResumeAgentEvents(ctx, jobID)
	if err != nil {
		return nil, nil, err
	}
	return application, events, nil
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
	unlock := service.lockJob(jobID)
	defer unlock()

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

	userMessage, err := service.messages.CreateJobMatchChatMessage(ctx, models.JobMatchChatMessage{
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
	if resume == nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load base resume for chat: no resume found")
	}
	application, created, err := service.applications.CreateApplicationResume(ctx, jobID, userMessage.ID, *resume)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("create application resume: %w", err)
	}
	if created {
		if _, err := service.applications.CreateApplicationResumeAgentEvent(ctx, models.ApplicationResumeAgentEvent{TriggerMessageID: userMessage.ID, Type: "resume_snapshot_created", Detail: "Revision 0 created from the base resume"}); err != nil {
			return models.JobMatchChatMessage{}, fmt.Errorf("record application resume snapshot event: %w", err)
		}
	}
	if len(application.Revisions) == 0 {
		return models.JobMatchChatMessage{}, fmt.Errorf("load application resume: no snapshot found")
	}
	currentRevision := application.Revisions[len(application.Revisions)-1]
	history, err := service.messages.JobMatchChatMessages(ctx, jobID)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("load job match chat history: %w", err)
	}

	contextContent, err := json.Marshal(struct {
		Job      *models.BrowseJob      `json:"job"`
		Match    *models.JobMatchRecord `json:"match"`
		Profile  *models.UserProfile    `json:"profile"`
		Resume   models.Resume          `json:"applicationResume"`
		Revision int                    `json:"applicationResumeRevision"`
	}{job, match, profile, currentRevision.Resume, currentRevision.RevisionNumber})
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("encode job match chat context: %w", err)
	}
	request := openai.ChatRequest{
		Messages:        []openai.Message{{Role: "system", Content: jobMatchChatInstructions + "\n\nCurrent context:\n" + string(contextContent)}},
		ReasoningEffort: service.reasoning,
		Tools:           []openai.Tool{resumePatchTool},
	}
	for _, message := range history {
		request.Messages = append(request.Messages, openai.Message{Role: message.Role, Content: message.Content})
	}
	return service.completeReply(ctx, jobID, userMessage, requestID, application, currentRevision, request)
}

func (service *JobMatchChat) lockJob(jobID int64) func() {
	value, _ := service.jobLocks.LoadOrStore(jobID, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (service *JobMatchChat) completeReply(ctx context.Context, jobID int64, userMessage models.JobMatchChatMessage, requestID string, application *models.ApplicationResume, currentRevision models.ApplicationResumeRevision, request openai.ChatRequest) (models.JobMatchChatMessage, error) {
	sessionID := newLLMSessionID()
	for attempt := 1; attempt <= resumePatchAttempts; attempt++ {
		response, err := service.client.Complete(ctx, service.model, sessionID, request)
		if err != nil {
			return models.JobMatchChatMessage{}, fmt.Errorf("complete job match chat: %w", err)
		}
		if len(response.ToolCalls) == 0 {
			return service.saveAssistantMessage(ctx, jobID, requestID, response.Content)
		}
		toolCall := response.ToolCalls[0]
		if toolCall.Type != "function" || toolCall.Function.Name != resumePatchTool.Function.Name {
			return models.JobMatchChatMessage{}, fmt.Errorf("complete job match chat: unexpected tool call %q", toolCall.Function.Name)
		}
		var patch resumePatch
		err = json.Unmarshal([]byte(toolCall.Function.Arguments), &patch)
		if err == nil && patch.BaseRevision != currentRevision.RevisionNumber {
			err = fmt.Errorf("patch was based on revision %d, but the current revision is %d", patch.BaseRevision, currentRevision.RevisionNumber)
		}
		updated := currentRevision.Resume
		if err == nil {
			err = applyResumePatch(&updated, patch)
		}
		if err != nil {
			detail := err.Error()
			if _, eventErr := service.applications.CreateApplicationResumeAgentEvent(ctx, models.ApplicationResumeAgentEvent{TriggerMessageID: userMessage.ID, Type: "patch_rejected", Detail: detail}); eventErr != nil {
				return models.JobMatchChatMessage{}, fmt.Errorf("record rejected resume patch: %w", eventErr)
			}
			if attempt == resumePatchAttempts {
				if _, eventErr := service.applications.CreateApplicationResumeAgentEvent(ctx, models.ApplicationResumeAgentEvent{TriggerMessageID: userMessage.ID, Type: "retry_limit_reached", Detail: "No resume changes were made"}); eventErr != nil {
					return models.JobMatchChatMessage{}, fmt.Errorf("record resume patch retry limit: %w", eventErr)
				}
				return service.saveAssistantMessage(ctx, jobID, requestID, "I couldn't safely apply a resume change after three attempts. No resume changes were made.")
			}
			request.Messages = append(request.Messages,
				openai.Message{Role: "assistant", Content: response.Content, ToolCalls: response.ToolCalls},
				openai.Message{Role: "tool", ToolCallID: toolCall.ID, Content: `{"status":"rejected","error":` + jsonString(detail) + `}`},
			)
			if _, eventErr := service.applications.CreateApplicationResumeAgentEvent(ctx, models.ApplicationResumeAgentEvent{TriggerMessageID: userMessage.ID, Type: "patch_retrying", Detail: fmt.Sprintf("Attempt %d was rejected; asking the model to correct it", attempt)}); eventErr != nil {
				return models.JobMatchChatMessage{}, fmt.Errorf("record resume patch retry: %w", eventErr)
			}
			continue
		}
		if source, err := service.messages.JobMatchChatMessageByRequestID(ctx, jobID, requestID, "user"); err != nil || source == nil {
			if err != nil {
				return models.JobMatchChatMessage{}, fmt.Errorf("verify chat message before resume revision: %w", err)
			}
			return models.JobMatchChatMessage{}, fmt.Errorf("chat message was reverted before resume revision could be saved")
		}
		if current, err := service.applications.ApplicationResume(ctx, jobID); err != nil || current == nil {
			if err != nil {
				return models.JobMatchChatMessage{}, fmt.Errorf("verify application resume before revision: %w", err)
			}
			return models.JobMatchChatMessage{}, fmt.Errorf("application resume was reverted before revision could be saved")
		}
		assistant, err := service.saveAssistantMessage(ctx, jobID, requestID, patch.Summary)
		if err != nil {
			return models.JobMatchChatMessage{}, err
		}
		revision, err := service.applications.CreateApplicationResumeRevision(ctx, application.ID, userMessage.ID, assistant.ID, updated, strings.TrimSpace(patch.Summary))
		if err != nil {
			return models.JobMatchChatMessage{}, fmt.Errorf("save application resume revision: %w", err)
		}
		if _, err := service.applications.CreateApplicationResumeAgentEvent(ctx, models.ApplicationResumeAgentEvent{TriggerMessageID: userMessage.ID, RevisionID: revision.ID, Type: "revision_created", Detail: fmt.Sprintf("Revision %d created: %s", revision.RevisionNumber, revision.Summary)}); err != nil {
			return models.JobMatchChatMessage{}, fmt.Errorf("record application resume revision: %w", err)
		}
		return assistant, nil
	}
	return models.JobMatchChatMessage{}, fmt.Errorf("complete job match chat: exhausted resume patch attempts")
}

func (service *JobMatchChat) saveAssistantMessage(ctx context.Context, jobID int64, requestID, content string) (models.JobMatchChatMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		content = "I couldn't produce a response for that request."
	}
	message, err := service.messages.CreateJobMatchChatMessage(ctx, models.JobMatchChatMessage{JobID: jobID, Role: "assistant", Content: content, CreatedAt: time.Now().UTC().Format(time.RFC3339), RequestID: requestID})
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("save assistant chat message: %w", err)
	}
	return message, nil
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func applyResumePatch(resume *models.Resume, patch resumePatch) error {
	if strings.TrimSpace(patch.Summary) == "" || len(patch.Operations) == 0 {
		return errors.New("patch must include a summary and at least one operation")
	}
	for _, operation := range patch.Operations {
		value := strings.TrimSpace(operation.Value)
		expected := strings.TrimSpace(operation.Expected)
		switch operation.Section {
		case "headline":
			if operation.Op != "replace" || operation.ID != 0 || strings.TrimSpace(resume.Headline) != expected || value == "" {
				return errors.New("headline replacement did not match the current value")
			}
			resume.Headline = value
		case "summary":
			if err := patchResumeText(&resume.SummaryParagraphs, operation, value, expected); err != nil {
				return fmt.Errorf("summary patch: %w", err)
			}
		case "skill":
			if err := patchResumeSkill(&resume.Skills, operation, value, expected); err != nil {
				return fmt.Errorf("skill patch: %w", err)
			}
		case "competencyBullet":
			competency := findCompetency(resume.Competencies, operation.ParentID)
			if competency == nil {
				return fmt.Errorf("competency %d does not exist", operation.ParentID)
			}
			if err := patchResumeText(&competency.Bullets, operation, value, expected); err != nil {
				return fmt.Errorf("competency bullet patch: %w", err)
			}
		case "experienceBullet":
			experience := findExperience(resume.Experience, operation.ParentID)
			if experience == nil {
				return fmt.Errorf("experience %d does not exist", operation.ParentID)
			}
			if err := patchResumeText(&experience.Bullets, operation, value, expected); err != nil {
				return fmt.Errorf("experience bullet patch: %w", err)
			}
		default:
			return fmt.Errorf("unsupported patch section %q", operation.Section)
		}
	}
	return nil
}

func patchResumeText(values *[]models.ResumeText, operation resumePatchOperation, value, expected string) error {
	switch operation.Op {
	case "replace":
		for index := range *values {
			if (*values)[index].ID == operation.ID {
				if strings.TrimSpace((*values)[index].Content) != expected || value == "" {
					return errors.New("expected text did not match")
				}
				(*values)[index].Content = value
				return nil
			}
		}
		return fmt.Errorf("text ID %d does not exist", operation.ID)
	case "add":
		if operation.ID != 0 || value == "" {
			return errors.New("new text must have no ID and nonempty content")
		}
		*values = append(*values, models.ResumeText{ID: nextResumeTextID(*values), Content: value})
		return nil
	case "remove":
		for index := range *values {
			if (*values)[index].ID == operation.ID {
				if strings.TrimSpace((*values)[index].Content) != expected {
					return errors.New("expected text did not match")
				}
				*values = append((*values)[:index], (*values)[index+1:]...)
				return nil
			}
		}
		return fmt.Errorf("text ID %d does not exist", operation.ID)
	default:
		return fmt.Errorf("unsupported operation %q", operation.Op)
	}
}

func patchResumeSkill(values *[]models.ResumeSkill, operation resumePatchOperation, value, expected string) error {
	switch operation.Op {
	case "replace":
		for index := range *values {
			if (*values)[index].ID == operation.ID {
				if strings.TrimSpace((*values)[index].Name) != expected || value == "" {
					return errors.New("expected skill did not match")
				}
				(*values)[index].Name = value
				return nil
			}
		}
	case "add":
		if operation.ID == 0 && value != "" {
			*values = append(*values, models.ResumeSkill{ID: nextResumeSkillID(*values), Name: value})
			return nil
		}
	case "remove":
		for index := range *values {
			if (*values)[index].ID == operation.ID {
				if strings.TrimSpace((*values)[index].Name) != expected {
					return errors.New("expected skill did not match")
				}
				*values = append((*values)[:index], (*values)[index+1:]...)
				return nil
			}
		}
	default:
		return fmt.Errorf("unsupported operation %q", operation.Op)
	}
	return fmt.Errorf("skill ID %d does not exist", operation.ID)
}

func findCompetency(values []models.ResumeCompetency, id int64) *models.ResumeCompetency {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}

func findExperience(values []models.ResumeExperience, id int64) *models.ResumeExperience {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}

func nextResumeTextID(values []models.ResumeText) int64 {
	var maximum int64
	for _, value := range values {
		if value.ID > maximum {
			maximum = value.ID
		}
	}
	return maximum + 1
}

func nextResumeSkillID(values []models.ResumeSkill) int64 {
	var maximum int64
	for _, value := range values {
		if value.ID > maximum {
			maximum = value.ID
		}
	}
	return maximum + 1
}
