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
	require.Len(t, settings.LinkedIn, 2)
	assert.Equal(t, "Europe", settings.LinkedIn[0].Location)
	assert.Equal(t, "Serbia", settings.LinkedIn[1].Location)
	assert.Empty(t, settings.LinkedIn[0].PostedWithin)
	assert.Empty(t, settings.LinkedIn[0].Workplace)
	assert.Empty(t, settings.LinkedIn[0].ExperienceLevel)

	settings.LinkedIn[0].PostedWithin = "r604800"
	settings.LinkedIn[0].Location = "Europe"
	settings.LinkedIn[0].Workplace = "2"
	settings.LinkedIn[0].ExperienceLevel = "4"
	_, err = repository.SaveDiscoverySettings(context.Background(), settings)
	require.NoError(t, err)

	actual, err := repository.DiscoverySettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "r604800", actual.LinkedIn[0].PostedWithin)
	assert.Equal(t, "2", actual.LinkedIn[0].Workplace)
	assert.Equal(t, "4", actual.LinkedIn[0].ExperienceLevel)
}
