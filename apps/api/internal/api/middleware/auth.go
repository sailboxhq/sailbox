package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/auth"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
)

// RateLimit returns middleware that limits requests per IP.
// maxAttempts per window duration.
func RateLimit(maxAttempts int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	attempts := make(map[string]*entry)

	// Cleanup old entries periodically
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for k, e := range attempts {
				if now.After(e.reset) {
					delete(attempts, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		e, ok := attempts[ip]
		if !ok || time.Now().After(e.reset) {
			attempts[ip] = &entry{count: 1, reset: time.Now().Add(window)}
			mu.Unlock()
			c.Next()
			return
		}
		e.count++
		if e.count > maxAttempts {
			mu.Unlock()
			httputil.RespondError(c, apierr.ErrTooManyRequests.WithDetail("too many attempts, try again later"))
			c.Abort()
			return
		}
		mu.Unlock()
		c.Next()
	}
}

const (
	// Context keys for user info
	CtxUserID = "user_id"
	CtxOrgID  = "org_id"
	CtxRole   = "role"
)

// Auth returns a middleware that validates JWT tokens.
func Auth(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("missing authorization header"))
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail("invalid authorization format"))
			return
		}

		claims, err := jwtManager.ValidateAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(401, apierr.ErrUnauthorized.WithDetail(err.Error()))
			return
		}

		// Store user info in context
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxOrgID, claims.OrgID)
		c.Set(CtxRole, claims.Role)

		c.Next()
	}
}

// GetUserID extracts user ID from context.
func GetUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(CtxUserID); ok {
		return v.(uuid.UUID)
	}
	return uuid.Nil
}

// GetOrgID extracts organization ID from context.
func GetOrgID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(CtxOrgID); ok {
		return v.(uuid.UUID)
	}
	return uuid.Nil
}

// GetUserRole extracts user role from context.
func GetUserRole(c *gin.Context) string {
	if v, ok := c.Get(CtxRole); ok {
		return v.(string)
	}
	return ""
}

// RequireRole returns a middleware that restricts access to users with one of the given roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetUserRole(c)
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, map[string]any{
			"type":   "https://sailbox.dev/errors/forbidden",
			"title":  "Forbidden",
			"status": 403,
			"detail": "insufficient permissions",
		})
	}
}
