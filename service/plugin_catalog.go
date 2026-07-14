package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ── Internal types for marketplace parsing ──────────

type marketplaceEntry struct {
	Name     string          `json:"name"`
	Source   json.RawMessage `json:"source"`
	Policy   json.RawMessage `json:"policy"`
	Category string          `json:"category"`
}

type marketplaceFile struct {
	Name      string                 `json:"name"`
	Interface *marketplaceInterface  `json:"interface,omitempty"`
	Plugins   []marketplaceEntry     `json:"plugins"`
}

type marketplaceInterface struct {
	DisplayName string `json:"displayName,omitempty"`
}

type pluginManifest struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Keywords    []string               `json:"keywords"`
	Interface   *pluginManifestInterface `json:"interface,omitempty"`
}

type pluginManifestInterface struct {
	DisplayName         string   `json:"displayName,omitempty"`
	ShortDescription    string   `json:"shortDescription,omitempty"`
	LongDescription     string   `json:"longDescription,omitempty"`
	DeveloperName       string   `json:"developerName,omitempty"`
	Category            string   `json:"category,omitempty"`
	Capabilities        []string `json:"capabilities,omitempty"`
	WebsiteURL          string   `json:"websiteURL,omitempty"`
	PrivacyPolicyURL    string   `json:"privacyPolicyURL,omitempty"`
	TermsOfServiceURL   string   `json:"termsOfServiceURL,omitempty"`
	BrandColor          string   `json:"brandColor,omitempty"`
	Logo                string   `json:"logo,omitempty"`
	ComposerIcon        string   `json:"composerIcon,omitempty"`
	DefaultPrompt       json.RawMessage `json:"defaultPrompt,omitempty"`
	Screenshots         []string `json:"screenshots,omitempty"`
}

type appManifest struct {
	Apps map[string]appEntry `json:"apps"`
}

type appEntry struct {
	ID string `json:"id"`
}

// ── Catalog Entry ───────────────────────────────────

type PluginCatalogEntry struct {
	MarketplaceName      string
	MarketplaceDisplay   string
	QualifiedID          string
	Name                 string
	Version              string
	Description          string
	Category             string
	DisplayName          string
	InstallationPolicy   string
	AuthenticationPolicy string
	AppIDs               []string
	Keywords             []string
	Skills               []dto.PluginSkill
	ShortDescription     string
	Capabilities         []string
	LogoURL              string
	ScreenshotURLs       []string
	DefaultPrompts       []string
	AppManifestData      interface{}
	PluginDir            string
}

// ── Catalog Cache ───────────────────────────────────

var (
	catalogCache      []PluginCatalogEntry
	catalogCacheMu    sync.RWMutex
	catalogCacheTime  time.Time
	catalogCacheTTL   = 5 * time.Minute
)

const defaultPluginStorePath = "plugins/.tmp"

