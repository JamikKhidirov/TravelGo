package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/middleware"
	"go-travel/internal/model"
	"go-travel/internal/service"
)

// AuthHandler exposes registration, login, MFA, passkey, session and OAuth2
// endpoints.
type AuthHandler struct {
	authSvc *service.AuthService
}

func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup, mw *middleware.AuthMiddleware) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", h.register)
		auth.POST("/login", h.login)
		auth.POST("/login/mfa", h.loginMFA)
		auth.POST("/refresh", h.refresh)
		auth.POST("/passkeys/login/begin", h.passkeyLoginBegin)
		auth.POST("/passkeys/login/finish", h.passkeyLoginFinish)

		protected := auth.Group("", mw.Required())
		{
			protected.GET("/me", h.me)
			protected.GET("/profile", h.profile)
			protected.PUT("/profile", h.updateProfile)
			protected.POST("/change-password", h.changePassword)
			protected.POST("/logout", h.logout)
			protected.POST("/logout-all", h.logoutAll)
			protected.GET("/sessions", h.sessions)
			protected.GET("/devices", h.sessions)
			protected.DELETE("/sessions/:id", h.revokeSession)
			protected.GET("/mfa/status", h.mfaStatus)
			protected.POST("/mfa/setup", h.mfaSetup)
			protected.POST("/mfa/enable", h.mfaEnable)
			protected.POST("/mfa/disable", h.mfaDisable)
			protected.POST("/passkeys/register/begin", h.passkeyRegisterBegin)
			protected.POST("/passkeys/register/finish", h.passkeyRegisterFinish)
			protected.GET("/passkeys", h.passkeys)
			protected.DELETE("/passkeys/:id", h.deletePasskey)
		}
	}

	oauth := r.Group("/oauth")
	{
		oauth.POST("/authorize", h.oauthAuthorize)
		oauth.GET("/authorize", h.oauthAuthorize)
		oauth.POST("/token", h.oauthToken)
		oauth.POST("/revoke", mw.Required(), h.oauthRevoke)
		oauth.GET("/userinfo", mw.Required(), h.oauthUserInfo)
		oauth.POST("/userinfo", mw.Required(), h.oauthUserInfo)
		oauth.GET("/jwks", h.oauthJWKS)
	}
	r.GET("/.well-known/openid-configuration", h.oidcDiscovery)
}

type registerRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Phone     string `json:"phone"`
	Role      string `json:"role" binding:"omitempty,oneof=user guide partner"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type mfaLoginRequest struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

func deviceInfo(c *gin.Context) *model.DeviceInfo {
	ua := c.Request.UserAgent()
	platform := c.GetHeader("X-Device-Platform")
	if platform == "" {
		platform = detectPlatform(ua)
	}
	name := c.GetHeader("X-Device-Name")
	if name == "" {
		name = friendlyDeviceName(ua)
	}
	return &model.DeviceInfo{
		Name:      name,
		Platform:  platform,
		UserAgent: ua,
		IP:        c.ClientIP(),
	}
}

func detectPlatform(ua string) string {
	low := strings.ToLower(ua)
	switch {
	case strings.Contains(low, "iphone"), strings.Contains(low, "ipad"), strings.Contains(low, "ios"):
		return "ios"
	case strings.Contains(low, "android"):
		return "android"
	default:
		return "web"
	}
}

func friendlyDeviceName(ua string) string {
	low := strings.ToLower(ua)
	switch {
	case strings.Contains(low, "iphone"):
		return "iPhone"
	case strings.Contains(low, "ipad"):
		return "iPad"
	case strings.Contains(low, "android"):
		return "Android device"
	case strings.Contains(low, "windows"):
		return "Windows device"
	case strings.Contains(low, "mac os"), strings.Contains(low, "macintosh"):
		return "macOS device"
	case strings.Contains(low, "linux"):
		return "Linux device"
	case ua == "":
		return "API client"
	default:
		return "Unknown device"
	}
}

func authResponse(u *model.User, pair *model.TokenPair) gin.H {
	return gin.H{
		"user":   u,
		"token":  pair.AccessToken, // backwards compatible field
		"tokens": pair,
	}
}

func (h *AuthHandler) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	role := req.Role
	if role == "" {
		role = "user"
	}
	u, pair, err := h.authSvc.Register(c.Request.Context(), &model.LoginRequest{Email: req.Email, Password: req.Password}, req.FirstName, req.LastName, req.Phone, role, deviceInfo(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: authResponse(u, pair)})
}

func (h *AuthHandler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	u, pair, err := h.authSvc.Login(c.Request.Context(), &model.LoginRequest{Email: req.Email, Password: req.Password}, deviceInfo(c))
	if errors.Is(err, service.ErrMFARequired) {
		mfaToken, tokenErr := h.authSvc.IssueMFAToken(u)
		if tokenErr != nil {
			c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: tokenErr.Error()})
			return
		}
		c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
			"mfa_required": true,
			"mfa_token":    mfaToken,
			"user_id":      u.ID,
		}})
		return
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: authResponse(u, pair)})
}

func (h *AuthHandler) loginMFA(c *gin.Context) {
	var req mfaLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	u, pair, err := h.authSvc.LoginMFA(c.Request.Context(), req.MFAToken, req.Code, deviceInfo(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: authResponse(u, pair)})
}

func (h *AuthHandler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.RefreshToken == "" {
		req.RefreshToken = c.GetHeader("X-Refresh-Token")
	}
	pair, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken, deviceInfo(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: pair})
}

func (h *AuthHandler) logout(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	if sid, ok := currentSessionID(c); ok {
		if err := h.authSvc.Logout(c.Request.Context(), sid, userID); err != nil {
			c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "logged out"}})
		return
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.authSvc.LogoutByRefresh(c.Request.Context(), req.RefreshToken, userID); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "logged out"}})
}

func (h *AuthHandler) logoutAll(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	var keep *uuid.UUID
	if c.Query("keep_current") == "true" {
		if sid, ok := currentSessionID(c); ok {
			keep = &sid
		}
	}
	if err := h.authSvc.LogoutAll(c.Request.Context(), userID, keep); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "all sessions revoked"}})
}

func (h *AuthHandler) sessions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	cur, _ := currentSessionID(c)
	items, err := h.authSvc.Sessions(c.Request.Context(), userID, cur)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}

	sessions := make([]gin.H, 0, len(items))
	platforms := map[string]int{}
	ips := []string{}
	active := 0
	var lastLogin, lastActivity time.Time
	for _, s := range items {
		isActive := s.RevokedAt == nil && time.Now().Before(s.ExpiresAt)
		if isActive {
			active++
			platforms[s.Platform]++
			ipSeen := false
			for _, ip := range ips {
				if ip == s.IP {
					ipSeen = true
					break
				}
			}
			if !ipSeen && s.IP != "" {
				ips = append(ips, s.IP)
			}
		}
		if s.CreatedAt.After(lastLogin) {
			lastLogin = s.CreatedAt
		}
		if s.LastSeenAt.After(lastActivity) {
			lastActivity = s.LastSeenAt
		}
		sessions = append(sessions, gin.H{
			"id":           s.ID,
			"session_id":   s.ID,
			"device":       s.DeviceName,
			"device_name":  s.DeviceName,
			"platform":     s.Platform,
			"ip":           s.IP,
			"user_agent":   s.UserAgent,
			"login_at":     s.CreatedAt,
			"last_seen_at": s.LastSeenAt,
			"expires_at":   s.ExpiresAt,
			"revoked_at":   s.RevokedAt,
			"is_current":   s.Current,
			"is_active":    isActive,
		})
	}

	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"sessions": sessions,
		"summary": gin.H{
			"total_sessions":     len(items),
			"active_sessions":    active,
			"platforms":          platforms,
			"unique_ips":         ips,
			"last_login_at":      lastLogin,
			"last_activity_at":   lastActivity,
			"current_session_id": c.GetString("session_id"),
		},
	}})
}

func (h *AuthHandler) revokeSession(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	sid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid session id"})
		return
	}
	if err := h.authSvc.Logout(c.Request.Context(), sid, userID); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "session revoked"}})
}

func (h *AuthHandler) me(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	u, err := h.authSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"user":       u,
		"user_id":    u.ID,
		"role":       u.Role,
		"session_id": c.GetString("session_id"),
	}})
}

func (h *AuthHandler) profile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	u, err := h.authSvc.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: u})
}

func (h *AuthHandler) updateProfile(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	u, err := h.authSvc.UpdateProfile(c.Request.Context(), userID, req.FirstName, req.LastName, req.Phone)
	if err != nil {
		c.JSON(http.StatusNotFound, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: u})
}

func (h *AuthHandler) changePassword(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "password changed"}})
}

// --- MFA ---

func (h *AuthHandler) mfaStatus(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	enabled, configured := h.authSvc.MFAStatus(c.Request.Context(), userID)
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"enabled":    enabled,
		"configured": configured,
		"type":       "totp",
		"issuer":     "GoTravel",
	}})
}

func (h *AuthHandler) mfaSetup(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	secret, url, err := h.authSvc.SetupTOTP(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"secret":       secret,
		"otpauth_url":  url,
		"issuer":       "GoTravel",
		"digits":       6,
		"period":       30,
		"algorithm":    "SHA1",
		"instructions": "Add this secret to Google Authenticator, then call /auth/mfa/enable with the 6-digit code.",
	}})
}

func (h *AuthHandler) mfaEnable(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.authSvc.EnableTOTP(c.Request.Context(), userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"enabled": true, "message": "mfa enabled"}})
}

func (h *AuthHandler) mfaDisable(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.authSvc.DisableTOTP(c.Request.Context(), userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"enabled": false, "message": "mfa disabled"}})
}

// --- Passkeys ---

func (h *AuthHandler) passkeyRegisterBegin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	options, err := h.authSvc.BeginPasskeyRegistration(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: options})
}

func (h *AuthHandler) passkeyRegisterFinish(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	var req struct {
		CredentialID      string `json:"credential_id" binding:"required"`
		ClientDataJSON    string `json:"client_data_json"`
		AttestationObject string `json:"attestation_object"`
		Transports        string `json:"transports"`
		ChallengeToken    string `json:"challenge_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	p, err := h.authSvc.FinishPasskeyRegistration(c.Request.Context(), userID, req.CredentialID, req.ClientDataJSON, req.AttestationObject, req.Transports, req.ChallengeToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, model.Response{Success: true, Data: p})
}

