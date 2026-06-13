package controller

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	oauthserversvc "github.com/QuantumNous/new-api/service/oauthserver"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	oauthServerMu        sync.Mutex
	oauthServerInstance  *oauthserversvc.Service
	oauthServerInitError error
	oauthServerTestSvc   *oauthserversvc.Service
)

type oauthAuthorizationTemplateData struct {
	ClientName  string
	ClientID    string
	Scopes      []string
	RedirectURI string
	State       string
	Nonce       string
	Challenge   string
	Method      string
}

type oauthErrorPayload struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	State            string `json:"state,omitempty"`
}

type oauthTokenRequestPayload struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	RefreshToken string `json:"refresh_token"`
}

type oauthServerUserGrantPayload struct {
	ID         int        `json:"id"`
	ClientID   string     `json:"client_id"`
	ClientName string     `json:"client_name"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type oauthServerClientStatusPayload struct {
	ClientID      string   `json:"client_id"`
	ClientName    string   `json:"client_name"`
	Public        bool     `json:"public"`
	RedirectURIs  []string `json:"redirect_uris"`
	AllowedScopes []string `json:"allowed_scopes"`
	Enabled       bool     `json:"enabled"`
}

type oauthServerAdminStatusPayload struct {
	Enabled              bool                           `json:"enabled"`
	Issuer               string                         `json:"issuer"`
	SigningKeyConfigured bool                           `json:"signing_key_configured"`
	SigningKeyID         string                         `json:"signing_key_id,omitempty"`
	Error                string                         `json:"error,omitempty"`
	CodexClient          oauthServerClientStatusPayload `json:"codex_client"`
}

var oauthAuthorizeTemplate = template.Must(template.New("oauth_authorize").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Authorize {{.ClientName}}</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; color: #111827; background: #f8fafc; }
    main { max-width: 560px; margin: 8vh auto; padding: 32px; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; }
    h1 { font-size: 22px; line-height: 1.25; margin: 0 0 16px; }
    p { color: #4b5563; }
    ul { padding-left: 20px; }
    .actions { display: flex; gap: 12px; margin-top: 24px; }
    button { border: 0; border-radius: 6px; padding: 10px 16px; cursor: pointer; font-weight: 600; }
    .approve { background: #111827; color: #fff; }
    .deny { background: #e5e7eb; color: #111827; }
  </style>
</head>
<body>
<main>
  <h1>Authorize {{.ClientName}}</h1>
  <p>{{.ClientName}} is requesting access to your account.</p>
  <ul>{{range .Scopes}}<li>{{.}}</li>{{end}}</ul>
  <form method="post" action="/oauth/authorize">
    <input type="hidden" name="response_type" value="code">
    <input type="hidden" name="client_id" value="{{.ClientID}}">
    <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
    <input type="hidden" name="scope" value="{{range $i, $s := .Scopes}}{{if $i}} {{end}}{{$s}}{{end}}">
    <input type="hidden" name="state" value="{{.State}}">
    <input type="hidden" name="nonce" value="{{.Nonce}}">
    <input type="hidden" name="code_challenge" value="{{.Challenge}}">
    <input type="hidden" name="code_challenge_method" value="{{.Method}}">
    <div class="actions">
      <button class="approve" type="submit" name="decision" value="approve">Authorize</button>
      <button class="deny" type="submit" name="decision" value="deny">Deny</button>
    </div>
  </form>
</main>
</body>
</html>`))

func SetOAuthServerServiceForTesting(svc *oauthserversvc.Service) func() {
	oauthServerMu.Lock()
	previous := oauthServerTestSvc
	oauthServerTestSvc = svc
	oauthServerMu.Unlock()
	return func() {
		oauthServerMu.Lock()
		oauthServerTestSvc = previous
		oauthServerMu.Unlock()
	}
}

