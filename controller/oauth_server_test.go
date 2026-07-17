package controller

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	oauthserversvc "github.com/QuantumNous/new-api/service/oauthserver"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOAuthServerControllerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
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
		Id:          7,
		Username:    "ada",
		DisplayName: "Ada Lovelace",
		Email:       "ada@example.com",
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)

	privateKeyPEM := generateOAuthServerControllerTestKey(t)
	svc, err := oauthserversvc.New(db, oauthserversvc.Config{
		Issuer:               "https://issuer.example.test",
		SigningPrivateKeyPEM: privateKeyPEM,
		SigningKeyID:         "test-kid",
		AuthorizationCodeTTL: time.Minute,
		AccessTokenTTL:       time.Hour,
		IDTokenTTL:           time.Hour,
		RefreshTokenTTL:      time.Hour,
	})
	require.NoError(t, err)
	require.NoError(t, svc.EnsureDefaultCodexClient(context.Background()))

	restore := SetOAuthServerServiceForTesting(svc)
	t.Cleanup(restore)

	router := gin.New()
	store := cookie.NewStore([]byte("oauth-server-controller-test-secret"))
	store.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600,
	})
	router.Use(sessions.Sessions("session", store))
	RegisterOAuthServerRoutes(router)
	return router, db
}

func TestOAuthAuthorizeMissingSessionRedirectsToThemeLogin(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	previousTheme := common.GetTheme()
	common.SetTheme("classic")
	t.Cleanup(func() { common.SetTheme(previousTheme) })

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+validAuthorizeQuery().Encode(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	require.True(t, strings.HasPrefix(location, "/login?redirect="), location)
}

func TestOAuthAuthorizeLoggedInGETRendersConsentHTML(t *testing.T) {
	// Given a signed-in user authorizing the built-in Ruijie Codex client.
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	// When the authorization page is requested.
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+validAuthorizeQuery().Encode(), nil)
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Then the first-party page is concise, branded, and keeps callback transparency.
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "<form method=\"post\" action=\"/oauth/authorize\">")
	require.Contains(t, body, "锐捷TokenHub")
	require.Contains(t, body, "锐捷Codex 认证")
	require.Contains(t, body, "授权后，锐捷Codex 将使用你的锐捷TokenHub 账户。")
	require.Contains(t, body, "<summary>查看认证回调地址</summary>")
	require.Contains(t, body, "http://localhost:1455/auth/callback")
	require.Contains(t, body, "name=\"decision\" value=\"approve\">授权</button>")
	require.NotContains(t, body, "Permissions requested")
	require.NotContains(t, body, "name=\"decision\" value=\"deny\"")
	require.NotContains(t, body, "�")
	require.Equal(t, 1, strings.Count(body, "<button"))
}

func TestOAuthAuthorizeThirdPartyGETPreservesConsentDetails(t *testing.T) {
	// Given a signed-in user authorizing a third-party OAuth client.
	router, db := setupOAuthServerControllerTest(t)
	client := model.OAuthServerClient{
		ClientId:      "third-party-client",
		ClientName:    "第三方工具",
		Public:        true,
		RedirectURIs:  model.OAuthServerStringList{"https://third-party.example/callback"},
		AllowedScopes: model.OAuthServerStringList{"openid", "profile"},
		Enabled:       true,
	}
	require.NoError(t, db.Create(&client).Error)
	cookieHeader := oauthServerLoginCookie(t, router, 7)
	query := validAuthorizeQuery()
	query.Set("client_id", client.ClientId)
	query.Set("redirect_uri", client.RedirectURIs[0])
	query.Set("scope", "openid profile")

	// When the authorization page is requested.
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil)
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Then permission disclosure and both consent decisions remain available.
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "授权 第三方工具")
	require.Contains(t, body, "申请以下权限")
	require.Contains(t, body, "openid")
	require.Contains(t, body, "profile")
	require.Contains(t, body, "https://third-party.example/callback")
	require.Contains(t, body, "name=\"decision\" value=\"deny\">拒绝</button>")
	require.Contains(t, body, "name=\"decision\" value=\"approve\">授权</button>")
	require.Equal(t, 2, strings.Count(body, "<button"))
}

