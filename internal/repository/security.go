package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

func (r *TravelRepository) CreateAuthSession(ctx context.Context, s *model.AuthSession) error {
	query := `
		INSERT INTO auth_sessions (id, user_id, refresh_token_hash, device_name, platform, user_agent, ip, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at, last_seen_at`
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query), s.ID, s.UserID, s.RefreshTokenHash, s.DeviceName, s.Platform, s.UserAgent, s.IP, s.ExpiresAt).
		Scan(&s.CreatedAt, &s.LastSeenAt)
}

func (r *TravelRepository) GetAuthSession(ctx context.Context, id uuid.UUID) (*model.AuthSession, error) {
	var s model.AuthSession
	if err := r.db.GetContext(ctx, &s, r.db.Rebind("SELECT * FROM auth_sessions WHERE id = $1"), id); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TravelRepository) GetSessionByRefreshHash(ctx context.Context, hash string) (*model.AuthSession, error) {
	var s model.AuthSession
	if err := r.db.GetContext(ctx, &s, r.db.Rebind("SELECT * FROM auth_sessions WHERE refresh_token_hash = $1"), hash); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TravelRepository) TouchSession(ctx context.Context, id uuid.UUID) {
	_, _ = r.db.ExecContext(ctx, r.db.Rebind("UPDATE auth_sessions SET last_seen_at = NOW() WHERE id = $1"), id)
}

func (r *TravelRepository) UpdateSessionRefresh(ctx context.Context, id uuid.UUID, hash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE auth_sessions SET refresh_token_hash = $1, expires_at = $2, last_seen_at = NOW(), revoked_at = NULL WHERE id = $3"), hash, expiresAt, id)
	return err
}

func (r *TravelRepository) RevokeSession(ctx context.Context, id, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE auth_sessions SET revoked_at = NOW() WHERE id = $1 AND user_id = $2"), id, userID)
	return err
}

