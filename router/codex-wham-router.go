package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterCodexWhamRoutes(router *gin.Engine) {
	// ── EXISTING: MCP endpoint (unchanged) ───────────
	for _, path := range []string{
		"/backend-api/wham/apps",
		"/api/codex/apps",
		"/codex-backend/codex/apps",
	} {
		router.POST(path, middleware.RouteTag("relay"), controller.CodexWhamAppsMCP)
		router.DELETE(path, middleware.RouteTag("relay"), controller.CodexWhamAppsMCP)
	}

	// ── EXISTING: Profile endpoint (unchanged) ───────
	for _, path := range []string{
		"/wham/profiles/me",
		"/backend-api/wham/profiles/me",
		"/api/codex/profiles/me",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			middleware.TokenAuth(),
			controller.CodexWhamProfileMe,
		)
	}

	// ── Plugin Static Routes (must be BEFORE :plugin_id routes) ──

	// Plugin list
	for _, path := range []string{
		"/ps/plugins/list",
		"/backend-api/ps/plugins/list",
		"/api/codex/ps/plugins/list",
		"/codex-backend/codex/ps/plugins/list",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginList,
		)
	}

	// Plugin installed list
	for _, path := range []string{
		"/ps/plugins/installed",
		"/backend-api/ps/plugins/installed",
		"/api/codex/ps/plugins/installed",
		"/codex-backend/codex/ps/plugins/installed",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			middleware.TokenAuth(),
			controller.CodexPluginInstalled,
		)
	}

	// Suggested plugins
	for _, path := range []string{
		"/ps/plugins/suggested",
		"/backend-api/ps/plugins/suggested",
		"/api/codex/ps/plugins/suggested",
		"/codex-backend/codex/ps/plugins/suggested",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginSuggested,
		)
	}

	// Workspace shared (stub)
	for _, path := range []string{
		"/ps/plugins/workspace/shared",
		"/backend-api/ps/plugins/workspace/shared",
		"/api/codex/ps/plugins/workspace/shared",
		"/codex-backend/codex/ps/plugins/workspace/shared",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginWorkspaceShared,
		)
	}

	// Workspace created (stub)
	for _, path := range []string{
		"/ps/plugins/workspace/created",
		"/backend-api/ps/plugins/workspace/created",
		"/api/codex/ps/plugins/workspace/created",
		"/codex-backend/codex/ps/plugins/workspace/created",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginWorkspaceCreated,
		)
	}

	// ── Plugin Dynamic Routes (:plugin_id) ───────────

	// Plugin detail
	for _, path := range []string{
		"/ps/plugins/:plugin_id",
		"/backend-api/ps/plugins/:plugin_id",
		"/api/codex/ps/plugins/:plugin_id",
		"/codex-backend/codex/ps/plugins/:plugin_id",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginDetail,
		)
	}

	// Plugin install
	for _, path := range []string{
		"/ps/plugins/:plugin_id/install",
		"/backend-api/ps/plugins/:plugin_id/install",
		"/api/codex/ps/plugins/:plugin_id/install",
		"/codex-backend/codex/ps/plugins/:plugin_id/install",
	} {
		router.POST(path,
			middleware.RouteTag("relay"),
			middleware.TokenAuth(),
			controller.CodexPluginInstall,
		)
	}

	// Plugin uninstall
	for _, path := range []string{
		"/ps/plugins/:plugin_id/uninstall",
		"/backend-api/ps/plugins/:plugin_id/uninstall",
		"/api/codex/ps/plugins/:plugin_id/uninstall",
		"/codex-backend/codex/ps/plugins/:plugin_id/uninstall",
	} {
		router.POST(path,
			middleware.RouteTag("relay"),
			middleware.TokenAuth(),
			controller.CodexPluginUninstall,
		)
	}

	// Plugin shares (stub)
	for _, path := range []string{
		"/ps/plugins/:plugin_id/shares",
		"/backend-api/ps/plugins/:plugin_id/shares",
		"/api/codex/ps/plugins/:plugin_id/shares",
		"/codex-backend/codex/ps/plugins/:plugin_id/shares",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginShares,
		)
	}

	// ── Other Plugin Routes ──────────────────────────

	// Featured plugins (different prefix — no /ps/)
	for _, path := range []string{
		"/plugins/featured",
		"/backend-api/plugins/featured",
		"/api/codex/plugins/featured",
		"/codex-backend/codex/plugins/featured",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPluginsFeatured,
		)
	}

	// Connector directory
	for _, path := range []string{
		"/connectors/directory/list",
		"/backend-api/connectors/directory/list",
		"/api/codex/connectors/directory/list",
		"/codex-backend/codex/connectors/directory/list",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexConnectorDirectory,
		)
	}

	// Connector directory workspace
	for _, path := range []string{
		"/connectors/directory/list_workspace",
		"/backend-api/connectors/directory/list_workspace",
		"/api/codex/connectors/directory/list_workspace",
		"/codex-backend/codex/connectors/directory/list_workspace",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexConnectorDirectoryWorkspace,
		)
	}

	// Account settings
	for _, path := range []string{
		"/accounts/:account_id/settings",
		"/backend-api/accounts/:account_id/settings",
		"/api/codex/accounts/:account_id/settings",
		"/codex-backend/codex/accounts/:account_id/settings",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexAccountSettings,
		)
	}

	// Analytics events
	for _, path := range []string{
		"/codex/analytics-events/events",
		"/backend-api/codex/analytics-events/events",
		"/api/codex/codex/analytics-events/events",
		"/codex-backend/codex/codex/analytics-events/events",
	} {
		router.POST(path,
			middleware.RouteTag("relay"),
			controller.CodexAnalyticsEvents,
		)
	}
}