func RegisterOAuthServerRoutes(router gin.IRouter) {
	router.GET("/.well-known/oauth-authorization-server", OAuthServerMetadata)
	router.GET("/.well-known/openid-configuration", OAuthOpenIDConfiguration)
	router.GET("/.well-known/jwks.json", OAuthJWKS)
	router.GET("/oauth/authorize", OAuthAuthorize)
	router.POST("/oauth/authorize", OAuthAuthorize)
	router.GET("/api/oauth/authorize/meta", OAuthAuthorizeMeta)
	router.POST("/oauth/token", OAuthToken)
	router.POST("/oauth/revoke", OAuthRevoke)
	router.POST("/oauth/introspect", OAuthIntrospect)
	router.GET("/oauth/userinfo", OAuthUserInfo)
	router.GET("/account", OAuthAccount)
}

func OAuthServerMetadata(c *gin.Context) {
	discovery, err := oauthDiscovery(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}
	c.JSON(http.StatusOK, discovery)
}

func OAuthOpenIDConfiguration(c *gin.Context) {
	OAuthServerMetadata(c)
}

func OAuthJWKS(c *gin.Context) {
	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}
	c.JSON(http.StatusOK, svc.JWKS())
}

func OAuthAuthorize(c *gin.Context) {
	userID, ok := oauthSessionUserID(c)
	if !ok {
		redirectToLogin(c)
		return
	}

	req := oauthAuthorizationRequestFromRequest(c, userID)
	if c.Request.Method == http.MethodGet {
		renderOAuthAuthorize(c, req)
		return
	}

	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}
	if _, err := svc.ValidateAuthorizationRequest(c.Request.Context(), req); err != nil {
		handleAuthorizeServiceError(c, err, req.RedirectURI, c.PostForm("state"))
		return
	}
	if strings.TrimSpace(c.PostForm("decision")) != "approve" {
		redirectOAuthError(c, req.RedirectURI, "access_denied", "The user denied the authorization request.", c.PostForm("state"))
		return
	}
	codexToken, err := model.GetOrCreateNamedUserToken(userID, "codex-token")
	if err != nil {
		oauthJSONError(c, http.StatusInternalServerError, "server_error", "Failed to create Codex token.")
		return
	}
	result, err := svc.CreateAuthorizationCode(c.Request.Context(), req)
	if err != nil {
		handleAuthorizeServiceError(c, err, req.RedirectURI, c.PostForm("state"))
		return
	}
	values := url.Values{}
	values.Set("code", result.Code)
	values.Set("codex-token", "sk-"+codexToken.GetFullKey())
	if state := strings.TrimSpace(c.PostForm("state")); state != "" {
		values.Set("state", state)
	}
	redirectWithQuery(c, req.RedirectURI, values)
}

func OAuthToken(c *gin.Context) {
	req, err := parseOAuthTokenRequest(c)
	if err != nil {
		oauthJSONError(c, http.StatusBadRequest, "invalid_request", "Invalid form body.")
		return
	}
	clientID, clientSecret := oauthClientCredentialsForValues(c, req.ClientID, req.ClientSecret)
	grantType := strings.TrimSpace(req.GrantType)

	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}

	var token *oauthserversvc.TokenResult
	switch grantType {
	case "authorization_code":
		token, err = svc.ExchangeAuthorizationCode(c.Request.Context(), oauthserversvc.AuthorizationCodeTokenRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Code:         req.Code,
			RedirectURI:  req.RedirectURI,
			CodeVerifier: req.CodeVerifier,
		})
	case "refresh_token":
		token, err = svc.Refresh(c.Request.Context(), oauthserversvc.RefreshTokenRequest{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RefreshToken: req.RefreshToken,
		})
	default:
		oauthJSONError(c, http.StatusBadRequest, "unsupported_grant_type", "Unsupported grant_type.")
		return
	}
	if err != nil {
		handleTokenServiceError(c, err)
		return
	}
	payload := gin.H{
		"access_token": token.AccessToken,
		"token_type":   token.TokenType,
		"expires_in":   token.ExpiresIn,
		"scope":        token.Scope,
	}
	if token.IDToken != "" {
		payload["id_token"] = token.IDToken
	}
	if token.RefreshToken != "" {
		payload["refresh_token"] = token.RefreshToken
	}
	c.JSON(http.StatusOK, payload)
}

