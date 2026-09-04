package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/clients/openai"
	"nice/internal/models"
	"nice/internal/repositories"
)

var (
	ErrJobMatchChatUnavailable         = errors.New("a current match is required to start a chat")
	ErrEmptyJobMatchChatMessage        = errors.New("message must not be empty")
	ErrMissingJobMatchChatRequestID    = errors.New("message request ID is required")
	ErrJobMatchChatUserMessageNotFound = errors.New("user chat message not found")
	ErrJobMatchChatTurnActive          = errors.New("a chat turn is already active")
	ErrJobMatchChatUnansweredMessage   = errors.New("remove the unanswered message before starting another turn")
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
	jobs        repositories.JobRepository
	matches     repositories.JobMatchRepository
	items       repositories.JobMatchChatRepository
	profiles    repositories.UserProfileRepository
	resumes     repositories.ResumeRepository
	client      JobCompletionClient
	model       string
	reasoning   string
	jobLocks    sync.Map
	activeMu    sync.Mutex
	active      map[int64]*jobMatchChatTurn
	subscribers map[int64]map[chan JobMatchChatUpdate]struct{}
	rootContext context.Context
	cancel      context.CancelFunc
	logger      *zap.Logger
}

type jobMatchChatTurn struct {
	requestID string
	cancel    context.CancelFunc
	done      chan struct{}
}

type JobMatchChatUpdate struct {
	Item  *models.JobMatchChatItem
	Reset bool
}

func NewJobMatchChat(jobs repositories.JobRepository, matches repositories.JobMatchRepository, items repositories.JobMatchChatRepository, profiles repositories.UserProfileRepository, resumes repositories.ResumeRepository, client JobCompletionClient, model, reasoning string, logger *zap.Logger) *JobMatchChat {
	rootContext, cancel := context.WithCancel(context.Background())
	return &JobMatchChat{jobs: jobs, matches: matches, items: items, profiles: profiles, resumes: resumes, client: client, model: model, reasoning: reasoning, active: make(map[int64]*jobMatchChatTurn), subscribers: make(map[int64]map[chan JobMatchChatUpdate]struct{}), rootContext: rootContext, cancel: cancel, logger: logger}
}

func (service *JobMatchChat) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{OnStop: service.Shutdown})
}

func (service *JobMatchChat) Shutdown(ctx context.Context) error {
	service.cancel()
	service.activeMu.Lock()
	turns := make([]*jobMatchChatTurn, 0, len(service.active))
	for _, turn := range service.active {
		turns = append(turns, turn)
	}
	service.activeMu.Unlock()
	for _, turn := range turns {
		select {
		case <-turn.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (service *JobMatchChat) Items(ctx context.Context, jobID, afterSequence int64) ([]models.JobMatchChatItem, error) {
	return service.items.JobMatchChatItems(ctx, jobID, afterSequence)
}

func (service *JobMatchChat) Subscribe(jobID int64) (<-chan JobMatchChatUpdate, func()) {
	updates := make(chan JobMatchChatUpdate, 1)
	service.activeMu.Lock()
	if service.subscribers[jobID] == nil {
		service.subscribers[jobID] = make(map[chan JobMatchChatUpdate]struct{})
	}
	service.subscribers[jobID][updates] = struct{}{}
	service.activeMu.Unlock()
	return updates, func() {
		service.activeMu.Lock()
		delete(service.subscribers[jobID], updates)
		if len(service.subscribers[jobID]) == 0 {
			delete(service.subscribers, jobID)
		}
		service.activeMu.Unlock()
	}
}

func (service *JobMatchChat) Send(ctx context.Context, jobID int64, content, requestID string) (models.JobMatchChatItem, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.JobMatchChatItem{}, ErrEmptyJobMatchChatMessage
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return models.JobMatchChatItem{}, ErrMissingJobMatchChatRequestID
	}
	unlock := service.lockJob(jobID)
	defer unlock()

	existing, err := service.items.JobMatchChatItemByRequestID(ctx, jobID, requestID, "user_message")
	if err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("load existing user chat item: %w", err)
	}
	if existing != nil {
		return *existing, nil
	}
	if service.turn(jobID) != nil {
		return models.JobMatchChatItem{}, ErrJobMatchChatTurnActive
	}
	if err := service.ensureInitialItems(ctx, jobID); err != nil {
		return models.JobMatchChatItem{}, err
	}
	history, err := service.items.JobMatchChatItems(ctx, jobID, 0)
	if err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("load job match chat history: %w", err)
	}
	if unansweredUserMessage(history) {
		return models.JobMatchChatItem{}, ErrJobMatchChatUnansweredMessage
	}
	user, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "user_message", RequestID: requestID, Payload: payload(map[string]string{"content": content})})
	if err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("save user chat item: %w", err)
	}
	turnContext, cancel := context.WithCancel(service.rootContext)
	turn := &jobMatchChatTurn{requestID: requestID, cancel: cancel, done: make(chan struct{})}
	service.activeMu.Lock()
	service.active[jobID] = turn
	service.activeMu.Unlock()
	go service.runTurn(turnContext, jobID, requestID, turn)
	return user, nil
}

