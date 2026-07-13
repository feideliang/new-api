package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterCodexWhamRoutes(router *gin.Engine) {
	for _, path := range []string{
		"/backend-api/wham/apps",
		"/api/codex/apps",
		"/codex-backend/codex/apps",
	} {
		router.POST(path, middleware.RouteTag("relay"), controller.CodexWhamAppsMCP)
		router.DELETE(path, middleware.RouteTag("relay"), controller.CodexWhamAppsMCP)
	}

	// Profile endpoint for Codex CLI
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

	// Plugin store list endpoint for Codex CLI
	for _, path := range []string{
		"/ps/plugins/list",
		"/backend-api/ps/plugins/list",
		"/api/codex/ps/plugins/list",
		"/codex-backend/codex/ps/plugins/list",
	} {
		router.GET(path,
			middleware.RouteTag("relay"),
			controller.CodexPsPluginsList,
		)
	}
}
