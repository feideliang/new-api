package model

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOAuthServerModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&OAuthServerClient{},
		&OAuthServerAuthorizationCode{},
		&OAuthServerAccessToken{},
		&OAuthServerRefreshToken{},
		&OAuthServerUserGrant{},
	))
	return db
}

func TestOAuthServerModelsAutoMigrateAndRoundTrip(t *testing.T) {
	db := setupOAuthServerModelTestDB(t)
	now := time.Now().UTC()

	client := OAuthServerClient{
		ClientId:      "app_EMoamEEZ73f0CkXaXp7hrann",
		ClientName:    "Codex CLI",
		Public:        true,
		RedirectURIs:  OAuthServerStringList{"http://localhost:1455/auth/callback", "http://127.0.0.1:1455/auth/callback"},
		AllowedScopes: OAuthServerStringList{"openid", "profile", "offline_access", "api.connectors.invoke"},
		Enabled:       true,
	}
	require.NoError(t, db.Create(&client).Error)

	code := OAuthServerAuthorizationCode{
		CodeHash:            strings.Repeat("a", 64),
		UserId:              7,
		ClientId:            client.ClientId,
		RedirectURI:         "http://localhost:1455/auth/callback",
		Scopes:              OAuthServerStringList{"openid", "offline_access"},
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           now.Add(10 * time.Minute),
	}
	require.NoError(t, db.Create(&code).Error)

	refresh := OAuthServerRefreshToken{
		TokenHash: strings.Repeat("b", 64),
		UserId:    7,
		ClientId:  client.ClientId,
		Scopes:    OAuthServerStringList{"openid", "offline_access"},
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	require.NoError(t, db.Create(&refresh).Error)

	access := OAuthServerAccessToken{
		TokenHash:      strings.Repeat("c", 64),
		JwtId:          "jwt-id",
		UserId:         7,
		ClientId:       client.ClientId,
		Scopes:         OAuthServerStringList{"openid"},
		RefreshTokenId: &refresh.Id,
		ExpiresAt:      now.Add(time.Hour),
	}
	require.NoError(t, db.Create(&access).Error)

	grant := OAuthServerUserGrant{
		UserId:   7,
		ClientId: client.ClientId,
		Scopes:   OAuthServerStringList{"openid", "profile"},
	}
	require.NoError(t, db.Create(&grant).Error)

	var gotClient OAuthServerClient
	require.NoError(t, db.Where("client_id = ?", client.ClientId).First(&gotClient).Error)
	require.Equal(t, OAuthServerStringList{"http://localhost:1455/auth/callback", "http://127.0.0.1:1455/auth/callback"}, gotClient.RedirectURIs)
	require.Equal(t, OAuthServerStringList{"openid", "profile", "offline_access", "api.connectors.invoke"}, gotClient.AllowedScopes)

	var gotCode OAuthServerAuthorizationCode
	require.NoError(t, db.Where("code_hash = ?", code.CodeHash).First(&gotCode).Error)
	require.Equal(t, OAuthServerStringList{"openid", "offline_access"}, gotCode.Scopes)

	var gotAccess OAuthServerAccessToken
	require.NoError(t, db.Where("token_hash = ?", access.TokenHash).First(&gotAccess).Error)
	require.Equal(t, OAuthServerStringList{"openid"}, gotAccess.Scopes)
	require.NotNil(t, gotAccess.RefreshTokenId)
	require.Equal(t, refresh.Id, *gotAccess.RefreshTokenId)

	var gotRefresh OAuthServerRefreshToken
	require.NoError(t, db.Where("token_hash = ?", refresh.TokenHash).First(&gotRefresh).Error)
	require.Equal(t, OAuthServerStringList{"openid", "offline_access"}, gotRefresh.Scopes)

	var gotGrant OAuthServerUserGrant
	require.NoError(t, db.Where("user_id = ? AND client_id = ?", 7, client.ClientId).First(&gotGrant).Error)
	require.Equal(t, OAuthServerStringList{"openid", "profile"}, gotGrant.Scopes)
}

func TestOAuthServerJSONListColumnsUseText(t *testing.T) {
	db := setupOAuthServerModelTestDB(t)

	assertColumnIsText(t, db, &OAuthServerClient{}, "redirect_uris")
	assertColumnIsText(t, db, &OAuthServerClient{}, "allowed_scopes")
	assertColumnIsText(t, db, &OAuthServerAuthorizationCode{}, "scopes")
	assertColumnIsText(t, db, &OAuthServerAccessToken{}, "scopes")
	assertColumnIsText(t, db, &OAuthServerRefreshToken{}, "scopes")
	assertColumnIsText(t, db, &OAuthServerUserGrant{}, "scopes")
}

func assertColumnIsText(t *testing.T, db *gorm.DB, model any, column string) {
	t.Helper()
	types, err := db.Migrator().ColumnTypes(model)
	require.NoError(t, err)
	for _, typ := range types {
		if typ.Name() == column {
			require.Equal(t, "text", strings.ToLower(typ.DatabaseTypeName()))
			return
		}
	}
	t.Fatalf("column %s was not migrated", column)
}

func TestOAuthServerTokenModelsStoreOnlyHashes(t *testing.T) {
	assertNoPlaintextFields(t, reflect.TypeOf(OAuthServerAuthorizationCode{}), "Code")
	assertNoPlaintextFields(t, reflect.TypeOf(OAuthServerAccessToken{}), "Token", "AccessToken")
	assertNoPlaintextFields(t, reflect.TypeOf(OAuthServerRefreshToken{}), "Token", "RefreshToken")
}

func assertNoPlaintextFields(t *testing.T, typ reflect.Type, forbidden ...string) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		for _, name := range forbidden {
			require.NotEqual(t, name, field.Name)
		}
	}
}