func (service *JobMatchChat) Stop(jobID int64, requestID string) bool {
	service.activeMu.Lock()
	defer service.activeMu.Unlock()
	turn := service.active[jobID]
	if turn == nil || turn.requestID != requestID {
		return false
	}
	turn.cancel()
	return true
}

func (service *JobMatchChat) lockJob(jobID int64) func() {
	value, _ := service.jobLocks.LoadOrStore(jobID, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (service *JobMatchChat) Revert(ctx context.Context, jobID, sequence int64) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		service.cancelTurnAndWait(ctx, jobID)
		unlock := service.lockJob(jobID)
		if service.turn(jobID) != nil {
			unlock()
			continue
		}
		items, err := service.items.JobMatchChatItems(ctx, jobID, sequence-1)
		if err != nil {
			unlock()
			return fmt.Errorf("load user chat item to revert: %w", err)
		}
		if len(items) == 0 || items[0].Sequence != sequence || items[0].Type != "user_message" {
			unlock()
			return ErrJobMatchChatUserMessageNotFound
		}
		deleted, err := service.items.DeleteJobMatchChatItemsFrom(ctx, jobID, sequence)
		unlock()
		if err != nil {
			return fmt.Errorf("delete chat items to revert: %w", err)
		}
		if !deleted {
			return ErrJobMatchChatUserMessageNotFound
		}
		service.publish(jobID, JobMatchChatUpdate{Reset: true})
		return nil
	}
}

func (service *JobMatchChat) ensureInitialItems(ctx context.Context, jobID int64) error {
	items, err := service.items.JobMatchChatItems(ctx, jobID, 0)
	if err != nil {
		return fmt.Errorf("load initial chat items: %w", err)
	}
	if len(items) > 0 {
		return nil
	}
	match, err := service.matches.JobMatch(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load current job match: %w", err)
	}
	if match == nil {
		return ErrJobMatchChatUnavailable
	}
	job, err := service.jobs.Job(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job for chat: %w", err)
	}
	if job == nil {
		return fmt.Errorf("load job for chat: job %d not found", jobID)
	}
	profile, err := service.profiles.UserProfile(ctx)
	if err != nil {
		return fmt.Errorf("load profile for chat: %w", err)
	}
	resume, err := service.resumes.Resume(ctx)
	if err != nil {
		return fmt.Errorf("load base resume for chat: %w", err)
	}
	if resume == nil {
		return fmt.Errorf("load base resume for chat: no resume found")
	}
	for _, item := range []models.JobMatchChatItem{
		{JobID: jobID, Type: "initial_instructions", Payload: payload(map[string]string{"content": jobMatchChatInstructions})},
		{JobID: jobID, Type: "initial_context", Payload: payload(struct {
			Job     *models.BrowseJob      `json:"job"`
			Match   *models.JobMatchRecord `json:"match"`
			Profile *models.UserProfile    `json:"profile"`
		}{job, match, profile})},
		{JobID: jobID, Type: "tool_definition", Payload: payload(resumePatchTool)},
		{JobID: jobID, Type: "resume_revision", Payload: payload(resumeRevisionPayload{Revision: 0, Resume: *resume, Summary: "Snapshot of the base resume"})},
	} {
		if _, err := service.append(ctx, item); err != nil {
			return fmt.Errorf("save initial chat item: %w", err)
		}
	}
	return nil
}

