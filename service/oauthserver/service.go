package oauthserver

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	DefaultCodexClientID   = "app_EMoamEEZ73f0CkXaXp7hrann"
	DefaultCodexClientName = "Codex CLI"
	CodexClaimNamespace    = "https://api.openai.com/auth"
	CodexDefaultPlanType   = "pro"
	ScopeConnectorsInvoke  = "api.connectors.invoke"

	TokenTypeBearer  = "Bearer"
	TokenHintAccess  = "access_token"
	TokenHintRefresh = "refresh_token"

	pkceMethodS256 = "S256"
)

var (
	ErrInvalidClient       = errors.New("invalid oauth client")
	ErrInvalidRedirectURI  = errors.New("invalid redirect_uri")
	ErrInvalidScope        = errors.New("invalid scope")
	ErrInvalidUser         = errors.New("invalid user")
	ErrGrantNotFound       = errors.New("oauth grant not found")
	ErrInvalidPKCE         = errors.New("invalid pkce")
	ErrPKCEMismatch        = errors.New("pkce verification failed")
	ErrInvalidToken        = errors.New("invalid token")
	ErrAuthorizationCode   = errors.New("invalid authorization code")
	ErrCodeConsumed        = errors.New("authorization code already consumed")
	ErrCodeExpired         = errors.New("authorization code expired")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshExpired      = errors.New("refresh token expired")
	ErrSigningKeyRequired  = errors.New("oauth signing private key is required")
)

type Config struct {
	Issuer               string
	SigningPrivateKeyPEM string
	SigningKeyID         string
	AuthorizationCodeTTL time.Duration
	AccessTokenTTL       time.Duration
	IDTokenTTL           time.Duration
	RefreshTokenTTL      time.Duration
	AllowPKCEPlain       bool
	Now                  func() time.Time
	RandomReader         io.Reader
}

type Service struct {
	db                   *gorm.DB
	issuer               string
	signingKey           *rsa.PrivateKey
	signingKeyID         string
	authorizationCodeTTL time.Duration
	accessTokenTTL       time.Duration
	idTokenTTL           time.Duration
	refreshTokenTTL      time.Duration
	allowPKCEPlain       bool
	now                  func() time.Time
	randomReader         io.Reader
}

type AuthorizationRequest struct {
	UserID              int
	ClientID            string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
}

type AuthorizationCodeResult struct {
	Code      string
	ExpiresAt time.Time
	Scopes    []string
}

type AuthorizationPrompt struct {
	ClientID    string
	ClientName  string
	RedirectURI string
	Scopes      []string
}

type AuthorizationCodeTokenRequest struct {
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type RefreshTokenRequest struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

type TokenResult struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
	Scope        string
	ExpiresAt    time.Time
}

type IntrospectionResult struct {
	Active    bool
	TokenType string
	Scope     string
	ClientID  string
	Username  string
	Subject   string
	Audience  string
	Issuer    string
	ExpiresAt int64
	IssuedAt  int64
}

type RelayAccessTokenAuth struct {
	AccessTokenID int
	UserID        int
	ClientID      string
	ClientName    string
	Scopes        []string
	ExpiresAt     time.Time
}

type UserInfoResult struct {
	Subject           string
	Email             string
	Name              string
	PreferredUsername string
	CodexAccountID    string
	CodexPlanType     string
}

type UserGrantResult struct {
	ID         int
	ClientID   string
	ClientName string
	Scopes     []string
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type JWKSet struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

type codexAuthClaims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	ChatGPTPlanType  string `json:"chatgpt_plan_type"`
}

type oauthJWTClaims struct {
	Scope             string           `json:"scope"`
	Email             string           `json:"email,omitempty"`
	Name              string           `json:"name,omitempty"`
	PreferredUsername string           `json:"preferred_username,omitempty"`
	Nonce             string           `json:"nonce,omitempty"`
	CodexAuth         *codexAuthClaims `json:"https://api.openai.com/auth,omitempty"`
	jwt.RegisteredClaims
}