func TestOAuthAuthorizeMetaRequiresLogin(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize/meta?"+validAuthorizeQuery().Encode(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOAuthAuthorizeMetaReturnsClientInfo(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/authorize/meta?"+validAuthorizeQuery().Encode(), nil)
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload["success"].(bool))
	require.Equal(t, oauthserversvc.DefaultCodexClientID, payload["client_id"])
	require.NotEmpty(t, payload["client_name"])
	require.Equal(t, "http://localhost:1455/auth/callback", payload["redirect_uri"])
	require.Equal(t, "state-1", payload["state"])
	scopes, ok := payload["scopes"].([]any)
	require.True(t, ok)
	require.Contains(t, scopes, "openid")
	require.Contains(t, scopes, "api.connectors.invoke")
}

func TestOAuthAuthorizeApproveRedirectsWithCodeAndState(t *testing.T) {
	router, db := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	form := validAuthorizeQuery()
	form.Set("decision", "approve")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:1455/auth/callback", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	require.Equal(t, "state-1", parsed.Query().Get("state"))
	require.NotEmpty(t, parsed.Query().Get("code"))
	codexToken := parsed.Query().Get("codex-token")
	require.True(t, strings.HasPrefix(codexToken, "sk-"))

	var codes []model.OAuthServerAuthorizationCode
	require.NoError(t, db.Find(&codes).Error)
	require.Len(t, codes, 1)
	require.Equal(t, 7, codes[0].UserId)

	var token model.Token
	require.NoError(t, db.Where("user_id = ? AND name = ?", 7, "codex-token").First(&token).Error)
	require.Equal(t, strings.TrimPrefix(codexToken, "sk-"), token.Key)
	require.Equal(t, common.TokenStatusEnabled, token.Status)
	require.True(t, token.UnlimitedQuota)
	require.Equal(t, int64(-1), token.ExpiredTime)
}

func TestOAuthAuthorizeApproveReusesExistingCodexToken(t *testing.T) {
	router, db := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)
	reused := model.Token{
		UserId:         7,
		Name:           "codex-token",
		Key:            "existingcodextokenkey",
		Status:         common.TokenStatusDisabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    123,
		UnlimitedQuota: false,
	}
	require.NoError(t, db.Create(&reused).Error)

	form := validAuthorizeQuery()
	form.Set("decision", "approve")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	parsed, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "sk-existingcodextokenkey", parsed.Query().Get("codex-token"))

	var tokens []model.Token
	require.NoError(t, db.Where("user_id = ? AND name = ?", 7, "codex-token").Find(&tokens).Error)
	require.Len(t, tokens, 1)
	require.Equal(t, common.TokenStatusEnabled, tokens[0].Status)
	require.Equal(t, int64(-1), tokens[0].ExpiredTime)
	require.True(t, tokens[0].UnlimitedQuota)
}

func TestOAuthAuthorizeDenyRedirectsAccessDeniedWithState(t *testing.T) {
	router, db := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	form := validAuthorizeQuery()
	form.Set("decision", "deny")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	parsed, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "access_denied", parsed.Query().Get("error"))
	require.Equal(t, "state-1", parsed.Query().Get("state"))

	var count int64
	require.NoError(t, db.Model(&model.OAuthServerAuthorizationCode{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestOAuthAuthorizeInvalidClientReturnsOAuthError(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	form := validAuthorizeQuery()
	form.Set("client_id", "missing-client")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "invalid_client", payload["error"])
}

func TestOAuthAuthorizeInvalidRedirectReturnsOAuthError(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	form := validAuthorizeQuery()
	form.Set("redirect_uri", "http://evil.example/callback")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "invalid_request", payload["error"])
}

func TestOAuthAuthorizeInvalidScopeRedirectsOAuthErrorWithState(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)

	form := validAuthorizeQuery()
	form.Set("scope", "openid unknown.scope")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	parsed, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "invalid_scope", parsed.Query().Get("error"))
	require.Equal(t, "state-1", parsed.Query().Get("state"))
}

