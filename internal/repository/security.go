package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
)

// sessionColumns is explicit on purpose: SELECT * silently breaks whenever a
// new column is added to auth_sessions.
const sessionColumns = `id, user_id, refresh_token_hash, device_name, platform, user_agent, ip,
	created_at, last_seen_at, expires_at, revoked_at, device_id, last_ip, last_method, request_count`

func (r *TravelRepository) CreateAuthSession(ctx context.Context, s *model.AuthSession) error {
	query := `
		INSERT INTO auth_sessions (id, user_id, refresh_token_hash, device_name, platform, user_agent, ip, expires_at, device_id, last_ip, last_method)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING created_at, last_seen_at`
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query),
		s.ID, s.UserID, s.RefreshTokenHash, s.DeviceName, s.Platform, s.UserAgent, s.IP, s.ExpiresAt,
		s.DeviceID, s.LastIP, s.LastMethod).
		Scan(&s.CreatedAt, &s.LastSeenAt)
}

func (r *TravelRepository) GetAuthSession(ctx context.Context, id uuid.UUID) (*model.AuthSession, error) {
	var s model.AuthSession
	query := "SELECT " + sessionColumns + " FROM auth_sessions WHERE id = $1"
	if err := r.db.GetContext(ctx, &s, r.db.Rebind(query), id); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TravelRepository) GetSessionByRefreshHash(ctx context.Context, hash string) (*model.AuthSession, error) {
	var s model.AuthSession
	query := "SELECT " + sessionColumns + " FROM auth_sessions WHERE refresh_token_hash = $1"
	if err := r.db.GetContext(ctx, &s, r.db.Rebind(query), hash); err != nil {
		return nil, err
	}
	return &s, nil
}

// TouchSession refreshes last_seen_at, remembers the last IP and appends the IP
// to the per-session IP history. Callers throttle this to about once a minute.
func (r *TravelRepository) TouchSession(ctx context.Context, id uuid.UUID, ip string) {
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions
		SET last_seen_at = NOW(),
		    last_ip = COALESCE(NULLIF($2, ''), last_ip),
		    request_count = request_count + 1
		WHERE id = $1`), id, ip)
	if ip != "" {
		_, _ = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO session_ips (session_id, ip)
			VALUES ($1, $2)
			ON CONFLICT (session_id, ip) DO UPDATE
			SET last_seen_at = NOW(), request_count = session_ips.request_count + 1`), id, ip)
	}
}

// UpdateSessionRefresh rotates the refresh hash atomically. The old hash is
// part of the WHERE clause, so two concurrent refreshes cannot both succeed.
func (r *TravelRepository) UpdateSessionRefresh(ctx context.Context, id uuid.UUID, oldHash, newHash string, expiresAt time.Time, ip string) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions
		SET refresh_token_hash = $1, expires_at = $2, last_seen_at = NOW(),
		    last_ip = COALESCE(NULLIF($3, ''), last_ip), request_count = request_count + 1
		WHERE id = $4 AND refresh_token_hash = $5 AND revoked_at IS NULL`),
		newHash, expiresAt, ip, id, oldHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 1 && ip != "" {
		_, _ = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO session_ips (session_id, ip)
			VALUES ($1, $2)
			ON CONFLICT (session_id, ip) DO UPDATE
			SET last_seen_at = NOW(), request_count = session_ips.request_count + 1`), id, ip)
	}
	return n == 1, nil
}