func New(db *gorm.DB, cfg Config) (*Service, error) {
	if db == nil {
		return nil, errors.New("oauth database is required")
	}
	issuer := strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	if issuer == "" {
		return nil, errors.New("oauth issuer is required")
	}
	parsedIssuer, err := url.Parse(issuer)
	if err != nil || parsedIssuer.Scheme == "" || parsedIssuer.Host == "" {
		return nil, errors.New("oauth issuer must be an absolute URL")
	}
	key, err := parseRSAPrivateKey(cfg.SigningPrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	keyID := strings.TrimSpace(cfg.SigningKeyID)
	if keyID == "" {
		keyID, err = deriveRSAKeyID(&key.PublicKey)
		if err != nil {
			return nil, err
		}
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	randomReader := cfg.RandomReader
	if randomReader == nil {
		randomReader = rand.Reader
	}
	return &Service{
		db:                   db,
		issuer:               issuer,
		signingKey:           key,
		signingKeyID:         keyID,
		authorizationCodeTTL: durationOrDefault(cfg.AuthorizationCodeTTL, 10*time.Minute),
		accessTokenTTL:       durationOrDefault(cfg.AccessTokenTTL, time.Hour),
		idTokenTTL:           durationOrDefault(cfg.IDTokenTTL, time.Hour),
		refreshTokenTTL:      durationOrDefault(cfg.RefreshTokenTTL, 30*24*time.Hour),
		allowPKCEPlain:       cfg.AllowPKCEPlain,
		now:                  now,
		randomReader:         randomReader,
	}, nil
}

func DefaultCodexClient() model.OAuthServerClient {
	return model.OAuthServerClient{
		ClientId:      DefaultCodexClientID,
		ClientName:    DefaultCodexClientName,
		Public:        true,
		RedirectURIs:  model.OAuthServerStringList{"http://localhost:1455/auth/callback", "http://127.0.0.1:1455/auth/callback"},
		AllowedScopes: model.OAuthServerStringList{"openid", "profile", "email", "offline_access", "api.connectors.read", ScopeConnectorsInvoke},
		Enabled:       true,
	}
}

func (s *Service) EnsureDefaultCodexClient(ctx context.Context) error {
	client := DefaultCodexClient()
	err := s.db.WithContext(ctx).Where("client_id = ?", client.ClientId).First(&model.OAuthServerClient{}).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.WithContext(ctx).Create(&client).Error
}

func (s *Service) ValidateAuthorizationRequest(ctx context.Context, req AuthorizationRequest) (*AuthorizationPrompt, error) {
	client, scopes, err := s.validateAuthorizationRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return &AuthorizationPrompt{
		ClientID:    client.ClientId,
		ClientName:  client.ClientName,
		RedirectURI: strings.TrimSpace(req.RedirectURI),
		Scopes:      append([]string(nil), scopes...),
	}, nil
}

func (s *Service) ValidateClientCredentials(ctx context.Context, clientID string, clientSecret string) error {
	_, err := s.validateTokenClient(s.db.WithContext(ctx), clientID, clientSecret)
	return err
}

func (s *Service) CreateAuthorizationCode(ctx context.Context, req AuthorizationRequest) (*AuthorizationCodeResult, error) {
	now := s.now().UTC()
	client, scopes, err := s.validateAuthorizationRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	code, err := s.randomToken("oc")
	if err != nil {
		return nil, err
	}
	record := model.OAuthServerAuthorizationCode{
		CodeHash:            tokenHash(code),
		UserId:              req.UserID,
		ClientId:            client.ClientId,
		RedirectURI:         strings.TrimSpace(req.RedirectURI),
		Scopes:              model.OAuthServerStringList(scopes),
		CodeChallenge:       strings.TrimSpace(req.CodeChallenge),
		CodeChallengeMethod: strings.TrimSpace(req.CodeChallengeMethod),
		Nonce:               strings.TrimSpace(req.Nonce),
		ExpiresAt:           now.Add(s.authorizationCodeTTL),
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		return upsertUserGrant(tx, req.UserID, client.ClientId, scopes, now)
	})
	if err != nil {
		return nil, err
	}
	return &AuthorizationCodeResult{
		Code:      code,
		ExpiresAt: record.ExpiresAt,
		Scopes:    append([]string(nil), scopes...),
	}, nil
}

func (s *Service) ExchangeAuthorizationCode(ctx context.Context, req AuthorizationCodeTokenRequest) (*TokenResult, error) {
	now := s.now().UTC()
	var result *TokenResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		client, err := s.validateTokenClient(tx, req.ClientID, req.ClientSecret)
		if err != nil {
			return err
		}
		var codeRecord model.OAuthServerAuthorizationCode
		if err := tx.Where("code_hash = ?", tokenHash(req.Code)).First(&codeRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuthorizationCode
			}
			return err
		}
		if codeRecord.ConsumedAt != nil {
			return ErrCodeConsumed
		}
		if !now.Before(codeRecord.ExpiresAt) {
			return ErrCodeExpired
		}
		if codeRecord.ClientId != client.ClientId || codeRecord.RedirectURI != strings.TrimSpace(req.RedirectURI) {
			return ErrAuthorizationCode
		}
		if err := verifyPKCE(codeRecord.CodeChallenge, codeRecord.CodeChallengeMethod, req.CodeVerifier, s.allowPKCEPlain); err != nil {
			return err
		}
		user, err := getEnabledUser(tx, codeRecord.UserId)
		if err != nil {
			return err
		}
		update := tx.Model(&model.OAuthServerAuthorizationCode{}).
			Where("id = ? AND consumed_at IS NULL", codeRecord.Id).
			Update("consumed_at", now)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrCodeConsumed
		}
		tokenResult, err := s.issueTokenPair(tx, user, client.ClientId, []string(codeRecord.Scopes), codeRecord.Nonce, now)
		if err != nil {
			return err
		}
		result = tokenResult
		return nil
	})
	return result, err
}

