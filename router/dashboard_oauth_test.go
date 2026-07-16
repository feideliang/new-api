package router

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/oauthserver"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDashboardOAuthBillingTest(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalDisplayTokenStatEnabled := common.DisplayTokenStatEnabled
	originalQuotaPerUnit := common.QuotaPerUnit
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	model.InitCol()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.OAuthServerClient{},
		&model.OAuthServerAuthorizationCode{},
		&model.OAuthServerAccessToken{},
		&model.OAuthServerRefreshToken{},
		&model.OAuthServerUserGrant{},
	))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.DisplayTokenStatEnabled = true
	common.QuotaPerUnit = 500_000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD

	require.NoError(t, db.Create(&model.User{
		Id:          42,
		Username:    "oauth-dashboard-user",
		DisplayName: "OAuth Dashboard User",
		Email:       "oauth-dashboard@example.test",
		Status:      common.UserStatusEnabled,
		Quota:       37_500_000,
		UsedQuota:   12_500_000,
		Group:       "default",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          1,
		UserId:      99,
		Key:         "unrelated-dashboard-token",
		Status:      common.TokenStatusEnabled,
		Name:        "unrelated dashboard token",
		RemainQuota: 2_500_000,
		ExpiredTime: -1,
	}).Error)
	client := oauthserver.DefaultCodexClient()
	require.NoError(t, db.Create(&client).Error)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}))
	svc, err := oauthserver.New(db, oauthserver.Config{
		Issuer:               "https://new-api.example.test",
		SigningPrivateKeyPEM: privateKeyPEM,
		SigningKeyID:         "dashboard-test",
		AuthorizationCodeTTL: time.Minute,
		AccessTokenTTL:       time.Hour,
		IDTokenTTL:           time.Hour,
		RefreshTokenTTL:      time.Hour,
	})
	require.NoError(t, err)
	verifier := strings.Repeat("v", 48)
	challengeSum := sha256.Sum256([]byte(verifier))
	code, err := svc.CreateAuthorizationCode(context.Background(), oauthserver.AuthorizationRequest{
		UserID:              42,
		ClientID:            oauthserver.DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               "openid profile api.connectors.invoke",
		CodeChallenge:       base64.RawURLEncoding.EncodeToString(challengeSum[:]),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), oauthserver.AuthorizationCodeTokenRequest{
		ClientID:     oauthserver.DefaultCodexClientID,
		Code:         code.Code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.DisplayTokenStatEnabled = originalDisplayTokenStatEnabled
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		model.InitCol()
	})
	return tokens.AccessToken
}

func TestDashboardBillingOAuthReturnsWalletTotalsWhenTokenIDsCollide(t *testing.T) {
	accessToken := setupDashboardOAuthBillingTest(t)
	router := gin.New()
	SetDashboardRouter(router)

	t.Run("subscription", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/dashboard/billing/subscription", nil)
		request.Header.Set("Authorization", "Bearer "+accessToken)
		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		var response struct {
			HardLimitUSD float64 `json:"hard_limit_usd"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, 100.0, response.HardLimitUSD)
	})

	t.Run("usage", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/dashboard/billing/usage", nil)
		request.Header.Set("Authorization", "Bearer "+accessToken)
		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		var response struct {
			TotalUsage float64 `json:"total_usage"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.Equal(t, 2500.0, response.TotalUsage)
	})
}
