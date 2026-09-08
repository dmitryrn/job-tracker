package services

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"nice/internal/models"
	"nice/internal/repositories"
)

const MaxResumePhotoBytes = 5 << 20

var (
	ErrInvalidResumePhoto = errors.New("resume photo must be a JPEG or PNG smaller than 5 MB")
	ErrResumeNotFound     = errors.New("base resume not found")
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
	resume.Town = strings.TrimSpace(resume.Town)
	resume.Country = strings.TrimSpace(resume.Country)
	resume.Email = strings.TrimSpace(resume.Email)
	resume.Phone = strings.TrimSpace(resume.Phone)
	resume.SummaryParagraphs = cleanResumeParagraphs(resume.SummaryParagraphs)
	resume.Links = cleanResumeLinks(resume.Links)
	resume.Skills = cleanResumeSkills(resume.Skills)
	resume.Experience = cleanResumeExperience(resume.Experience)
	resume.Education = cleanResumeEducation(resume.Education)
	return service.repository.SaveResume(ctx, resume)
}

func (service *ResumeService) Photo(ctx context.Context) (*models.ResumePhoto, error) {
	return service.repository.ResumePhoto(ctx)
}

func (service *ResumeService) SavePhoto(ctx context.Context, data []byte) (*models.Resume, error) {
	if len(data) == 0 || len(data) > MaxResumePhotoBytes {
		return nil, ErrInvalidResumePhoto
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 6000 || config.Height > 6000 {
		return nil, ErrInvalidResumePhoto
	}
	contentType := ""
	switch format {
	case "jpeg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	default:
		return nil, ErrInvalidResumePhoto
	}
	if err := service.repository.SaveResumePhoto(ctx, models.ResumePhoto{ContentType: contentType, Data: data}); err != nil {
		return nil, err
	}
	return service.Resume(ctx)
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

func cleanResumeParagraphs(paragraphs []models.ResumeText) []models.ResumeText {
	cleaned := make([]models.ResumeText, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph.Content = strings.TrimSpace(paragraph.Content)
		if paragraph.Content != "" {
			cleaned = append(cleaned, paragraph)
		}
	}
	return cleaned
}

func cleanResumeSkills(skills []models.ResumeSkill) []models.ResumeSkill {
	cleaned := make([]models.ResumeSkill, 0, len(skills))
	for _, skill := range skills {
		skill.Name = strings.TrimSpace(skill.Name)
		if skill.Name != "" {
			cleaned = append(cleaned, skill)
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

func cleanResumeBullets(bullets []models.ResumeText) []models.ResumeText {
	cleaned := make([]models.ResumeText, 0, len(bullets))
	for _, bullet := range bullets {
		bullet.Content = strings.TrimSpace(bullet.Content)
		if bullet.Content != "" {
			cleaned = append(cleaned, bullet)
		}
	}
	return cleaned
}