func (s *Service) Refresh(ctx context.Context, req RefreshTokenRequest) (*TokenResult, error) {
	now := s.now().UTC()
	var result *TokenResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		client, err := s.validateTokenClient(tx, req.ClientID, req.ClientSecret)
		if err != nil {
			return err
		}
		var oldRefresh model.OAuthServerRefreshToken
		if err := tx.Where("token_hash = ?", tokenHash(req.RefreshToken)).First(&oldRefresh).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidRefreshToken
			}
			return err
		}
		if oldRefresh.ClientId != client.ClientId || oldRefresh.RevokedAt != nil || oldRefresh.ReplacedByTokenId != nil {
			return ErrInvalidRefreshToken
		}
		if !now.Before(oldRefresh.ExpiresAt) {
			return ErrRefreshExpired
		}
		user, err := getEnabledUser(tx, oldRefresh.UserId)
		if err != nil {
			return err
		}
		plainRefresh, newRefresh, err := s.createRefreshToken(tx, user.Id, client.ClientId, []string(oldRefresh.Scopes), &oldRefresh.Id, now)
		if err != nil {
			return err
		}
		tokenResult, err := s.issueAccessAndIDTokens(tx, user, client.ClientId, []string(oldRefresh.Scopes), "", now, &newRefresh.Id)
		if err != nil {
			return err
		}
		update := tx.Model(&model.OAuthServerRefreshToken{}).
			Where("id = ? AND revoked_at IS NULL AND replaced_by_token_id IS NULL", oldRefresh.Id).
			Updates(map[string]any{
				"last_used_at":         now,
				"revoked_at":           now,
				"replaced_by_token_id": newRefresh.Id,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrInvalidRefreshToken
		}
		tokenResult.RefreshToken = plainRefresh
		result = tokenResult
		return nil
	})
	return result, err
}

func (s *Service) Revoke(ctx context.Context, token string, tokenTypeHint string) error {
	hash := tokenHash(token)
	now := s.now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch strings.TrimSpace(tokenTypeHint) {
		case TokenHintAccess:
			return revokeAccessTokenHash(tx, hash, now)
		case TokenHintRefresh:
			return revokeRefreshTokenHash(tx, hash, now)
		default:
			if changed, err := revokeAccessTokenHashChanged(tx, hash, now); err != nil || changed {
				return err
			}
			return revokeRefreshTokenHash(tx, hash, now)
		}
	})
}

