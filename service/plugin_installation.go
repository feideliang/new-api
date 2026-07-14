package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// InstallPlugin installs a plugin for the given user.
// Returns the install response with app_ids_needing_auth if requested.
func InstallPlugin(userId int, accountId string, pluginID string, includeAppsNeedingAuth bool) (*dto.PluginInstallResponse, error) {
	// Check plugin exists in catalog
	entry, err := GetPluginCatalogEntry(pluginID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil // plugin not found
	}

	// Check if already installed
	existing, found := model.IsPluginInstalled(userId, pluginID)
	if found {
		// Already installed — re-enable if disabled
		if !existing.Enabled {
			existing.Enabled = true
			if err := model.UpdateInstallation(existing); err != nil {
				return nil, err
			}
		}
	} else {
		// Create new installation
		inst := &model.PluginInstallation{
			UserID:           userId,
			AccountID:        accountId,
			PluginID:         pluginID,
			PluginName:       entry.Name,
			Scope:            "GLOBAL",
			Enabled:          true,
			DisabledSkills:   "[]",
			InstalledVersion: entry.Version,
		}
		if err := model.CreateInstallation(inst); err != nil {
			return nil, err
		}
	}

	resp := &dto.PluginInstallResponse{
		ID:      pluginID,
		Enabled: true,
	}

	if includeAppsNeedingAuth && len(entry.AppIDs) > 0 {
		resp.AppIDsNeedingAuth = entry.AppIDs
	}

	return resp, nil
}

// UninstallPlugin removes a plugin installation for the given user.
func UninstallPlugin(userId int, pluginID string) (*dto.PluginUninstallResponse, error) {
	// Find the installation
	existing, found := model.IsPluginInstalled(userId, pluginID)
	if found {
		// Soft uninstall — set enabled to false rather than deleting
		existing.Enabled = false
		if err := model.UpdateInstallation(existing); err != nil {
			return nil, err
		}
	}

	return &dto.PluginUninstallResponse{
		ID:      pluginID,
		Enabled: false,
	}, nil
}

// GetInstalledPlugins returns the installed plugins list for a user,
// merging catalog data with installation state.
func GetInstalledPlugins(userId int, scope string) (*dto.PluginListResponse, error) {
	entries, err := LoadPluginCatalog()
	if err != nil {
		return nil, err
	}

	installMap, err := model.GetInstalledPluginIDsMap(userId, scope)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.PluginItem, 0, len(installMap))
	for _, entry := range entries {
		inst, found := installMap[entry.QualifiedID]
		if !found {
			continue
		}

		enabled := inst.Enabled
		item := buildPluginItem(entry, false, &enabled)
		item.Enabled = &enabled
		items = append(items, item)
	}

	return &dto.PluginListResponse{
		Plugins: items,
		Pagination: dto.PluginPagination{
			Limit: 200, NextPageToken: nil,
		},
	}, nil
}
