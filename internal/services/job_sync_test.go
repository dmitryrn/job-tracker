package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"nice/internal/models"
)

func TestJobSyncSkipsDisabledProviders(t *testing.T) {
	runs := &providerRunRecorder{}
	syncer := JobSync{
		providerRuns: runs,
		settings: discoverySettingsStub{settings: models.DiscoverySettings{
			Adzuna:   models.AdzunaSearchSettings{Enabled: false},
			Remotive: models.RemotiveSearchSettings{Enabled: false},
			Jobicy:   models.JobicySearchSettings{Enabled: false},
			LinkedIn: models.LinkedInSearchSettings{Enabled: false},
		}},
		logger: zap.NewNop(),
	}

	syncer.sync(context.Background())

	assert.Zero(t, runs.started)
}

type discoverySettingsStub struct {
	settings models.DiscoverySettings
}

func (stub discoverySettingsStub) DiscoverySettings(context.Context) (models.DiscoverySettings, error) {
	return stub.settings, nil
}

func (discoverySettingsStub) SaveDiscoverySettings(context.Context, models.DiscoverySettings) (models.DiscoverySettings, error) {
	return models.DiscoverySettings{}, nil
}

type providerRunRecorder struct {
	started int
}

func (recorder *providerRunRecorder) StartProviderRun(context.Context, string, time.Duration, time.Time) (bool, error) {
	recorder.started++
	return true, nil
}

func (*providerRunRecorder) CompleteProviderRun(context.Context, string, error, time.Time) error {
	return nil
}