func (service *JobMatchChat) runTurn(ctx context.Context, jobID int64, requestID string, turn *jobMatchChatTurn) {
	defer close(turn.done)
	defer func() {
		service.activeMu.Lock()
		if service.active[jobID] == turn {
			delete(service.active, jobID)
		}
		service.activeMu.Unlock()
	}()

	unlock := service.lockJob(jobID)
	defer unlock()
	if err := service.executeTurn(ctx, jobID, requestID); err != nil {
		service.logger.Error("job match chat turn failed", zap.Int64("job_id", jobID), zap.String("request_id", requestID), zap.Error(err))
		return
	}
	service.logger.Info("job match chat turn completed", zap.Int64("job_id", jobID), zap.String("request_id", requestID))
}

func (service *JobMatchChat) executeTurn(ctx context.Context, jobID int64, requestID string) error {
	history, err := service.items.JobMatchChatItems(ctx, jobID, 0)
	if err != nil {
		return fmt.Errorf("load turn history: %w", err)
	}
	current, err := latestResumeRevision(history)
	if err != nil {
		return fmt.Errorf("load current application resume revision: %w", err)
	}
	sessionID := newLLMSessionID()
	patchAttempts := 0
	for {
		if err := ctx.Err(); err != nil {
			service.recordTerminal(ctx, jobID, requestID, "turn_stopped", "turn stopped by cancellation")
			service.logger.Info("job match chat turn stopped", zap.Int64("job_id", jobID), zap.String("request_id", requestID))
			return nil
		}
		request, err := providerRequest(history, service.reasoning)
		if err != nil {
			return service.recordFailure(ctx, jobID, requestID, fmt.Errorf("assemble provider request: %w", err))
		}
		if _, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "provider_request", RequestID: requestID, Payload: payload(map[string]any{"model": service.model, "reasoningEffort": service.reasoning, "messageCount": len(request.Messages)})}); err != nil {
			return fmt.Errorf("record provider request: %w", err)
		}
		response, err := service.client.Complete(ctx, service.model, sessionID, request)
		if err != nil {
			if ctx.Err() != nil {
				service.recordTerminal(ctx, jobID, requestID, "turn_stopped", "turn stopped by cancellation")
				service.logger.Info("job match chat turn stopped", zap.Int64("job_id", jobID), zap.String("request_id", requestID))
				return nil
			}
			return service.recordFailure(ctx, jobID, requestID, fmt.Errorf("complete job match chat: %w", err))
		}
		responseItem, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "provider_response", RequestID: requestID, Payload: payload(map[string]any{"model": response.Model, "finishReason": response.FinishReason, "refusal": response.Refusal, "usage": rawJSON(response.Usage), "providerMetadata": rawJSON(response.ProviderMetadata)})})
		if err != nil {
			return fmt.Errorf("record provider response: %w", err)
		}
		history = append(history, responseItem)
		if response.ReasoningSummary != "" || len(response.Reasoning) > 0 {
			reasoning, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "assistant_reasoning", RequestID: requestID, Payload: payload(map[string]any{"summary": response.ReasoningSummary, "compatibility": rawJSON(response.Reasoning)})})
			if err != nil {
				return fmt.Errorf("record assistant reasoning: %w", err)
			}
			history = append(history, reasoning)
		}
		if len(response.ToolCalls) == 0 {
			content := strings.TrimSpace(response.Content)
			if content == "" {
				return service.recordFailure(ctx, jobID, requestID, errors.New("provider returned no assistant message"))
			}
			assistant, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "assistant_message", RequestID: requestID, Payload: payload(map[string]string{"content": content})})
			if err != nil {
				return fmt.Errorf("record assistant message: %w", err)
			}
			history = append(history, assistant)
			service.recordTerminal(ctx, jobID, requestID, "turn_completed", "assistant reply completed")
			return nil
		}
		if len(response.ToolCalls) != 1 {
			return service.recordFailure(ctx, jobID, requestID, fmt.Errorf("provider returned %d tool calls; exactly one is supported", len(response.ToolCalls)))
		}
		toolCall := response.ToolCalls[0]
		toolItem, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "assistant_tool_call", RequestID: requestID, Payload: payload(toolCall)})
		if err != nil {
			return fmt.Errorf("record assistant tool call: %w", err)
		}
		history = append(history, toolItem)
		patchAttempts++
		if toolCall.Type != "function" || toolCall.Function.Name != resumePatchTool.Function.Name {
			return service.recordFailure(ctx, jobID, requestID, fmt.Errorf("unexpected tool call %q", toolCall.Function.Name))
		}
		var patch resumePatch
		err = json.Unmarshal([]byte(toolCall.Function.Arguments), &patch)
		if err == nil && patch.BaseRevision != current.Revision {
			err = fmt.Errorf("patch was based on revision %d, but the current revision is %d", patch.BaseRevision, current.Revision)
		}
		updated := current.Resume
		if err == nil {
			err = applyResumePatch(&updated, patch)
		}
		if err != nil {
			result, resultErr := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "tool_result", RequestID: requestID, Payload: payload(toolResultPayload{ToolCallID: toolCall.ID, Status: "rejected", Error: err.Error()})})
			if resultErr != nil {
				return fmt.Errorf("record rejected tool result: %w", resultErr)
			}
			history = append(history, result)
			if patchAttempts >= resumePatchAttempts {
				service.recordTerminal(ctx, jobID, requestID, "retry_limit_reached", "no resume changes were made after three rejected patches")
				service.recordTerminal(ctx, jobID, requestID, "turn_halted", "resume patch retry limit reached")
				return nil
			}
			retry, retryErr := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "patch_retrying", RequestID: requestID, Payload: payload(map[string]any{"attempt": patchAttempts, "detail": "patch rejected; request a corrected patch"})})
			if retryErr != nil {
				return fmt.Errorf("record patch retry: %w", retryErr)
			}
			history = append(history, retry)
			continue
		}
		result, err := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "tool_result", RequestID: requestID, Payload: payload(toolResultPayload{ToolCallID: toolCall.ID, Status: "accepted", Revision: current.Revision + 1, Resume: &updated, Summary: strings.TrimSpace(patch.Summary)})})
		if err != nil {
			return fmt.Errorf("record accepted tool result: %w", err)
		}
		history = append(history, result)
		current = resumeRevisionPayload{Revision: current.Revision + 1, Resume: updated, Summary: strings.TrimSpace(patch.Summary)}
	}
}

