package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go-travel/internal/model"
	"go-travel/internal/repository"
	"go-travel/internal/security"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMFARequired        = errors.New("mfa required")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrAccountBlocked     = errors.New("account is blocked")
	ErrDeviceBlocked      = errors.New("device is blocked")
	ErrTooManyAttempts    = errors.New("too many failed login attempts, try again later")
	ErrInvalidMFACode     = errors.New("invalid mfa code")
)

const zeroUUIDString = "00000000-0000-0000-0000-000000000000"

// AuthOptions tunes brute-force protection and token lifetimes.
type AuthOptions struct {
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	MaxFailedLogins int
	LockoutWindow   time.Duration
}

// AuthService implements password login, JWT access tokens, rotating refresh
// tokens, device sessions, TOTP MFA, passkeys and the OAuth2/OIDC code flow.
// Every authentication attempt is written to the auth_events audit trail and
// every session records its device and the full IP history.
type AuthService struct {
	repo            *repository.TravelRepository
	jwtSecret       []byte
	issuer          string
	accessTTL       time.Duration
	refreshTTL      time.Duration
	idTTL           time.Duration
	maxFailedLogins int
	lockoutWindow   time.Duration
}

func NewAuthService(repo *repository.TravelRepository, jwtSecret string, opts ...AuthOptions) *AuthService {
	issuer := os.Getenv("OAUTH_ISSUER")
	if issuer == "" {
		issuer = "gotravel"
	}
	svc := &AuthService{
		repo:            repo,
		jwtSecret:       []byte(jwtSecret),
		issuer:          issuer,
		accessTTL:       30 * time.Minute,
		refreshTTL:      30 * 24 * time.Hour,
		idTTL:           time.Hour,
		maxFailedLogins: 5,
		lockoutWindow:   15 * time.Minute,
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.AccessTTL > 0 {
			svc.accessTTL = o.AccessTTL
		}
		if o.RefreshTTL > 0 {
			svc.refreshTTL = o.RefreshTTL
		}
		if o.MaxFailedLogins > 0 {
			svc.maxFailedLogins = o.MaxFailedLogins
		}
		if o.LockoutWindow > 0 {
			svc.lockoutWindow = o.LockoutWindow
		}
	}
	if v := os.Getenv("AUTH_MAX_FAILED_LOGINS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			svc.maxFailedLogins = n
		}
	}
	if v := os.Getenv("AUTH_LOCKOUT_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			svc.lockoutWindow = time.Duration(n) * time.Minute
		}
	}
	return svc
}

func normalizeDeviceInfo(info *model.DeviceInfo) *model.DeviceInfo {
	if info == nil {
		info = &model.DeviceInfo{}
	}
	info.Name = firstNonEmpty(info.Name, "Unknown device")
	info.Platform = security.NormalizePlatform(info.Platform)
	if info.Fingerprint == "" {
		info.Fingerprint = security.DeviceFingerprint(info.Name, info.Platform, info.UserAgent)
	}
	return info
}

// track writes one audit event. Audit failures never break the auth flow.
func (s *AuthService) track(ctx context.Context, info *model.DeviceInfo, u *model.User, email, event, reason string, success bool, sessionID, deviceID *uuid.UUID) {
	if info == nil {
		info = &model.DeviceInfo{}
	}
	e := &model.AuthEvent{
		Email:      email,
		Event:      event,
		Success:    success,
		Reason:     reason,
		SessionID:  sessionID,
		DeviceID:   deviceID,
		IP:         info.IP,
		UserAgent:  info.UserAgent,
		Platform:   info.Platform,
		DeviceName: info.Name,
		RequestID:  info.RequestID,
		Metadata:   model.JSONMap{},
	}
	if u != nil {
		uid := u.ID
		e.UserID = &uid
		if e.Email == "" {
			e.Email = u.Email
		}
	}
	_ = s.repo.CreateAuthEvent(ctx, e)
}

func (s *AuthService) lookupUser(ctx context.Context, email string) *model.User {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil
	}
	return u
}

