package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"nice/internal/clients/openai"
)

const validatedLLMResponseAttempts = 3

type LLMResponseRejection struct {
	Attempt int
	Model   string
	Reason  string
}

type LLMRetryMetadata struct {
	Rejections []LLMResponseRejection
}

type llmResponseValidationError struct {
	metadata LLMRetryMetadata
	err      error
}

func (err *llmResponseValidationError) Error() string {
	return err.err.Error()
}

func (err *llmResponseValidationError) Unwrap() error {
	return err.err
}

func newLLMSessionID() string {
	return uuid.NewString()
}

func correctedLLMMessages(messages []openai.Message, response string, err error) []openai.Message {
	corrected := make([]openai.Message, len(messages), len(messages)+2)
	copy(corrected, messages)
	return append(corrected,
		openai.Message{Role: "assistant", Content: response},
		openai.Message{Role: "user", Content: fmt.Sprintf("The previous response was invalid: %s\n\nReturn a complete corrected response that satisfies all prior instructions and the required response schema.", err)},
	)
}

func llmResponseRejection(attempt int, model string, err error) LLMResponseRejection {
	reason := "validation_failed"
	message := err.Error()
	switch {
	case strings.Contains(message, "quote") && strings.Contains(message, "not in source"):
		reason = "quote_not_in_source"
	case strings.Contains(message, "JSON"), strings.Contains(message, "invalid character"), strings.Contains(message, "unexpected end of JSON"):
		reason = "invalid_json"
	}

	return LLMResponseRejection{Attempt: attempt, Model: model, Reason: reason}
}

func llmRetryMetadataFromError(err error) (LLMRetryMetadata, bool) {
	var validationErr *llmResponseValidationError
	if !errors.As(err, &validationErr) {
		return LLMRetryMetadata{}, false
	}

	return validationErr.metadata, true
}
