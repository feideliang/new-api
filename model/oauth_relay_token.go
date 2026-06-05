package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// GetOrCreateNamedUserToken finds or creates a token with the given user_id and name.
// If a token already exists:
//   - enabled with correct expiry/quota → return as-is
//   - disabled or misconfigured → reactivate with default settings
// If no token exists → create a new one with generated key, unlimited quota, never expires.
// Ensures at most one token per (user_id, name) pair.
func GetOrCreateNamedUserToken(userId int, name string) (*Token, error) {
	var token Token
	result := DB.Where("user_id = ? AND name = ?", userId, name).First(&token)

	if result.Error == nil {
		// Token exists — check if it's in good state
		if token.Status == common.TokenStatusEnabled && token.ExpiredTime == -1 && token.UnlimitedQuota {
			return &token, nil
		}
		// Reactivate
		token.Status = common.TokenStatusEnabled
		token.ExpiredTime = -1
		token.UnlimitedQuota = true
		if err := token.Update(); err != nil {
			return nil, err
		}
		return &token, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}

	// Create new token
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}

	token = Token{
		UserId:        userId,
		Key:           "oauth-" + key,
		Name:          name,
		Status:        common.TokenStatusEnabled,
		ExpiredTime:   -1,
		UnlimitedQuota: true,
		CreatedTime:   time.Now().UnixMilli(),
	}

	if err := token.Insert(); err != nil {
		return nil, err
	}

	return &token, nil
}