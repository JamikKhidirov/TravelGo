package service

import (
	"context"
	"errors"
	"fmt"
	"os"
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
)

const zeroUUIDString = "00000000-0000-0000-0000-000000000000"

// AuthService implements password login, JWT access tokens, rotating refresh
// tokens, device sessions, TOTP MFA, passkeys and the OAuth2/OIDC code flow.
type AuthService struct {
	repo       *repository.TravelRepository
	jwtSecret  []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	idTTL      time.Duration
}

func NewAuthService(repo *repository.TravelRepository, jwtSecret string) *AuthService {
	issuer := os.Getenv("OAUTH_ISSUER")
	if issuer == "" {
		issuer = "gotravel"
	}
	return &AuthService{
		repo:       repo,
		jwtSecret:  []byte(jwtSecret),
		issuer:     issuer,
		accessTTL:  30 * time.Minute,
		refreshTTL: 30 * 24 * time.Hour,
		idTTL:      time.Hour,
	}
}

// Register creates a public account. Admin accounts are intentionally not
// self-service: only user, guide and partner roles can be requested.
func (s *AuthService) Register(ctx context.Context, req *model.LoginRequest, firstName, lastName, phone, role string, info *model.DeviceInfo) (*model.User, *model.TokenPair, error) {
	if role != "guide" && role != "partner" {
		role = "user"
	}
	if _, err := s.repo.GetUserByEmail(ctx, req.Email); err == nil {
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

	pair, err := s.newSession(ctx, u, info, "")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// Login validates the password. When MFA is enabled it returns ErrMFARequired
// together with the user so the caller can ask for a TOTP code.
func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest, info *model.DeviceInfo) (*model.User, *model.TokenPair, error) {
	u, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if u.MFAEnabled {
		return u, nil, ErrMFARequired
	}
	pair, err := s.newSession(ctx, u, info, "")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

// Authenticate validates email/password without creating a session. It is
// used by the OAuth authorize endpoint.
func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*model.User, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if u.MFAEnabled {
		return u, ErrMFARequired
	}
	return u, nil
}

// RevokeToken accepts either an access token or a refresh token and revokes
// the session it belongs to.
func (s *AuthService) RevokeToken(ctx context.Context, token string, userID uuid.UUID) error {
	if session, err := s.repo.GetSessionByRefreshHash(ctx, security.HashToken(token)); err == nil {
		if session.UserID == userID {
			return s.repo.RevokeSession(ctx, session.ID, userID)
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
	return s.repo.RevokeSession(ctx, sid, userID)
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
	if ok, err := s.VerifyTOTP(ctx, userID, code); err != nil || !ok {
		return nil, nil, errors.New("invalid mfa code")
	}
	pair, err := s.newSession(ctx, u, info, "")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

func (s *AuthService) newSession(ctx context.Context, u *model.User, info *model.DeviceInfo, nonce string) (*model.TokenPair, error) {
	if info == nil {
		info = &model.DeviceInfo{Name: "api", Platform: "web"}
	}
	refresh, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	session := &model.AuthSession{
		ID:               uuid.New(),
		UserID:           u.ID,
		RefreshTokenHash: security.HashToken(refresh),
		DeviceName:       firstNonEmpty(info.Name, "Unknown device"),
		Platform:         firstNonEmpty(info.Platform, "web"),
		UserAgent:        info.UserAgent,
		IP:               info.IP,
		ExpiresAt:        time.Now().Add(s.refreshTTL),
	}
	if err := s.repo.CreateAuthSession(ctx, session); err != nil {
		return nil, err
	}
	return s.buildTokens(u, session, refresh, nonce)
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

	return &model.TokenPair{
		AccessToken:  access,
		Token:        access,
		RefreshToken: refresh,
		IDToken:      idToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
		SessionID:    session.ID.String(),
	}, nil
}

// Refresh rotates the refresh token. The old token stops working immediately.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, info *model.DeviceInfo) (*model.TokenPair, error) {
	if refreshToken == "" {
		return nil, ErrInvalidToken
	}
	session, err := s.repo.GetSessionByRefreshHash(ctx, security.HashToken(refreshToken))
	if err != nil || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	u, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	next, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(s.refreshTTL)
	if err := s.repo.UpdateSessionRefresh(ctx, session.ID, security.HashToken(next), expires); err != nil {
		return nil, err
	}
	session.ExpiresAt = expires
	return s.buildTokens(u, session, next, "")
}

func (s *AuthService) Logout(ctx context.Context, sessionID, userID uuid.UUID) error {
	return s.repo.RevokeSession(ctx, sessionID, userID)
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
	return s.repo.RevokeSession(ctx, session.ID, userID)
}

// LogoutAll revokes every session of the user; when keep is not nil that
// session stays active (used by "logout other devices").
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID, keep *uuid.UUID) error {
	return s.repo.RevokeAllSessions(ctx, userID, keep)
}

func (s *AuthService) Sessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]model.AuthSession, error) {
	items, err := s.repo.ListSessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Current = items[i].ID == currentSessionID
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
	if err != nil {
		return nil, nil, ErrInvalidToken
	}
	return u, session, nil
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

func (s *AuthService) VerifyTOTP(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	m, err := s.repo.GetMFASecret(ctx, userID)
	if err != nil {
		return false, errors.New("mfa is not configured")
	}
	return security.ValidateTOTP(m.Secret, code, time.Now()), nil
}

func (s *AuthService) EnableTOTP(ctx context.Context, userID uuid.UUID, code string) error {
	ok, err := s.VerifyTOTP(ctx, userID, code)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("invalid mfa code")
	}
	return s.repo.SetMFAEnabled(ctx, userID, true)
}

