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
}
