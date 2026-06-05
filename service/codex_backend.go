package service

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
)

var defaultCodexBackendModels = []string{
	"gpt-5",
	"gpt-5-codex",
	"gpt-5-codex-mini",
	"gpt-5.1",
	"gpt-5.1-codex",
	"gpt-5.1-codex-max",
	"gpt-5.1-codex-mini",
	"gpt-5.2",
	"gpt-5.2-codex",
	"gpt-5.3-codex",
	"gpt-5.3-codex-spark",
	"gpt-5.4",
}

type CodexBackendModelsResponse struct {
	Models []CodexBackendModelMetadata `json:"models"`
}

type CodexBackendModelMetadata struct {
	Slug                          string                       `json:"slug"`
	DisplayName                   string                       `json:"display_name"`
	Description                   string                       `json:"description,omitempty"`
	DefaultReasoningLevel         string                       `json:"default_reasoning_level"`
	SupportedReasoningLevels      []CodexBackendReasoningLevel `json:"supported_reasoning_levels"`
	ShellType                     string                       `json:"shell_type"`
	Visibility                    string                       `json:"visibility"`
	SupportedInAPI                bool                         `json:"supported_in_api"`
	Priority                      int                          `json:"priority"`
	AdditionalSpeedTiers          []string                     `json:"additional_speed_tiers"`
	ServiceTiers                  []CodexBackendServiceTier    `json:"service_tiers"`
	DefaultServiceTier            *string                      `json:"default_service_tier"`
	Upgrade                       any                          `json:"upgrade"`
	BaseInstructions              string                       `json:"base_instructions"`
	ModelMessages                 any                          `json:"model_messages"`
	SupportsReasoningSummaries    bool                         `json:"supports_reasoning_summaries"`
	DefaultReasoningSummary       string                       `json:"default_reasoning_summary"`
	SupportVerbosity              bool                         `json:"support_verbosity"`
	DefaultVerbosity              *string                      `json:"default_verbosity"`
	AvailabilityNux               any                          `json:"availability_nux"`
	ApplyPatchToolType            *string                      `json:"apply_patch_tool_type"`
	WebSearchToolType             string                       `json:"web_search_tool_type"`
	TruncationPolicy              CodexBackendTruncationPolicy `json:"truncation_policy"`
	SupportsParallelToolCalls     bool                         `json:"supports_parallel_tool_calls"`
	SupportsImageDetailOriginal   bool                         `json:"supports_image_detail_original"`
	ContextWindow                 int                          `json:"context_window"`
	MaxContextWindow              *int                         `json:"max_context_window"`
	AutoCompactTokenLimit         *int                         `json:"auto_compact_token_limit"`
	EffectiveContextWindowPercent int                          `json:"effective_context_window_percent"`
	ExperimentalSupportedTools    []string                     `json:"experimental_supported_tools"`
	InputModalities               []string                     `json:"input_modalities"`
	UsedFallbackModelMetadata     bool                         `json:"used_fallback_model_metadata"`
	SupportsSearchTool            bool                         `json:"supports_search_tool"`
}

type CodexBackendReasoningLevel struct {
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type CodexBackendServiceTier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CodexBackendTruncationPolicy struct {
	Mode  string `json:"mode"`
	Limit int    `json:"limit"`
}

func BuildCodexBackendModelsResponse(rows []model.CodexBackendModel) CodexBackendModelsResponse {
	seen := map[string]struct{}{}
	models := make([]CodexBackendModelMetadata, 0, len(rows)+len(defaultCodexBackendModels))
	for _, row := range rows {
		slug := strings.TrimSpace(row.Slug)
		if slug == "" {
			continue
		}
		seen[slug] = struct{}{}
		displayName := strings.TrimSpace(row.DisplayName)
		if displayName == "" {
			displayName = slug
		}
		models = append(models, newCodexBackendModelMetadata(slug, displayName, row.Description, row.Priority))
	}
	for i, slug := range defaultCodexBackendModels {
		if _, ok := seen[slug]; ok {
			continue
		}
		models = append(models, newCodexBackendModelMetadata(slug, slug, "Codex model", len(defaultCodexBackendModels)-i))
	}
	return CodexBackendModelsResponse{Models: models}
}

func newCodexBackendModelMetadata(slug string, displayName string, description string, priority int) CodexBackendModelMetadata {
	return CodexBackendModelMetadata{
		Slug:                  slug,
		DisplayName:           displayName,
		Description:           description,
		DefaultReasoningLevel: "medium",
		SupportedReasoningLevels: []CodexBackendReasoningLevel{
			{Effort: "low", Description: "low"},
			{Effort: "medium", Description: "medium"},
			{Effort: "high", Description: "high"},
		},
		ShellType:                     "shell_command",
		Visibility:                    "list",
		SupportedInAPI:                true,
		Priority:                      priority,
		AdditionalSpeedTiers:          []string{},
		ServiceTiers:                  []CodexBackendServiceTier{},
		DefaultServiceTier:            nil,
		Upgrade:                       nil,
		BaseInstructions:              "",
		ModelMessages:                 nil,
		SupportsReasoningSummaries:    false,
		DefaultReasoningSummary:       "auto",
		SupportVerbosity:              false,
		DefaultVerbosity:              nil,
		AvailabilityNux:               nil,
		ApplyPatchToolType:            nil,
		WebSearchToolType:             "text",
		TruncationPolicy:              CodexBackendTruncationPolicy{Mode: "bytes", Limit: 200000},
		SupportsParallelToolCalls:     false,
		SupportsImageDetailOriginal:   false,
		ContextWindow:                 272000,
		MaxContextWindow:              nil,
		AutoCompactTokenLimit:         nil,
		EffectiveContextWindowPercent: 95,
		ExperimentalSupportedTools:    []string{},
		InputModalities:               []string{"text", "image"},
		UsedFallbackModelMetadata:     false,
		SupportsSearchTool:            false,
	}
}