func (s *Service) ListUserGrants(ctx context.Context, userID int) ([]UserGrantResult, error) {
	if _, err := getEnabledUser(s.db.WithContext(ctx), userID); err != nil {
		return nil, err
	}

	var grants []model.OAuthServerUserGrant
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("updated_at DESC").
		Find(&grants).Error; err != nil {
		return nil, err
	}

	results := make([]UserGrantResult, 0, len(grants))
	for _, grant := range grants {
		clientName := grant.ClientId
		var client model.OAuthServerClient
		if err := s.db.WithContext(ctx).Where("client_id = ?", grant.ClientId).First(&client).Error; err == nil {
			clientName = client.ClientName
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		results = append(results, UserGrantResult{
			ID:         grant.Id,
			ClientID:   grant.ClientId,
			ClientName: clientName,
			Scopes:     append([]string(nil), []string(grant.Scopes)...),
			LastUsedAt: grant.LastUsedAt,
			CreatedAt:  grant.CreatedAt,
			UpdatedAt:  grant.UpdatedAt,
		})
	}
	return results, nil
}

func (s *Service) RevokeUserGrant(ctx context.Context, userID int, clientID string) error {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return ErrGrantNotFound
	}
	now := s.now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := getEnabledUser(tx, userID); err != nil {
			return err
		}
		update := tx.Model(&model.OAuthServerUserGrant{}).
			Where("user_id = ? AND client_id = ? AND revoked_at IS NULL", userID, clientID).
			Update("revoked_at", now)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrGrantNotFound
		}
		if err := tx.Model(&model.OAuthServerAccessToken{}).
			Where("user_id = ? AND client_id = ? AND revoked_at IS NULL", userID, clientID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&model.OAuthServerRefreshToken{}).
			Where("user_id = ? AND client_id = ? AND revoked_at IS NULL", userID, clientID).
			Update("revoked_at", now).Error
	})
}

func (s *Service) Introspect(ctx context.Context, token string) (*IntrospectionResult, error) {
	hash := tokenHash(token)
	now := s.now().UTC()
	var access model.OAuthServerAccessToken
	err := s.db.WithContext(ctx).Where("token_hash = ?", hash).First(&access).Error
	if err == nil {
		return s.introspectAccess(ctx, access, now)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var refresh model.OAuthServerRefreshToken
	err = s.db.WithContext(ctx).Where("token_hash = ?", hash).First(&refresh).Error
	if err == nil {
		return s.introspectRefresh(ctx, refresh, now)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &IntrospectionResult{Active: false}, nil
	}
	return nil, err
}

func AuthenticateRelayAccessToken(ctx context.Context, db *gorm.DB, token string) (*RelayAccessTokenAuth, error) {
	if db == nil {
		return nil, errors.New("oauth database is required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrInvalidToken
	}

	var access model.OAuthServerAccessToken
	err := db.WithContext(ctx).Where("token_hash = ?", tokenHash(token)).First(&access).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	if access.RevokedAt != nil || !time.Now().UTC().Before(access.ExpiresAt) {
		return nil, ErrInvalidToken
	}

	user, err := getEnabledUser(db.WithContext(ctx), access.UserId)
	if err != nil {
		return nil, err
	}

	var client model.OAuthServerClient
	if err := db.WithContext(ctx).Where("client_id = ? AND enabled = ?", access.ClientId, true).First(&client).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidClient
		}
		return nil, err
	}

	return &RelayAccessTokenAuth{
		AccessTokenID: access.Id,
		UserID:        user.Id,
		ClientID:      access.ClientId,
		ClientName:    client.ClientName,
		Scopes:        []string(access.Scopes),
		ExpiresAt:     access.ExpiresAt,
	}, nil
}

func (s *Service) JWKS() JWKSet {
	publicKey := s.signingKey.PublicKey
	return JWKSet{Keys: []JWK{{
		KeyType:   "RSA",
		Use:       "sig",
		KeyID:     s.signingKeyID,
		Algorithm: "RS256",
		Modulus:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}}}
}

func (s *Service) SigningKeyID() string {
	return s.signingKeyID
}

func (s *Service) Issuer() string {
	return s.issuer
}

func (s *Service) UserInfo(ctx context.Context, token string) (*UserInfoResult, error) {
	introspection, err := s.Introspect(ctx, token)
	if err != nil {
		return nil, err
	}
	if introspection == nil || !introspection.Active || introspection.TokenType != TokenHintAccess {
		return nil, ErrInvalidToken
	}
	scopes := strings.Fields(introspection.Scope)
	if !containsString(scopes, "openid") {
		return nil, ErrInvalidToken
	}
	userID, err := strconv.Atoi(introspection.Subject)
	if err != nil {
		return nil, ErrInvalidUser
	}
	user, err := getEnabledUser(s.db.WithContext(ctx), userID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(user.Username)
	}
	result := &UserInfoResult{
		Subject:        strconv.Itoa(user.Id),
		CodexAccountID: stableCodexAccountID(user.Id),
		CodexPlanType:  CodexDefaultPlanType,
	}
	if containsString(scopes, "email") {
		result.Email = oauthEmail(user)
	}
	if containsString(scopes, "profile") {
		result.Name = name
		result.PreferredUsername = strings.TrimSpace(user.Username)
	}
	return result, nil
}

