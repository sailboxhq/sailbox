package httputil

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
)

// ============================================================================
// Pagination
// ============================================================================

// Pagination holds pagination metadata for list responses.
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ListResponse is a generic paginated list response.
type ListResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// NewListResponse creates a paginated list response.
func NewListResponse[T any](items []T, page, perPage, total int) ListResponse[T] {
	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}
	if items == nil {
		items = []T{}
	}
	return ListResponse[T]{
		Items: items,
		Pagination: Pagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// ============================================================================
// Success Helpers
// ============================================================================

func RespondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// RespondList responds with a slice, ensuring nil is serialized as [] not null.
func RespondList[T any](c *gin.Context, items []T) {
	if items == nil {
		items = []T{}
	}
	c.JSON(http.StatusOK, items)
}

func RespondCreated(c *gin.Context, data any, location string) {
	if location != "" {
		c.Header("Location", location)
	}
	c.JSON(http.StatusCreated, data)
}

func RespondAccepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, data)
}

func RespondNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// ============================================================================
// Error Helper
// ============================================================================

func RespondError(c *gin.Context, err error) {
	if pd, ok := err.(*apierr.ProblemDetail); ok {
		c.JSON(pd.Status, pd)
		return
	}
	// Log the actual error for debugging — never expose internals to client
	slog.Error("unhandled error",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.Any("error", err),
	)
	c.JSON(http.StatusInternalServerError, apierr.ErrInternal.WithDetail("An unexpected error occurred."))
}
