package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type DatabaseHandler struct {
	svc *service.DatabaseService
}

func NewDatabaseHandler(svc *service.DatabaseService) *DatabaseHandler {
	return &DatabaseHandler{svc: svc}
}

// dbErr wraps a service error as a ProblemDetail if it isn't one already.
func dbErr(err error) error {
	if _, ok := err.(*apierr.ProblemDetail); ok {
		return err
	}
	return apierr.ErrBadRequest.WithDetail(err.Error())
}

func (h *DatabaseHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	params := bindListParams(c)
	dbs, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(dbs, params.Page, params.PerPage, total))
}

func (h *DatabaseHandler) Create(c *gin.Context) {
	var input service.CreateDatabaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	db, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondCreated(c, db, fmt.Sprintf("/api/v1/databases/%s", db.ID))
}

func (h *DatabaseHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	db, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("database not found"))
		return
	}

	httputil.RespondOK(c, db)
}

func (h *DatabaseHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondNoContent(c)
}

func (h *DatabaseHandler) ListVersions(c *gin.Context) {
	engine := c.Query("engine")
	if engine != "" {
		versions, ok := model.SupportedVersions[model.DBEngine(engine)]
		if !ok {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("unknown engine"))
			return
		}
		c.JSON(http.StatusOK, versions)
		return
	}
	c.JSON(http.StatusOK, model.SupportedVersions)
}

func (h *DatabaseHandler) GetCredentials(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	creds, err := h.svc.GetCredentials(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, creds)
}

func (h *DatabaseHandler) GetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	status, err := h.svc.GetStatus(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, status)
}

func (h *DatabaseHandler) GetPods(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	pods, err := h.svc.GetPods(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondList(c, pods)
}

func (h *DatabaseHandler) TriggerBackup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	backup, err := h.svc.TriggerBackup(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondCreated(c, backup, fmt.Sprintf("/api/v1/databases/%s/backups", id))
}

func (h *DatabaseHandler) UpdateExternalAccess(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	var input struct {
		Enabled bool  `json:"enabled"`
		Port    int32 `json:"port"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	db, err := h.svc.UpdateExternalAccess(c.Request.Context(), id, input.Enabled, input.Port)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, db)
}

func (h *DatabaseHandler) UsedPorts(c *gin.Context) {
	ports, err := h.svc.UsedExternalPorts(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}
	if ports == nil {
		ports = []model.ExternalPortInfo{}
	}
	httputil.RespondOK(c, ports)
}

func (h *DatabaseHandler) UpdateBackupConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	var input service.UpdateBackupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	db, err := h.svc.UpdateBackupConfig(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, db)
}

func (h *DatabaseHandler) RestoreBackup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	backupID, err := uuid.Parse(c.Param("backupId"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid backup ID"))
		return
	}

	if err := h.svc.RestoreBackup(c.Request.Context(), id, backupID); err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "restore started"})
}

func (h *DatabaseHandler) ListBackups(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid database ID"))
		return
	}

	params := bindListParams(c)
	backups, total, err := h.svc.ListBackups(c.Request.Context(), id, params)
	if err != nil {
		httputil.RespondError(c, dbErr(err))
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(backups, params.Page, params.PerPage, total))
}