func (s *Service) validateAuthorizationRequest(ctx context.Context, req AuthorizationRequest) (*model.OAuthServerClient, []string, error) {
	client, err := s.getEnabledClient(s.db.WithContext(ctx), req.ClientID)
	if err != nil {
		return nil, nil, err
	}
	if !containsString([]string(client.RedirectURIs), strings.TrimSpace(req.RedirectURI)) {
		return nil, nil, ErrInvalidRedirectURI
	}
	scopes, err := validateScopes(req.Scope, []string(client.AllowedScopes))
	if err != nil {
		return nil, nil, err
	}
	if client.Public {
		method := strings.TrimSpace(req.CodeChallengeMethod)
		if strings.TrimSpace(req.CodeChallenge) == "" || method == "" {
			return nil, nil, ErrInvalidPKCE
		}
		if method != pkceMethodS256 && !(s.allowPKCEPlain && method == "plain") {
			return nil, nil, ErrInvalidPKCE
		}
	}
	if _, err := getEnabledUser(s.db.WithContext(ctx), req.UserID); err != nil {
		return nil, nil, err
	}
	return client, scopes, nil
}

func (s *Service) validateTokenClient(tx *gorm.DB, clientID string, clientSecret string) (*model.OAuthServerClient, error) {
	client, err := s.getEnabledClient(tx, clientID)
	if err != nil {
		return nil, err
	}
	if !client.Public {
		if strings.TrimSpace(clientSecret) == "" || strings.TrimSpace(client.ClientSecretHash) == "" {
			return nil, ErrInvalidClient
		}
		if !common.ValidatePasswordAndHash(clientSecret, client.ClientSecretHash) {
			return nil, ErrInvalidClient
		}
	}
	return client, nil
}

func (s *Service) getEnabledClient(tx *gorm.DB, clientID string) (*model.OAuthServerClient, error) {
	var client model.OAuthServerClient
	if err := tx.Where("client_id = ? AND enabled = ?", strings.TrimSpace(clientID), true).First(&client).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidClient
		}
		return nil, err
	}
	return &client, nil
}

func (s *Service) issueTokenPair(tx *gorm.DB, user *model.User, clientID string, scopes []string, nonce string, now time.Time) (*TokenResult, error) {
	var refreshToken string
	var refreshID *int
	if containsString(scopes, "offline_access") {
		plain, refresh, err := s.createRefreshToken(tx, user.Id, clientID, scopes, nil, now)
		if err != nil {
			return nil, err
		}
		refreshToken = plain
		refreshID = &refresh.Id
	}
	result, err := s.issueAccessAndIDTokens(tx, user, clientID, scopes, nonce, now, refreshID)
	if err != nil {
		return nil, err
	}
	result.RefreshToken = refreshToken
	return result, nil
}

func (s *Service) issueAccessAndIDTokens(tx *gorm.DB, user *model.User, clientID string, scopes []string, nonce string, now time.Time, refreshID *int) (*TokenResult, error) {
	accessExpires := now.Add(s.accessTokenTTL)
	jti, err := s.randomToken("jti")
	if err != nil {
		return nil, err
	}
	accessToken, err := s.signJWT(user, clientID, scopes, nonce, jti, now, accessExpires, true)
	if err != nil {
		return nil, err
	}
	accessRecord := model.OAuthServerAccessToken{
		TokenHash:      tokenHash(accessToken),
		JwtId:          jti,
		UserId:         user.Id,
		ClientId:       clientID,
		Scopes:         model.OAuthServerStringList(scopes),
		RefreshTokenId: refreshID,
		ExpiresAt:      accessExpires,
	}
	if err := tx.Create(&accessRecord).Error; err != nil {
		return nil, err
	}
	var idToken string
	if containsString(scopes, "openid") {
		idJTI, err := s.randomToken("jti")
		if err != nil {
			return nil, err
		}
		idToken, err = s.signJWT(user, clientID, scopes, nonce, idJTI, now, now.Add(s.idTokenTTL), true)
		if err != nil {
			return nil, err
		}
	}
	return &TokenResult{
		AccessToken: accessToken,
		IDToken:     idToken,
		TokenType:   TokenTypeBearer,
		ExpiresIn:   int(s.accessTokenTTL.Seconds()),
		Scope:       strings.Join(scopes, " "),
		ExpiresAt:   accessExpires,
	}, nil
}

