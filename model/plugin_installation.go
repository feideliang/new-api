package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type PluginInstallation struct {
	ID               int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID           int    `json:"user_id" gorm:"not null;index:idx_plugin_user,unique;index"`
	AccountID        string `json:"account_id" gorm:"type:varchar(128);not null;index"`
	PluginID         string `json:"plugin_id" gorm:"type:varchar(255);not null;index:idx_plugin_user,unique"`
	PluginName       string `json:"plugin_name" gorm:"type:varchar(128);not null"`
	Scope            string `json:"scope" gorm:"type:varchar(16);not null;default:GLOBAL"`
	Enabled          bool   `json:"enabled" gorm:"default:true"`
	DisabledSkills   string `json:"disabled_skills" gorm:"type:text"`
	InstalledVersion string `json:"installed_version" gorm:"type:varchar(64)"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime      int64  `json:"updated_time" gorm:"bigint"`
}

func (PluginInstallation) TableName() string {
	return "plugin_installations"
}

// ── CRUD Functions ──────────────────────────────────

func GetInstallationsByUser(userId int, scope string) ([]*PluginInstallation, error) {
	var installations []*PluginInstallation
	query := DB.Where("user_id = ?", userId)
	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	err := query.Find(&installations).Error
	return installations, err
}

func GetInstallation(userId int, pluginId string) (*PluginInstallation, error) {
	var installation PluginInstallation
	err := DB.Where("user_id = ? AND plugin_id = ?", userId, pluginId).First(&installation).Error
	if err != nil {
		return nil, err
	}
	return &installation, nil
}

func CreateInstallation(installation *PluginInstallation) error {
	now := time.Now().Unix()
	installation.CreatedTime = now
	installation.UpdatedTime = now
	return DB.Create(installation).Error
}

func UpdateInstallation(installation *PluginInstallation) error {
	installation.UpdatedTime = time.Now().Unix()
	return DB.Save(installation).Error
}

func DeleteInstallation(userId int, pluginId string) error {
	return DB.Where("user_id = ? AND plugin_id = ?", userId, pluginId).Delete(&PluginInstallation{}).Error
}

func GetInstalledPluginIDsMap(userId int, scope string) (map[string]*PluginInstallation, error) {
	installations, err := GetInstallationsByUser(userId, scope)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*PluginInstallation, len(installations))
	for _, inst := range installations {
		result[inst.PluginID] = inst
	}
	return result, nil
}

// IsPluginInstalled checks if a plugin is installed for a user
func IsPluginInstalled(userId int, pluginId string) (*PluginInstallation, bool) {
	inst, err := GetInstallation(userId, pluginId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false
		}
		return nil, false
	}
	return inst, true
}