func OAuthRevoke(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		oauthJSONError(c, http.StatusBadRequest, "invalid_request", "Invalid form body.")
		return
	}
	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}
	clientID, clientSecret := oauthClientCredentials(c)
	if err := svc.ValidateClientCredentials(c.Request.Context(), clientID, clientSecret); err != nil {
		handleTokenServiceError(c, err)
		return
	}
	if err := svc.Revoke(c.Request.Context(), c.PostForm("token"), c.PostForm("token_type_hint")); err != nil {
		oauthJSONError(c, http.StatusInternalServerError, "server_error", "Failed to revoke token.")
		return
	}
	c.Status(http.StatusOK)
}

func OAuthIntrospect(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		oauthJSONError(c, http.StatusBadRequest, "invalid_request", "Invalid form body.")
		return
	}
	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}
	clientID, clientSecret := oauthClientCredentials(c)
	if err := svc.ValidateClientCredentials(c.Request.Context(), clientID, clientSecret); err != nil {
		handleTokenServiceError(c, err)
		return
	}
	result, err := svc.Introspect(c.Request.Context(), c.PostForm("token"))
	if err != nil {
		oauthJSONError(c, http.StatusInternalServerError, "server_error", "Failed to introspect token.")
		return
	}
	if result == nil || !result.Active {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"active":     true,
		"token_type": result.TokenType,
		"scope":      result.Scope,
		"client_id":  result.ClientID,
		"username":   result.Username,
		"sub":        result.Subject,
		"aud":        result.Audience,
		"iss":        result.Issuer,
		"exp":        result.ExpiresAt,
		"iat":        result.IssuedAt,
	})
}

func OAuthUserInfo(c *gin.Context) {
	info, ok := oauthUserInfoFromBearer(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sub":                info.Subject,
		"email":              info.Email,
		"name":               info.Name,
		"preferred_username": info.PreferredUsername,
		oauthserversvc.CodexClaimNamespace: gin.H{
			"chatgpt_account_id": info.CodexAccountID,
			"chatgpt_plan_type":  oauthCodexPlanType(info),
		},
	})
}

// OAuthAccount returns the Codex app-server protocol account response.
// Codex expects the shape:
//
//	{ "account": { "type": "chatgpt", "email": "...", "planType": "..." },
//	  "requiresOpenaiAuth": true }
func OAuthAccount(c *gin.Context) {
	token, ok := bearerToken(c)
	logOAuthAccountRequest(c, token)
	if !ok {
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		logOAuthAccountFailure(c, http.StatusUnauthorized, "missing_bearer_token", "")
		oauthJSONError(c, http.StatusUnauthorized, "invalid_token", "Missing bearer token.")
		return
	}

	svc, err := getOAuthServerService(c)
	if err != nil {
		logOAuthAccountFailure(c, http.StatusServiceUnavailable, "oauth_server_unavailable", err.Error())
		oauthServerUnavailable(c, err)
		return
	}
	info, err := svc.UserInfo(c.Request.Context(), token)
	if err != nil {
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		logOAuthAccountFailure(c, http.StatusUnauthorized, "invalid_bearer_token", err.Error())
		oauthJSONError(c, http.StatusUnauthorized, "invalid_token", "Invalid bearer token.")
		return
	}

	email := strings.TrimSpace(info.Email)
	if email == "" {
		if username := strings.TrimSpace(info.PreferredUsername); username != "" {
			email = username + "@localhost"
		} else {
			email = "user-" + info.Subject + "@localhost"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"account": gin.H{
			"type":     "chatgpt",
			"email":    email,
			"planType": oauthCodexPlanType(info),
		},
		"requiresOpenaiAuth": true,
	})
	logOAuthAccountSuccess(c, info, email)
}

