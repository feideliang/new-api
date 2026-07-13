package controller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const codexWhamAppsProtocolVersion = "2025-06-18"

type codexWhamAppsJSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type codexWhamAppsInitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type codexWhamAppsResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

type codexWhamAppsInitializeResult struct {
	ProtocolVersion string                       `json:"protocolVersion"`
	Capabilities    codexWhamAppsCapabilities    `json:"capabilities"`
	ServerInfo      codexWhamAppsServerInfo      `json:"serverInfo"`
	Instructions    string                       `json:"instructions,omitempty"`
	Meta            map[string]map[string]string `json:"_meta,omitempty"`
}

type codexWhamAppsCapabilities struct {
	Tools map[string]any `json:"tools"`
}

type codexWhamAppsServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type codexWhamAppsToolsListResult struct {
	Tools []any `json:"tools"`
}

type codexWhamAppsJSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func CodexWhamAppsMCP(c *gin.Context) {
	if c.Request.Method == http.MethodDelete {
		c.Status(http.StatusNoContent)
		return
	}

	var message codexWhamAppsJSONRPCMessage
	if err := common.DecodeJson(c.Request.Body, &message); err != nil {
		writeCodexWhamAppsJSON(c, http.StatusBadRequest, codexWhamAppsResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error: codexWhamAppsJSONRPCError{
				Code:    -32700,
				Message: "parse error",
			},
		})
		return
	}

	if len(message.ID) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	switch message.Method {
	case "initialize":
		writeCodexWhamAppsJSON(c, http.StatusOK, codexWhamAppsResponse{
			JSONRPC: "2.0",
			ID:      message.ID,
			Result:  buildCodexWhamAppsInitializeResult(message.Params),
		})
	case "tools/list":
		writeCodexWhamAppsJSON(c, http.StatusOK, codexWhamAppsResponse{
			JSONRPC: "2.0",
			ID:      message.ID,
			Result: codexWhamAppsToolsListResult{
				Tools: []any{},
			},
		})
	default:
		writeCodexWhamAppsJSON(c, http.StatusOK, codexWhamAppsResponse{
			JSONRPC: "2.0",
			ID:      message.ID,
			Error: codexWhamAppsJSONRPCError{
				Code:    -32601,
				Message: "method not found",
			},
		})
	}
}

func buildCodexWhamAppsInitializeResult(params json.RawMessage) codexWhamAppsInitializeResult {
	protocolVersion := codexWhamAppsProtocolVersion
	if len(params) > 0 {
		var initializeParams codexWhamAppsInitializeParams
		if err := common.Unmarshal(params, &initializeParams); err == nil && initializeParams.ProtocolVersion != "" {
			protocolVersion = initializeParams.ProtocolVersion
		}
	}

	return codexWhamAppsInitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: codexWhamAppsCapabilities{
			Tools: map[string]any{},
		},
		ServerInfo: codexWhamAppsServerInfo{
			Name:    "codex_apps",
			Version: "v0.0.0",
		},
	}
}

func writeCodexWhamAppsJSON(c *gin.Context, status int, payload any) {
	body, err := common.Marshal(payload)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(status, "application/json; charset=utf-8", body)
}

// Codex plugin store (ps/plugins/list) implementation.

type codexPluginStoreEntry struct {
	Name     string          `json:"name"`
	Source   json.RawMessage `json:"source"`
	Policy   json.RawMessage `json:"policy"`
	Category string          `json:"category"`
}

type codexPluginStoreManifest struct {
	Name      string                   `json:"name"`
	Interface *codexPluginManifestInterface `json:"interface,omitempty"`
}

type codexPluginManifestInterface struct {
	DisplayName     string `json:"displayName,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	Logo            string `json:"logo,omitempty"`
	Category        string `json:"category,omitempty"`
}

type codexPluginStoreResponse struct {
	Plugins []*codexPluginStoreItem `json:"plugins"`
}

type codexPluginStoreItem struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Logo        string `json:"logo,omitempty"`
}

const defaultPluginStorePath = "plugins/.tmp"

func getPluginStorePath() string {
	path := defaultPluginStorePath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try from executable directory if running as a binary
		exe, err := os.Executable()
		if err == nil {
			altPath := filepath.Join(filepath.Dir(exe), defaultPluginStorePath)
			if _, err := os.Stat(altPath); err == nil {
				return altPath
			}
		}
	}
	return path
}

func loadPluginManifest(pluginsDir, pluginName string) *codexPluginManifestInterface {
	manifestPath := filepath.Join(pluginsDir, pluginName, ".codex-plugin", "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var manifest codexPluginStoreManifest
	if err := common.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	return manifest.Interface
}

func CodexPsPluginsList(c *gin.Context) {
	storePath := getPluginStorePath()
	marketplacePath := filepath.Join(storePath, "plugins", ".agents", "plugins", "marketplace.json")

	data, err := os.ReadFile(marketplacePath)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var store struct {
		Plugins []codexPluginStoreEntry `json:"plugins"`
	}
	if err := common.Unmarshal(data, &store); err != nil {
		common.ApiError(c, err)
		return
	}

	pluginsDir := filepath.Join(storePath, "plugins", "plugins")
	items := make([]*codexPluginStoreItem, 0, len(store.Plugins))

	for _, entry := range store.Plugins {
		item := &codexPluginStoreItem{
			Name:     entry.Name,
			Category: entry.Category,
		}

		manifestInterface := loadPluginManifest(pluginsDir, entry.Name)
		if manifestInterface != nil {
			item.DisplayName = manifestInterface.DisplayName
			item.Description = manifestInterface.ShortDescription
			item.Logo = manifestInterface.Logo
		}
		if item.DisplayName == "" {
			item.DisplayName = entry.Name
		}

		items = append(items, item)
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, codexPluginStoreResponse{Plugins: items})
}
