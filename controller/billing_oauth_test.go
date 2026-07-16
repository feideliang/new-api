package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBillingOAuthTest(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalDisplayTokenStatEnabled := common.DisplayTokenStatEnabled
	originalQuotaPerUnit := common.QuotaPerUnit
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.DisplayTokenStatEnabled = true
	common.QuotaPerUnit = 500_000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD

	require.NoError(t, db.Create(&model.User{
		Id:        42,
		Username:  "oauth-wallet-user",
		Status:    common.UserStatusEnabled,
		Quota:     37_500_000,
		UsedQuota: 12_500_000,
		Group:     "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             1,
		UserId:         99,
		Key:            "unrelated-token",
		Status:         common.TokenStatusEnabled,
		Name:           "unrelated token with colliding id",
		RemainQuota:    2_500_000,
		UsedQuota:      0,
		UnlimitedQuota: false,
		ExpiredTime:    -1,
	}).Error)

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.DisplayTokenStatEnabled = originalDisplayTokenStatEnabled
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
	})
}

func performBillingRequest(t *testing.T, handler gin.HandlerFunc, authType string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.GET("/billing", func(c *gin.Context) {
		c.Set("id", 42)
		c.Set("token_id", 1)
		c.Set("token_auth_type", authType)
		handler(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/billing", nil)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestBillingOAuthUsesUserWalletWhenTokenIdsCollide(t *testing.T) {
	setupBillingOAuthTest(t)

	t.Run("subscription reads wallet total", func(t *testing.T) {
		recorder := performBillingRequest(t, GetSubscription, "oauth")

		require.Equal(t, http.StatusOK, recorder.Code)
		var response OpenAISubscriptionResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, 100.0, response.HardLimitUSD)
		assert.Equal(t, 100.0, response.SoftLimitUSD)
		assert.Equal(t, int64(0), response.AccessUntil)
	})

	t.Run("usage reads wallet consumption", func(t *testing.T) {
		recorder := performBillingRequest(t, GetUsage, "oauth")

		require.Equal(t, http.StatusOK, recorder.Code)
		var response OpenAIUsageResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, 2500.0, response.TotalUsage)
	})
}

func TestBillingStandardAPITokenStillUsesTokenStatistics(t *testing.T) {
	setupBillingOAuthTest(t)

	recorder := performBillingRequest(t, GetSubscription, "api_token")

	require.Equal(t, http.StatusOK, recorder.Code)
	var response OpenAISubscriptionResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, 5.0, response.HardLimitUSD)
}
