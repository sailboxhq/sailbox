package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

// SystemBackupHandler handles system backup API endpoints.
type SystemBackupHandler struct {
	svc *service.SystemBackupService
}

// NewSystemBackupHandler creates a new SystemBackupHandler.
func NewSystemBackupHandler(svc *service.SystemBackupService) *SystemBackupHandler {
	return &SystemBackupHandler{svc: svc}
}

// GetConfig returns the current system backup configuration.
func (h *SystemBackupHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, cfg)
}

// SaveConfig updates the system backup configuration.
func (h *SystemBackupHandler) SaveConfig(c *gin.Context) {
	var input service.SystemBackupConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.SaveConfig(c.Request.Context(), &input); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "saved"})
}

// TriggerBackup starts an immediate system backup.
func (h *SystemBackupHandler) TriggerBackup(c *gin.Context) {
	backup, err := h.svc.TriggerBackup(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, backup)
}

// ListBackups returns a paginated list of system backups.
func (h *SystemBackupHandler) ListBackups(c *gin.Context) {
	params := bindListParams(c)
	backups, total, err := h.svc.ListBackups(c.Request.Context(), params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(backups, params.Page, params.PerPage, total))
}

// ScanS3Backups lists available backup files in an S3 bucket.
func (h *SystemBackupHandler) ScanS3Backups(c *gin.Context) {
	var input struct {
		Endpoint  string `json:"endpoint" binding:"required"`
		Bucket    string `json:"bucket" binding:"required"`
		AccessKey string `json:"access_key" binding:"required"`
		SecretKey string `json:"secret_key" binding:"required"`
		Path      string `json:"path"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	s3Config := orchestrator.S3Config{
		Endpoint:  input.Endpoint,
		Bucket:    input.Bucket,
		AccessKey: input.AccessKey,
		SecretKey: input.SecretKey,
	}

	files, err := h.svc.ScanS3Backups(c.Request.Context(), s3Config, input.Path)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, files)
}

// RestoreFromS3 downloads a backup from S3 and restores it.
func (h *SystemBackupHandler) RestoreFromS3(c *gin.Context) {
	var input struct {
		Endpoint  string `json:"endpoint" binding:"required"`
		Bucket    string `json:"bucket" binding:"required"`
		AccessKey string `json:"access_key" binding:"required"`
		SecretKey string `json:"secret_key" binding:"required"`
		S3Key     string `json:"s3_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	s3Config := orchestrator.S3Config{
		Endpoint:  input.Endpoint,
		Bucket:    input.Bucket,
		AccessKey: input.AccessKey,
		SecretKey: input.SecretKey,
	}

	if err := h.svc.RestoreFromS3(c.Request.Context(), s3Config, input.S3Key); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"status": "restored"})
}
