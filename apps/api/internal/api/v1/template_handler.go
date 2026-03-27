package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type TemplateHandler struct {
	svc *service.TemplateService
}

func NewTemplateHandler(svc *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

func (h *TemplateHandler) List(c *gin.Context) {
	params := bindListParams(c)
	templates, total, err := h.svc.List(c.Request.Context(), params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(templates, params.Page, params.PerPage, total))
}

func (h *TemplateHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid template ID"))
		return
	}

	tpl, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("template not found"))
		return
	}

	httputil.RespondOK(c, tpl)
}
