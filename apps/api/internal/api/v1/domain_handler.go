package v1

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type DomainHandler struct {
	svc   *service.DomainService
	store store.Store
}

func NewDomainHandler(svc *service.DomainService, s store.Store) *DomainHandler {
	return &DomainHandler{svc: svc, store: s}
}

func (h *DomainHandler) verifyAppOrg(c *gin.Context, appID uuid.UUID) error {
	app, err := h.store.Applications().GetByID(c.Request.Context(), appID)
	if err != nil {
		return err
	}
	project, err := h.store.Projects().GetByID(c.Request.Context(), app.ProjectID)
	if err != nil {
		return err
	}
	if project.OrgID != middleware.GetOrgID(c) {
		return fmt.Errorf("access denied")
	}
	return nil
}

func (h *DomainHandler) verifyDomainOrg(c *gin.Context, domainID uuid.UUID) error {
	domain, err := h.store.Domains().GetByID(c.Request.Context(), domainID)
	if err != nil {
		return err
	}
	return h.verifyAppOrg(c, domain.AppID)
}

func (h *DomainHandler) ListByApp(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.verifyAppOrg(c, appID); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}
	domains, err := h.svc.ListByApp(c.Request.Context(), appID)
	if err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail("failed to list domains"))
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
	if err := h.verifyAppOrg(c, appID); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
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
	if err := h.verifyAppOrg(c, appID); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
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
	if err := h.verifyDomainOrg(c, id); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, apierr.ErrInternal.WithDetail("domain deletion failed"))
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
	if err := h.verifyDomainOrg(c, id); err != nil {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
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
		msg := err.Error()
		// Validation errors → 400 with detail; operational errors → 500 without internals
		if strings.Contains(msg, "already in use") ||
			strings.Contains(msg, "hostname") ||
			strings.Contains(msg, "invalid character") ||
			strings.Contains(msg, "label") ||
			strings.Contains(msg, "cannot be empty") ||
			strings.Contains(msg, "characters or fewer") ||
			strings.Contains(msg, "at least two") ||
			strings.Contains(msg, "cannot rename") {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(msg))
		} else {
			httputil.RespondError(c, apierr.ErrInternal.WithDetail("domain update failed"))
		}
		return
	}
	httputil.RespondOK(c, domain)
}
