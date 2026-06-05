package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCodexBackendControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CodexBackendModel{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
	return db
}

func performCodexBackendModelsRequest(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/codex-backend/codex/models", CodexBackendModels)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/codex-backend/codex/models?client_version=0.99.0", nil)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestCodexBackendModelsReturnsCodexClientShape(t *testing.T) {
	setupCodexBackendControllerTestDB(t)

	recorder := performCodexBackendModelsRequest(t)

	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotEmpty(t, body.Models)

	first := body.Models[0]
	require.NotEmpty(t, first["slug"])
	require.NotEmpty(t, first["display_name"])
	require.Equal(t, "medium", first["default_reasoning_level"])
	require.Equal(t, "shell_command", first["shell_type"])
	require.Equal(t, "list", first["visibility"])
	require.Equal(t, true, first["supported_in_api"])
	require.Equal(t, false, first["supports_parallel_tool_calls"])
	require.Contains(t, first, "supported_reasoning_levels")
	require.Contains(t, first, "truncation_policy")
	require.Contains(t, first, "input_modalities")
}

func TestCodexBackendModelsIncludesEnabledDatabaseRowsFirstByPriority(t *testing.T) {
	db := setupCodexBackendControllerTestDB(t)
	require.NoError(t, db.Create(&model.CodexBackendModel{
		Slug:        "gpt-custom-codex",
		DisplayName: "GPT Custom Codex",
		Description: "custom",
		Priority:    100,
		Enabled:     true,
	}).Error)
	require.NoError(t, db.Create(&model.CodexBackendModel{
		Slug:        "gpt-disabled-codex",
		DisplayName: "GPT Disabled Codex",
		Description: "disabled",
		Priority:    200,
		Enabled:     false,
	}).Error)

	recorder := performCodexBackendModelsRequest(t)

	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Models []map[string]any `json:"models"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotEmpty(t, body.Models)
	require.Equal(t, "gpt-custom-codex", body.Models[0]["slug"])
	require.NotContains(t, collectCodexBackendModelSlugs(body.Models), "gpt-disabled-codex")
}

func collectCodexBackendModelSlugs(models []map[string]any) []string {
	out := make([]string, 0, len(models))
	for _, item := range models {
		if slug, ok := item["slug"].(string); ok {
			out = append(out, slug)
		}
	}
	return out
}
