package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openai"
	"nice/internal/models"
)

func TestLatestResumeRevisionUsesLatestAcceptedRevision(t *testing.T) {
	base := models.Resume{Headline: "Software engineer"}
	updated := models.Resume{Headline: "Backend engineer"}
	basePayload, err := json.Marshal(resumeRevisionPayload{Revision: 0, Resume: base})
	require.NoError(t, err)
	acceptedPayload, err := json.Marshal(toolResultPayload{Status: "accepted", Revision: 1, Resume: &updated})
	require.NoError(t, err)
	rejectedPayload, err := json.Marshal(toolResultPayload{Status: "rejected", Revision: 2})
	require.NoError(t, err)

	latest, err := latestResumeRevision([]models.JobMatchChatItem{
		{Type: "resume_revision", Payload: basePayload},
		{Type: "tool_result", Payload: acceptedPayload},
		{Type: "tool_result", Payload: rejectedPayload},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, latest.Revision)
	assert.Equal(t, updated, latest.Resume)
}

func TestLatestResumeRevisionReturnsNotFoundWithoutSnapshot(t *testing.T) {
	_, err := latestResumeRevision(nil)

	assert.ErrorIs(t, err, ErrApplicationResumeNotFound)
}

func TestProviderRequestOmitsCandidateIdentityAndInstitutions(t *testing.T) {
	resume := models.Resume{
		FullName: "Avery Patel",
		Phone:    "+1 555 0100",
		Town:     "Mapleton",
		Country:  "Stale country",
		Email:    "avery.patel@example.test",
		SummaryParagraphs: []models.ResumeText{{
			Content: "Avery Patel built systems at Orchid Labs in Mapleton, Canada.",
		}},
		Experience: []models.ResumeExperience{{Company: "Orchid Labs"}},
		Education:  []models.ResumeEducation{{Institution: "Northfield Institute"}},
	}
	updatedResume := models.Resume{
		FullName:   "Avery Patel",
		Phone:      "+1 555 0101",
		Town:       "Lakeside",
		Country:    "Canada",
		Email:      "avery.patel+new@example.test",
		Experience: []models.ResumeExperience{{Company: "Harbor Works"}},
		Education:  []models.ResumeEducation{{Institution: "Stonebridge University"}},
	}
	initialContext, err := json.Marshal(jobMatchChatInitialContext{Profile: &models.UserProfile{
		WorkAuthorization: "Authorized to work in Canada",
		WorkHistory:       []models.UserProfileWorkHistory{{Company: "Cedar Systems"}},
		Education:         []models.UserProfileEducation{{Institution: "Riverside College"}},
	}})
	require.NoError(t, err)
	baseRevision, err := json.Marshal(resumeRevisionPayload{Revision: 0, Resume: resume})
	require.NoError(t, err)
	acceptedRevision, err := json.Marshal(toolResultPayload{ToolCallID: "call-1", Status: "accepted", Revision: 1, Resume: &updatedResume})
	require.NoError(t, err)
	toolCall, err := json.Marshal(openai.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: openai.ToolFunction{
			Name:      resumePatchTool.Function.Name,
			Arguments: `{"value":"Avery Patel at Orchid Labs"}`,
		},
	})
	require.NoError(t, err)

	request, err := providerRequest([]models.JobMatchChatItem{
		{Type: "initial_instructions", Payload: payload(map[string]string{"content": jobMatchChatInstructions})},
		{Type: "initial_context", Payload: initialContext},
		{Type: "resume_revision", Payload: baseRevision},
		{Type: "user_message", Payload: payload(map[string]string{"content": "Please update Avery Patel's application for Orchid Labs."})},
		{Type: "assistant_tool_call", Payload: toolCall},
		{Type: "tool_result", Payload: acceptedRevision},
	}, "low", "Canada")
	require.NoError(t, err)

	providerPayload, err := json.Marshal(request)
	require.NoError(t, err)
	for _, sensitiveValue := range []string{
		"Avery Patel",
		"+1 555 0100",
		"+1 555 0101",
		"Mapleton",
		"Lakeside",
		"avery.patel@example.test",
		"avery.patel+new@example.test",
		"Northfield Institute",
		"Stonebridge University",
		"Riverside College",
	} {
		assert.NotContains(t, string(providerPayload), sensitiveValue)
	}
	for _, company := range []string{"Orchid Labs", "Harbor Works", "Cedar Systems"} {
		assert.Contains(t, string(providerPayload), company)
	}
	assert.Contains(t, string(providerPayload), `\"country\":\"Canada\"`)
	assert.NotContains(t, string(providerPayload), "Stale country")
	assert.Contains(t, string(providerPayload), "Authorized to work in Canada")
	assert.NotContains(t, string(providerPayload), "[redacted]")
	assert.Equal(t, "Avery Patel", resume.FullName)
	assert.Equal(t, "Orchid Labs", resume.Experience[0].Company)
}

