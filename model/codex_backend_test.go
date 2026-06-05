package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexBackendModelsUseGptCodexTablePrefix(t *testing.T) {
	db := setupOAuthServerModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&CodexBackendModel{}))

	require.True(t, db.Migrator().HasTable("gpt_codex_models"))
	require.False(t, db.Migrator().HasTable("codex_backend_models"))
}

func TestCodexBackendModelRoundTrip(t *testing.T) {
	db := setupOAuthServerModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&CodexBackendModel{}))

	require.NoError(t, db.Create(&CodexBackendModel{
		Slug:        "gpt-test-codex",
		DisplayName: "GPT Test Codex",
		Description: "test model",
		Priority:    7,
		Enabled:     true,
	}).Error)

	var got CodexBackendModel
	require.NoError(t, db.Where("slug = ?", "gpt-test-codex").First(&got).Error)
	require.Equal(t, "GPT Test Codex", got.DisplayName)
	require.True(t, got.Enabled)
}
