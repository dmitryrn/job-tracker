package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nice/internal/clients/openai"
)

func TestCompletionClientsSelectProvider(t *testing.T) {
	openCode := &openai.Client{}
	openAI := &openai.Client{}
	clients := completionClients{openCode: openCode, openAI: openAI}

	client, err := clients.forProvider("opencode")
	require.NoError(t, err)
	assert.Same(t, openCode, client)

	client, err = clients.forProvider("openai")
	require.NoError(t, err)
	assert.Same(t, openAI, client)
}

func TestCompletionClientsRejectUnknownProvider(t *testing.T) {
	_, err := (completionClients{}).forProvider("unsupported")

	assert.EqualError(t, err, `unsupported completion provider "unsupported"`)
}
