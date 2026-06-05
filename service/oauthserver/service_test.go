package oauthserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOAuthServerServiceTest(t *testing.T) (*gorm.DB, *Service, *rsa.PrivateKey, time.Time) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.OAuthServerClient{},
		&model.OAuthServerAuthorizationCode{},
		&model.OAuthServerAccessToken{},
		&model.OAuthServerRefreshToken{},
		&model.OAuthServerUserGrant{},
	))

	user := model.User{
		Id:          7,
		Username:    "ada",
		DisplayName: "Ada Lovelace",
		Email:       "ada@example.com",
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)
	client := DefaultCodexClient()
	require.NoError(t, db.Create(&client).Error)

	key, privateKeyPEM := generateTestRSAKey(t)
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	svc, err := New(db, Config{
		Issuer:               "https://new-api.example.test/",
		SigningPrivateKeyPEM: privateKeyPEM,
		SigningKeyID:         "test-kid",
		AuthorizationCodeTTL: 10 * time.Minute,
		AccessTokenTTL:       time.Hour,
		IDTokenTTL:           time.Hour,
		RefreshTokenTTL:      30 * 24 * time.Hour,
		Now:                  func() time.Time { return now },
	})
	require.NoError(t, err)
	return db, svc, key, now
}

func TestAuthorizationCodeExchangeIssuesJWTAndRefreshToken(t *testing.T) {
	_, svc, key, now := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "correct verifier")

	code, err := svc.CreateAuthorizationCode(context.Background(), AuthorizationRequest{
		UserID:              7,
		ClientID:            DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               "openid profile email offline_access api.connectors.invoke",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		Nonce:               "nonce-1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, code.Code)
	require.Equal(t, now.Add(10*time.Minute), code.ExpiresAt)

	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code.Code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)
	require.Equal(t, TokenTypeBearer, tokens.TokenType)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.IDToken)
	require.NotEmpty(t, tokens.RefreshToken)
	require.Equal(t, "openid profile email offline_access api.connectors.invoke", tokens.Scope)

	claims := parseAndVerifyAccessToken(t, key, tokens.AccessToken)
	require.Equal(t, "https://new-api.example.test", claims["iss"])
	require.Equal(t, "7", claims["sub"])
	require.Equal(t, "openid profile email offline_access api.connectors.invoke", claims["scope"])
	require.Equal(t, "ada@example.com", claims["email"])
	require.Equal(t, "Ada Lovelace", claims["name"])
	codexClaims, ok := claims[CodexClaimNamespace].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user-7", codexClaims["chatgpt_account_id"])
	require.Equal(t, "pro", codexClaims["chatgpt_plan_type"])

	idClaims := parseAndVerifyAccessToken(t, key, tokens.IDToken)
	require.Equal(t, "ada@example.com", idClaims["email"])
	idCodexClaims, ok := idClaims[CodexClaimNamespace].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user-7", idCodexClaims["chatgpt_account_id"])
	require.Equal(t, "pro", idCodexClaims["chatgpt_plan_type"])

	jwks := svc.JWKS()
	require.Len(t, jwks.Keys, 1)
	require.Equal(t, "test-kid", jwks.Keys[0].KeyID)
	require.Equal(t, "RS256", jwks.Keys[0].Algorithm)
	header := parseJWTHeader(t, tokens.AccessToken)
	require.Equal(t, jwks.Keys[0].KeyID, header["kid"])

	introspection, err := svc.Introspect(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	require.True(t, introspection.Active)
	require.Equal(t, TokenHintAccess, introspection.TokenType)
	require.Equal(t, "7", introspection.Subject)
	require.Equal(t, DefaultCodexClientID, introspection.Audience)
}