func (s *Service) createRefreshToken(tx *gorm.DB, userID int, clientID string, scopes []string, parentID *int, now time.Time) (string, *model.OAuthServerRefreshToken, error) {
	plain, err := s.randomToken("or")
	if err != nil {
		return "", nil, err
	}
	refresh := &model.OAuthServerRefreshToken{
		TokenHash:     tokenHash(plain),
		UserId:        userID,
		ClientId:      clientID,
		Scopes:        model.OAuthServerStringList(scopes),
		ParentTokenId: parentID,
		ExpiresAt:     now.Add(s.refreshTokenTTL),
	}
	if err := tx.Create(refresh).Error; err != nil {
		return "", nil, err
	}
	return plain, refresh, nil
}

func (s *Service) signJWT(user *model.User, clientID string, scopes []string, nonce string, jwtID string, issuedAt time.Time, expiresAt time.Time, includeCodexClaims bool) (string, error) {
	name := strings.TrimSpace(user.DisplayName)
	if name == "" {
		name = strings.TrimSpace(user.Username)
	}
	claims := oauthJWTClaims{
		Scope: strings.Join(scopes, " "),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   strconv.Itoa(user.Id),
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ID:        jwtID,
		},
	}
	if containsString(scopes, "email") {
		claims.Email = oauthEmail(user)
	}
	if containsString(scopes, "profile") {
		claims.Name = name
		claims.PreferredUsername = strings.TrimSpace(user.Username)
	}
	if includeCodexClaims {
		claims.CodexAuth = &codexAuthClaims{
			ChatGPTAccountID: stableCodexAccountID(user.Id),
			ChatGPTPlanType:  CodexDefaultPlanType,
		}
	}
	if strings.TrimSpace(nonce) != "" {
		claims.Nonce = strings.TrimSpace(nonce)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.signingKeyID
	return token.SignedString(s.signingKey)
}

func (s *Service) introspectAccess(ctx context.Context, access model.OAuthServerAccessToken, now time.Time) (*IntrospectionResult, error) {
	if access.RevokedAt != nil || !now.Before(access.ExpiresAt) {
		return &IntrospectionResult{Active: false}, nil
	}
	user, err := getEnabledUser(s.db.WithContext(ctx), access.UserId)
	if err != nil {
		if errors.Is(err, ErrInvalidUser) {
			return &IntrospectionResult{Active: false}, nil
		}
		return nil, err
	}
	return &IntrospectionResult{
		Active:    true,
		TokenType: TokenHintAccess,
		Scope:     strings.Join([]string(access.Scopes), " "),
		ClientID:  access.ClientId,
		Username:  user.Username,
		Subject:   strconv.Itoa(access.UserId),
		Audience:  access.ClientId,
		Issuer:    s.issuer,
		ExpiresAt: access.ExpiresAt.Unix(),
		IssuedAt:  access.CreatedAt.Unix(),
	}, nil
}

func (s *Service) introspectRefresh(ctx context.Context, refresh model.OAuthServerRefreshToken, now time.Time) (*IntrospectionResult, error) {
	if refresh.RevokedAt != nil || refresh.ReplacedByTokenId != nil || !now.Before(refresh.ExpiresAt) {
		return &IntrospectionResult{Active: false}, nil
	}
	user, err := getEnabledUser(s.db.WithContext(ctx), refresh.UserId)
	if err != nil {
		if errors.Is(err, ErrInvalidUser) {
			return &IntrospectionResult{Active: false}, nil
		}
		return nil, err
	}
	return &IntrospectionResult{
		Active:    true,
		TokenType: TokenHintRefresh,
		Scope:     strings.Join([]string(refresh.Scopes), " "),
		ClientID:  refresh.ClientId,
		Username:  user.Username,
		Subject:   strconv.Itoa(refresh.UserId),
		Audience:  refresh.ClientId,
		Issuer:    s.issuer,
		ExpiresAt: refresh.ExpiresAt.Unix(),
		IssuedAt:  refresh.CreatedAt.Unix(),
	}, nil
}

