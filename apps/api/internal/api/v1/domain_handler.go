package v1

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type DomainHandler struct {
	svc *service.DomainService
}

func NewDomainHandler(svc *service.DomainService) *DomainHandler {
	return &DomainHandler{svc: svc}
}

func (h *DomainHandler) ListByApp(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	domains, err := h.svc.ListByApp(c.Request.Context(), appID)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, domains)
}

func (h *DomainHandler) Create(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var input service.CreateDomainInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	domain, err := h.svc.Create(c.Request.Context(), appID, input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondCreated(c, domain, fmt.Sprintf("/api/v1/domains/%s", domain.ID))
}

func (h *DomainHandler) Generate(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	domain, err := h.svc.GenerateTraefikDomain(c.Request.Context(), appID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondCreated(c, domain, "")
}

func (h *DomainHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid domain ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}

func (h *DomainHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid domain ID"))
		return
	}
	var input struct {
		Host       *string `json:"host"`
		ForceHTTPS *bool   `json:"force_https"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	domain, err := h.svc.Update(c.Request.Context(), id, input.Host, input.ForceHTTPS)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, domain)
}
