package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthSession struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	RefreshTokenHash string     `json:"-" db:"refresh_token_hash"`
	DeviceName       string     `json:"device_name" db:"device_name"`
	Platform         string     `json:"platform" db:"platform"`
	UserAgent        string     `json:"user_agent" db:"user_agent"`
	IP               string     `json:"ip" db:"ip"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	LastSeenAt       time.Time  `json:"last_seen_at" db:"last_seen_at"`
	ExpiresAt        time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	Current          bool       `json:"current" db:"-"`
}

type OAuthCode struct {
	Code                string     `db:"code"`
	UserID              uuid.UUID  `db:"user_id"`
	ClientID            string     `db:"client_id"`
	RedirectURI         string     `db:"redirect_uri"`
	Scope               string     `db:"scope"`
	CodeChallenge       string     `db:"code_challenge"`
	CodeChallengeMethod string     `db:"code_challenge_method"`
	Nonce               string     `db:"nonce"`
	ExpiresAt           time.Time  `db:"expires_at"`
	UsedAt              *time.Time `db:"used_at"`
}

type MFASecret struct {
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	Secret     string     `json:"-" db:"secret"`
	Enabled    bool       `json:"enabled" db:"enabled"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	VerifiedAt *time.Time `json:"verified_at,omitempty" db:"verified_at"`
}

type PasskeyCredential struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       uuid.UUID  `json:"user_id" db:"user_id"`
	CredentialID string     `json:"credential_id" db:"credential_id"`
	PublicKey    string     `json:"-" db:"public_key"`
	Attestation  string     `json:"-" db:"attestation"`
	Transports   string     `json:"transports" db:"transports"`
	SignCount    int        `json:"sign_count" db:"sign_count"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	SessionID    string `json:"session_id"`
}

type DeviceInfo struct {
	Name      string
	Platform  string
	UserAgent string
	IP        string
}
