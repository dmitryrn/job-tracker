package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
