package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestOAuthRelayTokenKeyUsesSecret(t *testing.T) {
	originalCryptoSecret := common.CryptoSecret
	common.CryptoSecret = "oauth-relay-test-secret"
	t.Cleanup(func() {
		common.CryptoSecret = originalCryptoSecret
	})

	clientID := "app_EMoamEEZ73f0CkXaXp7hrann"
	legacySum := sha256.Sum256([]byte(fmt.Sprintf("oauth-relay:%d:%s", 42, clientID)))
	legacyKey := "oauth-" + hex.EncodeToString(legacySum[:])

	require.NotEqual(t, legacyKey, OAuthRelayTokenKey(42, clientID))
	require.Equal(t, OAuthRelayTokenKey(42, clientID), OAuthRelayTokenKey(42, clientID))
}

func TestGetOrCreateOAuthRelayTokenRotatesLegacyDeterministicKey(t *testing.T) {
	db := setupOAuthServerModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Token{}))
	DB = db

	originalCryptoSecret := common.CryptoSecret
	common.CryptoSecret = "oauth-relay-test-secret"
	t.Cleanup(func() {
		common.CryptoSecret = originalCryptoSecret
	})

	clientID := "app_EMoamEEZ73f0CkXaXp7hrann"
	legacyKey := legacyOAuthRelayTokenKey(42, clientID)
	require.NoError(t, db.Create(&Token{
		UserId:      42,
		Key:         legacyKey,
		Status:      common.TokenStatusEnabled,
		Name:        "OAuth: Codex CLI",
		ExpiredTime: -1,
	}).Error)

	token, err := GetOrCreateOAuthRelayToken(42, clientID, "Codex CLI")
	require.NoError(t, err)
	require.NotEqual(t, legacyKey, token.Key)
	require.Equal(t, OAuthRelayTokenKey(42, clientID), token.Key)

	_, err = GetTokenByKey(legacyKey, true)
	require.Error(t, err)
}

func TestGetOrCreateNamedUserTokenCreatesAndReusesToken(t *testing.T) {
	db := setupOAuthServerModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Token{}))
	DB = db

	created, err := GetOrCreateNamedUserToken(7, "codex-token")
	require.NoError(t, err)
	require.Equal(t, 7, created.UserId)
	require.Equal(t, "codex-token", created.Name)
	require.NotEmpty(t, created.Key)
	require.Equal(t, common.TokenStatusEnabled, created.Status)
	require.Equal(t, int64(-1), created.ExpiredTime)
	require.True(t, created.UnlimitedQuota)

	reused, err := GetOrCreateNamedUserToken(7, "codex-token")
	require.NoError(t, err)
	require.Equal(t, created.Id, reused.Id)
	require.Equal(t, created.Key, reused.Key)

	require.NoError(t, db.Model(reused).Updates(map[string]interface{}{
		"status":          common.TokenStatusDisabled,
		"expired_time":    int64(123),
		"unlimited_quota": false,
	}).Error)
	restored, err := GetOrCreateNamedUserToken(7, "codex-token")
	require.NoError(t, err)
	require.Equal(t, reused.Id, restored.Id)
	require.Equal(t, reused.Key, restored.Key)
	require.Equal(t, common.TokenStatusEnabled, restored.Status)
	require.Equal(t, int64(-1), restored.ExpiredTime)
	require.True(t, restored.UnlimitedQuota)

	var count int64
	require.NoError(t, db.Model(&Token{}).Where("user_id = ? AND name = ?", 7, "codex-token").Count(&count).Error)
	require.Equal(t, int64(1), count)
}
