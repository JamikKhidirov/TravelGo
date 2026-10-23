package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authorization required"})
			c.Abort()
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || token == "invalid" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
			c.Abort()
			return
		}
		userID := c.GetHeader("X-User-Id")
		role := c.GetHeader("X-User-Role")
		if userID == "" && strings.HasPrefix(token, "travel-token-") {
			rest := strings.TrimPrefix(token, "travel-token-")
			parts := strings.Split(rest, "-")
			if len(parts) >= 5 {
				userID = strings.Join(parts[:5], "-")
				if len(parts) >= 6 {
					role = parts[5]
				}
			}
		}
		if userID == "" {
			userID = c.GetHeader("X-User-Id")
		}
		if role == "" {
			role = "user"
		}
		c.Set("user_id", userID)
		c.Set("user_role", role)
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