func (s *AuthService) lockedOut(ctx context.Context, email string) bool {
	if s.maxFailedLogins <= 0 {
		return false
	}
	n, err := s.repo.CountFailedLoginsSince(ctx, email, time.Now().Add(-s.lockoutWindow))
	if err != nil {
		return false
	}
	return n >= s.maxFailedLogins
}

// Register creates a public account. Admin accounts are intentionally not
// self-service: only user, guide and partner roles can be requested.
func (s *AuthService) Register(ctx context.Context, req *model.LoginRequest, firstName, lastName, phone, role string, info *model.DeviceInfo) (*model.User, *model.TokenPair, error) {
	info = normalizeDeviceInfo(info)
	if role != "guide" && role != "partner" {
		role = "user"
	}
	if _, err := s.repo.GetUserByEmail(ctx, req.Email); err == nil {
		s.track(ctx, info, nil, req.Email, "register_failed", "user already exists", false, nil, nil)
		return nil, nil, errors.New("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	u := &model.User{
		ID:        uuid.New(),
		Email:     req.Email,
		Password:  string(hash),
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
		Role:      role,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, nil, err
	}

	pair, err := s.startSession(ctx, u, info, "", "register")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// Login validates the password. When MFA is enabled it returns ErrMFARequired
// together with the user so the caller can ask for a TOTP code.
func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest, info *model.DeviceInfo) (*model.User, *model.TokenPair, error) {
	info = normalizeDeviceInfo(info)

	if s.lockedOut(ctx, req.Email) {
		s.track(ctx, info, s.lookupUser(ctx, req.Email), req.Email, "login_blocked", "too many failed attempts", false, nil, nil)
		return nil, nil, ErrTooManyAttempts
	}

	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.track(ctx, info, nil, req.Email, "login_failed", "user not found", false, nil, nil)
		return nil, nil, ErrInvalidCredentials
	}
	if u.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", firstNonEmpty(u.BlockedReason, "account blocked"), false, nil, nil)
		return nil, nil, ErrAccountBlocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		s.track(ctx, info, u, u.Email, "login_failed", "invalid password", false, nil, nil)
		return nil, nil, ErrInvalidCredentials
	}
	if u.MFAEnabled {
		s.track(ctx, info, u, u.Email, "mfa_required", "password accepted, waiting for TOTP", true, nil, nil)
		return u, nil, ErrMFARequired
	}
	pair, err := s.startSession(ctx, u, info, "", "password")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// Authenticate validates email/password without creating a session. It is
// used by the OAuth authorize endpoint.
func (s *AuthService) Authenticate(ctx context.Context, email, password string, info *model.DeviceInfo) (*model.User, error) {
	info = normalizeDeviceInfo(info)
	if s.lockedOut(ctx, email) {
		s.track(ctx, info, s.lookupUser(ctx, email), email, "login_blocked", "too many failed attempts", false, nil, nil)
		return nil, ErrTooManyAttempts
	}
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		s.track(ctx, info, nil, email, "login_failed", "user not found", false, nil, nil)
		return nil, ErrInvalidCredentials
	}
	if u.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", "account blocked", false, nil, nil)
		return nil, ErrAccountBlocked
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		s.track(ctx, info, u, u.Email, "login_failed", "invalid password", false, nil, nil)
		return nil, ErrInvalidCredentials
	}
	if u.MFAEnabled {
		s.track(ctx, info, u, u.Email, "mfa_required", "password accepted, waiting for TOTP", true, nil, nil)
		return u, ErrMFARequired
	}
	return u, nil
}

