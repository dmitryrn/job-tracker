package services

import (
	"context"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

type ResumeService struct {
	repository repositories.ResumeRepository
}

func NewResumeService(repository repositories.ResumeRepository) *ResumeService {
	return &ResumeService{repository: repository}
}

func (service *ResumeService) Resume(ctx context.Context) (*models.Resume, error) {
	return service.repository.Resume(ctx)
}

func (service *ResumeService) Save(ctx context.Context, resume models.Resume) (models.Resume, error) {
	resume.FullName = strings.TrimSpace(resume.FullName)
	resume.Headline = strings.TrimSpace(resume.Headline)
	resume.Location = strings.TrimSpace(resume.Location)
	resume.Email = strings.TrimSpace(resume.Email)
	resume.Phone = strings.TrimSpace(resume.Phone)
	resume.Summary = strings.TrimSpace(resume.Summary)
	resume.Links = cleanResumeLinks(resume.Links)
	resume.Skills = cleanResumeSkills(resume.Skills)
	resume.Competencies = cleanResumeCompetencies(resume.Competencies)
	resume.Experience = cleanResumeExperience(resume.Experience)
	resume.Education = cleanResumeEducation(resume.Education)
	return service.repository.SaveResume(ctx, resume)
}

func cleanResumeLinks(links []models.ResumeLink) []models.ResumeLink {
	cleaned := make([]models.ResumeLink, 0, len(links))
	for _, link := range links {
		link.Label = strings.TrimSpace(link.Label)
		link.URL = strings.TrimSpace(link.URL)
		if link.Label != "" && link.URL != "" {
			cleaned = append(cleaned, link)
		}
	}
	return cleaned
}

func cleanResumeSkills(skills []models.ResumeSkill) []models.ResumeSkill {
	cleaned := make([]models.ResumeSkill, 0, len(skills))
	for _, skill := range skills {
		skill.Name = strings.TrimSpace(skill.Name)
		skill.Level = strings.TrimSpace(skill.Level)
		if skill.Name != "" {
			cleaned = append(cleaned, skill)
		}
	}
	return cleaned
}

func cleanResumeCompetencies(competencies []models.ResumeCompetency) []models.ResumeCompetency {
	cleaned := make([]models.ResumeCompetency, 0, len(competencies))
	for _, competency := range competencies {
		competency.Title = strings.TrimSpace(competency.Title)
		competency.Bullets = cleanResumeBullets(competency.Bullets)
		if competency.Title != "" {
			cleaned = append(cleaned, competency)
		}
	}
	return cleaned
}

func cleanResumeExperience(experience []models.ResumeExperience) []models.ResumeExperience {
	cleaned := make([]models.ResumeExperience, 0, len(experience))
	for _, entry := range experience {
		entry.Company = strings.TrimSpace(entry.Company)
		entry.Title = strings.TrimSpace(entry.Title)
		entry.Location = strings.TrimSpace(entry.Location)
		entry.StartDate = strings.TrimSpace(entry.StartDate)
		entry.EndDate = strings.TrimSpace(entry.EndDate)
		entry.Stack = strings.TrimSpace(entry.Stack)
		entry.Bullets = cleanResumeBullets(entry.Bullets)
		if entry.Company != "" {
			cleaned = append(cleaned, entry)
		}
	}
	return cleaned
}

func cleanResumeEducation(education []models.ResumeEducation) []models.ResumeEducation {
	cleaned := make([]models.ResumeEducation, 0, len(education))
	for _, entry := range education {
		entry.Institution = strings.TrimSpace(entry.Institution)
		entry.Location = strings.TrimSpace(entry.Location)
		entry.Degree = strings.TrimSpace(entry.Degree)
		entry.FieldOfStudy = strings.TrimSpace(entry.FieldOfStudy)
		entry.StartDate = strings.TrimSpace(entry.StartDate)
		entry.EndDate = strings.TrimSpace(entry.EndDate)
		entry.Details = strings.TrimSpace(entry.Details)
		if entry.Institution != "" {
			cleaned = append(cleaned, entry)
		}
	}
	return cleaned
}

func cleanResumeBullets(bullets []string) []string {
	cleaned := make([]string, 0, len(bullets))
	for _, bullet := range bullets {
		if bullet = strings.TrimSpace(bullet); bullet != "" {
			cleaned = append(cleaned, bullet)
		}
	}
	return cleaned
}