type resumeRevisionPayload struct {
	Revision int           `json:"revision"`
	Resume   models.Resume `json:"resume"`
	Summary  string        `json:"summary"`
}

type toolResultPayload struct {
	ToolCallID string         `json:"toolCallId"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	Revision   int            `json:"revision,omitempty"`
	Resume     *models.Resume `json:"resume,omitempty"`
	Summary    string         `json:"summary,omitempty"`
}

func providerRequest(items []models.JobMatchChatItem, reasoning string) (openai.ChatRequest, error) {
	request := openai.ChatRequest{ReasoningEffort: reasoning, Tools: []openai.Tool{resumePatchTool}}
	for _, item := range items {
		switch item.Type {
		case "initial_instructions":
			var value struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(item.Payload, &value); err != nil {
				return request, err
			}
			request.Messages = append(request.Messages, openai.Message{Role: "system", Content: value.Content})
		case "initial_context":
			request.Messages = append(request.Messages, openai.Message{Role: "system", Content: "Immutable application context:\n" + string(item.Payload)})
		case "resume_revision":
			request.Messages = append(request.Messages, openai.Message{Role: "system", Content: "Application resume revision 0:\n" + string(item.Payload)})
		case "user_message", "assistant_message":
			var value struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(item.Payload, &value); err != nil {
				return request, err
			}
			role := "user"
			if item.Type == "assistant_message" {
				role = "assistant"
			}
			request.Messages = append(request.Messages, openai.Message{Role: role, Content: value.Content})
		case "assistant_tool_call":
			var call openai.ToolCall
			if err := json.Unmarshal(item.Payload, &call); err != nil {
				return request, err
			}
			request.Messages = append(request.Messages, openai.Message{Role: "assistant", ToolCalls: []openai.ToolCall{call}})
		case "tool_result":
			var result toolResultPayload
			if err := json.Unmarshal(item.Payload, &result); err != nil {
				return request, err
			}
			request.Messages = append(request.Messages, openai.Message{Role: "tool", ToolCallID: result.ToolCallID, Content: string(item.Payload)})
		}
	}
	return request, nil
}

func latestResumeRevision(items []models.JobMatchChatItem) (resumeRevisionPayload, error) {
	var current resumeRevisionPayload
	found := false
	for _, item := range items {
		if item.Type == "resume_revision" {
			if err := json.Unmarshal(item.Payload, &current); err != nil {
				return current, err
			}
			found = true
		}
		if item.Type == "tool_result" {
			var result toolResultPayload
			if err := json.Unmarshal(item.Payload, &result); err != nil {
				return current, err
			}
			if result.Status == "accepted" && result.Resume != nil {
				current = resumeRevisionPayload{Revision: result.Revision, Resume: *result.Resume, Summary: result.Summary}
				found = true
			}
		}
	}
	if !found {
		return current, errors.New("initial resume revision not found")
	}
	return current, nil
}

func unansweredUserMessage(items []models.JobMatchChatItem) bool {
	last := ""
	for _, item := range items {
		if item.Type == "user_message" || item.Type == "assistant_message" {
			last = item.Type
		}
	}
	return last == "user_message"
}

func payload(value any) json.RawMessage { encoded, _ := json.Marshal(value); return encoded }
func rawJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return json.RawMessage(value)
}

func (service *JobMatchChat) append(ctx context.Context, item models.JobMatchChatItem) (models.JobMatchChatItem, error) {
	created, err := service.items.CreateJobMatchChatItem(ctx, item)
	if err == nil {
		service.publish(created.JobID, JobMatchChatUpdate{Item: &created})
	}
	return created, err
}

func (service *JobMatchChat) publish(jobID int64, update JobMatchChatUpdate) {
	service.activeMu.Lock()
	defer service.activeMu.Unlock()
	for subscriber := range service.subscribers[jobID] {
		select {
		case subscriber <- update:
		default:
		}
	}
}

func (service *JobMatchChat) turn(jobID int64) *jobMatchChatTurn {
	service.activeMu.Lock()
	defer service.activeMu.Unlock()
	return service.active[jobID]
}

func (service *JobMatchChat) cancelTurnAndWait(ctx context.Context, jobID int64) {
	service.activeMu.Lock()
	turn := service.active[jobID]
	service.activeMu.Unlock()
	if turn == nil {
		return
	}
	turn.cancel()
	select {
	case <-turn.done:
	case <-ctx.Done():
	}
}

func (service *JobMatchChat) recordFailure(ctx context.Context, jobID int64, requestID string, err error) error {
	if _, recordErr := service.append(ctx, models.JobMatchChatItem{JobID: jobID, Type: "turn_error", RequestID: requestID, Payload: payload(map[string]string{"error": err.Error()})}); recordErr != nil {
		return fmt.Errorf("%w; record turn error: %v", err, recordErr)
	}
	service.recordTerminal(ctx, jobID, requestID, "turn_halted", "provider turn failed")
	return err
}

func (service *JobMatchChat) recordTerminal(ctx context.Context, jobID int64, requestID, itemType, detail string) {
	if _, err := service.append(context.WithoutCancel(ctx), models.JobMatchChatItem{JobID: jobID, Type: itemType, RequestID: requestID, Payload: payload(map[string]string{"detail": detail})}); err != nil {
		service.logger.Error("record job match chat terminal item failed", zap.Int64("job_id", jobID), zap.String("request_id", requestID), zap.String("item_type", itemType), zap.Error(err))
	}
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
