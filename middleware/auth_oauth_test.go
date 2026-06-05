package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/oauthserver"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenAuthOAuthTest(t *testing.T) (*gorm.DB, *oauthserver.Service) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

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

	user := model.User{
		Id:          42,
		Username:    "relay-user",
		DisplayName: "Relay User",
		Email:       "relay@example.com",
		Status:      common.UserStatusEnabled,
		Quota:       100000,
		Group:       "default",
	}
	require.NoError(t, db.Create(&user).Error)

	client := oauthserver.DefaultCodexClient()
	require.NoError(t, db.Create(&client).Error)

	svc, err := oauthserver.New(db, oauthserver.Config{
		Issuer:               "https://new-api.example.test",
		SigningPrivateKeyPEM: generateTokenAuthOAuthPrivateKeyPEM(t),
		SigningKeyID:         "relay-test",
		AuthorizationCodeTTL: time.Minute,
		AccessTokenTTL:       time.Hour,
		IDTokenTTL:           time.Hour,
		RefreshTokenTTL:      time.Hour,
		Now:                  func() time.Time { return time.Now().UTC().Truncate(time.Second) },
	})
	require.NoError(t, err)
	return db, svc
}

func generateTokenAuthOAuthPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

func issueTokenAuthOAuthAccessToken(t *testing.T, svc *oauthserver.Service, scope string) string {
	t.Helper()
	verifier := strings.Repeat("v", 48)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	code, err := svc.CreateAuthorizationCode(context.Background(), oauthserver.AuthorizationRequest{
		UserID:              42,
		ClientID:            oauthserver.DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               scope,
		CodeChallenge:       challenge,
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
	return tokens.AccessToken
}

func performTokenAuthRequest(t *testing.T, authorization string, assertContext func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		assertContext(c)
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[]}`))
	request.Header.Set("Authorization", authorization)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestTokenAuthAcceptsOAuthAccessTokenForRelayAttribution(t *testing.T) {
	_, svc := setupTokenAuthOAuthTest(t)
	accessToken := issueTokenAuthOAuthAccessToken(t, svc, "openid profile api.connectors.invoke")

	recorder := performTokenAuthRequest(t, "Bearer "+accessToken, func(c *gin.Context) {
		require.Equal(t, 42, c.GetInt("id"))
		require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUserGroup))
		require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.NotZero(t, c.GetInt("token_id"))
		require.NotEmpty(t, c.GetString("token_key"))
		require.Equal(t, "OAuth: Codex CLI", c.GetString("token_name"))
		err := service.PreConsumeTokenQuota(&relaycommon.RelayInfo{
			UserId:         c.GetInt("id"),
			TokenId:        c.GetInt("token_id"),
			TokenKey:       c.GetString("token_key"),
			TokenUnlimited: c.GetBool("token_unlimited_quota"),
		}, 1)
		require.NoError(t, err)
	})

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestTokenAuthRejectsRevokedOAuthAccessToken(t *testing.T) {
	_, svc := setupTokenAuthOAuthTest(t)
	accessToken := issueTokenAuthOAuthAccessToken(t, svc, "openid profile api.connectors.invoke")
	require.NoError(t, svc.Revoke(context.Background(), accessToken, oauthserver.TokenHintAccess))

	recorder := performTokenAuthRequest(t, "Bearer "+accessToken, func(c *gin.Context) {
		t.Fatalf("revoked oauth token should not reach handler")
	})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestTokenAuthRejectsOAuthAccessTokenWithoutRelayScope(t *testing.T) {
	_, svc := setupTokenAuthOAuthTest(t)
	accessToken := issueTokenAuthOAuthAccessToken(t, svc, "openid profile")

	recorder := performTokenAuthRequest(t, "Bearer "+accessToken, func(c *gin.Context) {
		t.Fatalf("oauth token without relay scope should not reach handler")
	})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestTokenAuthRejectsLegacyDeterministicOAuthRelayTokenKey(t *testing.T) {
	_, svc := setupTokenAuthOAuthTest(t)
	accessToken := issueTokenAuthOAuthAccessToken(t, svc, "openid profile api.connectors.invoke")
	recorder := performTokenAuthRequest(t, "Bearer "+accessToken, func(c *gin.Context) {})
	require.Equal(t, http.StatusNoContent, recorder.Code)

	legacySum := sha256.Sum256([]byte(fmt.Sprintf("oauth-relay:%d:%s", 42, oauthserver.DefaultCodexClientID)))
	legacyKey := "oauth-" + hex.EncodeToString(legacySum[:])
	recorder = performTokenAuthRequest(t, "Bearer sk-"+legacyKey, func(c *gin.Context) {
		t.Fatalf("legacy deterministic oauth relay token key should not reach handler")
	})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestTokenAuthStillAcceptsStandardSKToken(t *testing.T) {
	db, _ := setupTokenAuthOAuthTest(t)
	require.NoError(t, db.Create(&model.Token{
		UserId:         42,
		Key:            "regular",
		Status:         common.TokenStatusEnabled,
		Name:           "regular",
		ExpiredTime:    -1,
		RemainQuota:    100000,
		UnlimitedQuota: false,
		Group:          "",
	}).Error)

	recorder := performTokenAuthRequest(t, "Bearer sk-regular", func(c *gin.Context) {
		require.Equal(t, 42, c.GetInt("id"))
		require.Equal(t, "regular", c.GetString("token_name"))
		require.Equal(t, "regular", c.GetString("token_key"))
	})

	require.Equal(t, http.StatusNoContent, recorder.Code)
}