func TestApplicationResumeSnapshotOmitsBaseDetailsAndRestoresThemForDownload(t *testing.T) {
	base := models.Resume{
		ID:        1,
		FullName:  "Avery Patel",
		Headline:  "Software engineer",
		Town:      "Mapleton",
		Country:   "Canada",
		Email:     "avery.patel@example.test",
		Phone:     "+1 555 0100",
		Links:     []models.ResumeLink{{ID: 2, Label: "Portfolio", URL: "https://example.test"}},
		HasPhoto:  true,
		UpdatedAt: "2026-09-07T00:00:00Z",
		Experience: []models.ResumeExperience{{
			ID: 3, Company: "Orchid Labs", Title: "Engineer", Location: "Mapleton", Bullets: []models.ResumeText{{ID: 4, Content: "Built systems."}},
		}},
		Education: []models.ResumeEducation{{
			ID: 5, Institution: "Northfield Institute", Location: "Mapleton", Degree: "MSc",
		}},
	}

	snapshot := applicationResumeSnapshot(base)

	assert.Zero(t, snapshot.ID)
	assert.Empty(t, snapshot.FullName)
	assert.Empty(t, snapshot.Town)
	assert.Empty(t, snapshot.Country)
	assert.Empty(t, snapshot.Email)
	assert.Empty(t, snapshot.Phone)
	assert.Empty(t, snapshot.Links)
	assert.False(t, snapshot.HasPhoto)
	assert.Empty(t, snapshot.UpdatedAt)
	assert.Empty(t, snapshot.Experience[0].Location)
	assert.Empty(t, snapshot.Education[0].Institution)
	assert.Empty(t, snapshot.Education[0].Location)
	assert.Equal(t, "Software engineer", snapshot.Headline)
	assert.Equal(t, "Built systems.", snapshot.Experience[0].Bullets[0].Content)
	assert.Equal(t, "Mapleton", base.Experience[0].Location)

	snapshot.Headline = "Backend engineer"
	resume := applicationResumeForDownload(base, snapshot)

	assert.Equal(t, "Backend engineer", resume.Headline)
	assert.Equal(t, base.FullName, resume.FullName)
	assert.Equal(t, base.Town, resume.Town)
	assert.Equal(t, base.Country, resume.Country)
	assert.Equal(t, base.Email, resume.Email)
	assert.Equal(t, base.Phone, resume.Phone)
	assert.Equal(t, base.Links, resume.Links)
	assert.Equal(t, base.HasPhoto, resume.HasPhoto)
	assert.Equal(t, base.Experience[0].Location, resume.Experience[0].Location)
	assert.Equal(t, base.Education[0].Institution, resume.Education[0].Institution)
	assert.Equal(t, base.Education[0].Location, resume.Education[0].Location)
}

func TestApplyResumePatchEditsEducationDetails(t *testing.T) {
	resume := models.Resume{Education: []models.ResumeEducation{{
		ID: 7, Details: "Dean's list",
	}}}

	err := applyResumePatch(&resume, resumePatch{Operations: []resumePatchOperation{{
		Op: "replace", Section: "educationDetails", ID: 7, Expected: "Dean's list", Value: "Thesis: distributed systems",
	}}})

	require.NoError(t, err)
	assert.Equal(t, "Thesis: distributed systems", resume.Education[0].Details)
}

func TestApplyResumePatchRejectsEducationDetailsWithWrongExpectedValue(t *testing.T) {
	resume := models.Resume{Education: []models.ResumeEducation{{
		ID: 7, Details: "Dean's list",
	}}}

	err := applyResumePatch(&resume, resumePatch{Operations: []resumePatchOperation{{
		Op: "replace", Section: "educationDetails", ID: 7, Expected: "Honors", Value: "Thesis: distributed systems",
	}}})

	assert.Error(t, err)
	assert.Equal(t, "Dean's list", resume.Education[0].Details)
}

func TestChatResumeForLLMIncludesEducationID(t *testing.T) {
	resume := models.Resume{Education: []models.ResumeEducation{{
		ID: 7, Details: "Dean's list",
	}}}

	chat := chatResumeForLLM(resume, "Canada")

	require.Len(t, chat.Education, 1)
	assert.Equal(t, int64(7), chat.Education[0].ID)
}

func TestApplyResumePatchEditsCompetencyTitleAndSections(t *testing.T) {
	resume := models.Resume{Competencies: []models.ResumeCompetency{{
		ID: 7, Title: "Backend",
	}, {
		ID: 9, Title: "Frontend",
	}}}

	err := applyResumePatch(&resume, resumePatch{Operations: []resumePatchOperation{
		{Op: "replace", Section: "competencyTitle", ID: 7, Expected: "Backend", Value: "Platform engineering"},
		{Op: "remove", Section: "competency", ID: 9, Expected: "Frontend"},
		{Op: "add", Section: "competency", Value: "Data systems"},
	}})

	require.NoError(t, err)
	assert.Equal(t, []models.ResumeCompetency{{ID: 7, Title: "Platform engineering"}, {ID: 8, Title: "Data systems"}}, resume.Competencies)
}

func TestApplyResumePatchRejectsCompetencyTitleWithWrongExpectedValue(t *testing.T) {
	resume := models.Resume{Competencies: []models.ResumeCompetency{{
		ID: 7, Title: "Backend",
	}}}

	err := applyResumePatch(&resume, resumePatch{Operations: []resumePatchOperation{{
		Op: "replace", Section: "competencyTitle", ID: 7, Expected: "Frontend", Value: "Platform engineering",
	}}})

	assert.Error(t, err)
	assert.Equal(t, "Backend", resume.Competencies[0].Title)
}