func (h *AuthHandler) passkeyLoginBegin(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	options, err := h.authSvc.BeginPasskeyLogin(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: options})
}

func (h *AuthHandler) passkeyLoginFinish(c *gin.Context) {
	var req struct {
		CredentialID   string `json:"credential_id" binding:"required"`
		ClientDataJSON string `json:"client_data_json"`
		ChallengeToken string `json:"challenge_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	u, pair, err := h.authSvc.FinishPasskeyLogin(c.Request.Context(), req.CredentialID, req.ClientDataJSON, req.ChallengeToken, deviceInfo(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: authResponse(u, pair)})
}

func (h *AuthHandler) passkeys(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	items, err := h.authSvc.Passkeys(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: items})
}

func (h *AuthHandler) deletePasskey(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "invalid passkey id"})
		return
	}
	if err := h.authSvc.DeletePasskey(c.Request.Context(), userID, id); err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{"message": "passkey deleted"}})
}

// --- OAuth2 / OIDC ---

type oauthAuthorizeRequest struct {
	ResponseType        string `json:"response_type" form:"response_type"`
	ClientID            string `json:"client_id" form:"client_id" binding:"required"`
	RedirectURI         string `json:"redirect_uri" form:"redirect_uri" binding:"required"`
	Scope               string `json:"scope" form:"scope"`
	State               string `json:"state" form:"state"`
	CodeChallenge       string `json:"code_challenge" form:"code_challenge" binding:"required"`
	CodeChallengeMethod string `json:"code_challenge_method" form:"code_challenge_method"`
	Nonce               string `json:"nonce" form:"nonce"`
	Email               string `json:"email" form:"email"`
	Password            string `json:"password" form:"password"`
}

func (h *AuthHandler) oauthAuthorize(c *gin.Context) {
	var req oauthAuthorizeRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	if req.ResponseType != "" && req.ResponseType != "code" {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "only response_type=code is supported"})
		return
	}
	if req.CodeChallengeMethod != "" && req.CodeChallengeMethod != "S256" {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: "only S256 PKCE is supported"})
		return
	}

	var user *model.User
	if uid, ok := currentUserID(c); ok {
		u, err := h.authSvc.GetUser(c.Request.Context(), uid)
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "invalid user"})
			return
		}
		user = u
	} else if req.Email != "" && req.Password != "" {
		u, err := h.authSvc.Authenticate(c.Request.Context(), req.Email, req.Password)
		if errors.Is(err, service.ErrMFARequired) {
			c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "mfa required; complete /auth/login/mfa first"})
			return
		}
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: err.Error()})
			return
		}
		user = u
	} else {
		c.JSON(http.StatusUnauthorized, model.Response{Success: false, Error: "authorization required or provide email/password"})
		return
	}

	code, err := h.authSvc.AuthorizationCode(c.Request.Context(), user, req.ClientID, req.RedirectURI, req.Scope, req.CodeChallenge, req.CodeChallengeMethod, req.Nonce)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, model.Response{Success: true, Data: gin.H{
		"code":           code,
		"state":          req.State,
		"redirect_uri":   req.RedirectURI,
		"expires_in":     300,
		"token_endpoint": "/api/v1/oauth/token",
	}})
}

type oauthTokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type" binding:"required"`
	Code         string `json:"code" form:"code"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	ClientID     string `json:"client_id" form:"client_id"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

func (h *AuthHandler) oauthToken(c *gin.Context) {
	var req oauthTokenRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": err.Error()})
		return
	}
	switch req.GrantType {
	case "authorization_code":
		if req.Code == "" || req.ClientID == "" || req.RedirectURI == "" || req.CodeVerifier == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "code, client_id, redirect_uri and code_verifier are required"})
			return
		}
		pair, err := h.authSvc.ExchangeCode(c.Request.Context(), req.Code, req.ClientID, req.RedirectURI, req.CodeVerifier, deviceInfo(c))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"access_token":  pair.AccessToken,
			"token_type":    "Bearer",
			"expires_in":    pair.ExpiresIn,
			"refresh_token": pair.RefreshToken,
			"id_token":      pair.IDToken,
			"session_id":    pair.SessionID,
			"scope":         "openid profile email",
		})
	case "refresh_token":
		pair, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken, deviceInfo(c))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"access_token":  pair.AccessToken,
			"token_type":    "Bearer",
			"expires_in":    pair.ExpiresIn,
			"refresh_token": pair.RefreshToken,
			"id_token":      pair.IDToken,
			"session_id":    pair.SessionID,
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
	}
}

