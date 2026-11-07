package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JSONMap is a small helper for JSONB columns. It implements sql.Scanner and
// driver.Valuer so it can be used directly with sqlx.
type JSONMap map[string]interface{}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = JSONMap{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported JSONB type %T", value)
	}
	if len(raw) == 0 {
		*m = JSONMap{}
		return nil
	}
	decoded := JSONMap{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*m = decoded
	return nil
}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

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
	Email            string     `json:"email,omitempty" db:"email"`
	DeviceID         *uuid.UUID `json:"device_id,omitempty" db:"device_id"`
	LastIP           string     `json:"last_ip" db:"last_ip"`
	LastMethod       string     `json:"last_method" db:"last_method"`
	RequestCount     int64      `json:"request_count" db:"request_count"`
	Current          bool       `json:"current" db:"-"`
	IPs              []string   `json:"ips,omitempty" db:"-"`
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
	LastStep   int64      `json:"-" db:"last_step"`
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

// UserDevice is an aggregated device profile. Multiple auth sessions created
// from the same browser/phone share one device record.
type UserDevice struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	Fingerprint    string    `json:"fingerprint" db:"fingerprint"`
	Name           string    `json:"name" db:"name"`
	Platform       string    `json:"platform" db:"platform"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	FirstIP        string    `json:"first_ip" db:"first_ip"`
	LastIP         string    `json:"last_ip" db:"last_ip"`
	LoginCount     int       `json:"login_count" db:"login_count"`
	FirstSeenAt    time.Time `json:"first_seen_at" db:"first_seen_at"`
	LastSeenAt     time.Time `json:"last_seen_at" db:"last_seen_at"`
	Trusted        bool      `json:"trusted" db:"trusted"`
	Blocked        bool      `json:"blocked" db:"blocked"`
	ActiveSessions int       `json:"active_sessions" db:"active_sessions"`
	Current        bool      `json:"current" db:"-"`
	IPs            []string  `json:"ips,omitempty" db:"-"`
	SessionIDs     []string  `json:"session_ids,omitempty" db:"-"`
}

// SessionIP records every IP observed for a session. It gives a per-device IP
// history instead of only the IP captured at login.
type SessionIP struct {
	SessionID    uuid.UUID `json:"session_id" db:"session_id"`
	IP           string    `json:"ip" db:"ip"`
	FirstSeenAt  time.Time `json:"first_seen_at" db:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at" db:"last_seen_at"`
	RequestCount int64     `json:"request_count" db:"request_count"`
}

// AuthEvent is the security audit trail. Failed logins are stored too, which
// makes brute-force attempts and suspicious IPs visible to the account owner
// and to admins.
type AuthEvent struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Email      string     `json:"email,omitempty" db:"email"`
	Event      string     `json:"event" db:"event"`
	Success    bool       `json:"success" db:"success"`
	Reason     string     `json:"reason,omitempty" db:"reason"`
	SessionID  *uuid.UUID `json:"session_id,omitempty" db:"session_id"`
	DeviceID   *uuid.UUID `json:"device_id,omitempty" db:"device_id"`
	IP         string     `json:"ip" db:"ip"`
	UserAgent  string     `json:"user_agent" db:"user_agent"`
	Platform   string     `json:"platform" db:"platform"`
	DeviceName string     `json:"device_name" db:"device_name"`
	RequestID  string     `json:"request_id,omitempty" db:"request_id"`
	Metadata   JSONMap    `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}

type AuthEventFilter struct {
	UserID  *uuid.UUID
	Email   string
	Event   string
	IP      string
	Success *bool
	From    *time.Time
	To      *time.Time
	Limit   int
	Offset  int
}

type SecuritySummary struct {
	TotalDevices        int         `json:"total_devices"`
	TrustedDevices      int         `json:"trusted_devices"`
	BlockedDevices      int         `json:"blocked_devices"`
	TotalSessions       int         `json:"total_sessions"`
	ActiveSessions      int         `json:"active_sessions"`
	UniqueIPs           []string    `json:"unique_ips"`
	LastLoginAt         *time.Time  `json:"last_login_at,omitempty"`
	LastFailedLoginAt   *time.Time  `json:"last_failed_login_at,omitempty"`
	SuccessfulLogins24h int         `json:"successful_logins_24h"`
	FailedLogins24h     int         `json:"failed_logins_24h"`
	MFAEnabled          bool        `json:"mfa_enabled"`
	Passkeys            int         `json:"passkeys"`
	AccountBlocked      bool        `json:"account_blocked"`
	RecentEvents        []AuthEvent `json:"recent_events"`
	RecentIPs           []IPCount   `json:"recent_ips"`
}

type IPCount struct {
	IP    string `json:"ip" db:"ip"`
	Count int    `json:"count" db:"count"`
}

type DeviceCount struct {
	Device   string `json:"device" db:"device"`
	Platform string `json:"platform" db:"platform"`
	Count    int    `json:"count" db:"count"`
}

type DayCount struct {
	Day     string `json:"day" db:"day"`
	Success int    `json:"success" db:"success"`
	Failed  int    `json:"failed" db:"failed"`
}

type AdminSecurityStats struct {
	Events24h           int           `json:"events_24h"`
	SuccessfulLogins24h int           `json:"successful_logins_24h"`
	FailedLogins24h     int           `json:"failed_logins_24h"`
	BlockedLogins24h    int           `json:"blocked_logins_24h"`
	UniqueIPs24h        int           `json:"unique_ips_24h"`
	ActiveSessions      int           `json:"active_sessions"`
	TotalDevices        int           `json:"total_devices"`
	BlockedUsers        int           `json:"blocked_users"`
	TopIPs              []IPCount     `json:"top_ips"`
	TopDevices          []DeviceCount `json:"top_devices"`
	EventsByDay         []DayCount    `json:"events_by_day"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	SessionID    string `json:"session_id"`
	DeviceID     string `json:"device_id,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type DeviceInfo struct {
	Name        string
	Platform    string
	UserAgent   string
	IP          string
	RequestID   string
	Fingerprint string
}