// RevokeSession returns false when the session does not belong to the user (or
// does not exist), which lets handlers return 404 instead of a silent 200.
func (r *TravelRepository) RevokeSession(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions
		SET revoked_at = NOW() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`), id, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *TravelRepository) RevokeAllSessions(ctx context.Context, userID uuid.UUID, except *uuid.UUID) (int64, error) {
	var (
		res sql.Result
		err error
	)
	if except != nil {
		res, err = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions
			SET revoked_at = NOW() WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`), userID, *except)
	} else {
		res, err = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions
			SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`), userID)
	}
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *TravelRepository) RevokeSessionsByDevice(ctx context.Context, deviceID, userID uuid.UUID) (int64, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions
		SET revoked_at = NOW() WHERE device_id = $1 AND user_id = $2 AND revoked_at IS NULL`), deviceID, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *TravelRepository) ListSessions(ctx context.Context, userID uuid.UUID) ([]model.AuthSession, error) {
	var sessions []model.AuthSession
	query := "SELECT " + sessionColumns + " FROM auth_sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 200"
	if err := r.db.SelectContext(ctx, &sessions, r.db.Rebind(query), userID); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *TravelRepository) ListAllSessions(ctx context.Context, limit, offset int) ([]model.AuthSession, int, error) {
	var sessions []model.AuthSession
	query := `SELECT s.id, s.user_id, s.refresh_token_hash, s.device_name, s.platform, s.user_agent, s.ip,
		s.created_at, s.last_seen_at, s.expires_at, s.revoked_at, s.device_id, s.last_ip, s.last_method, s.request_count,
		u.email AS email
		FROM auth_sessions s LEFT JOIN users u ON u.id = s.user_id
		ORDER BY s.created_at DESC LIMIT $1 OFFSET $2`
	if err := r.db.SelectContext(ctx, &sessions, r.db.Rebind(query), limit, offset); err != nil {
		return nil, 0, err
	}
	var total int
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM auth_sessions"); err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

func (r *TravelRepository) ListSessionIPs(ctx context.Context, sessionID, userID uuid.UUID) ([]model.SessionIP, error) {
	var items []model.SessionIP
	query := `SELECT si.session_id, si.ip, si.first_seen_at, si.last_seen_at, si.request_count
		FROM session_ips si JOIN auth_sessions s ON s.id = si.session_id
		WHERE si.session_id = $1 AND s.user_id = $2
		ORDER BY si.last_seen_at DESC`
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind(query), sessionID, userID); err != nil {
		return nil, err
	}
	return items, nil
}

// --- devices ---

func (r *TravelRepository) UpsertUserDevice(ctx context.Context, d *model.UserDevice) error {
	query := `INSERT INTO user_devices (user_id, fingerprint, name, platform, user_agent, first_ip, last_ip, login_count)
		VALUES ($1,$2,$3,$4,$5,$6,$7,1)
		ON CONFLICT (user_id, fingerprint) DO UPDATE SET
			name = EXCLUDED.name,
			platform = EXCLUDED.platform,
			user_agent = EXCLUDED.user_agent,
			last_ip = EXCLUDED.last_ip,
			last_seen_at = NOW(),
			login_count = user_devices.login_count + 1
		RETURNING id, first_ip, first_seen_at, last_seen_at, login_count, trusted, blocked`
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query),
		d.UserID, d.Fingerprint, d.Name, d.Platform, d.UserAgent, d.FirstIP, d.LastIP).
		Scan(&d.ID, &d.FirstIP, &d.FirstSeenAt, &d.LastSeenAt, &d.LoginCount, &d.Trusted, &d.Blocked)
}

func (r *TravelRepository) GetUserDevice(ctx context.Context, deviceID, userID uuid.UUID) (*model.UserDevice, error) {
	var d model.UserDevice
	query := `SELECT d.id, d.user_id, d.fingerprint, d.name, d.platform, d.user_agent, d.first_ip, d.last_ip,
		d.login_count, d.first_seen_at, d.last_seen_at, d.trusted, d.blocked,
		(SELECT COUNT(*) FROM auth_sessions s
		 WHERE s.device_id = d.id AND s.revoked_at IS NULL AND s.expires_at > NOW()) AS active_sessions
		FROM user_devices d WHERE d.id = $1 AND d.user_id = $2`
	if err := r.db.GetContext(ctx, &d, r.db.Rebind(query), deviceID, userID); err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *TravelRepository) ListUserDevices(ctx context.Context, userID uuid.UUID) ([]model.UserDevice, error) {
	var items []model.UserDevice
	query := `SELECT d.id, d.user_id, d.fingerprint, d.name, d.platform, d.user_agent, d.first_ip, d.last_ip,
		d.login_count, d.first_seen_at, d.last_seen_at, d.trusted, d.blocked,
		COUNT(s.id) FILTER (WHERE s.revoked_at IS NULL AND s.expires_at > NOW()) AS active_sessions
		FROM user_devices d
		LEFT JOIN auth_sessions s ON s.device_id = d.id
		WHERE d.user_id = $1
		GROUP BY d.id
		ORDER BY d.last_seen_at DESC`
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind(query), userID); err != nil {
		return nil, err
	}
	for i := range items {
		ips, _ := r.listDeviceIPs(ctx, items[i].ID)
		items[i].IPs = ips
		ids, _ := r.listDeviceSessionIDs(ctx, items[i].ID)
		items[i].SessionIDs = ids
	}
	return items, nil
}