func ListOAuthServerUserGrants(c *gin.Context) {
	svc, err := getOAuthServerService(c)
	if err != nil {
		common.ApiErrorMsg(c, "OAuth server is not configured.")
		return
	}
	grants, err := svc.ListUserGrants(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	payload := make([]oauthServerUserGrantPayload, 0, len(grants))
	for _, grant := range grants {
		payload = append(payload, oauthServerUserGrantPayload{
			ID:         grant.ID,
			ClientID:   grant.ClientID,
			ClientName: grant.ClientName,
			Scopes:     grant.Scopes,
			LastUsedAt: grant.LastUsedAt,
			CreatedAt:  grant.CreatedAt,
			UpdatedAt:  grant.UpdatedAt,
		})
	}
	common.ApiSuccess(c, payload)
}

func RevokeOAuthServerUserGrant(c *gin.Context) {
	svc, err := getOAuthServerService(c)
	if err != nil {
		common.ApiErrorMsg(c, "OAuth server is not configured.")
		return
	}
	if err := svc.RevokeUserGrant(c.Request.Context(), c.GetInt("id"), c.Param("client_id")); err != nil {
		if errors.Is(err, oauthserversvc.ErrGrantNotFound) {
			common.ApiErrorMsg(c, "OAuth grant not found.")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func OAuthServerAdminStatus(c *gin.Context) {
	keyPEM, keyErr := oauthSigningPrivateKeyPEM()
	payload := oauthServerAdminStatusPayload{
		SigningKeyConfigured: keyErr == nil && strings.TrimSpace(keyPEM) != "",
		CodexClient:          oauthServerClientStatus(oauthserversvc.DefaultCodexClient()),
	}

	svc, err := getOAuthServerService(c)
	if err != nil {
		payload.Enabled = false
		payload.Error = "OAuth server is not configured."
		common.ApiSuccess(c, payload)
		return
	}
	payload.Enabled = true
	payload.Issuer = svc.Issuer()
	payload.SigningKeyID = svc.SigningKeyID()

	var client model.OAuthServerClient
	if err := model.DB.Where("client_id = ?", oauthserversvc.DefaultCodexClientID).First(&client).Error; err == nil {
		payload.CodexClient = oauthServerClientStatus(client)
	}
	common.ApiSuccess(c, payload)
}

func oauthDiscovery(c *gin.Context) (gin.H, error) {
	svc, err := getOAuthServerService(c)
	if err != nil {
		return nil, err
	}
	issuer := svc.Issuer()
	return gin.H{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/oauth/token",
		"revocation_endpoint":                   issuer + "/oauth/revoke",
		"introspection_endpoint":                issuer + "/oauth/introspect",
		"userinfo_endpoint":                     issuer + "/oauth/userinfo",
		"jwks_uri":                              issuer + "/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{svc.JWKS().Keys[0].Algorithm},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access", "api.connectors.read", "api.connectors.invoke"},
	}, nil
}

func oauthServerClientStatus(client model.OAuthServerClient) oauthServerClientStatusPayload {
	return oauthServerClientStatusPayload{
		ClientID:      client.ClientId,
		ClientName:    client.ClientName,
		Public:        client.Public,
		RedirectURIs:  append([]string(nil), []string(client.RedirectURIs)...),
		AllowedScopes: append([]string(nil), []string(client.AllowedScopes)...),
		Enabled:       client.Enabled,
	}
}

func renderOAuthAuthorize(c *gin.Context, req oauthserversvc.AuthorizationRequest) {
	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}
	prompt, err := svc.ValidateAuthorizationRequest(c.Request.Context(), req)
	if err != nil {
		handleAuthorizeServiceError(c, err, req.RedirectURI, c.Query("state"))
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := oauthAuthorizeTemplate.Execute(c.Writer, oauthAuthorizationTemplateData{
		ClientName:  prompt.ClientName,
		ClientID:    prompt.ClientID,
		Scopes:      prompt.Scopes,
		RedirectURI: prompt.RedirectURI,
		State:       c.Query("state"),
		Nonce:       req.Nonce,
		Challenge:   req.CodeChallenge,
		Method:      req.CodeChallengeMethod,
	}); err != nil {
		c.Status(http.StatusInternalServerError)
	}
}

func getOAuthServerService(c *gin.Context) (*oauthserversvc.Service, error) {
	oauthServerMu.Lock()
	defer oauthServerMu.Unlock()

	if oauthServerTestSvc != nil {
		return oauthServerTestSvc, nil
	}
	if oauthServerInstance != nil || oauthServerInitError != nil {
		return oauthServerInstance, oauthServerInitError
	}
	keyPEM, err := oauthSigningPrivateKeyPEM()
	if err != nil {
		oauthServerInitError = err
		return nil, err
	}
	svc, err := oauthserversvc.New(model.DB, oauthserversvc.Config{
		Issuer:               oauthIssuer(c),
		SigningPrivateKeyPEM: keyPEM,
		SigningKeyID:         os.Getenv("OAUTH_SERVER_SIGNING_KEY_ID"),
	})
	if err != nil {
		oauthServerInitError = err
		return nil, err
	}
	if err := svc.EnsureDefaultCodexClient(c.Request.Context()); err != nil {
		oauthServerInitError = err
		return nil, err
	}
	oauthServerInstance = svc
	return oauthServerInstance, nil
}

func oauthSigningPrivateKeyPEM() (string, error) {
	if value := strings.TrimSpace(os.Getenv("OAUTH_SERVER_SIGNING_PRIVATE_KEY")); value != "" {
		return value, nil
	}
	if path := strings.TrimSpace(os.Getenv("OAUTH_SERVER_SIGNING_PRIVATE_KEY_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	// Auto-generate and persist in /data directory
	autoPath := "/data/oauth-server-signing-key.pem"
	if data, err := os.ReadFile(autoPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		return string(data), nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", fmt.Errorf("failed to generate OAuth signing key: %w", err)
	}
	pemBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: pemBytes}
	var buf bytes.Buffer
	if err := pem.Encode(&buf, pemBlock); err != nil {
		return "", err
	}
	keyPEM := buf.String()
	if err := os.WriteFile(autoPath, []byte(keyPEM), 0600); err != nil {
		common.SysLog("failed to persist OAuth signing key: " + err.Error())
	}
	return keyPEM, nil
}

func oauthIssuer(c *gin.Context) string {
	if issuer := strings.TrimRight(strings.TrimSpace(os.Getenv("OAUTH_SERVER_ISSUER")), "/"); issuer != "" {
		return issuer
	}
	if issuer := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/"); issuer != "" {
		return issuer
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func oauthAuthorizationRequestFromRequest(c *gin.Context, userID int) oauthserversvc.AuthorizationRequest {
	values := c.Request.URL.Query()
	if c.Request.Method == http.MethodPost {
		_ = c.Request.ParseForm()
		values = c.Request.PostForm
	}
	return oauthserversvc.AuthorizationRequest{
		UserID:              userID,
		ClientID:            values.Get("client_id"),
		RedirectURI:         values.Get("redirect_uri"),
		Scope:               values.Get("scope"),
		CodeChallenge:       values.Get("code_challenge"),
		CodeChallengeMethod: values.Get("code_challenge_method"),
		Nonce:               values.Get("nonce"),
	}
}

func parseOAuthTokenRequest(c *gin.Context) (oauthTokenRequestPayload, error) {
	if c.ContentType() == "application/json" {
		var req oauthTokenRequestPayload
		if err := common.DecodeJson(c.Request.Body, &req); err != nil {
			return oauthTokenRequestPayload{}, err
		}
		return req, nil
	}
	if err := c.Request.ParseForm(); err != nil {
		return oauthTokenRequestPayload{}, err
	}
	return oauthTokenRequestPayload{
		GrantType:    c.PostForm("grant_type"),
		ClientID:     c.PostForm("client_id"),
		ClientSecret: c.PostForm("client_secret"),
		Code:         c.PostForm("code"),
		RedirectURI:  c.PostForm("redirect_uri"),
		CodeVerifier: c.PostForm("code_verifier"),
		RefreshToken: c.PostForm("refresh_token"),
	}, nil
}

func oauthSessionUserID(c *gin.Context) (int, bool) {
	id := sessions.Default(c).Get("id")
	switch v := id.(type) {
	case int:
		return v, v > 0
	case int64:
		return int(v), v > 0
	case float64:
		return int(v), v > 0
	default:
		return 0, false
	}
}

func oauthClientCredentials(c *gin.Context) (string, string) {
	return oauthClientCredentialsForValues(c, c.PostForm("client_id"), c.PostForm("client_secret"))
}

func oauthClientCredentialsForValues(c *gin.Context, rawClientID string, rawClientSecret string) (string, string) {
	clientID := strings.TrimSpace(rawClientID)
	clientSecret := strings.TrimSpace(rawClientSecret)
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "basic ") {
		return clientID, clientSecret
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(auth[len("Basic "):]))
	if err != nil {
		return clientID, clientSecret
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return clientID, clientSecret
	}
	return parts[0], parts[1]
}

func oauthCodexPlanType(info *oauthserversvc.UserInfoResult) string {
	if info != nil {
		if planType := strings.TrimSpace(info.CodexPlanType); planType != "" {
			return planType
		}
	}
	return oauthserversvc.CodexDefaultPlanType
}

func oauthUserInfoFromBearer(c *gin.Context) (*oauthserversvc.UserInfoResult, bool) {
	token, ok := bearerToken(c)
	if !ok {
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		oauthJSONError(c, http.StatusUnauthorized, "invalid_token", "Missing bearer token.")
		return nil, false
	}
	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return nil, false
	}
	info, err := svc.UserInfo(c.Request.Context(), token)
	if err != nil {
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		oauthJSONError(c, http.StatusUnauthorized, "invalid_token", "Invalid bearer token.")
		return nil, false
	}
	return info, true
}

func bearerToken(c *gin.Context) (string, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(auth[len("Bearer "):])
	return token, token != ""
}

func logOAuthAccountRequest(c *gin.Context, token string) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	authScheme := "none"
	if fields := strings.Fields(auth); len(fields) > 0 {
		authScheme = strings.ToLower(fields[0])
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf(
		"Codex account/read request method=%s path=%q client_ip=%s user_agent=%q auth_present=%t auth_scheme=%s token_sha256_prefix=%s",
		c.Request.Method,
		c.Request.RequestURI,
		c.ClientIP(),
		c.Request.UserAgent(),
		token != "",
		authScheme,
		oauthAccountTokenFingerprint(token),
	))
}

func logOAuthAccountSuccess(c *gin.Context, info *oauthserversvc.UserInfoResult, email string) {
	logger.LogInfo(c.Request.Context(), fmt.Sprintf(
		"Codex account/read response status=%d subject=%q email=%q account_id=%q plan_type=%q requires_openai_auth=true",
		http.StatusOK,
		info.Subject,
		email,
		info.CodexAccountID,
		oauthCodexPlanType(info),
	))
}

func logOAuthAccountFailure(c *gin.Context, status int, code string, detail string) {
	if strings.TrimSpace(detail) != "" {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf(
			"Codex account/read response status=%d error=%q detail=%q",
			status,
			code,
			detail,
		))
		return
	}
	logger.LogWarn(c.Request.Context(), fmt.Sprintf(
		"Codex account/read response status=%d error=%q",
		status,
		code,
	))
}

func oauthAccountTokenFingerprint(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "none"
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:12]
}

