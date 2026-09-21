package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"nice/internal/clients/typesafe"
	"nice/internal/models"
)

const typeSafeScoreLevels = 10

type TypeSafeSystemOneClient interface {
	SystemOne(context.Context, any, map[string]typesafe.Question) (typesafe.Response, error)
}

type JobProfileScorer struct {
	client TypeSafeSystemOneClient
}

func NewJobProfileScorer(client TypeSafeSystemOneClient) *JobProfileScorer {
	return &JobProfileScorer{client: client}
}

func (scorer *JobProfileScorer) Score(ctx context.Context, job models.Job, profile models.UserProfile) (int, error) {
	state, err := json.Marshal(struct {
		Job struct {
			Title          string `json:"title"`
			Company        string `json:"company"`
			Location       string `json:"location"`
			Workplace      string `json:"workplace"`
			EmploymentType string `json:"employmentType"`
			Description    string `json:"description"`
		} `json:"job"`
		Profile llmUserProfile `json:"profile"`
	}{
		Job: struct {
			Title          string `json:"title"`
			Company        string `json:"company"`
			Location       string `json:"location"`
			Workplace      string `json:"workplace"`
			EmploymentType string `json:"employmentType"`
			Description    string `json:"description"`
		}{
			Title:          job.Title,
			Company:        job.Company,
			Location:       job.Location,
			Workplace:      job.Workplace,
			EmploymentType: job.EmploymentType,
			Description:    normalizeJobDescription(job.BodyText),
		},
		Profile: profileForLLM(profile),
	})
	if err != nil {
		return 0, fmt.Errorf("encode profile score input: %w", err)
	}

	state = []byte(redactSensitiveText(string(state), normalizedRedactionValues(appendUserProfileRedactionValues(nil, profile))))

	response, err := scorer.client.SystemOne(ctx, json.RawMessage(state), map[string]typesafe.Question{
		"overall_fit": {
			Type:         "score",
			Instructions: "How well does this candidate profile match this job overall? Judge professional relevance, demonstrated capability, seniority, responsibilities, and explicit work constraints. Ignore identity and contact information.",
			Criteria: []string{
				"No meaningful professional match is established.",
				"Only negligible or unrelated experience is established.",
				"A small amount of adjacent experience is established, with major gaps.",
				"Some relevant overlap is established, but the role is mostly a stretch.",
				"Partial match with several material gaps or unclear requirements.",
				"Mixed match: relevant experience exists, but strengths and gaps are balanced.",
				"Solid match for the core work, with some meaningful gaps or uncertainty.",
				"Strong match for most responsibilities and requirements, with limited gaps.",
				"Very strong match with direct evidence for nearly all important requirements.",
				"Exceptional or near-perfect match for the role's demonstrated requirements and scope.",
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("score job against profile with TypeSafe: %w", err)
	}

	answer, found := response.Answers["overall_fit"]
	if !found || answer.Type != "score" {
		return 0, fmt.Errorf("TypeSafe response did not contain an overall_fit score")
	}

	if math.IsNaN(answer.Score) || math.IsInf(answer.Score, 0) || answer.Score < 0 || answer.Score > typeSafeScoreLevels-1 {
		return 0, fmt.Errorf("TypeSafe overall_fit score %.2f is outside the supported 0 to %d level range", answer.Score, typeSafeScoreLevels-1)
	}

	return int(math.Round(answer.Score * 10 / (typeSafeScoreLevels - 1))), nil
}
