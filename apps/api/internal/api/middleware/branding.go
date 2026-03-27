package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/version"
)

// AGPL v3 Section 7(b) — see NOTICE. DO NOT REMOVE.
func Branding() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Powered-By", version.Name)
		c.Next()
	}
}