func TestAuthorizationCodeExchangeUsesFallbackEmailWhenUserEmailMissing(t *testing.T) {
	db, svc, key, _ := setupOAuthServerServiceTest(t)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 7).Update("email", "").Error)
	verifier, challenge := testPKCEPair(t, "missing email verifier")

	code, err := svc.CreateAuthorizationCode(context.Background(), AuthorizationRequest{
		UserID:              7,
		ClientID:            DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               "openid profile email offline_access api.connectors.invoke",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code.Code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	claims := parseAndVerifyAccessToken(t, key, tokens.AccessToken)
	require.Equal(t, "ada@localhost", claims["email"])
	idClaims := parseAndVerifyAccessToken(t, key, tokens.IDToken)
	require.Equal(t, "ada@localhost", idClaims["email"])

	info, err := svc.UserInfo(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "ada@localhost", info.Email)
}

func TestJWTAndUserInfoRespectProfileScopes(t *testing.T) {
	_, svc, key, _ := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "scoped verifier")

	code, err := svc.CreateAuthorizationCode(context.Background(), AuthorizationRequest{
		UserID:              7,
		ClientID:            DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               "openid api.connectors.invoke",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code.Code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	claims := parseAndVerifyAccessToken(t, key, tokens.AccessToken)
	require.NotContains(t, claims, "email")
	require.NotContains(t, claims, "name")
	require.NotContains(t, claims, "preferred_username")
	codexClaims, ok := claims[CodexClaimNamespace].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "user-7", codexClaims["chatgpt_account_id"])

	info, err := svc.UserInfo(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "7", info.Subject)
	require.Empty(t, info.Email)
	require.Empty(t, info.Name)
	require.Empty(t, info.PreferredUsername)
	require.Equal(t, "user-7", info.CodexAccountID)
	require.Equal(t, CodexDefaultPlanType, info.CodexPlanType)
}

func TestUserInfoRejectsTokenWithoutOpenIDScope(t *testing.T) {
	_, svc, _, _ := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "no openid verifier")

	code, err := svc.CreateAuthorizationCode(context.Background(), AuthorizationRequest{
		UserID:              7,
		ClientID:            DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               ScopeConnectorsInvoke,
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code.Code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	_, err = svc.UserInfo(context.Background(), tokens.AccessToken)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestAuthorizationCodeReuseIsRejected(t *testing.T) {
	_, svc, _, _ := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "single use verifier")
	code := createTestAuthorizationCode(t, svc, verifier, challenge)

	_, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	_, err = svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.ErrorIs(t, err, ErrCodeConsumed)
}

func TestExpiredAuthorizationCodeIsRejected(t *testing.T) {
	db, _, _, now := setupOAuthServerServiceTest(t)
	key, privateKeyPEM := generateTestRSAKey(t)
	_ = key
	expiredNow := now.Add(11 * time.Minute)
	svc, err := New(db, Config{
		Issuer:               "https://new-api.example.test",
		SigningPrivateKeyPEM: privateKeyPEM,
		SigningKeyID:         "test-kid",
		AuthorizationCodeTTL: 10 * time.Minute,
		AccessTokenTTL:       time.Hour,
		RefreshTokenTTL:      30 * 24 * time.Hour,
		Now:                  func() time.Time { return now },
	})
	require.NoError(t, err)
	verifier, challenge := testPKCEPair(t, "expired verifier")
	code := createTestAuthorizationCode(t, svc, verifier, challenge)

	expiredSvc, err := New(db, Config{
		Issuer:               "https://new-api.example.test",
		SigningPrivateKeyPEM: privateKeyPEM,
		SigningKeyID:         "test-kid",
		AuthorizationCodeTTL: 10 * time.Minute,
		AccessTokenTTL:       time.Hour,
		RefreshTokenTTL:      30 * 24 * time.Hour,
		Now:                  func() time.Time { return expiredNow },
	})
	require.NoError(t, err)
	_, err = expiredSvc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.ErrorIs(t, err, ErrCodeExpired)
}

func TestPKCEMismatchIsRejected(t *testing.T) {
	_, svc, _, _ := setupOAuthServerServiceTest(t)
	_, challenge := testPKCEPair(t, "expected verifier")
	code := createTestAuthorizationCode(t, svc, "expected verifier", challenge)

	_, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: "wrong verifier",
	})
	require.ErrorIs(t, err, ErrPKCEMismatch)
}