// RevokeToken accepts either an access token or a refresh token and revokes
// the session it belongs to.
func (s *AuthService) RevokeToken(ctx context.Context, token string, userID uuid.UUID) error {
	if session, err := s.repo.GetSessionByRefreshHash(ctx, security.HashToken(token)); err == nil {
		if session.UserID == userID {
			ok, err := s.repo.RevokeSession(ctx, session.ID, userID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidToken
			}
			return nil
		}
		return ErrInvalidToken
	}
	claims, err := security.ParseJWT(s.jwtSecret, token)
	if err != nil {
		return ErrInvalidToken
	}
	sid, err := uuid.Parse(security.StringClaim(claims, "sid"))
	if err != nil {
		return ErrInvalidToken
	}
	ok, err := s.repo.RevokeSession(ctx, sid, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidToken
	}
	return nil
}

// IssueMFAToken returns a short-lived token that proves the password step and
// can only be used with /auth/login/mfa.
func (s *AuthService) IssueMFAToken(u *model.User) (string, error) {
	now := time.Now()
	return security.SignJWT(s.jwtSecret, security.Claims{
		"iss": s.issuer,
		"sub": u.ID.String(),
		"typ": "mfa",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})
}

func (s *AuthService) LoginMFA(ctx context.Context, mfaToken, code string, info *model.DeviceInfo) (*model.User, *model.TokenPair, error) {
	info = normalizeDeviceInfo(info)
	claims, err := security.ParseJWT(s.jwtSecret, mfaToken)
	if err != nil || security.StringClaim(claims, "typ") != "mfa" {
		return nil, nil, ErrInvalidToken
	}
	userID, err := uuid.Parse(security.StringClaim(claims, "sub"))
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	if u.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", "account blocked", false, nil, nil)
		return nil, nil, ErrAccountBlocked
	}
	if ok, err := s.VerifyTOTP(ctx, userID, code); err != nil || !ok {
		s.track(ctx, info, u, u.Email, "mfa_failed", "invalid or replayed TOTP code", false, nil, nil)
		return nil, nil, ErrInvalidMFACode
	}
	pair, err := s.startSession(ctx, u, info, "", "mfa")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// startSession upserts the device, creates the session, stores the initial IP
// and writes the success audit event.
func (s *AuthService) startSession(ctx context.Context, u *model.User, info *model.DeviceInfo, nonce, method string) (*model.TokenPair, error) {
	info = normalizeDeviceInfo(info)
	if u.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", firstNonEmpty(u.BlockedReason, "account blocked"), false, nil, nil)
		return nil, ErrAccountBlocked
	}

	device := &model.UserDevice{
		UserID:      u.ID,
		Fingerprint: info.Fingerprint,
		Name:        info.Name,
		Platform:    info.Platform,
		UserAgent:   info.UserAgent,
		FirstIP:     info.IP,
		LastIP:      info.IP,
	}
	if err := s.repo.UpsertUserDevice(ctx, device); err != nil {
		return nil, err
	}
	if device.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", "device is blocked", false, nil, &device.ID)
		return nil, ErrDeviceBlocked
	}

	refresh, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	session := &model.AuthSession{
		ID:               uuid.New(),
		UserID:           u.ID,
		RefreshTokenHash: security.HashToken(refresh),
		DeviceName:       info.Name,
		Platform:         info.Platform,
		UserAgent:        info.UserAgent,
		IP:               info.IP,
		LastIP:           info.IP,
		LastMethod:       method,
		DeviceID:         &device.ID,
		ExpiresAt:        time.Now().Add(s.refreshTTL),
	}
	if err := s.repo.CreateAuthSession(ctx, session); err != nil {
		return nil, err
	}
	// Records the first entry in session_ips.
	s.repo.TouchSession(ctx, session.ID, info.IP)
	_ = s.repo.UpdateUserLastLogin(ctx, u.ID)

	pair, err := s.buildTokens(u, session, refresh, nonce)
	if err != nil {
		return nil, err
	}
	pair.DeviceID = device.ID.String()
	pair.Scope = "openid profile email"

	event := "login_success"
	switch method {
	case "register":
		event = "register"
	case "mfa":
		event = "mfa_success"
	case "passkey":
		event = "passkey_login"
	case "oauth":
		event = "oauth_login"
	}
	s.track(ctx, info, u, u.Email, event, "", true, &session.ID, &device.ID)
	return pair, nil
}