func getPluginStorePath() string {
	path := defaultPluginStorePath
	if _, err := os.Stat(path); os.IsNotExist(err) {
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

// ── Public API ──────────────────────────────────────

func LoadPluginCatalog() ([]PluginCatalogEntry, error) {
	catalogCacheMu.RLock()
	if len(catalogCache) > 0 && time.Since(catalogCacheTime) < catalogCacheTTL {
		entries := catalogCache
		catalogCacheMu.RUnlock()
		return entries, nil
	}
	catalogCacheMu.RUnlock()

	return loadPluginCatalogFresh()
}

func loadPluginCatalogFresh() ([]PluginCatalogEntry, error) {
	storePath := getPluginStorePath()

	// Find all marketplace.json files
	marketplaceFiles := findMarketplaceFiles(storePath)

	var allEntries []PluginCatalogEntry
	for _, mf := range marketplaceFiles {
		entries, err := loadMarketplace(mf)
		if err != nil {
			common.SysLog("plugin catalog: error loading marketplace " + mf + ": " + err.Error())
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	catalogCacheMu.Lock()
	catalogCache = allEntries
	catalogCacheTime = time.Now()
	catalogCacheMu.Unlock()

	return allEntries, nil
}

func GetPluginCatalogEntry(qualifiedID string) (*PluginCatalogEntry, error) {
	entries, err := LoadPluginCatalog()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.QualifiedID == qualifiedID {
			return &e, nil
		}
	}
	return nil, nil
}

func GetFeaturedPluginIDs() []string {
	// Return first 10 plugins as featured
	entries, err := LoadPluginCatalog()
	if err != nil || len(entries) == 0 {
		return []string{}
	}
	n := 10
	if len(entries) < n {
		n = len(entries)
	}
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = entries[i].QualifiedID
	}
	return ids
}

func GetSuggestedPluginIDs() []string {
	// Return same as featured for now
	return GetFeaturedPluginIDs()
}

func BuildPluginListResponse(scope string, limit int, collection string) (*dto.PluginListResponse, error) {
	entries, err := LoadPluginCatalog()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}

	items := make([]*dto.PluginItem, 0, len(entries))
	for _, e := range entries {
		item := buildPluginItem(e, false, nil)
		items = append(items, item)
	}

	// Trim to limit
	if len(items) > limit {
		items = items[:limit]
	}

	return &dto.PluginListResponse{
		Plugins: items,
		Pagination: dto.PluginPagination{
			Limit:         limit,
			NextPageToken: nil,
		},
	}, nil
}

func BuildPluginListWithInstallState(userId int, scope string) (*dto.PluginListResponse, error) {
	entries, err := LoadPluginCatalog()
	if err != nil {
		return nil, err
	}

	// Not using model directly here to avoid circular imports.
	// We'll handle this in the controller layer or make a thin wrapper.
	_ = userId
	_ = scope

	items := make([]*dto.PluginItem, 0, len(entries))
	for _, e := range entries {
		item := buildPluginItem(e, false, nil)
		items = append(items, item)
	}

	return &dto.PluginListResponse{
		Plugins: items,
		Pagination: dto.PluginPagination{
			Limit: 200, NextPageToken: nil,
		},
	}, nil
}

func GetPluginDetail(pluginID string, includeDownloadUrls bool) (*dto.PluginItem, error) {
	entry, err := GetPluginCatalogEntry(pluginID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	item := buildPluginItem(*entry, true, nil)

	if includeDownloadUrls {
		// Currently all plugins are on local filesystem, so no bundle download URL needed
		// In future, could serve tar.gz from plugin directory
	}

	return item, nil
}

// ── Internal Helpers ────────────────────────────────

func findMarketplaceFiles(storePath string) []string {
	var files []string
	candidates := []string{
		filepath.Join(storePath, "plugins", ".agents", "plugins", "marketplace.json"),
	}
	// Additional marketplaces
	// bundled-marketplaces/*/.../marketplace.json
	globPath := filepath.Join(storePath, "bundled-marketplaces", "*", ".agents", "plugins", "marketplace.json")
	if matches, err := filepath.Glob(globPath); err == nil {
		candidates = append(candidates, matches...)
	}
	// marketplaces/*/.../marketplace.json
	globPath = filepath.Join(storePath, "marketplaces", "*", ".agents", "plugins", "marketplace.json")
	if matches, err := filepath.Glob(globPath); err == nil {
		candidates = append(candidates, matches...)
	}

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			files = append(files, f)
		}
	}
	return files
}

