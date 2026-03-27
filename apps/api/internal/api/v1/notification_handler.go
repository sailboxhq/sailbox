package v1

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) ListChannels(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	channels, err := h.svc.ListChannels(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, channels)
}

func (h *NotificationHandler) SaveChannel(c *gin.Context) {
	var input struct {
		Type    string          `json:"type" binding:"required"`
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.SaveChannel(c.Request.Context(), orgID, input.Type, input.Enabled, input.Config); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "saved"})
}

func (h *NotificationHandler) TestChannel(c *gin.Context) {
	var input struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	orgID := middleware.GetOrgID(c)
	if err := h.svc.TestChannel(c.Request.Context(), orgID, input.Type); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "sent"})
}

func (h *NotificationHandler) GetSMTPConfig(c *gin.Context) {
	cfg, err := h.svc.GetSMTPConfig(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	// Mask password in response
	masked := *cfg
	if masked.Password != "" {
		masked.Password = "••••••••"
	}
	httputil.RespondOK(c, masked)
}

func (h *NotificationHandler) SaveSMTPConfig(c *gin.Context) {
	var input service.SMTPConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.SaveSMTPConfig(c.Request.Context(), &input); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "saved"})
}

func (h *NotificationHandler) TestSMTP(c *gin.Context) {
	if err := h.svc.TestSMTP(c.Request.Context()); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "sent"})
}
