package repositories

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"nice/internal/migrations"
)

func TestDiscoverySettingsPersistLinkedInFilters(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, migrations.Apply(db))

	repository := NewSQLite(db)
	settings, err := repository.DiscoverySettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Europe", settings.LinkedIn.Location)
	assert.Empty(t, settings.LinkedIn.PostedWithin)
	assert.Empty(t, settings.LinkedIn.Workplace)
	assert.Empty(t, settings.LinkedIn.ExperienceLevel)

	settings.LinkedIn.PostedWithin = "r604800"
	settings.LinkedIn.Location = "Europe"
	settings.LinkedIn.Workplace = "2"
	settings.LinkedIn.ExperienceLevel = "4"
	_, err = repository.SaveDiscoverySettings(context.Background(), settings)
	require.NoError(t, err)

	actual, err := repository.DiscoverySettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "r604800", actual.LinkedIn.PostedWithin)
	assert.Equal(t, "2", actual.LinkedIn.Workplace)
	assert.Equal(t, "4", actual.LinkedIn.ExperienceLevel)
}