func (s *AuthService) DisableTOTP(ctx context.Context, userID uuid.UUID, code string) error {
	ok, err := s.VerifyTOTP(ctx, userID, code)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("invalid mfa code")
	}
	return s.repo.SetMFAEnabled(ctx, userID, false)
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
	claims, err := security.ParseJWT(s.jwtSecret, challengeToken)
	if err != nil || security.StringClaim(claims, "typ") != "passkey_login" {
		return nil, nil, ErrInvalidToken
	}
	challenge := security.StringClaim(claims, "challenge")
	if clientDataJSON != "" && !security.ChallengeValid(clientDataJSON, challenge) {
		return nil, nil, errors.New("passkey challenge mismatch")
	}
	passkey, err := s.repo.GetPasskeyByCredentialID(ctx, credentialID)
	if err != nil || passkey.UserID.String() != security.StringClaim(claims, "sub") {
		return nil, nil, ErrInvalidCredentials
	}
	u, err := s.repo.GetUserByID(ctx, passkey.UserID)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	s.repo.TouchPasskey(ctx, passkey.ID)
	pair, err := s.newSession(ctx, u, info, "")
	if err != nil {
		return nil, nil, err
	}
	return u, pair, nil
}

func (s *AuthService) Passkeys(ctx context.Context, userID uuid.UUID) ([]model.PasskeyCredential, error) {
	return s.repo.GetPasskeysByUser(ctx, userID)
}

func (s *AuthService) DeletePasskey(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.DeletePasskey(ctx, id, userID)
}

// --- OAuth2 / OIDC ---

func (s *AuthService) AuthorizationCode(ctx context.Context, user *model.User, clientID, redirectURI, scope, challenge, method, nonce string) (string, error) {
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
	return code, nil
}

func (s *AuthService) ExchangeCode(ctx context.Context, code, clientID, redirectURI, verifier string, info *model.DeviceInfo) (*model.TokenPair, error) {
	c, err := s.repo.ConsumeOAuthCode(ctx, code)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if c.ClientID != clientID || c.RedirectURI != redirectURI {
		return nil, errors.New("client_id or redirect_uri mismatch")
	}
	if !security.VerifyPKCE(verifier, c.CodeChallenge) {
		return nil, errors.New("invalid code_verifier")
	}
	u, err := s.repo.GetUserByID(ctx, c.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return s.newSession(ctx, u, info, c.Nonce)
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
