package model

import (
	"time"

	"gorm.io/gorm"
)

type CodexBackendModel struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	Slug        string `json:"slug" gorm:"type:varchar(128);uniqueIndex;not null"`
	DisplayName string `json:"display_name" gorm:"type:varchar(128);not null"`
	Description string `json:"description" gorm:"type:text"`
	Priority    int    `json:"priority" gorm:"default:0;index"`
	Enabled     bool   `json:"enabled" gorm:"not null;index"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func (CodexBackendModel) TableName() string {
	return "gpt_codex_models"
}

func (m *CodexBackendModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if m.CreatedTime == 0 {
		m.CreatedTime = now
	}
	if m.UpdatedTime == 0 {
		m.UpdatedTime = now
	}
	return nil
}

func (m *CodexBackendModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedTime = time.Now().Unix()
	return nil
}

func GetEnabledCodexBackendModels() ([]CodexBackendModel, error) {
	if DB == nil {
		return nil, nil
	}
	var models []CodexBackendModel
	err := DB.Where("enabled = ?", true).Order("priority desc").Order("id asc").Find(&models).Error
	return models, err
}