func loadMarketplace(marketplacePath string) ([]PluginCatalogEntry, error) {
	data, err := os.ReadFile(marketplacePath)
	if err != nil {
		return nil, err
	}

	var mf marketplaceFile
	if err := common.Unmarshal(data, &mf); err != nil {
		return nil, err
	}

	marketplaceName := mf.Name
	marketplaceDisplay := marketplaceName
	if mf.Interface != nil && mf.Interface.DisplayName != "" {
		marketplaceDisplay = mf.Interface.DisplayName
	}

	// Base directory for resolving relative plugin paths
	baseDir := filepath.Dir(marketplacePath)

	var entries []PluginCatalogEntry
	for _, entry := range mf.Plugins {
		pluginDir := resolvePluginDir(baseDir, entry.Source)
		if pluginDir == "" {
			continue
		}

		// Generate remote plugin ID in Codex CLI format: plugins~Plugin_{name}
		qualifiedID := "plugins~Plugin_" + entry.Name

		catEntry := PluginCatalogEntry{
			MarketplaceName:      marketplaceName,
			MarketplaceDisplay:   marketplaceDisplay,
			QualifiedID:          qualifiedID,
			Name:                 entry.Name,
			Category:             entry.Category,
			InstallationPolicy:   "AVAILABLE",
			AuthenticationPolicy: "ON_INSTALL",
			PluginDir:            pluginDir,
		}

		// Parse policy
		if len(entry.Policy) > 0 {
			parsePolicy(entry.Policy, &catEntry)
		}

		// Load plugin.json manifest
		loadPluginManifest(pluginDir, &catEntry)

		// Load .app.json for app IDs
		loadAppManifest(pluginDir, &catEntry)

		// Load skills
		loadSkills(pluginDir, &catEntry)

		entries = append(entries, catEntry)
	}

	return entries, nil
}

func resolvePluginDir(baseDir string, source json.RawMessage) string {
	// Try to parse source as object with "path" field first
	var sourceObj struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	if err := common.Unmarshal(source, &sourceObj); err == nil && sourceObj.Path != "" {
		return resolvePluginPath(baseDir, sourceObj.Path)
	}

	// Try as plain string path
	var pathStr string
	if err := common.Unmarshal(source, &pathStr); err == nil && pathStr != "" {
		return resolvePluginPath(baseDir, pathStr)
	}

	return ""
}

// resolvePluginPath tries multiple base directories to resolve the plugin path.
// The marketplace.json can be nested deep (e.g. .agents/plugins/marketplace.json)
// while the actual plugin directories are at a higher level.
func resolvePluginPath(baseDir string, relPath string) string {
	// Try direct resolution from baseDir
	resolved := filepath.Join(baseDir, relPath)
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return resolved
	}

	// Try going up 1 level (marketplace in .agents/plugins/)
	parent := filepath.Dir(baseDir)
	resolved = filepath.Join(parent, relPath)
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return resolved
	}

	// Try going up 2 levels
	parent = filepath.Dir(parent)
	resolved = filepath.Join(parent, relPath)
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return resolved
	}

	// Try going up 3 levels
	parent = filepath.Dir(parent)
	resolved = filepath.Join(parent, relPath)
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return resolved
	}

	return ""
}

func parsePolicy(data json.RawMessage, entry *PluginCatalogEntry) {
	var policy struct {
		Installation   string `json:"installation"`
		Authentication string `json:"authentication"`
		Products       []string `json:"products"`
	}
	if err := common.Unmarshal(data, &policy); err != nil {
		return
	}
	if policy.Installation != "" {
		entry.InstallationPolicy = policy.Installation
	}
	if policy.Authentication != "" {
		entry.AuthenticationPolicy = policy.Authentication
	}
}

func loadPluginManifest(pluginDir string, entry *PluginCatalogEntry) {
	manifestPath := filepath.Join(pluginDir, ".codex-plugin", "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return
	}

	var manifest pluginManifest
	if err := common.Unmarshal(data, &manifest); err != nil {
		return
	}

	entry.Name = manifest.Name
	entry.Version = manifest.Version
	entry.Description = manifest.Description
	entry.Keywords = manifest.Keywords

	if manifest.Interface != nil {
		if manifest.Interface.DisplayName != "" {
			entry.DisplayName = manifest.Interface.DisplayName
		}
		entry.ShortDescription = manifest.Interface.ShortDescription
		entry.Capabilities = manifest.Interface.Capabilities
		entry.ScreenshotURLs = manifest.Interface.Screenshots
		if entry.Category == "" {
			entry.Category = manifest.Interface.Category
		}
		// Parse Logo as a local path (will be resolved to URL by the frontend)
		if manifest.Interface.Logo != "" {
			entry.LogoURL = manifest.Interface.Logo
		}
		// Parse defaultPrompt — can be a string or array of strings
		if len(manifest.Interface.DefaultPrompt) > 0 {
			entry.DefaultPrompts = parseDefaultPrompt(manifest.Interface.DefaultPrompt)
		}
	}

	// Fallback display name
	if entry.DisplayName == "" {
		entry.DisplayName = entry.Name
	}
}

