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

func TestProviderRequestRedactsCandidateIdentityAndResumeOrganizations(t *testing.T) {
	resume := models.Resume{
		FullName: "Avery Patel",
		Phone:    "+1 555 0100",
		Location: "Mapleton, Canada",
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
		Location:   "Lakeside, Canada",
		Email:      "avery.patel+new@example.test",
		Experience: []models.ResumeExperience{{Company: "Harbor Works"}},
		Education:  []models.ResumeEducation{{Institution: "Stonebridge University"}},
	}
	initialContext, err := json.Marshal(jobMatchChatInitialContext{Profile: &models.UserProfile{
		WorkHistory: []models.UserProfileWorkHistory{{Company: "Cedar Systems"}},
		Education:   []models.UserProfileEducation{{Institution: "Riverside College"}},
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
	}, "low")
	require.NoError(t, err)

	providerPayload, err := json.Marshal(request)
	require.NoError(t, err)
	for _, sensitiveValue := range []string{
		"Avery Patel",
		"+1 555 0100",
		"+1 555 0101",
		"Mapleton, Canada",
		"Lakeside, Canada",
		"avery.patel@example.test",
		"avery.patel+new@example.test",
		"Orchid Labs",
		"Harbor Works",
		"Cedar Systems",
		"Northfield Institute",
		"Stonebridge University",
		"Riverside College",
	} {
		assert.NotContains(t, string(providerPayload), sensitiveValue)
	}
	assert.Contains(t, string(providerPayload), redactedChatValue)
	assert.Equal(t, "Avery Patel", resume.FullName)
	assert.Equal(t, "Orchid Labs", resume.Experience[0].Company)
}
