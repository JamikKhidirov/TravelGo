package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-travel/internal/repository"
	"go-travel/internal/security"
)

// AuthMiddleware validates signed access tokens, checks that the backing
// session has not been revoked and exposes user_id/user_role to handlers.
//
// For local development and scripts without a token it still accepts the
// legacy X-User-Id / X-User-Role pair. This fallback must not be used in
// production; put the service behind a trusted gateway if you keep it enabled.
type AuthMiddleware struct {
	secret       []byte
	repo         *repository.TravelRepository
	headerFallbk bool
}

func NewAuthMiddleware(secret []byte, repo *repository.TravelRepository) *AuthMiddleware {
	return &AuthMiddleware{secret: secret, repo: repo, headerFallbk: true}
}

func (m *AuthMiddleware) resolve(c *gin.Context) {
	if c.GetString("user_id") != "" {
		return
	}

	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token != "" {
			claims, err := security.ParseJWT(m.secret, token)
			if err == nil {
				typ := security.StringClaim(claims, "typ")
				if typ == "" || typ == "access" {
					if m.applyClaims(c, claims) {
						return
					}
				}
			}
		}
		// An explicit but invalid bearer token must never fall back to headers.
		return
	}

	if m.headerFallbk {
		uid := c.GetHeader("X-User-Id")
		if uid != "" {
			role := c.GetHeader("X-User-Role")
			if role == "" {
				role = "user"
			}
			c.Set("user_id", uid)
			c.Set("user_role", role)
			c.Set("auth_method", "header")
		}
	}
}

func (m *AuthMiddleware) applyClaims(c *gin.Context, claims security.Claims) bool {
	sub := security.StringClaim(claims, "sub")
	if sub == "" {
		return false
	}
	sidStr := security.StringClaim(claims, "sid")
	role := security.StringClaim(claims, "role")

	if m.repo != nil && sidStr != "" {
		sid, err := uuid.Parse(sidStr)
		if err != nil {
			return false
		}
		session, err := m.repo.GetAuthSession(c.Request.Context(), sid)
		if err != nil || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
			return false
		}
		c.Set("session_id", sidStr)
		// Keep "last seen" fresh without writing on every single request.
		if time.Since(session.LastSeenAt) > time.Minute {
			m.repo.TouchSession(c.Request.Context(), sid)
		}
		if uid, err := uuid.Parse(sub); err == nil {
			if u, err := m.repo.GetUserByID(c.Request.Context(), uid); err == nil {
				role = u.Role
			}
		}
	}

	c.Set("user_id", sub)
	c.Set("user_role", role)
	c.Set("auth_method", "jwt")
	return true
}

func (m *AuthMiddleware) Optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.resolve(c)
		c.Next()
	}
}

func (m *AuthMiddleware) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.resolve(c)
		if c.GetString("user_id") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authorization required"})
			return
		}
		c.Next()
	}
}

func (m *AuthMiddleware) RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		m.resolve(c)
		if c.GetString("user_id") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authorization required"})
			return
		}
		if _, ok := allowed[c.GetString("user_role")]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "insufficient role"})
			return
		}
		c.Next()
	}
}

// Legacy helpers kept for compatibility with existing route wiring.

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authorization required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("user_role")
		if role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "admin required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