func upsertUserGrant(tx *gorm.DB, userID int, clientID string, scopes []string, now time.Time) error {
	var grant model.OAuthServerUserGrant
	err := tx.Where("user_id = ? AND client_id = ?", userID, clientID).First(&grant).Error
	if err == nil {
		return tx.Model(&grant).Updates(map[string]any{
			"scopes":       model.OAuthServerStringList(scopes),
			"revoked_at":   nil,
			"last_used_at": now,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	grant = model.OAuthServerUserGrant{
		UserId:     userID,
		ClientId:   clientID,
		Scopes:     model.OAuthServerStringList(scopes),
		LastUsedAt: &now,
	}
	return tx.Create(&grant).Error
}

func getEnabledUser(tx *gorm.DB, userID int) (*model.User, error) {
	if userID == 0 {
		return nil, ErrInvalidUser
	}
	var user model.User
	if err := tx.Omit("password").First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidUser
		}
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, ErrInvalidUser
	}
	return &user, nil
}

func revokeAccessTokenHash(tx *gorm.DB, hash string, now time.Time) error {
	_, err := revokeAccessTokenHashChanged(tx, hash, now)
	return err
}

func revokeAccessTokenHashChanged(tx *gorm.DB, hash string, now time.Time) (bool, error) {
	update := tx.Model(&model.OAuthServerAccessToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Update("revoked_at", now)
	if update.Error != nil {
		return false, update.Error
	}
	return update.RowsAffected > 0, nil
}

func revokeRefreshTokenHash(tx *gorm.DB, hash string, now time.Time) error {
	var refresh model.OAuthServerRefreshToken
	err := tx.Where("token_hash = ?", hash).First(&refresh).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if refresh.RevokedAt == nil {
		if err := tx.Model(&model.OAuthServerRefreshToken{}).Where("id = ?", refresh.Id).Update("revoked_at", now).Error; err != nil {
			return err
		}
	}
	return tx.Model(&model.OAuthServerAccessToken{}).
		Where("refresh_token_id = ? AND revoked_at IS NULL", refresh.Id).
		Update("revoked_at", now).Error
}

func validateScopes(scope string, allowed []string) ([]string, error) {
	requested := strings.Fields(scope)
	seen := make(map[string]bool, len(requested))
	scopes := make([]string, 0, len(requested))
	for _, item := range requested {
		if item == "" || seen[item] {
			continue
		}
		if !containsString(allowed, item) {
			return nil, ErrInvalidScope
		}
		seen[item] = true
		scopes = append(scopes, item)
	}
	return scopes, nil
}

func verifyPKCE(challenge string, method string, verifier string, allowPlain bool) error {
	challenge = strings.TrimSpace(challenge)
	method = strings.TrimSpace(method)
	verifier = strings.TrimSpace(verifier)
	if challenge == "" || verifier == "" {
		return ErrInvalidPKCE
	}
	switch method {
	case pkceMethodS256:
		sum := sha256.Sum256([]byte(verifier))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge {
			return ErrPKCEMismatch
		}
		return nil
	case "plain":
		if !allowPlain {
			return ErrInvalidPKCE
		}
		if verifier != challenge {
			return ErrPKCEMismatch
		}
		return nil
	default:
		return ErrInvalidPKCE
	}
}

func parseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	value := strings.TrimSpace(privateKeyPEM)
	if value == "" {
		return nil, ErrSigningKeyRequired
	}
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("invalid oauth signing private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid oauth signing private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("oauth signing private key must be RSA")
	}
	return key, nil
}

func deriveRSAKeyID(publicKey *rsa.PublicKey) (string, error) {
	data, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(sum[:16]), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func stableCodexAccountID(userID int) string {
	return "user-" + strconv.Itoa(userID)
}

func oauthEmail(user *model.User) string {
	email := strings.TrimSpace(user.Email)
	if email != "" {
		return email
	}
	username := strings.TrimSpace(user.Username)
	if username != "" {
		return username + "@localhost"
	}
	return stableCodexAccountID(user.Id) + "@localhost"
}

func (s *Service) randomToken(prefix string) (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(s.randomReader, buf); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func durationOrDefault(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