func redirectToLogin(c *gin.Context) {
	values := url.Values{}
	values.Set("redirect", c.Request.URL.RequestURI())
	loginPath := "/sign-in"
	if common.GetTheme() == "classic" {
		loginPath = "/login"
	}
	c.Redirect(http.StatusFound, loginPath+"?"+values.Encode())
}

// OAuthAuthorizeMeta returns the consent-screen metadata for a logged-in user
// to render the SPA authorize page. It does NOT issue any tokens.
func OAuthAuthorizeMeta(c *gin.Context) {
	userID, ok := oauthSessionUserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	svc, err := getOAuthServerService(c)
	if err != nil {
		oauthServerUnavailable(c, err)
		return
	}

	req := oauthAuthorizationRequestFromRequest(c, userID)
	if _, err := svc.ValidateAuthorizationRequest(c.Request.Context(), req); err != nil {
		handleAuthorizeServiceError(c, err, req.RedirectURI, c.Request.URL.Query().Get("state"))
		return
	}

	scopes := splitScopes(req.Scope)
	var client model.OAuthServerClient
	clientName := ""
	if err := model.DB.WithContext(c.Request.Context()).
		Where("client_id = ? AND enabled = ?", req.ClientID, true).
		First(&client).Error; err == nil {
		clientName = client.ClientName
	}

	payload := gin.H{
		"success":               true,
		"client_id":             req.ClientID,
		"client_name":           clientName,
		"redirect_uri":          req.RedirectURI,
		"state":                 c.Request.URL.Query().Get("state"),
		"scopes":                scopes,
		"scope":                 req.Scope,
		"code_challenge":        req.CodeChallenge,
		"code_challenge_method": req.CodeChallengeMethod,
		"nonce":                 req.Nonce,
	}
	c.JSON(http.StatusOK, payload)
}

