package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOAuthTokenQuotaTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalRedisEnabled := common.RedisEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitCol()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	model.DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		model.DB = originalDB
		common.RedisEnabled = originalRedisEnabled
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		model.InitCol()
	})
	return db
}

func TestPreConsumeTokenQuotaOAuthDoesNotTouchAPITokenTable(t *testing.T) {
	db := setupOAuthTokenQuotaTest(t)
	require.NoError(t, db.Create(&model.Token{
		Id:          1,
		UserId:      99,
		Key:         "unrelated-token",
		Status:      common.TokenStatusEnabled,
		Name:        "unrelated token",
		RemainQuota: 5_000,
		UsedQuota:   0,
	}).Error)
	relayInfo := &relaycommon.RelayInfo{
		TokenId:        0,
		TokenKey:       "oauth-access-token-is-not-an-api-token",
		TokenUnlimited: true,
		TokenAuthType:  constant.TokenAuthTypeOAuth,
	}

	err := PreConsumeTokenQuota(relayInfo, 100)

	require.NoError(t, err)
	var token model.Token
	require.NoError(t, db.First(&token, 1).Error)
	assert.Equal(t, 5_000, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)
}

func TestPreConsumeTokenQuotaStandardAPITokenStillTracksUsage(t *testing.T) {
	db := setupOAuthTokenQuotaTest(t)
	require.NoError(t, db.Create(&model.Token{
		Id:          1,
		UserId:      42,
		Key:         "standard-token",
		Status:      common.TokenStatusEnabled,
		Name:        "standard token",
		RemainQuota: 5_000,
		UsedQuota:   0,
	}).Error)
	relayInfo := &relaycommon.RelayInfo{
		TokenId:        1,
		TokenKey:       "standard-token",
		TokenUnlimited: false,
		TokenAuthType:  constant.TokenAuthTypeAPIToken,
	}

	err := PreConsumeTokenQuota(relayInfo, 100)

	require.NoError(t, err)
	var token model.Token
	require.NoError(t, db.First(&token, 1).Error)
	assert.Equal(t, 4_900, token.RemainQuota)
	assert.Equal(t, 100, token.UsedQuota)
}
