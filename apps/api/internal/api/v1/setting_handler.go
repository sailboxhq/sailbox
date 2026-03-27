package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type SettingHandler struct {
	svc *service.SettingService
}

func NewSettingHandler(svc *service.SettingService) *SettingHandler {
	return &SettingHandler{svc: svc}
}

func (h *SettingHandler) GetAll(c *gin.Context) {
	settings, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	// Convert to map for easier frontend consumption
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	httputil.RespondOK(c, result)
}

func (h *SettingHandler) Update(c *gin.Context) {
	var input struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.Set(c.Request.Context(), input.Key, input.Value); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"key": input.Key, "value": input.Value})
}
