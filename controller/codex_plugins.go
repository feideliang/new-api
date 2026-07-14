package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ── Plugin List (enhanced) ──────────────────────────
// GET /backend-api/ps/plugins/list
func CodexPluginList(c *gin.Context) {
	scope := c.DefaultQuery("scope", "GLOBAL")
	limitStr := c.DefaultQuery("limit", "200")
	collection := c.Query("collection")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 200
	}

	resp, err := service.BuildPluginListResponse(scope, limit, collection)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Plugin Installed ────────────────────────────────
// GET /backend-api/ps/plugins/installed
func CodexPluginInstalled(c *gin.Context) {
	scope := c.DefaultQuery("scope", "GLOBAL")
	userId := c.GetInt("id")

	resp, err := service.GetInstalledPlugins(userId, scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Plugin Detail ───────────────────────────────────
// GET /backend-api/ps/plugins/:plugin_id
func CodexPluginDetail(c *gin.Context) {
	pluginId := c.Param("plugin_id")
	includeUrls := c.Query("includeDownloadUrls") == "true"

	item, err := service.GetPluginDetail(pluginId, includeUrls)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found: " + pluginId})
		return
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(item)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Plugin Install ──────────────────────────────────
// POST /backend-api/ps/plugins/:plugin_id/install
func CodexPluginInstall(c *gin.Context) {
	pluginId := c.Param("plugin_id")
	includeAppsNeedingAuth := c.Query("includeAppsNeedingAuth") == "true"
	userId := c.GetInt("id")
	accountId := c.GetString("account_id")
	if accountId == "" {
		accountId = "default"
	}

	resp, err := service.InstallPlugin(userId, accountId, pluginId, includeAppsNeedingAuth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if resp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found: " + pluginId})
		return
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Plugin Uninstall ────────────────────────────────
// POST /backend-api/ps/plugins/:plugin_id/uninstall
func CodexPluginUninstall(c *gin.Context) {
	pluginId := c.Param("plugin_id")
	userId := c.GetInt("id")

	resp, err := service.UninstallPlugin(userId, pluginId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Suggested Plugins ───────────────────────────────
// GET /backend-api/ps/plugins/suggested
func CodexPluginSuggested(c *gin.Context) {
	ids := service.GetSuggestedPluginIDs()

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(ids)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Featured Plugins ────────────────────────────────
// GET /backend-api/plugins/featured
func CodexPluginsFeatured(c *gin.Context) {
	ids := service.GetFeaturedPluginIDs()

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(ids)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Workspace Shared (stub) ─────────────────────────
// GET /backend-api/ps/plugins/workspace/shared
func CodexPluginWorkspaceShared(c *gin.Context) {
	resp := dto.PluginListResponse{
		Plugins:    []*dto.PluginItem{},
		Pagination: dto.PluginPagination{Limit: 50, NextPageToken: nil},
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Workspace Created (stub) ────────────────────────
// GET /backend-api/ps/plugins/workspace/created
func CodexPluginWorkspaceCreated(c *gin.Context) {
	resp := dto.PluginListResponse{
		Plugins:    []*dto.PluginItem{},
		Pagination: dto.PluginPagination{Limit: 50, NextPageToken: nil},
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Plugin Shares (stub) ────────────────────────────
// GET /backend-api/ps/plugins/:plugin_id/shares
func CodexPluginShares(c *gin.Context) {
	resp := map[string]interface{}{
		"shares": []interface{}{},
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Connector Directory ─────────────────────────────
// GET /backend-api/connectors/directory/list
func CodexConnectorDirectory(c *gin.Context) {
	resp, err := service.BuildConnectorDirectory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Connector Directory Workspace ───────────────────
// GET /backend-api/connectors/directory/list_workspace
func CodexConnectorDirectoryWorkspace(c *gin.Context) {
	// Same as directory list for now
	CodexConnectorDirectory(c)
}

// ── Account Settings ────────────────────────────────
// GET /backend-api/accounts/:account_id/settings
func CodexAccountSettings(c *gin.Context) {
	resp := dto.AccountSettingsResponse{
		BetaSettings: dto.AccountSettingsBetaSettings{
			EnablePlugins: true,
		},
	}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ── Analytics Events ────────────────────────────────
// POST /backend-api/codex/analytics-events/events
func CodexAnalyticsEvents(c *gin.Context) {
	resp := dto.AnalyticsEventsResponse{Status: "ok"}

	c.Header("Cache-Control", "no-store")
	body, _ := common.Marshal(resp)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}
