package services

import (
	"context"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

type UserProfileService struct {
	repository repositories.UserProfileRepository
}

func NewUserProfileService(repository repositories.UserProfileRepository) *UserProfileService {
	return &UserProfileService{repository: repository}
}

func (service *UserProfileService) Profile(ctx context.Context) (*models.UserProfile, error) {
	return service.repository.UserProfile(ctx)
}

func (service *UserProfileService) Save(ctx context.Context, profile models.UserProfile) (models.UserProfile, error) {
	profile.Headline = strings.TrimSpace(profile.Headline)
	profile.WorkAuthorization = strings.TrimSpace(profile.WorkAuthorization)
	profile.GitHubURL = strings.TrimSpace(profile.GitHubURL)
	profile.LinkedInURL = strings.TrimSpace(profile.LinkedInURL)
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

	workHistory := make([]models.UserProfileWorkHistory, 0, len(profile.WorkHistory))
	for _, experience := range profile.WorkHistory {
		experience.Company = strings.TrimSpace(experience.Company)
		experience.Title = strings.TrimSpace(experience.Title)
		experience.StartDate = strings.TrimSpace(experience.StartDate)
		experience.EndDate = strings.TrimSpace(experience.EndDate)
		experience.Body = strings.TrimSpace(experience.Body)
		if experience.Company != "" {
			workHistory = append(workHistory, experience)
		}
	}

	profile.WorkHistory = workHistory

	education := make([]models.UserProfileEducation, 0, len(profile.Education))
	for _, entry := range profile.Education {
		entry.Institution = strings.TrimSpace(entry.Institution)
		entry.Degree = strings.TrimSpace(entry.Degree)
		entry.StartDate = strings.TrimSpace(entry.StartDate)
		entry.EndDate = strings.TrimSpace(entry.EndDate)
		entry.Body = strings.TrimSpace(entry.Body)
		if entry.Institution != "" {
			education = append(education, entry)
		}
	}

	profile.Education = education
	return service.repository.SaveUserProfile(ctx, profile)
}