func TestOAuthTokenAndUserInfoFlow(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)
	verifier, challenge := oauthServerControllerPKCEPair(t, "token verifier")

	form := validAuthorizeQuery()
	form.Set("code_challenge", challenge)
	form.Set("decision", "approve")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	callback, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)

	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("client_id", oauthserversvc.DefaultCodexClientID)
	tokenForm.Set("code", callback.Query().Get("code"))
	tokenForm.Set("redirect_uri", "http://localhost:1455/auth/callback")
	tokenForm.Set("code_verifier", verifier)
	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, tokenReq)

	require.Equal(t, http.StatusOK, tokenRec.Code)
	var tokenPayload map[string]any
	require.NoError(t, common.Unmarshal(tokenRec.Body.Bytes(), &tokenPayload))
	accessToken, _ := tokenPayload["access_token"].(string)
	require.NotEmpty(t, accessToken)
	require.Equal(t, "Bearer", tokenPayload["token_type"])

	userReq := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	userReq.Header.Set("Authorization", "Bearer "+accessToken)
	userRec := httptest.NewRecorder()
	router.ServeHTTP(userRec, userReq)

	require.Equal(t, http.StatusOK, userRec.Code)
	var userPayload map[string]any
	require.NoError(t, common.Unmarshal(userRec.Body.Bytes(), &userPayload))
	require.Equal(t, "7", userPayload["sub"])
	require.Equal(t, "ada@example.com", userPayload["email"])
	require.Equal(t, "Ada Lovelace", userPayload["name"])
	codexClaims, ok := userPayload[oauthserversvc.CodexClaimNamespace].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user-7", codexClaims["chatgpt_account_id"])
	require.Equal(t, "pro", codexClaims["chatgpt_plan_type"])
}

func TestOAuthTokenAcceptsJSONRefreshRequest(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	cookieHeader := oauthServerLoginCookie(t, router, 7)
	verifier, challenge := oauthServerControllerPKCEPair(t, "json refresh verifier")

	form := validAuthorizeQuery()
	form.Set("code_challenge", challenge)
	form.Set("decision", "approve")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	callback, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)

	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("client_id", oauthserversvc.DefaultCodexClientID)
	tokenForm.Set("code", callback.Query().Get("code"))
	tokenForm.Set("redirect_uri", "http://localhost:1455/auth/callback")
	tokenForm.Set("code_verifier", verifier)
	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, tokenReq)
	require.Equal(t, http.StatusOK, tokenRec.Code)

	var tokenPayload map[string]any
	require.NoError(t, common.Unmarshal(tokenRec.Body.Bytes(), &tokenPayload))
	refreshToken, _ := tokenPayload["refresh_token"].(string)
	require.NotEmpty(t, refreshToken)

	refreshBody, err := common.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     oauthserversvc.DefaultCodexClientID,
		"refresh_token": refreshToken,
	})
	require.NoError(t, err)
	refreshReq := httptest.NewRequest(http.MethodPost, "/oauth/token", bytes.NewReader(refreshBody))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshRec := httptest.NewRecorder()
	router.ServeHTTP(refreshRec, refreshReq)

	require.Equal(t, http.StatusOK, refreshRec.Code)
	var refreshPayload map[string]any
	require.NoError(t, common.Unmarshal(refreshRec.Body.Bytes(), &refreshPayload))
	require.NotEmpty(t, refreshPayload["access_token"])
	require.NotEmpty(t, refreshPayload["refresh_token"])
	require.Equal(t, "Bearer", refreshPayload["token_type"])
}

func TestOAuthDiscoveryAndJWKS(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)

	discoveryReq := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	discoveryRec := httptest.NewRecorder()
	router.ServeHTTP(discoveryRec, discoveryReq)
	require.Equal(t, http.StatusOK, discoveryRec.Code)
	var discovery map[string]any
	require.NoError(t, common.Unmarshal(discoveryRec.Body.Bytes(), &discovery))
	require.Equal(t, "https://issuer.example.test", discovery["issuer"])
	require.Equal(t, "https://issuer.example.test/oauth/token", discovery["token_endpoint"])

	jwksReq := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	jwksRec := httptest.NewRecorder()
	router.ServeHTTP(jwksRec, jwksReq)
	require.Equal(t, http.StatusOK, jwksRec.Code)
	var jwks map[string]any
	require.NoError(t, common.Unmarshal(jwksRec.Body.Bytes(), &jwks))
	keys, ok := jwks["keys"].([]any)
	require.True(t, ok)
	require.Len(t, keys, 1)
}

func validAuthorizeQuery() url.Values {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", oauthserversvc.DefaultCodexClientID)
	q.Set("redirect_uri", "http://localhost:1455/auth/callback")
	q.Set("scope", "openid profile email offline_access api.connectors.invoke")
	q.Set("state", "state-1")
	q.Set("code_challenge", "placeholder-challenge")
	q.Set("code_challenge_method", "S256")
	q.Set("nonce", "nonce-1")
	return q
}

func oauthServerLoginCookie(t *testing.T, router *gin.Engine, userID int) string {
	t.Helper()

	router.GET("/test-login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", userID)
		session.Set("username", "ada")
		session.Set("role", common.RoleCommonUser)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-login", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	return rec.Header().Get("Set-Cookie")
}

func generateOAuthServerControllerTestKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	data := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: data}))
}