func (s *AuthService) buildTokens(u *model.User, session *model.AuthSession, refresh, nonce string) (*model.TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(s.accessTTL)
	access, err := security.SignJWT(s.jwtSecret, security.Claims{
		"iss":   s.issuer,
		"sub":   u.ID.String(),
		"sid":   session.ID.String(),
		"role":  u.Role,
		"email": u.Email,
		"typ":   "access",
		"iat":   now.Unix(),
		"exp":   accessExp.Unix(),
		"jti":   security.NewJTI(),
	})
	if err != nil {
		return nil, err
	}

	idClaims := security.Claims{
		"iss":            s.issuer,
		"sub":            u.ID.String(),
		"aud":            "gotravel",
		"email":          u.Email,
		"email_verified": true,
		"name":           strings.TrimSpace(u.FirstName + " " + u.LastName),
		"given_name":     u.FirstName,
		"family_name":    u.LastName,
		"role":           u.Role,
		"typ":            "id",
		"iat":            now.Unix(),
		"exp":            now.Add(s.idTTL).Unix(),
	}
	if nonce != "" {
		idClaims["nonce"] = nonce
	}
	idToken, err := security.SignJWT(s.jwtSecret, idClaims)
	if err != nil {
		return nil, err
	}

	deviceID := ""
	if session.DeviceID != nil {
		deviceID = session.DeviceID.String()
	}
	return &model.TokenPair{
		AccessToken:  access,
		Token:        access,
		RefreshToken: refresh,
		IDToken:      idToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
		SessionID:    session.ID.String(),
		DeviceID:     deviceID,
		Scope:        "openid profile email",
	}, nil
}

// Refresh rotates the refresh token atomically: the old hash is part of the
// UPDATE predicate, so two concurrent refreshes can never both succeed.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, info *model.DeviceInfo) (*model.TokenPair, error) {
	info = normalizeDeviceInfo(info)
	if refreshToken == "" {
		return nil, ErrInvalidToken
	}
	oldHash := security.HashToken(refreshToken)
	session, err := s.repo.GetSessionByRefreshHash(ctx, oldHash)
	if err != nil || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		s.track(ctx, info, nil, "", "refresh_failed", "invalid or expired refresh token", false, nil, nil)
		return nil, ErrInvalidToken
	}
	u, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if u.Blocked {
		_, _ = s.repo.RevokeSession(ctx, session.ID, u.ID)
		s.track(ctx, info, u, u.Email, "login_blocked", "account blocked", false, &session.ID, session.DeviceID)
		return nil, ErrAccountBlocked
	}
	if session.DeviceID != nil {
		if d, err := s.repo.GetUserDevice(ctx, *session.DeviceID, u.ID); err == nil && d.Blocked {
			_, _ = s.repo.RevokeSession(ctx, session.ID, u.ID)
			s.track(ctx, info, u, u.Email, "login_blocked", "device is blocked", false, &session.ID, session.DeviceID)
			return nil, ErrDeviceBlocked
		}
	}

	next, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.refreshTTL)
	rotated, err := s.repo.UpdateSessionRefresh(ctx, session.ID, oldHash, security.HashToken(next), expires, info.IP)
	if err != nil {
		return nil, err
	}
	if !rotated {
		s.track(ctx, info, u, u.Email, "refresh_failed", "refresh token reuse detected", false, &session.ID, session.DeviceID)
		return nil, ErrInvalidToken
	}
	session.ExpiresAt = expires
	session.LastIP = info.IP
	pair, err := s.buildTokens(u, session, next, "")
	if err != nil {
		return nil, err
	}
	s.track(ctx, info, u, u.Email, "token_refresh", "", true, &session.ID, session.DeviceID)
	return pair, nil
}