func (h *AuthHandler) oauthUserInfo(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	u, err := h.authSvc.UserInfo(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	enabled, _ := h.authSvc.MFAStatus(c.Request.Context(), u.ID)
	c.JSON(http.StatusOK, gin.H{
		"sub":            u.ID.String(),
		"email":          u.Email,
		"email_verified": true,
		"name":           strings.TrimSpace(u.FirstName + " " + u.LastName),
		"given_name":     u.FirstName,
		"family_name":    u.LastName,
		"phone":          u.Phone,
		"role":           u.Role,
		"mfa_enabled":    enabled,
	})
}

func (h *AuthHandler) oauthRevoke(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	var req struct {
		Token string `json:"token" form:"token"`
	}
	if err := c.ShouldBind(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	if err := h.authSvc.RevokeToken(c.Request.Context(), req.Token, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *AuthHandler) oauthJWKS(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"keys": []interface{}{},
		"note": "GoTravel signs tokens with HS256 and a shared server secret; JWKS is not applicable.",
	})
}

func (h *AuthHandler) oidcDiscovery(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	base := scheme + "://" + c.Request.Host
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                h.authSvc.Issuer(),
		"authorization_endpoint":                base + "/api/v1/oauth/authorize",
		"token_endpoint":                        base + "/api/v1/oauth/token",
		"userinfo_endpoint":                     base + "/api/v1/oauth/userinfo",
		"revocation_endpoint":                   base + "/api/v1/oauth/revoke",
		"jwks_uri":                              base + "/api/v1/oauth/jwks",
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access"},
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"HS256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
		"claims_supported":                      []string{"sub", "email", "email_verified", "name", "given_name", "family_name", "role", "mfa_enabled"},
		"created_at":                            time.Now().UTC(),
	})
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	s := c.GetString("user_id")
	if s == "" {
		s = userIDFromContext(c)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func currentSessionID(c *gin.Context) (uuid.UUID, bool) {
	s := c.GetString("session_id")
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
