package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/oauthserver"
	"github.com/gin-gonic/gin"
)

func CodexBackendModels(c *gin.Context) {
	rows, err := model.GetEnabledCodexBackendModels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, service.BuildCodexBackendModelsResponse(rows))
}

// CodexWhamProfileMe returns the authenticated user's profile for the Codex CLI.
// Endpoint: GET /wham/profiles/me
func CodexWhamProfileMe(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}

	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(user.Username)
	}
	email := strings.TrimSpace(user.Email)
	if email == "" {
		email = strings.TrimSpace(user.Username) + "@localhost"
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"id":                  oauthserver.StableCodexAccountID(userId),
		"name":                name,
		"preferred_username":  strings.TrimSpace(user.Username),
		"email":               email,
		"plan_type":           oauthserver.CodexDefaultPlanType,
		"account_id":          oauthserver.StableCodexAccountID(userId),
		"requires_openai_auth": true,
	})
}