// Logout revokes one session. It returns false when the session does not exist
// or belongs to another user, so the handler can answer 404.
func (s *AuthService) Logout(ctx context.Context, sessionID, userID uuid.UUID) (bool, error) {
	ok, err := s.repo.RevokeSession(ctx, sessionID, userID)
	if err == nil && ok {
		s.track(ctx, &model.DeviceInfo{}, nil, "", "logout", "", true, &sessionID, nil)
	}
	return ok, err
}

// LogoutByRefresh revokes the session that owns the supplied refresh token.
func (s *AuthService) LogoutByRefresh(ctx context.Context, refreshToken string, userID uuid.UUID) error {
	session, err := s.repo.GetSessionByRefreshHash(ctx, security.HashToken(refreshToken))
	if err != nil {
		return ErrInvalidToken
	}
	if session.UserID != userID {
		return ErrInvalidToken
	}
	if ok, err := s.repo.RevokeSession(ctx, session.ID, userID); err != nil || !ok {
		return ErrInvalidToken
	}
	s.track(ctx, &model.DeviceInfo{}, nil, "", "logout", "", true, &session.ID, session.DeviceID)
	return nil
}

// LogoutAll revokes every session of the user; when keep is not nil that
// session stays active (used by "logout other devices").
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID, keep *uuid.UUID) (int64, error) {
	n, err := s.repo.RevokeAllSessions(ctx, userID, keep)
	if err == nil {
		s.track(ctx, &model.DeviceInfo{}, nil, "", "logout_all", fmt.Sprintf("%d sessions revoked", n), true, keep, nil)
	}
	return n, err
}

func (s *AuthService) Sessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]model.AuthSession, error) {
	items, err := s.repo.ListSessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Current = items[i].ID == currentSessionID
		if ips, err := s.repo.ListSessionIPs(ctx, items[i].ID, userID); err == nil {
			for _, ip := range ips {
				items[i].IPs = append(items[i].IPs, ip.IP)
			}
		}
	}
	return items, nil
}

// ValidateAccessToken verifies the JWT signature, token type and the backing
// session. Revoked sessions make access tokens unusable immediately.
func (s *AuthService) ValidateAccessToken(ctx context.Context, token string) (*model.User, *model.AuthSession, error) {
	claims, err := security.ParseJWT(s.jwtSecret, token)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	if typ := security.StringClaim(claims, "typ"); typ != "" && typ != "access" {
		return nil, nil, ErrInvalidToken
	}
	userID, err := uuid.Parse(security.StringClaim(claims, "sub"))
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	sessionID, err := uuid.Parse(security.StringClaim(claims, "sid"))
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	session, err := s.repo.GetAuthSession(ctx, sessionID)
	if err != nil || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return nil, nil, ErrInvalidToken
	}
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || u.Blocked {
		return nil, nil, ErrInvalidToken
	}
	return u, session, nil
}

// --- security dashboard ---

func (s *AuthService) LoginHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.AuthEvent, int, error) {
	uid := userID
	return s.repo.ListAuthEvents(ctx, &model.AuthEventFilter{UserID: &uid, Limit: limit, Offset: offset})
}

func (s *AuthService) SecuritySummary(ctx context.Context, userID uuid.UUID) (*model.SecuritySummary, error) {
	return s.repo.SecuritySummary(ctx, userID)
}

func (s *AuthService) Devices(ctx context.Context, userID, currentSessionID uuid.UUID) ([]model.UserDevice, error) {
	items, err := s.repo.ListUserDevices(ctx, userID)
	if err != nil {
		return nil, err
	}
	current := currentSessionID.String()
	for i := range items {
		for _, sid := range items[i].SessionIDs {
			if sid == current {
				items[i].Current = true
				break
			}
		}
	}
	return items, nil
}

func (s *AuthService) Device(ctx context.Context, deviceID, userID uuid.UUID) (*model.UserDevice, error) {
	d, err := s.repo.GetUserDevice(ctx, deviceID, userID)
	if err != nil {
		return nil, err
	}
	if ips, err := s.repo.ListDeviceIPs(ctx, deviceID, userID); err == nil {
		d.IPs = ips
	}
	return d, nil
}