func TestRefreshTokenRotationRevokesOldRefreshToken(t *testing.T) {
	_, svc, _, _ := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "refresh verifier")
	code := createTestAuthorizationCode(t, svc, verifier, challenge)
	initial, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	rotated, err := svc.Refresh(context.Background(), RefreshTokenRequest{
		ClientID:     DefaultCodexClientID,
		RefreshToken: initial.RefreshToken,
	})
	require.NoError(t, err)
	require.NotEmpty(t, rotated.AccessToken)
	require.NotEmpty(t, rotated.RefreshToken)
	require.NotEqual(t, initial.RefreshToken, rotated.RefreshToken)

	_, err = svc.Refresh(context.Background(), RefreshTokenRequest{
		ClientID:     DefaultCodexClientID,
		RefreshToken: initial.RefreshToken,
	})
	require.ErrorIs(t, err, ErrInvalidRefreshToken)

	oldIntrospection, err := svc.Introspect(context.Background(), initial.RefreshToken)
	require.NoError(t, err)
	require.False(t, oldIntrospection.Active)
	newIntrospection, err := svc.Introspect(context.Background(), rotated.RefreshToken)
	require.NoError(t, err)
	require.True(t, newIntrospection.Active)
}

func TestRevokedTokenIntrospectsInactive(t *testing.T) {
	_, svc, _, _ := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "revoke verifier")
	code := createTestAuthorizationCode(t, svc, verifier, challenge)
	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	active, err := svc.Introspect(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	require.True(t, active.Active)

	require.NoError(t, svc.Revoke(context.Background(), tokens.AccessToken, TokenHintAccess))
	inactive, err := svc.Introspect(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	require.False(t, inactive.Active)
}

func TestRevokeUserGrantRevokesGrantAndClientTokens(t *testing.T) {
	db, svc, _, _ := setupOAuthServerServiceTest(t)
	verifier, challenge := testPKCEPair(t, "grant revoke verifier")
	code := createTestAuthorizationCode(t, svc, verifier, challenge)
	tokens, err := svc.ExchangeAuthorizationCode(context.Background(), AuthorizationCodeTokenRequest{
		ClientID:     DefaultCodexClientID,
		Code:         code,
		RedirectURI:  "http://localhost:1455/auth/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)

	grants, err := svc.ListUserGrants(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, DefaultCodexClientID, grants[0].ClientID)
	require.Equal(t, DefaultCodexClientName, grants[0].ClientName)
	require.Contains(t, grants[0].Scopes, "offline_access")

	require.NoError(t, svc.RevokeUserGrant(context.Background(), 7, DefaultCodexClientID))

	grants, err = svc.ListUserGrants(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, grants)

	access, err := svc.Introspect(context.Background(), tokens.AccessToken)
	require.NoError(t, err)
	require.False(t, access.Active)
	refresh, err := svc.Introspect(context.Background(), tokens.RefreshToken)
	require.NoError(t, err)
	require.False(t, refresh.Active)

	var grant model.OAuthServerUserGrant
	require.NoError(t, db.Where("user_id = ? AND client_id = ?", 7, DefaultCodexClientID).First(&grant).Error)
	require.NotNil(t, grant.RevokedAt)
}

func TestServiceRequiresConfiguredSigningKey(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	_, err = New(db, Config{Issuer: "https://new-api.example.test"})
	require.ErrorIs(t, err, ErrSigningKeyRequired)
}

func createTestAuthorizationCode(t *testing.T, svc *Service, verifier string, challenge string) string {
	t.Helper()
	if challenge == "" {
		_, challenge = testPKCEPair(t, verifier)
	}
	code, err := svc.CreateAuthorizationCode(context.Background(), AuthorizationRequest{
		UserID:              7,
		ClientID:            DefaultCodexClientID,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scope:               "openid profile email offline_access",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	return code.Code
}

func testPKCEPair(t *testing.T, verifier string) (string, string) {
	t.Helper()
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return key, string(pem.EncodeToMemory(block))
}

func parseAndVerifyAccessToken(t *testing.T, key *rsa.PrivateKey, tokenText string) map[string]any {
	t.Helper()
	parsed, err := jwt.Parse(tokenText, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, errors.New("unexpected signing method")
		}
		return &key.PublicKey, nil
	}, jwt.WithAudience(DefaultCodexClientID), jwt.WithIssuer("https://new-api.example.test"))
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	return map[string]any(claims)
}

func parseJWTHeader(t *testing.T, tokenText string) map[string]any {
	t.Helper()
	parts := strings.Split(tokenText, ".")
	require.Len(t, parts, 3)
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var header map[string]any
	require.NoError(t, common.Unmarshal(data, &header))
	return header
}
