package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func RegisterCodexBackendRoutes(router *gin.Engine) {
	prefixes := []string{
		"/codex-backend/codex",
		"/codex-backend/codex/v1",
		"/api/codex",
		"/api/codex/v1",
		"/backend-api/codex",
		"/backend-api/codex/v1",
	}
	for _, prefix := range prefixes {
		registerCodexBackendRouteGroup(router.Group(prefix))
	}
}

func registerCodexBackendRouteGroup(group *gin.RouterGroup) {
	group.Use(middleware.RouteTag("relay"))
	group.Use(middleware.SystemPerformanceCheck())
	group.Use(middleware.TokenAuth())

	group.GET("/models", controller.CodexBackendModels)

	relayRoute := group.Group("")
	relayRoute.Use(middleware.ModelRequestRateLimit())
	relayRoute.Use(middleware.Distribute())
	{
		relayRoute.POST("/responses", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponses)
		})
		relayRoute.POST("/responses/compact", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		})
		relayRoute.POST("/images/generations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		relayRoute.POST("/images/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
	}

	group.POST("/alpha/search", codexBackendUnsupportedEndpoint("alpha/search"))
	group.POST("/memories/trace_summarize", codexBackendUnsupportedEndpoint("memories/trace_summarize"))
	group.POST("/realtime/calls", codexBackendUnsupportedEndpoint("realtime/calls"))
}

func codexBackendUnsupportedEndpoint(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": gin.H{
				"message": "codex backend endpoint is not implemented: " + endpoint,
				"type":    "invalid_request_error",
				"code":    "not_implemented",
			},
		})
	}
}