func (s *AuthService) TrustDevice(ctx context.Context, deviceID, userID uuid.UUID, trusted bool) (bool, error) {
	ok, err := s.repo.SetDeviceTrusted(ctx, deviceID, userID, trusted)
	if err == nil && ok {
		event := "device_untrusted"
		if trusted {
			event = "device_trusted"
		}
		s.track(ctx, &model.DeviceInfo{}, nil, "", event, "", true, nil, &deviceID)
	}
	return ok, err
}

func (s *AuthService) BlockDevice(ctx context.Context, deviceID, userID uuid.UUID, blocked bool) (bool, error) {
	ok, err := s.repo.SetDeviceBlocked(ctx, deviceID, userID, blocked)
	if err == nil && ok {
		event := "device_unblocked"
		if blocked {
			event = "device_blocked"
			_, _ = s.repo.RevokeSessionsByDevice(ctx, deviceID, userID)
		}
		s.track(ctx, &model.DeviceInfo{}, nil, "", event, "", true, nil, &deviceID)
	}
	return ok, err
}

func (s *AuthService) RemoveDevice(ctx context.Context, deviceID, userID uuid.UUID) (bool, error) {
	ok, err := s.repo.DeleteUserDevice(ctx, deviceID, userID)
	if err == nil && ok {
		s.track(ctx, &model.DeviceInfo{}, nil, "", "device_removed", "", true, nil, &deviceID)
	}
	return ok, err
}

func (s *AuthService) SessionIPs(ctx context.Context, sessionID, userID uuid.UUID) ([]model.SessionIP, error) {
	return s.repo.ListSessionIPs(ctx, sessionID, userID)
}

func (s *AuthService) LogoutOthers(ctx context.Context, userID uuid.UUID, keep uuid.UUID) (int64, error) {
	return s.LogoutAll(ctx, userID, &keep)
}

// --- TOTP (Google Authenticator compatible) ---

func (s *AuthService) SetupTOTP(ctx context.Context, userID uuid.UUID) (string, string, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	secret, err := security.GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	if err := s.repo.UpsertMFASecret(ctx, userID, secret); err != nil {
		return "", "", err
	}
	url := security.OTPAuthURL("GoTravel", u.Email, secret)
	return secret, url, nil
}

// VerifyTOTP validates a code and atomically consumes its time-step. A code
// cannot be reused within its 30-second window.
func (s *AuthService) VerifyTOTP(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	m, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return false, errors.New("mfa is not configured")
	}
	step, ok := security.MatchTOTPStep(m.Secret, code, time.Now())
	if !ok {
		return false, nil
	}
	accepted, err := s.repo.ConsumeTOTPStep(ctx, userID, step)
	if err != nil {
		return false, err
	}
	return accepted, nil
}

func (s *AuthService) EnableTOTP(ctx context.Context, userID uuid.UUID, code string) error {
	ok, err := s.VerifyTOTP(ctx, userID, code)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("invalid mfa code")
	}
	if err := s.repo.SetMFAEnabled(ctx, userID, true); err != nil {
		return err
	}
	s.track(ctx, &model.DeviceInfo{}, nil, "", "mfa_enabled", "", true, nil, nil)
	return nil
}

func (s *AuthService) DisableTOTP(ctx context.Context, userID uuid.UUID, code string) error {
	ok, err := s.VerifyTOTP(ctx, userID, code)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("invalid mfa code")
	}
	if err := s.repo.SetMFAEnabled(ctx, userID, false); err != nil {
		return err
	}
	s.track(ctx, &model.DeviceInfo{}, nil, "", "mfa_disabled", "", true, nil, nil)
	return nil
}

func (s *AuthService) MFAStatus(ctx context.Context, userID uuid.UUID) (bool, bool) {
	m, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return false, false
	}
	return m.Enabled, true
}

// --- Passkeys / WebAuthn ---

