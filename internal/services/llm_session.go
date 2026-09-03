package services

import "github.com/google/uuid"

func newLLMSessionID() string {
	return uuid.NewString()
}
