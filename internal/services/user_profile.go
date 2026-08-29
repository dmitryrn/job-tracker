package services

import (
	"context"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

type UserProfileService struct {
	repository repositories.JobRepository
}

func NewUserProfileService(repository repositories.JobRepository) *UserProfileService {
	return &UserProfileService{repository: repository}
}

func (service *UserProfileService) Profile(ctx context.Context) (*models.UserProfile, error) {
	return service.repository.UserProfile(ctx)
}

func (service *UserProfileService) Save(ctx context.Context, profile models.UserProfile) (models.UserProfile, error) {
	profile.Headline = strings.TrimSpace(profile.Headline)
	profile.Location = strings.TrimSpace(profile.Location)
	profile.WorkAuthorization = strings.TrimSpace(profile.WorkAuthorization)
	profile.Summary = strings.TrimSpace(profile.Summary)

	skills := make([]models.UserProfileSkill, 0, len(profile.Skills))
	for _, skill := range profile.Skills {
		skill.Name = strings.TrimSpace(skill.Name)
		skill.Level = strings.TrimSpace(skill.Level)
		skill.Notes = strings.TrimSpace(skill.Notes)
		if skill.Name != "" {
			skills = append(skills, skill)
		}
	}
	profile.Skills = skills
	return service.repository.SaveUserProfile(ctx, profile)
}