func splitScopes(scope string) []string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return []string{}
	}
	parts := strings.Fields(scope)
	return parts
}

func handleAuthorizeServiceError(c *gin.Context, err error, redirectURI string, state string) {
	switch {
	case errors.Is(err, oauthserversvc.ErrInvalidClient):
		oauthJSONError(c, http.StatusBadRequest, "invalid_client", "Invalid OAuth client.")
	case errors.Is(err, oauthserversvc.ErrInvalidRedirectURI):
		oauthJSONError(c, http.StatusBadRequest, "invalid_request", "Invalid redirect_uri.")
	case errors.Is(err, oauthserversvc.ErrInvalidScope):
		redirectOAuthError(c, redirectURI, "invalid_scope", "Invalid scope.", state)
	case errors.Is(err, oauthserversvc.ErrInvalidPKCE):
		redirectOAuthError(c, redirectURI, "invalid_request", "Invalid PKCE parameters.", state)
	case errors.Is(err, oauthserversvc.ErrInvalidUser):
		oauthJSONError(c, http.StatusUnauthorized, "access_denied", "Invalid user session.")
	default:
		oauthJSONError(c, http.StatusInternalServerError, "server_error", "OAuth server error.")
	}
}

func handleTokenServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, oauthserversvc.ErrInvalidClient):
		c.Header("WWW-Authenticate", `Basic realm="oauth"`)
		oauthJSONError(c, http.StatusUnauthorized, "invalid_client", "Invalid OAuth client.")
	case errors.Is(err, oauthserversvc.ErrAuthorizationCode),
		errors.Is(err, oauthserversvc.ErrCodeConsumed),
		errors.Is(err, oauthserversvc.ErrCodeExpired),
		errors.Is(err, oauthserversvc.ErrInvalidRefreshToken),
		errors.Is(err, oauthserversvc.ErrRefreshExpired),
		errors.Is(err, oauthserversvc.ErrInvalidPKCE),
		errors.Is(err, oauthserversvc.ErrPKCEMismatch):
		oauthJSONError(c, http.StatusBadRequest, "invalid_grant", "Invalid OAuth grant.")
	default:
		oauthJSONError(c, http.StatusInternalServerError, "server_error", "OAuth server error.")
	}
}

func oauthServerUnavailable(c *gin.Context, err error) {
	common.SysLog("OAuth server unavailable: " + err.Error())
	oauthJSONError(c, http.StatusServiceUnavailable, "server_error", "OAuth server is not configured.")
}

func redirectOAuthError(c *gin.Context, redirectURI string, code string, description string, state string) {
	values := url.Values{}
	values.Set("error", code)
	if description != "" {
		values.Set("error_description", description)
	}
	if strings.TrimSpace(state) != "" {
		values.Set("state", state)
	}
	redirectWithQuery(c, redirectURI, values)
}

func redirectWithQuery(c *gin.Context, rawURL string, values url.Values) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		oauthJSONError(c, http.StatusBadRequest, "invalid_request", "Invalid redirect_uri.")
		return
	}
	query := parsed.Query()
	for key, items := range values {
		for _, item := range items {
			query.Add(key, item)
		}
	}
	parsed.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, parsed.String())
}

func oauthJSONError(c *gin.Context, status int, code string, description string) {
	c.JSON(status, oauthErrorPayload{
		Error:            code,
		ErrorDescription: description,
	})
}