func (r *TravelRepository) listDeviceIPs(ctx context.Context, deviceID uuid.UUID) ([]string, error) {
	var ips []string
	query := `SELECT DISTINCT si.ip FROM session_ips si
		JOIN auth_sessions s ON s.id = si.session_id
		WHERE s.device_id = $1 ORDER BY si.ip`
	if err := r.db.SelectContext(ctx, &ips, r.db.Rebind(query), deviceID); err != nil {
		return nil, err
	}
	return ips, nil
}

// ListDeviceIPs returns every IP seen for a device, scoped to its owner.
func (r *TravelRepository) ListDeviceIPs(ctx context.Context, deviceID, userID uuid.UUID) ([]string, error) {
	var ips []string
	query := `SELECT DISTINCT si.ip FROM session_ips si
		JOIN auth_sessions s ON s.id = si.session_id
		WHERE s.device_id = $1 AND s.user_id = $2 AND si.ip <> ''
		ORDER BY si.ip`
	if err := r.db.SelectContext(ctx, &ips, r.db.Rebind(query), deviceID, userID); err != nil {
		return nil, err
	}
	return ips, nil
}

func (r *TravelRepository) listDeviceSessionIDs(ctx context.Context, deviceID uuid.UUID) ([]string, error) {
	var ids []string
	query := `SELECT id::text FROM auth_sessions
		WHERE device_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC`
	if err := r.db.SelectContext(ctx, &ids, r.db.Rebind(query), deviceID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *TravelRepository) SetDeviceTrusted(ctx context.Context, deviceID, userID uuid.UUID, trusted bool) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(
		"UPDATE user_devices SET trusted = $1 WHERE id = $2 AND user_id = $3"), trusted, deviceID, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *TravelRepository) SetDeviceBlocked(ctx context.Context, deviceID, userID uuid.UUID, blocked bool) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(
		"UPDATE user_devices SET blocked = $1 WHERE id = $2 AND user_id = $3"), blocked, deviceID, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// DeleteUserDevice revokes all sessions that belong to the device and removes