func oauthServerControllerPKCEPair(t *testing.T, verifier string) (string, string) {
	t.Helper()

	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func captureOAuthServerControllerLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultWriter
	oldErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = &logs
	gin.DefaultErrorWriter = &logs
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = oldWriter
		gin.DefaultErrorWriter = oldErrorWriter
		common.LogWriterMu.Unlock()
	})
	return &logs
}

func oauthServerIssueAccessToken(t *testing.T, router *gin.Engine, userID int) string {
	t.Helper()

	cookieHeader := oauthServerLoginCookie(t, router, userID)
	verifier, challenge := oauthServerControllerPKCEPair(t, "account verifier")

	form := validAuthorizeQuery()
	form.Set("code_challenge", challenge)
	form.Set("decision", "approve")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookieHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	callback, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	code := callback.Query().Get("code")
	require.NotEmpty(t, code)

	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("client_id", oauthserversvc.DefaultCodexClientID)
	tokenForm.Set("code", code)
	tokenForm.Set("redirect_uri", "http://localhost:1455/auth/callback")
	tokenForm.Set("code_verifier", verifier)
	tokenReq := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, tokenReq)
	require.Equal(t, http.StatusOK, tokenRec.Code)
	var tokenPayload map[string]any
	require.NoError(t, common.Unmarshal(tokenRec.Body.Bytes(), &tokenPayload))
	accessToken, _ := tokenPayload["access_token"].(string)
	require.NotEmpty(t, accessToken)
	return accessToken
}

func TestOAuthAccountReturnsCodexChatgptEnvelopeWhenEmailPresent(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	accessToken := oauthServerIssueAccessToken(t, router, 7)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, true, payload["requiresOpenaiAuth"])
	account, ok := payload["account"].(map[string]any)
	require.True(t, ok, "account must be an object")
	require.Equal(t, "chatgpt", account["type"])
	require.Equal(t, "ada@example.com", account["email"])
	require.Equal(t, "pro", account["planType"], "planType is required for chatgpt auth")
}

func TestOAuthAccountAlwaysReturnsChatgptWithFallbackEmail(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	// Replace the seeded user with one that has no email; the response
	// must still advertise a chatgpt account with a non-empty email and
	// the default pro planType so Codex can complete TUI bootstrap.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 7).
		Update("email", "").Error)
	accessToken := oauthServerIssueAccessToken(t, router, 7)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &payload))
	account, ok := payload["account"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "chatgpt", account["type"], "must always be chatgpt for OAuth tokens")
	require.NotEmpty(t, account["email"], "fallback email must be non-empty")
	require.Equal(t, "pro", account["planType"])
}

func TestOAuthAccountLogsRequestAndResponseWithoutBearerSecret(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	accessToken := oauthServerIssueAccessToken(t, router, 7)
	logs := captureOAuthServerControllerLogs(t)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	logText := logs.String()
	require.Contains(t, logText, "Codex account/read request")
	require.Contains(t, logText, "auth_present=true")
	require.Contains(t, logText, "auth_scheme=bearer")
	require.Contains(t, logText, "token_sha256_prefix=")
	require.Contains(t, logText, "Codex account/read response status=200")
	require.Contains(t, logText, "email=\"ada@example.com\"")
	require.Contains(t, logText, "plan_type=\"pro\"")
	require.NotContains(t, logText, accessToken)
}

func TestOAuthAccountLogsMissingBearerFailure(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	logs := captureOAuthServerControllerLogs(t)

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	logText := logs.String()
	require.Contains(t, logText, "Codex account/read request")
	require.Contains(t, logText, "auth_present=false")
	require.Contains(t, logText, "Codex account/read response status=401")
	require.Contains(t, logText, "error=\"missing_bearer_token\"")
}

func TestOAuthAccountLogsInvalidBearerFailureWithoutBearerSecret(t *testing.T) {
	router, _ := setupOAuthServerControllerTest(t)
	logs := captureOAuthServerControllerLogs(t)
	invalidToken := "invalid-secret-token"

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.Header.Set("Authorization", "Bearer "+invalidToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	logText := logs.String()
	require.Contains(t, logText, "Codex account/read request")
	require.Contains(t, logText, "auth_present=true")
	require.Contains(t, logText, "auth_scheme=bearer")
	require.Contains(t, logText, "token_sha256_prefix=")
	require.Contains(t, logText, "Codex account/read response status=401")
	require.Contains(t, logText, "error=\"invalid_bearer_token\"")
	require.NotContains(t, logText, invalidToken)
}
