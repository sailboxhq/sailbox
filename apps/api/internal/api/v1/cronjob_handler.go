package v1

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type CronJobHandler struct {
	svc *service.CronJobService
}

func NewCronJobHandler(svc *service.CronJobService) *CronJobHandler {
	return &CronJobHandler{svc: svc}
}

func (h *CronJobHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}
	params := bindListParams(c)
	jobs, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(jobs, params.Page, params.PerPage, total))
}

func (h *CronJobHandler) Create(c *gin.Context) {
	var input service.CreateCronJobInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	cj, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondCreated(c, cj, fmt.Sprintf("/api/v1/cronjobs/%s", cj.ID))
}

func (h *CronJobHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	cj, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("cronjob not found"))
		return
	}
	httputil.RespondOK(c, cj)
}

func (h *CronJobHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	var input service.UpdateCronJobInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	cj, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, cj)
}

func (h *CronJobHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondNoContent(c)
}

func (h *CronJobHandler) Trigger(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	run, err := h.svc.Trigger(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondAccepted(c, run)
}

func (h *CronJobHandler) ListRuns(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid cronjob ID"))
		return
	}
	params := bindListParams(c)
	runs, total, err := h.svc.ListRuns(c.Request.Context(), id, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(runs, params.Page, params.PerPage, total))
}