// the device profile.
func (r *TravelRepository) DeleteUserDevice(ctx context.Context, deviceID, userID uuid.UUID) (bool, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE auth_sessions SET revoked_at = NOW()
		WHERE device_id = $1 AND user_id = $2 AND revoked_at IS NULL`), deviceID, userID); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	res, err := tx.ExecContext(ctx, r.db.Rebind("DELETE FROM user_devices WHERE id = $1 AND user_id = $2"), deviceID, userID)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

// --- security audit ---

func (r *TravelRepository) CreateAuthEvent(ctx context.Context, e *model.AuthEvent) error {
	query := `INSERT INTO auth_events
		(user_id, email, event, success, reason, session_id, device_id, ip, user_agent, platform, device_name, request_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at`
	return r.db.QueryRowxContext(ctx, r.db.Rebind(query),
		e.UserID, e.Email, e.Event, e.Success, e.Reason, e.SessionID, e.DeviceID,
		e.IP, e.UserAgent, e.Platform, e.DeviceName, e.RequestID, e.Metadata).
		Scan(&e.ID, &e.CreatedAt)
}

func (r *TravelRepository) ListAuthEvents(ctx context.Context, f *model.AuthEventFilter) ([]model.AuthEvent, int, error) {
	if f == nil {
		f = &model.AuthEventFilter{}
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var (
		where []string
		args  []interface{}
	)
	add := func(clause string, value interface{}) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(clause, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.UserID != nil {
		add("user_id = ?", *f.UserID)
	}
	if f.Email != "" {
		add("LOWER(email) = LOWER(?)", f.Email)
	}
	if f.Event != "" {
		add("event = ?", f.Event)
	}
	if f.IP != "" {
		add("ip = ?", f.IP)
	}
	if f.Success != nil {
		add("success = ?", *f.Success)
	}
	if f.From != nil {
		add("created_at >= ?", *f.From)
	}
	if f.To != nil {
		add("created_at <= ?", *f.To)
	}

	query := `SELECT id, user_id, email, event, success, reason, session_id, device_id, ip, user_agent,
		platform, device_name, request_id, metadata, created_at FROM auth_events`
	countQuery := "SELECT COUNT(*) FROM auth_events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
		countQuery += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)

	var total int
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, err
	}
	var events []model.AuthEvent
	if err := r.db.SelectContext(ctx, &events, r.db.Rebind(query), append(args, limit, offset)...); err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (r *TravelRepository) CountAuthEventsSince(ctx context.Context, since time.Time, event string, success *bool) (int, error) {
	query := "SELECT COUNT(*) FROM auth_events WHERE created_at >= $1"
	args := []interface{}{since}
	idx := 2
	if event != "" {
		query += " AND event = $" + strconv.Itoa(idx)
		args = append(args, event)
		idx++
	}
	if success != nil {
		query += " AND success = $" + strconv.Itoa(idx)
		args = append(args, *success)
	}
	var n int
	err := r.db.GetContext(ctx, &n, r.db.Rebind(query), args...)
	return n, err
}

func (r *TravelRepository) CountFailedLoginsSince(ctx context.Context, email string, since time.Time) (int, error) {
	var n int
	query := `SELECT COUNT(*) FROM auth_events
		WHERE LOWER(email) = LOWER($1) AND success = FALSE
		AND event IN ('login_failed','mfa_failed') AND created_at >= $2`
	err := r.db.GetContext(ctx, &n, r.db.Rebind(query), email, since)
	return n, err
}

func (r *TravelRepository) LastAuthEvent(ctx context.Context, userID uuid.UUID, events []string, success *bool) (*model.AuthEvent, error) {
	var e model.AuthEvent
	query := `SELECT id, user_id, email, event, success, reason, session_id, device_id, ip, user_agent,
		platform, device_name, request_id, metadata, created_at FROM auth_events
		WHERE user_id = $1`
	args := []interface{}{userID}
	idx := 2
	if len(events) > 0 {
		placeholders := make([]string, 0, len(events))
		for _, ev := range events {
			placeholders = append(placeholders, "$"+strconv.Itoa(idx))
			args = append(args, ev)
			idx++
		}
		query += " AND event IN (" + strings.Join(placeholders, ",") + ")"
	}
	if success != nil {
		query += " AND success = $" + strconv.Itoa(idx)
		args = append(args, *success)
	}
	query += " ORDER BY created_at DESC LIMIT 1"
	if err := r.db.GetContext(ctx, &e, r.db.Rebind(query), args...); err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *TravelRepository) ListRecentIPs(ctx context.Context, userID uuid.UUID, since time.Time, limit int) ([]model.IPCount, error) {
	var items []model.IPCount
	query := `SELECT ip, COUNT(*) AS count FROM auth_events
		WHERE user_id = $1 AND created_at >= $2 AND ip <> ''
		GROUP BY ip ORDER BY count DESC LIMIT $3`
	if err := r.db.SelectContext(ctx, &items, r.db.Rebind(query), userID, since, limit); err != nil {
		return nil, err
	}
	return items, nil
}

// SecuritySummary aggregates everything the account owner needs on one screen:
// devices, sessions, IPs, recent events and login counters.
func (r *TravelRepository) SecuritySummary(ctx context.Context, userID uuid.UUID) (*model.SecuritySummary, error) {
	out := &model.SecuritySummary{UniqueIPs: []string{}, RecentEvents: []model.AuthEvent{}, RecentIPs: []model.IPCount{}}

	_ = r.db.GetContext(ctx, &out.TotalDevices,
		r.db.Rebind("SELECT COUNT(*) FROM user_devices WHERE user_id = $1"), userID)
	_ = r.db.GetContext(ctx, &out.TrustedDevices,
		r.db.Rebind("SELECT COUNT(*) FROM user_devices WHERE user_id = $1 AND trusted = TRUE"), userID)
	_ = r.db.GetContext(ctx, &out.BlockedDevices,
		r.db.Rebind("SELECT COUNT(*) FROM user_devices WHERE user_id = $1 AND blocked = TRUE"), userID)
	_ = r.db.GetContext(ctx, &out.TotalSessions,
		r.db.Rebind("SELECT COUNT(*) FROM auth_sessions WHERE user_id = $1"), userID)
	_ = r.db.GetContext(ctx, &out.ActiveSessions,
		r.db.Rebind("SELECT COUNT(*) FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()"), userID)
	_ = r.db.GetContext(ctx, &out.Passkeys,
		r.db.Rebind("SELECT COUNT(*) FROM passkey_credentials WHERE user_id = $1"), userID)

	var blocked bool
	if err := r.db.GetContext(ctx, &blocked,
		r.db.Rebind("SELECT COALESCE(blocked, FALSE) FROM users WHERE id = $1"), userID); err == nil {
		out.AccountBlocked = blocked
	}
	var mfa bool
	if err := r.db.GetContext(ctx, &mfa,
		r.db.Rebind("SELECT COALESCE(mfa_enabled, FALSE) FROM users WHERE id = $1"), userID); err == nil {
		out.MFAEnabled = mfa
	}

	var lastLogin, lastFailed sql.NullTime
	_ = r.db.GetContext(ctx, &lastLogin, r.db.Rebind(`SELECT MAX(created_at) FROM auth_events
		WHERE user_id = $1 AND success = TRUE AND event IN ('login_success','mfa_success','passkey_login','oauth_login','register')`), userID)
	_ = r.db.GetContext(ctx, &lastFailed, r.db.Rebind(`SELECT MAX(created_at) FROM auth_events
		WHERE user_id = $1 AND success = FALSE AND event IN ('login_failed','mfa_failed')`), userID)
	if lastLogin.Valid {
		t := lastLogin.Time
		out.LastLoginAt = &t
	}
	if lastFailed.Valid {
		t := lastFailed.Time
		out.LastFailedLoginAt = &t
	}

	since := time.Now().Add(-24 * time.Hour)
	_ = r.db.GetContext(ctx, &out.SuccessfulLogins24h, r.db.Rebind(`SELECT COUNT(*) FROM auth_events
		WHERE user_id = $1 AND created_at >= $2 AND success = TRUE
		AND event IN ('login_success','mfa_success','passkey_login','oauth_login')`), userID, since)
	_ = r.db.GetContext(ctx, &out.FailedLogins24h, r.db.Rebind(`SELECT COUNT(*) FROM auth_events
		WHERE user_id = $1 AND created_at >= $2 AND success = FALSE
		AND event IN ('login_failed','mfa_failed')`), userID, since)

	var ips []string
	if err := r.db.SelectContext(ctx, &ips, r.db.Rebind(`SELECT DISTINCT ip FROM (
			SELECT ip FROM auth_events WHERE user_id = $1 AND ip <> ''
			UNION
			SELECT si.ip FROM session_ips si
			JOIN auth_sessions s ON s.id = si.session_id
			WHERE s.user_id = $1 AND si.ip <> ''
		) t ORDER BY ip`), userID); err == nil {
		out.UniqueIPs = ips
	}

	events, _, err := r.ListAuthEvents(ctx, &model.AuthEventFilter{UserID: &userID, Limit: 20})
	if err != nil {
		return nil, err
	}
	out.RecentEvents = events

	recentIPs, err := r.ListRecentIPs(ctx, userID, time.Now().Add(-30*24*time.Hour), 10)
	if err == nil {
		out.RecentIPs = recentIPs
	}
	return out, nil
}

func (r *TravelRepository) AdminSecurityStats(ctx context.Context, since time.Time) (*model.AdminSecurityStats, error) {
	stats := &model.AdminSecurityStats{}

	if err := r.db.GetContext(ctx, &stats.Events24h,
		r.db.Rebind("SELECT COUNT(*) FROM auth_events WHERE created_at >= $1"), since); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.SuccessfulLogins24h,
		r.db.Rebind("SELECT COUNT(*) FROM auth_events WHERE created_at >= $1 AND success = TRUE AND event IN ('login_success','mfa_success','passkey_login')"), since); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.FailedLogins24h,
		r.db.Rebind("SELECT COUNT(*) FROM auth_events WHERE created_at >= $1 AND success = FALSE AND event IN ('login_failed','mfa_failed')"), since); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.BlockedLogins24h,
		r.db.Rebind("SELECT COUNT(*) FROM auth_events WHERE created_at >= $1 AND event = 'login_blocked'"), since); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.UniqueIPs24h,
		r.db.Rebind("SELECT COUNT(DISTINCT ip) FROM auth_events WHERE created_at >= $1 AND ip <> ''"), since); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.ActiveSessions,
		r.db.Rebind("SELECT COUNT(*) FROM auth_sessions WHERE revoked_at IS NULL AND expires_at > NOW()")); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.TotalDevices,
		r.db.Rebind("SELECT COUNT(*) FROM user_devices")); err != nil {
		return nil, err
	}
	if err := r.db.GetContext(ctx, &stats.BlockedUsers,
		r.db.Rebind("SELECT COUNT(*) FROM users WHERE blocked = TRUE")); err != nil {
		return nil, err
	}

	topIPs := []model.IPCount{}
	if err := r.db.SelectContext(ctx, &topIPs, r.db.Rebind(`SELECT ip, COUNT(*) AS count FROM auth_events
		WHERE created_at >= $1 AND ip <> '' GROUP BY ip ORDER BY count DESC LIMIT 10`), since); err != nil {
		return nil, err
	}
	stats.TopIPs = topIPs

	topDevices := []model.DeviceCount{}
	if err := r.db.SelectContext(ctx, &topDevices, r.db.Rebind(`SELECT device_name AS device,
		COALESCE(platform,'') AS platform, COUNT(*) AS count FROM auth_events
		WHERE created_at >= $1 AND device_name <> ''
		GROUP BY device_name, platform ORDER BY count DESC LIMIT 10`), since); err != nil {
		return nil, err
	}
	stats.TopDevices = topDevices

	byDay := []model.DayCount{}
	if err := r.db.SelectContext(ctx, &byDay, r.db.Rebind(`SELECT
		to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS day,
		COUNT(*) FILTER (WHERE success) AS success,
		COUNT(*) FILTER (WHERE NOT success) AS failed
		FROM auth_events WHERE created_at >= $1
		GROUP BY 1 ORDER BY 1`), since); err != nil {
		return nil, err
	}
	stats.EventsByDay = byDay
	return stats, nil
}

type CleanupResult struct {
	RevokedSessions int64
	DeletedCodes    int64
	DeletedEvents   int64
	DeletedIPs      int64
}

func (r *TravelRepository) CleanupExpired(ctx context.Context, eventRetention time.Duration) (CleanupResult, error) {
	var out CleanupResult
	res, err := r.db.ExecContext(ctx, r.db.Rebind(
		"UPDATE auth_sessions SET revoked_at = NOW() WHERE revoked_at IS NULL AND expires_at < NOW()"))
	if err != nil {
		return out, err
	}
	out.RevokedSessions, _ = res.RowsAffected()

	res, err = r.db.ExecContext(ctx, r.db.Rebind(
		"DELETE FROM oauth_codes WHERE expires_at < NOW() - INTERVAL '1 day'"))
	if err != nil {
		return out, err
	}
	out.DeletedCodes, _ = res.RowsAffected()

	cutoff := time.Now().Add(-eventRetention)
	res, err = r.db.ExecContext(ctx, r.db.Rebind(
		"DELETE FROM auth_events WHERE created_at < $1"), cutoff)
	if err != nil {
		return out, err
	}
	out.DeletedEvents, _ = res.RowsAffected()

	res, err = r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM session_ips si
		USING auth_sessions s
		WHERE si.session_id = s.id AND s.expires_at < $1`), cutoff)
	if err != nil {
		return out, err
	}
	out.DeletedIPs, _ = res.RowsAffected()
	return out, nil
}