func loadAppManifest(pluginDir string, entry *PluginCatalogEntry) {
	appPath := filepath.Join(pluginDir, ".app.json")
	data, err := os.ReadFile(appPath)
	if err != nil {
		return
	}

	var am appManifest
	if err := common.Unmarshal(data, &am); err != nil {
		return
	}

	for _, app := range am.Apps {
		if app.ID != "" {
			entry.AppIDs = append(entry.AppIDs, app.ID)
		}
	}

	if len(am.Apps) > 0 {
		entry.AppManifestData = am.Apps
	}
}

func loadSkills(pluginDir string, entry *PluginCatalogEntry) {
	skillsDir := filepath.Join(pluginDir, "skills")
	skillDirs, err := os.ReadDir(skillsDir)
	if err != nil {
		return
	}

	for _, sd := range skillDirs {
		if !sd.IsDir() {
			continue
		}
		skillPath := filepath.Join(skillsDir, sd.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		name, description := parseSkillFrontmatter(string(data))
		if name == "" {
			name = sd.Name()
		}
		entry.Skills = append(entry.Skills, dto.PluginSkill{
			Name:        name,
			Description: description,
		})
	}
}

func parseSkillFrontmatter(content string) (name, description string) {
	// YAML frontmatter format:
	// ---
	// name: linear
	// description: ...
	// ---
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return "", ""
	}
	frontmatter := strings.TrimSpace(parts[1])
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		} else if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return
}

// parseDefaultPrompt handles defaultPrompt which can be a string or array of strings
func parseDefaultPrompt(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	// Try as array first
	var arr []string
	if err := common.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	// Try as single string
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return []string{s}
	}
	return nil
}

// nonNilSlice ensures nil slices are serialized as [] instead of null
func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func buildPluginItem(entry PluginCatalogEntry, detail bool, installState *bool) *dto.PluginItem {
	item := &dto.PluginItem{
		ID:                   entry.QualifiedID,
		Name:                 entry.Name,
		Scope:                "GLOBAL",
		InstallationPolicy:   entry.InstallationPolicy,
		AuthenticationPolicy: entry.AuthenticationPolicy,
		Status:               "ENABLED",
		Release: dto.PluginRelease{
			Version:     entry.Version,
			DisplayName: entry.DisplayName,
			Description: entry.Description,
			AppIDs:      nonNilSlice(entry.AppIDs),
			Keywords:    nonNilSlice(entry.Keywords),
			Interface: dto.PluginInterface{
				ShortDescription: entry.ShortDescription,
				Capabilities:     nonNilSlice(entry.Capabilities),
				LogoURL:          entry.LogoURL,
				ScreenshotURLs:   nonNilSlice(entry.ScreenshotURLs),
				DefaultPrompts:   nonNilSlice(entry.DefaultPrompts),
			},
			Skills: nonNilSlice(entry.Skills),
		},
	}

	if detail && entry.AppManifestData != nil {
		item.Release.AppManifest = entry.AppManifestData
	}

	if installState != nil {
		item.Enabled = installState
	}

	return item
}

// ── Connector Directory ─────────────────────────────

func BuildConnectorDirectory() (*dto.ConnectorDirectoryResponse, error) {
	entries, err := LoadPluginCatalog()
	if err != nil {
		return nil, err
	}

	apps := make([]dto.ConnectorApp, 0)
	seen := make(map[string]bool)

	for _, entry := range entries {
		for _, appID := range entry.AppIDs {
			if seen[appID] {
				continue
			}
			seen[appID] = true
			apps = append(apps, dto.ConnectorApp{
				ID:   appID,
				Name: entry.DisplayName,
			})
		}
	}

	return &dto.ConnectorDirectoryResponse{
		Apps:      apps,
		NextToken: nil,
	}, nil
}