func (r *TravelRepository) RevokeAllSessions(ctx context.Context, userID uuid.UUID, except *uuid.UUID) error {
	if except != nil {
		_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE auth_sessions SET revoked_at = NOW() WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL"), userID, *except)
		return err
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE auth_sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL"), userID)
	return err
}

func (r *TravelRepository) ListSessions(ctx context.Context, userID uuid.UUID) ([]model.AuthSession, error) {
	var sessions []model.AuthSession
	query := "SELECT * FROM auth_sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100"
	if err := r.db.SelectContext(ctx, &sessions, r.db.Rebind(query), userID); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *TravelRepository) CreateOAuthCode(ctx context.Context, c *model.OAuthCode) error {
	query := `
		INSERT INTO oauth_codes (code, user_id, client_id, redirect_uri, scope, code_challenge, code_challenge_method, nonce, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), c.Code, c.UserID, c.ClientID, c.RedirectURI, c.Scope, c.CodeChallenge, c.CodeChallengeMethod, c.Nonce, c.ExpiresAt)
	return err
}

func (r *TravelRepository) ConsumeOAuthCode(ctx context.Context, code string) (*model.OAuthCode, error) {
	var c model.OAuthCode
	query := `UPDATE oauth_codes SET used_at = NOW()
		WHERE code = $1 AND used_at IS NULL AND expires_at > NOW()
		RETURNING code, user_id, client_id, redirect_uri, scope, code_challenge, code_challenge_method, nonce, expires_at, used_at`
	if err := r.db.GetContext(ctx, &c, r.db.Rebind(query), code); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *TravelRepository) GetMFASecret(ctx context.Context, userID uuid.UUID) (*model.MFASecret, error) {
	var m model.MFASecret
	if err := r.db.GetContext(ctx, &m, r.db.Rebind("SELECT * FROM user_mfa WHERE user_id = $1"), userID); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *TravelRepository) UpsertMFASecret(ctx context.Context, userID uuid.UUID, secret string) error {
	query := `INSERT INTO user_mfa (user_id, secret, enabled) VALUES ($1,$2,FALSE)
		ON CONFLICT (user_id) DO UPDATE SET secret = EXCLUDED.secret, enabled = FALSE, verified_at = NULL`
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), userID, secret)
	return err
}

func (r *TravelRepository) SetMFAEnabled(ctx context.Context, userID uuid.UUID, enabled bool) error {
	if enabled {
		if _, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE user_mfa SET enabled = TRUE, verified_at = NOW() WHERE user_id = $1"), userID); err != nil {
			return err
		}
	} else {
		if _, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE user_mfa SET enabled = FALSE WHERE user_id = $1"), userID); err != nil {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE users SET mfa_enabled = $1, updated_at = NOW() WHERE id = $2"), enabled, userID)
	return err
}

func (r *TravelRepository) CreatePasskey(ctx context.Context, p *model.PasskeyCredential) error {
	query := `INSERT INTO passkey_credentials (id, user_id, credential_id, public_key, attestation, transports, sign_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (credential_id) DO UPDATE SET public_key = EXCLUDED.public_key, attestation = EXCLUDED.attestation, transports = EXCLUDED.transports
		RETURNING created_at`
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query), p.ID, p.UserID, p.CredentialID, p.PublicKey, p.Attestation, p.Transports, p.SignCount).Scan(&p.CreatedAt)
}

func (r *TravelRepository) GetPasskeysByUser(ctx context.Context, userID uuid.UUID) ([]model.PasskeyCredential, error) {
	var items []model.PasskeyCredential
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind("SELECT * FROM passkey_credentials WHERE user_id = $1 ORDER BY created_at DESC"), userID); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *TravelRepository) GetPasskeyByCredentialID(ctx context.Context, credentialID string) (*model.PasskeyCredential, error) {
	var p model.PasskeyCredential
	if err := r.db.GetContext(ctx, &p, r.db.Rebind("SELECT * FROM passkey_credentials WHERE credential_id = $1"), credentialID); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *TravelRepository) DeletePasskey(ctx context.Context, id, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("DELETE FROM passkey_credentials WHERE id = $1 AND user_id = $2"), id, userID)
	return err
}

func (r *TravelRepository) TouchPasskey(ctx context.Context, id uuid.UUID) {
	_, _ = r.db.ExecContext(ctx, r.db.Rebind("UPDATE passkey_credentials SET last_used_at = NOW(), sign_count = sign_count + 1 WHERE id = $1"), id)
}

func (r *TravelRepository) SetTourOwner(ctx context.Context, tourID, ownerID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE tours SET owner_id = $1, updated_at = NOW() WHERE id = $2"), ownerID, tourID)
	return err
}

func (r *TravelRepository) GetToursByOwner(ctx context.Context, ownerID uuid.UUID, destID *uuid.UUID, available *bool, p *model.Pagination) ([]model.Tour, int, error) {
	where := "WHERE owner_id = $1"
	args := []interface{}{ownerID}
	idx := 2
	if destID != nil {
		where += " AND destination_id = $2"
		args = append(args, *destID)
		idx++
	}
	if available != nil {
		where += " AND available = $" + itoa(idx)
		args = append(args, *available)
		idx++
	}
	var total int
	countQuery := "SELECT COUNT(*) FROM tours " + where
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, err
	}
	query := "SELECT * FROM tours " + where + " ORDER BY created_at DESC LIMIT $" + itoa(idx) + " OFFSET $" + itoa(idx+1)
	args = append(args, p.Limit(), p.Offset())
	var tours []model.Tour
	if err := r.db.SelectContext(ctx, &tours, r.db.Rebind(query), args...); err != nil {
		return nil, 0, err
	}
	return tours, total, nil
}

func (r *TravelRepository) UpdateOrderPartner(ctx context.Context, orderID, partnerID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE orders SET partner_id = $1, updated_at = NOW() WHERE id = $2"), partnerID, orderID)
	return err
}

func (r *TravelRepository) GetOrdersByPartner(ctx context.Context, partnerID uuid.UUID, p *model.Pagination) ([]model.Order, int, error) {
	var orders []model.Order
	query := "SELECT * FROM orders WHERE partner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3"
	if err := r.db.SelectContext(ctx, &orders, r.db.Rebind(query), partnerID, p.Limit(), p.Offset()); err != nil {
		return nil, 0, err
	}
	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind("SELECT COUNT(*) FROM orders WHERE partner_id = $1"), partnerID); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func itoa(v int) string { return strconv.Itoa(v) }
