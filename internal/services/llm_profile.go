package services

import (
	"regexp"

	"nice/internal/models"
)

var (
	llmEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	llmPhonePattern = regexp.MustCompile(`\+?\d[\d\s().-]{7,}\d`)
)

type llmUserProfile struct {
	Headline          string                    `json:"headline"`
	WorkAuthorization string                    `json:"workAuthorization"`
	Summary           string                    `json:"summary"`
	Skills            []models.UserProfileSkill `json:"skills"`
	WorkHistory       []llmWorkHistory          `json:"workHistory"`
	Education         []llmEducation            `json:"education"`
}

type llmWorkHistory struct {
	Company   string `json:"company"`
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Body      string `json:"body"`
}

type llmEducation struct {
	Degree    string `json:"degree"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Body      string `json:"body"`
}

// profileForLLM is the single allowlist for sending the editable profile to providers.
// Identity, timestamps, education institutions, and all resume contact fields stay out.
func profileForLLM(profile models.UserProfile) llmUserProfile {
	result := llmUserProfile{
		Headline:          redactProfileContactText(profile.Headline),
		WorkAuthorization: redactProfileContactText(profile.WorkAuthorization),
		Summary:           redactProfileContactText(profile.Summary),
		Skills:            make([]models.UserProfileSkill, 0, len(profile.Skills)),
		WorkHistory:       make([]llmWorkHistory, 0, len(profile.WorkHistory)),
		Education:         make([]llmEducation, 0, len(profile.Education)),
	}
	for _, skill := range profile.Skills {
		skill.Name = redactProfileContactText(skill.Name)
		skill.Notes = redactProfileContactText(skill.Notes)
		result.Skills = append(result.Skills, skill)
	}

	for _, experience := range profile.WorkHistory {
		result.WorkHistory = append(result.WorkHistory, llmWorkHistory{
			Company:   redactProfileContactText(experience.Company),
			Title:     redactProfileContactText(experience.Title),
			StartDate: experience.StartDate,
			EndDate:   experience.EndDate,
			Body:      redactProfileContactText(experience.Body),
		})
	}

	for _, education := range profile.Education {
		result.Education = append(result.Education, llmEducation{
			Degree:    redactProfileContactText(education.Degree),
			StartDate: education.StartDate,
			EndDate:   education.EndDate,
			Body:      redactProfileContactText(education.Body),
		})
	}

	return result
}

func redactProfileContactText(value string) string {
	value = llmEmailPattern.ReplaceAllString(value, "")
	return llmPhonePattern.ReplaceAllString(value, "")
}
