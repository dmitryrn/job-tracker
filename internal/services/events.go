package services

import (
	"context"
	"errors"

	"nice/internal/models"
	"nice/internal/repositories"
)

var ErrInvalidEventSearch = errors.New("invalid event search")

type EventLog struct {
	repository repositories.EventRepository
}

func NewEventLog(repository repositories.EventRepository) *EventLog {
	return &EventLog{repository: repository}
}

func (service *EventLog) Events(ctx context.Context, search models.EventSearch) (models.EventPage, error) {
	if search.Limit < 1 || search.Limit > 100 || search.Offset < 0 {
		return models.EventPage{}, ErrInvalidEventSearch
	}
	return service.repository.Events(ctx, search)
}
