package v1

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type ProjectHandler struct {
	svc   *service.ProjectService
	store store.Store
}

func NewProjectHandler(svc *service.ProjectService, s store.Store) *ProjectHandler {
	return &ProjectHandler{svc: svc, store: s}
}

func (h *ProjectHandler) List(c *gin.Context) {
	params := bindListParams(c)
	orgID := middleware.GetOrgID(c)

	// A member restricted to specific projects must not see the rest of the org
	// listed here — the IDs alone are enough to drive the per-project routes.
	granted, err := grantedProjectIDs(c, h.store)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	projects, total, err := h.svc.List(c.Request.Context(), orgID, params, granted)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(projects, params.Page, params.PerPage, total))
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var input service.CreateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	// Explicit project grants exist to narrow a member to those projects, so a
	// scoped member does not get to create more. Granting them access to what
	// they create would be worse still: a read-only viewer would come out of it
	// holding admin on a brand new project.
	granted, err := grantedProjectIDs(c, h.store)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	if len(granted) > 0 {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail(
			"your access is limited to specific projects — ask an owner or admin to create a new one"))
		return
	}

	orgID := middleware.GetOrgID(c)
	project, err := h.svc.Create(c.Request.Context(), orgID, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondCreated(c, project, fmt.Sprintf("/api/v1/projects/%s", project.ID))
}

func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	project, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("project not found"))
		return
	}

	httputil.RespondOK(c, project)
}

func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	var input service.UpdateProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	project, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, project)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondNoContent(c)
}

func (h *ProjectHandler) UpdateEnv(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}
	var input struct {
		EnvVars map[string]string `json:"env_vars" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	project, err := h.svc.UpdateEnvVars(c.Request.Context(), id, input.EnvVars)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, project)
}

// bindListParams extracts pagination params from query string.
func bindListParams(c *gin.Context) store.ListParams {
	params := store.DefaultListParams()
	type query struct {
		Page    int `form:"page"`
		PerPage int `form:"per_page"`
	}
	var q query
	if err := c.ShouldBindQuery(&q); err == nil {
		if q.Page > 0 {
			params.Page = q.Page
		}
		if q.PerPage > 0 && q.PerPage <= 100 {
			params.PerPage = q.PerPage
		}
	}
	return params
}
