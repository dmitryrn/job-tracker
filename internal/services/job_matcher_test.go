package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/models"
)

func TestJobMatcherMatchesExactSkillWithEstablishedExperienceDuration(t *testing.T) {
	minimumYears := 5
	match := NewJobMatcher().Match(JobMatchInput{
		AsOf: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Job: models.JobAnalysisDraft{
			Requirements: []models.JobRequirement{{
				ID: "professional-go-experience", Kind: "must_have", Concept: "golang", MinimumYears: &minimumYears,
				Quote: "Must have 5 years of professional Golang experience.",
			}},
		},
		Profile: models.CVProfileDraft{
			Experience: []models.ProfileExperience{{
				Title: "Backend Engineer", StartDate: "May 2021", EndDate: "Present",
				Evidence: []string{"Designed and operated Go services."},
			}},
			Skills: []models.ProfileSkill{{
				Name: "Go", Evidence: []string{"Designed and operated Go services."},
			}},
		},
	})

	require.Len(t, match.Requirements, 1)
	assert.Equal(t, "met", match.Requirements[0].Result)
	assert.Equal(t, "exact_concept_and_duration", match.Requirements[0].Basis)
	require.NotNil(t, match.Requirements[0].ExperienceYears)
	assert.GreaterOrEqual(t, *match.Requirements[0].ExperienceYears, 5.0)
	assert.Equal(t, "strong", match.ScreeningFit)
	assert.Equal(t, "apply", match.RecommendedAction)
}

func TestJobMatcherTreatsUnestablishedDurationAsPartial(t *testing.T) {
	minimumYears := 5
	match := NewJobMatcher().Match(JobMatchInput{
		Job: models.JobAnalysisDraft{
			Requirements: []models.JobRequirement{{
				ID: "professional-go-experience", Kind: "must_have", Concept: "go", MinimumYears: &minimumYears,
				Quote: "Must have 5 years of professional Go experience.",
			}},
		},
		Profile: models.CVProfileDraft{
			Skills: []models.ProfileSkill{{
				Name: "Go", Evidence: []string{"Built Go APIs."},
			}},
		},
	})

	require.Len(t, match.Requirements, 1)
	assert.Equal(t, "partial", match.Requirements[0].Result)
	assert.Equal(t, "exact_concept_unverified_duration", match.Requirements[0].Basis)
	assert.Nil(t, match.Requirements[0].ExperienceYears)
	assert.Equal(t, "plausible", match.ScreeningFit)
}

func TestJobMatcherKeepsMissingSkillEvidenceUnknown(t *testing.T) {
	match := NewJobMatcher().Match(JobMatchInput{
		Job: models.JobAnalysisDraft{
			Requirements: []models.JobRequirement{{
				ID: "go", Kind: "must_have", Concept: "go", Quote: "Go experience is required.",
			}},
		},
		Profile: models.CVProfileDraft{
			Skills: []models.ProfileSkill{{Name: "TypeScript", Evidence: []string{"Built TypeScript applications."}}},
		},
	})

	require.Len(t, match.Requirements, 1)
	assert.Equal(t, "unknown", match.Requirements[0].Result)
	assert.Equal(t, "weak", match.ScreeningFit)
	assert.Equal(t, "investigate", match.RecommendedAction)
}

func TestJobMatcherReportsVagueJobAsUnknown(t *testing.T) {
	match := NewJobMatcher().Match(JobMatchInput{
		Job: models.JobAnalysisDraft{
			Responsibilities: []models.JobEvidence{{
				Concept: "stakeholder_collaboration", Quote: "Work alongside shareholders and users.",
			}},
		},
	})

	assert.Empty(t, match.Requirements)
	assert.Equal(t, "unknown", match.ScreeningFit)
	assert.Equal(t, "investigate", match.RecommendedAction)
	assert.Equal(t, []string{"The posting states no assessable must-have requirements."}, match.MainRisks)
}

func TestJobMatcherFailsExplicitWorkplaceConflict(t *testing.T) {
	match := NewJobMatcher().Match(JobMatchInput{
		Job: models.JobAnalysisDraft{
			Constraints:  []models.JobConstraint{{Kind: "workplace", Value: "onsite", Quote: "This is an onsite role."}},
			Requirements: []models.JobRequirement{{ID: "go", Kind: "must_have", Concept: "go", Quote: "Go is required."}},
		},
		Profile: models.CVProfileDraft{
			Constraints: []models.ProfileConstraint{{Kind: "workplace", Value: "remote", Evidence: "Remote-only roles."}},
			Skills:      []models.ProfileSkill{{Name: "Go", Evidence: []string{"Built Go APIs."}}},
		},
	})

	require.Len(t, match.Eligibility.Checks, 1)
	assert.Equal(t, "fail", match.Eligibility.Checks[0].Result)
	assert.Equal(t, "fail", match.Eligibility.State)
	assert.Equal(t, "mismatch", match.ScreeningFit)
	assert.Equal(t, "skip", match.RecommendedAction)
}

func TestJobMatcherInvestigatesAnUnknownEligibilityGate(t *testing.T) {
	match := NewJobMatcher().Match(JobMatchInput{
		Job: models.JobAnalysisDraft{
			Constraints:  []models.JobConstraint{{Kind: "workplace", Value: "remote", Quote: "This is a remote role."}},
			Requirements: []models.JobRequirement{{ID: "go", Kind: "must_have", Concept: "go", Quote: "Go is required."}},
		},
		Profile: models.CVProfileDraft{
			Skills: []models.ProfileSkill{{Name: "Go", Evidence: []string{"Built Go APIs."}}},
		},
	})

	assert.Equal(t, "unknown", match.Eligibility.State)
	assert.Equal(t, "strong", match.ScreeningFit)
	assert.Equal(t, "investigate", match.RecommendedAction)
	assert.Contains(t, match.MainRisks, "At least one eligibility constraint is unknown.")
}
