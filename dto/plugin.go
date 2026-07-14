package dto

// ── Pagination ──────────────────────────────────────

type PluginPagination struct {
	Limit         int     `json:"limit"`
	NextPageToken *string `json:"next_page_token"`
}

// ── Plugin Interface (nested in release) ─────────────

type PluginInterface struct {
	ShortDescription string   `json:"short_description,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	LogoURL          string   `json:"logo_url,omitempty"`
	ScreenshotURLs   []string `json:"screenshot_urls,omitempty"`
	DefaultPrompts   []string `json:"default_prompts,omitempty"`
}

// ── Plugin Skill ─────────────────────────────────────

type PluginSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ── Plugin Release ───────────────────────────────────

type PluginRelease struct {
	Version           string          `json:"version"`
	DisplayName       string          `json:"display_name"`
	Description       string          `json:"description"`
	AppIDs            []string        `json:"app_ids"`
	Keywords          []string        `json:"keywords,omitempty"`
	Interface         PluginInterface `json:"interface"`
	Skills            []PluginSkill   `json:"skills,omitempty"`
	BundleDownloadURL string          `json:"bundle_download_url,omitempty"`
	AppManifest       interface{}     `json:"app_manifest,omitempty"`
}

// ── Plugin Item (in list / detail responses) ─────────

type PluginItem struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	Scope                string        `json:"scope"`
	InstallationPolicy   string        `json:"installation_policy"`
	AuthenticationPolicy string        `json:"authentication_policy"`
	Status               string        `json:"status"`
	Release              PluginRelease `json:"release"`
	// Only present in installed-list responses:
	Enabled            *bool    `json:"enabled,omitempty"`
	DisabledSkillNames []string `json:"disabled_skill_names,omitempty"`
}

// ── Plugin List Response ─────────────────────────────

type PluginListResponse struct {
	Plugins    []*PluginItem   `json:"plugins"`
	Pagination PluginPagination `json:"pagination"`
}

// ── Install Response ─────────────────────────────────

type PluginInstallResponse struct {
	ID               string   `json:"id"`
	Enabled          bool     `json:"enabled"`
	AppIDsNeedingAuth []string `json:"app_ids_needing_auth,omitempty"`
}

// ── Uninstall Response ───────────────────────────────

type PluginUninstallResponse struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// ── Plugin Detail Response (single plugin) ───────────

type PluginDetailResponse struct {
	ID                   string        `json:"id"`
	Name                 string        `json:"name"`
	Scope                string        `json:"scope"`
	InstallationPolicy   string        `json:"installation_policy"`
	AuthenticationPolicy string        `json:"authentication_policy"`
	Status               string        `json:"status"`
	Release              PluginRelease `json:"release"`
}

// ── Connector Directory ──────────────────────────────

type ConnectorApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	LogoURL     string `json:"logo_url,omitempty"`
}

type ConnectorDirectoryResponse struct {
	Apps      []ConnectorApp `json:"apps"`
	NextToken *string        `json:"next_token"`
}

// ── Account Settings ─────────────────────────────────

type AccountSettingsBetaSettings struct {
	EnablePlugins bool `json:"enable_plugins"`
}

type AccountSettingsResponse struct {
	BetaSettings AccountSettingsBetaSettings `json:"beta_settings"`
}

// ── Analytics Events ─────────────────────────────────

type AnalyticsEventsResponse struct {
	Status string `json:"status"`
}
