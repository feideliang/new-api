package router

import (
	"bytes"
	"embed"
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

func TestRegisterCodexBackendRoutesModelsAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CodexBackendModel{}, &model.Token{}, &model.User{}))
	originalDB := model.DB
	model.DB = db
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		common.RedisEnabled = originalRedisEnabled
	})

	router := gin.New()
	RegisterCodexBackendRoutes(router)

	for _, path := range []string{
		"/codex-backend/codex/models",
		"/codex-backend/codex/v1/models",
		"/api/codex/models",
		"/api/codex/v1/models",
		"/backend-api/codex/models",
		"/backend-api/codex/v1/models",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer test-token")

		router.ServeHTTP(recorder, request)

		require.NotEqual(t, http.StatusNotFound, recorder.Code, path)
	}
}

func TestWhamAppsMCPInitializeDoesNotFallThroughToWebIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterCodexWhamRoutes(router)
	SetWebRouter(router, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte("<html>New API</html>"),
		ClassicBuildFS:   embed.FS{},
		ClassicIndexPage: []byte("<html>Classic</html>"),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/backend-api/wham/apps",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"codex","version":"test"}}}`),
	)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	require.NotContains(t, recorder.Body.String(), "<html>")

	var body struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    struct {
				Tools map[string]any `json:"tools"`
			} `json:"capabilities"`
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "2.0", body.JSONRPC)
	require.Equal(t, 1, body.ID)
	require.NotEmpty(t, body.Result.ProtocolVersion)
	require.NotNil(t, body.Result.Capabilities.Tools)
	require.Equal(t, "codex_apps", body.Result.ServerInfo.Name)
}

func TestWhamAppsMCPToolsListReturnsEmptyToolSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterCodexWhamRoutes(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/backend-api/wham/apps",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":"tools-1","method":"tools/list","params":{}}`),
	)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")

	var body struct {
		JSONRPC string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  struct {
			Tools []any `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "2.0", body.JSONRPC)
	require.Equal(t, "tools-1", body.ID)
	require.Empty(t, body.Result.Tools)
}

func TestWhamAppsMCPInitializedNotificationIsAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterCodexWhamRoutes(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/backend-api/wham/apps",
		bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`),
	)
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, recorder.Body.String())
}

func TestPsPluginsListReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterCodexWhamRoutes(router)
	SetWebRouter(router, ThemeAssets{
		DefaultBuildFS:   embed.FS{},
		DefaultIndexPage: []byte("<html>New API</html>"),
		ClassicBuildFS:   embed.FS{},
		ClassicIndexPage: []byte("<html>Classic</html>"),
	})

	for _, path := range []string{
		"/ps/plugins/list",
		"/backend-api/ps/plugins/list",
		"/api/codex/ps/plugins/list",
		"/codex-backend/codex/ps/plugins/list",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer test-token")

		router.ServeHTTP(recorder, request)

		require.NotEqual(t, http.StatusNotFound, recorder.Code, "route must not be 404: "+path)
		require.NotContains(t, recorder.Body.String(), "<html>", "route must not fall through to web index: "+path)
	}
}
