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
		// simplified: set mock user id from header X-User-Id or accept any bearer token
		c.Set("user_id", c.GetHeader("X-User-Id"))
		c.Set("user_role", c.GetHeader("X-User-Role"))
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
