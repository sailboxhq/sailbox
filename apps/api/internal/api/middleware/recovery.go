package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
)

// Recovery returns a middleware that recovers from panics and returns a 500 error.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered",
					slog.Any("error", err),
					slog.String("stack", string(debug.Stack())),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError,
					apierr.ErrInternal.WithDetail("An unexpected error occurred."),
				)
			}
		}()
		c.Next()
	}
}