func (s *AuthService) BeginPasskeyRegistration(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	challenge, err := security.NewChallenge()
	if err != nil {
		return nil, err
	}
	token, err := security.SignJWT(s.jwtSecret, security.Claims{
		"iss": s.issuer, "sub": u.ID.String(), "typ": "passkey_reg",
		"challenge": challenge, "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	if err != nil {
		return nil, err
	}
	existing, _ := s.repo.GetPasskeysByUser(ctx, userID)
	exclude := make([]map[string]string, 0, len(existing))
	for _, p := range existing {
		exclude = append(exclude, map[string]string{"id": p.CredentialID, "type": "public-key"})
	}
	return map[string]interface{}{
		"challenge":          challenge,
		"challenge_token":    token,
		"rp":                 map[string]string{"name": "GoTravel", "id": "localhost"},
		"user":               map[string]string{"id": u.ID.String(), "name": u.Email, "displayName": strings.TrimSpace(u.FirstName + " " + u.LastName)},
		"pubKeyCredParams":   []map[string]interface{}{{"type": "public-key", "alg": -7}, {"type": "public-key", "alg": -257}},
		"timeout":            60000,
		"attestation":        "none",
		"excludeCredentials": exclude,
	}, nil
}

func (s *AuthService) FinishPasskeyRegistration(ctx context.Context, userID uuid.UUID, credentialID, clientDataJSON, attestation, transports, challengeToken string) (*model.PasskeyCredential, error) {
	claims, err := security.ParseJWT(s.jwtSecret, challengeToken)
	if err != nil || security.StringClaim(claims, "typ") != "passkey_reg" || security.StringClaim(claims, "sub") != userID.String() {
		return nil, ErrInvalidToken
	}
	challenge := security.StringClaim(claims, "challenge")
	if clientDataJSON != "" && !security.ChallengeValid(clientDataJSON, challenge) {
		return nil, errors.New("passkey challenge mismatch")
	}
	p := &model.PasskeyCredential{
		ID:           uuid.New(),
		UserID:       userID,
		CredentialID: credentialID,
		PublicKey:    attestation,
		Attestation:  attestation,
		Transports:   transports,
	}
	if err := s.repo.CreatePasskey(ctx, p); err != nil {
		return nil, err
	}
	s.track(ctx, &model.DeviceInfo{}, nil, "", "passkey_added", "", true, nil, nil)
	return p, nil
}

func (s *AuthService) BeginPasskeyLogin(ctx context.Context, email string) (map[string]interface{}, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	keys, err := s.repo.GetPasskeysByUser(ctx, u.ID)
	if err != nil || len(keys) == 0 {
		return nil, errors.New("no passkeys registered")
	}
	challenge, err := security.NewChallenge()
	if err != nil {
		return nil, err
	}
	token, err := security.SignJWT(s.jwtSecret, security.Claims{
		"iss": s.issuer, "sub": u.ID.String(), "typ": "passkey_login",
		"challenge": challenge, "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	if err != nil {
		return nil, err
	}
	allow := make([]map[string]string, 0, len(keys))
	for _, p := range keys {
		allow = append(allow, map[string]string{"id": p.CredentialID, "type": "public-key"})
	}
	return map[string]interface{}{
		"challenge":        challenge,
		"challenge_token":  token,
		"allowCredentials": allow,
		"timeout":          60000,
		"rpId":             "localhost",
	}, nil
}

func (s *AuthService) FinishPasskeyLogin(ctx context.Context, credentialID, clientDataJSON, challengeToken string, info *model.DeviceInfo) (*model.User, *model.TokenPair, error) {
	info = normalizeDeviceInfo(info)
	claims, err := security.ParseJWT(s.jwtSecret, challengeToken)
	if err != nil || security.StringClaim(claims, "typ") != "passkey_login" {
		s.track(ctx, info, nil, "", "passkey_login_failed", "invalid challenge token", false, nil, nil)
		return nil, nil, ErrInvalidToken
	}
	challenge := security.StringClaim(claims, "challenge")
	if clientDataJSON != "" && !security.ChallengeValid(clientDataJSON, challenge) {
		s.track(ctx, info, nil, "", "passkey_login_failed", "challenge mismatch", false, nil, nil)
		return nil, nil, errors.New("passkey challenge mismatch")
	}
	passkey, err := s.repo.GetPasskeyByCredentialID(ctx, credentialID)
	if err != nil || passkey.UserID.String() != security.StringClaim(claims, "sub") {
		s.track(ctx, info, nil, "", "passkey_login_failed", "unknown credential", false, nil, nil)
		return nil, nil, ErrInvalidCredentials
	}
	u, err := s.repo.GetUserByID(ctx, passkey.UserID)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if u.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", "account blocked", false, nil, nil)
		return nil, nil, ErrAccountBlocked
	}
	s.repo.TouchPasskey(ctx, passkey.ID)
	pair, err := s.startSession(ctx, u, info, "", "passkey")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

func (s *AuthService) Passkeys(ctx context.Context, userID uuid.UUID) ([]model.PasskeyCredential, error) {
	return s.repo.GetPasskeysByUser(ctx, userID)
}

func (s *AuthService) DeletePasskey(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	ok, err := s.repo.DeletePasskey(ctx, id, userID)
	if err == nil && ok {
		s.track(ctx, &model.DeviceInfo{}, nil, "", "passkey_removed", "", true, nil, nil)
	}
	return ok, err
}

// --- OAuth2 / OIDC ---

func (s *AuthService) AuthorizationCode(ctx context.Context, user *model.User, clientID, redirectURI, scope, challenge, method, nonce string, info *model.DeviceInfo) (string, error) {
	if clientID == "" || redirectURI == "" {
		return "", errors.New("client_id and redirect_uri are required")
	}
	if challenge == "" {
		return "", errors.New("PKCE code_challenge is required")
	}
	if method == "" {
		method = "S256"
	}
	if method != "S256" {
		return "", errors.New("only S256 PKCE is supported")
	}
	code, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	if scope == "" {
		scope = "openid profile email"
	}
	if err := s.repo.CreateOAuthCode(ctx, &model.OAuthCode{
		Code:                code,
		UserID:              user.ID,
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
		Nonce:               nonce,
		ExpiresAt:           time.Now().Add(5 * time.Minute),
	}); err != nil {
		return "", err
	}
	s.track(ctx, normalizeDeviceInfo(info), user, user.Email, "oauth_code_issued", "client "+clientID, true, nil, nil)
	return code, nil
}

func (s *AuthService) ExchangeCode(ctx context.Context, code, clientID, redirectURI, verifier string, info *model.DeviceInfo) (*model.TokenPair, error) {
	info = normalizeDeviceInfo(info)
	c, err := s.repo.ConsumeOAuthCode(ctx, code)
	if err != nil {
		s.track(ctx, info, nil, "", "oauth_code_failed", "invalid or expired code", false, nil, nil)
		return nil, ErrInvalidToken
	}
	if c.ClientID != clientID || c.RedirectURI != redirectURI {
		s.track(ctx, info, nil, "", "oauth_code_failed", "client_id or redirect_uri mismatch", false, nil, nil)
		return nil, errors.New("client_id or redirect_uri mismatch")
	}
	if !security.VerifyPKCE(verifier, c.CodeChallenge) {
		s.track(ctx, info, nil, "", "oauth_code_failed", "invalid code_verifier", false, nil, nil)
		return nil, errors.New("invalid code_verifier")
	}
	u, err := s.repo.GetUserByID(ctx, c.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if u.Blocked {
		s.track(ctx, info, u, u.Email, "login_blocked", "account blocked", false, nil, nil)
		return nil, ErrAccountBlocked
	}
	pair, err := s.startSession(ctx, u, info, c.Nonce, "oauth")
	if err != nil {
		return nil, err
	}
	pair.Scope = c.Scope
	return pair, nil
}

func (s *AuthService) UserInfo(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

func (s *AuthService) Issuer() string {
	return s.issuer
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
