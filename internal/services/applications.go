package services

import (
	"context"

	"nice/internal/models"
	"nice/internal/repositories"
)

type Applications struct {
	repository repositories.ApplicationRepository
}

func NewApplications(repository repositories.ApplicationRepository) *Applications {
	return &Applications{repository: repository}
}

func (applications *Applications) Apply(ctx context.Context, jobID int64) (*models.Application, error) {
	return applications.repository.CreateApplication(ctx, jobID)
}

func (applications *Applications) Unapply(ctx context.Context, jobID int64) (bool, error) {
	return applications.repository.DeleteApplication(ctx, jobID)
}

func (applications *Applications) Application(ctx context.Context, jobID int64) (*models.Application, error) {
	return applications.repository.Application(ctx, jobID)
}

func (applications *Applications) List(ctx context.Context, search models.ApplicationSearch) (models.ApplicationPage, error) {
	return applications.repository.Applications(ctx, search)
}
