package model

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type OAuthServerStringList []string

func (l OAuthServerStringList) Value() (driver.Value, error) {
	items := []string(l)
	if items == nil {
		items = []string{}
	}
	data, err := common.Marshal(items)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (l *OAuthServerStringList) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*l = OAuthServerStringList{}
		return nil
	case []byte:
		return l.scanBytes(v)
	case string:
		return l.scanBytes(common.StringToByteSlice(v))
	default:
		return fmt.Errorf("unsupported OAuthServerStringList database value %T", value)
	}
}

func (l *OAuthServerStringList) scanBytes(data []byte) error {
	var items []string
	if len(data) == 0 {
		*l = OAuthServerStringList{}
		return nil
	}
	if err := common.Unmarshal(data, &items); err != nil {
		return err
	}
	if items == nil {
		items = []string{}
	}
	*l = OAuthServerStringList(items)
	return nil
}

type OAuthServerClient struct {
	Id               int                   `json:"id" gorm:"primaryKey"`
	ClientId         string                `json:"client_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	ClientName       string                `json:"client_name" gorm:"type:varchar(128);not null"`
	ClientSecretHash string                `json:"-" gorm:"type:varchar(255);default:''"`
	Public           bool                  `json:"public" gorm:"not null;default:true"`
	RedirectURIs     OAuthServerStringList `json:"redirect_uris" gorm:"type:text;not null"`
	AllowedScopes    OAuthServerStringList `json:"allowed_scopes" gorm:"type:text;not null"`
	Enabled          bool                  `json:"enabled" gorm:"not null;default:true;index"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
	DeletedAt        gorm.DeletedAt        `json:"-" gorm:"index"`
}

func (OAuthServerClient) TableName() string {
	return "oauth_server_clients"
}

type OAuthServerAuthorizationCode struct {
	Id                  int                   `json:"id" gorm:"primaryKey"`
	CodeHash            string                `json:"-" gorm:"type:varchar(128);not null;uniqueIndex"`
	UserId              int                   `json:"user_id" gorm:"not null;index"`
	ClientId            string                `json:"client_id" gorm:"type:varchar(128);not null;index"`
	RedirectURI         string                `json:"redirect_uri" gorm:"type:varchar(512);not null"`
	Scopes              OAuthServerStringList `json:"scopes" gorm:"type:text;not null"`
	CodeChallenge       string                `json:"code_challenge" gorm:"type:varchar(256);not null"`
	CodeChallengeMethod string                `json:"code_challenge_method" gorm:"type:varchar(16);not null"`
	Nonce               string                `json:"nonce" gorm:"type:varchar(255);default:''"`
	ExpiresAt           time.Time             `json:"expires_at" gorm:"not null;index"`
	ConsumedAt          *time.Time            `json:"consumed_at" gorm:"index"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
}

func (OAuthServerAuthorizationCode) TableName() string {
	return "oauth_server_authorization_codes"
}

type OAuthServerAccessToken struct {
	Id             int                   `json:"id" gorm:"primaryKey"`
	TokenHash      string                `json:"-" gorm:"type:varchar(128);not null;uniqueIndex"`
	JwtId          string                `json:"jwt_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	UserId         int                   `json:"user_id" gorm:"not null;index"`
	ClientId       string                `json:"client_id" gorm:"type:varchar(128);not null;index"`
	Scopes         OAuthServerStringList `json:"scopes" gorm:"type:text;not null"`
	RefreshTokenId *int                  `json:"refresh_token_id" gorm:"index"`
	ExpiresAt      time.Time             `json:"expires_at" gorm:"not null;index"`
	RevokedAt      *time.Time            `json:"revoked_at" gorm:"index"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

func (OAuthServerAccessToken) TableName() string {
	return "oauth_server_access_tokens"
}

type OAuthServerRefreshToken struct {
	Id                int                   `json:"id" gorm:"primaryKey"`
	TokenHash         string                `json:"-" gorm:"type:varchar(128);not null;uniqueIndex"`
	UserId            int                   `json:"user_id" gorm:"not null;index"`
	ClientId          string                `json:"client_id" gorm:"type:varchar(128);not null;index"`
	Scopes            OAuthServerStringList `json:"scopes" gorm:"type:text;not null"`
	ParentTokenId     *int                  `json:"parent_token_id" gorm:"index"`
	ReplacedByTokenId *int                  `json:"replaced_by_token_id" gorm:"index"`
	ExpiresAt         time.Time             `json:"expires_at" gorm:"not null;index"`
	RevokedAt         *time.Time            `json:"revoked_at" gorm:"index"`
	LastUsedAt        *time.Time            `json:"last_used_at"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

func (OAuthServerRefreshToken) TableName() string {
	return "oauth_server_refresh_tokens"
}

type OAuthServerUserGrant struct {
	Id         int                   `json:"id" gorm:"primaryKey"`
	UserId     int                   `json:"user_id" gorm:"not null;uniqueIndex:ux_oauth_server_user_client"`
	ClientId   string                `json:"client_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_oauth_server_user_client"`
	Scopes     OAuthServerStringList `json:"scopes" gorm:"type:text;not null"`
	RevokedAt  *time.Time            `json:"revoked_at" gorm:"index"`
	LastUsedAt *time.Time            `json:"last_used_at"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

func (OAuthServerUserGrant) TableName() string {
	return "oauth_server_user_grants"
}