// --- OAuth codes ---

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

// --- MFA ---

func (r *TravelRepository) GetMFASecret(ctx context.Context, userID uuid.UUID) (*model.MFASecret, error) {
	var m model.MFASecret
	if err := r.db.GetContext(ctx, &m, r.db.Rebind(
		"SELECT user_id, secret, enabled, created_at, verified_at, last_step FROM user_mfa WHERE user_id = $1"), userID); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *TravelRepository) UpsertMFASecret(ctx context.Context, userID uuid.UUID, secret string) error {
	query := `INSERT INTO user_mfa (user_id, secret, enabled) VALUES ($1,$2,FALSE)
		ON CONFLICT (user_id) DO UPDATE SET secret = EXCLUDED.secret, enabled = FALSE, verified_at = NULL, last_step = 0`
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

// ConsumeTOTPStep atomically records a used TOTP time-step. It returns false if
// the same (or an older) step was already used, preventing code replay.
func (r *TravelRepository) ConsumeTOTPStep(ctx context.Context, userID uuid.UUID, step int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(
		"UPDATE user_mfa SET last_step = $1 WHERE user_id = $2 AND last_step < $1"), step, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// --- passkeys ---

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

func (r *TravelRepository) DeletePasskey(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind("DELETE FROM passkey_credentials WHERE id = $1 AND user_id = $2"), id, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *TravelRepository) TouchPasskey(ctx context.Context, id uuid.UUID) {
	_, _ = r.db.ExecContext(ctx, r.db.Rebind("UPDATE passkey_credentials SET last_used_at = NOW(), sign_count = sign_count + 1 WHERE id = $1"), id)
}

// --- ABAC ---

func (r *TravelRepository) SetTourOwner(ctx context.Context, tourID, ownerID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE tours SET owner_id = $1, updated_at = NOW() WHERE id = $2"), ownerID, tourID)
	return err
}

func (r *TravelRepository) GetToursByOwner(ctx context.Context, ownerID uuid.UUID, destID *uuid.UUID, available *bool, p *model.Pagination) ([]model.Tour, int, error) {
	where := "WHERE owner_id = $1"
	args := []interface{}{ownerID}
	idx := 2
	if destID != nil {
		where += " AND destination_id = $" + strconv.Itoa(idx)
		args = append(args, *destID)
		idx++
	}
	if available != nil {
		where += " AND available = $" + strconv.Itoa(idx)
		args = append(args, *available)
		idx++
	}
	var total int
	countQuery := "SELECT COUNT(*) FROM tours " + where
	if err := r.db.GetContext(ctx, &total, r.db.Rebind(countQuery), args...); err != nil {
		return nil, 0, err
	}
	query := "SELECT * FROM tours " + where + " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(idx) + " OFFSET $" + strconv.Itoa(idx+1)
	args = append(args, p.Limit(), p.Offset())
	var tours []model.Tour
	if err := r.db.SelectContext(ctx, &tours, r.db.Rebind(query), args...); err != nil {
		return nil, 0, err
	}
	return tours, total, nil
}

func (r *TravelRepository) UpdateOrderPartner(ctx context.Context, orderID, partnerID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind("UPDATE orders SET partner_id = $1, updated_at = NOW() WHERE id = $2"), partnerID, orderID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("order not found")
	}
	return nil
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
