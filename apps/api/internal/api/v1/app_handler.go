package v1

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

func parseMillicores(raw string) float64 {
	if raw == "" || raw == "0" {
		return 0
	}
	if strings.HasSuffix(raw, "n") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "n"), 64)
		return v / 1_000_000
	}
	if strings.HasSuffix(raw, "m") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "m"), 64)
		return v
	}
	v, _ := strconv.ParseFloat(raw, 64)
	return v * 1000
}

func parseMiB(raw string) float64 {
	if raw == "" || raw == "0" {
		return 0
	}
	if strings.HasSuffix(raw, "Ki") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "Ki"), 64)
		return v / 1024
	}
	if strings.HasSuffix(raw, "Mi") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "Mi"), 64)
		return v
	}
	if strings.HasSuffix(raw, "Gi") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(raw, "Gi"), 64)
		return v * 1024
	}
	v, _ := strconv.ParseFloat(raw, 64)
	return v / (1024 * 1024)
}

type AppHandler struct {
	svc     *service.AppService
	metrics *service.MetricsCollector
	store   store.Store
}

func NewAppHandler(svc *service.AppService, metrics *service.MetricsCollector, s store.Store) *AppHandler {
	return &AppHandler{svc: svc, metrics: metrics, store: s}
}

// AppOrgGuard is a middleware that verifies the :id app belongs to the caller's org.
// Attach to any route group that uses :id as an app ID.
func (h *AppHandler) AppOrgGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
			c.Abort()
			return
		}
		app, err := h.store.Applications().GetByID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondError(c, apierr.ErrNotFound.WithDetail("application not found"))
			c.Abort()
			return
		}
		project, err := h.store.Projects().GetByID(c.Request.Context(), app.ProjectID)
		if err != nil || project.OrgID != middleware.GetOrgID(c) {
			httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *AppHandler) ListAll(c *gin.Context) {
	params := bindListParams(c)
	filter := store.AppListFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
	}
	apps, total, err := h.svc.ListAll(c.Request.Context(), params, filter)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(apps, params.Page, params.PerPage, total))
}

func (h *AppHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid project ID"))
		return
	}

	params := bindListParams(c)
	apps, total, err := h.svc.List(c.Request.Context(), projectID, params)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondOK(c, httputil.NewListResponse(apps, params.Page, params.PerPage, total))
}

func (h *AppHandler) Create(c *gin.Context) {
	var input service.CreateAppInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	// Verify project belongs to caller's org
	project, pErr := h.store.Projects().GetByID(c.Request.Context(), input.ProjectID)
	if pErr != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("project not found"))
		return
	}
	if project.OrgID != middleware.GetOrgID(c) {
		httputil.RespondError(c, apierr.ErrForbidden.WithDetail("access denied"))
		return
	}

	app, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondCreated(c, app, fmt.Sprintf("/api/v1/apps/%s", app.ID))
}

func (h *AppHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	app, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, apierr.ErrNotFound.WithDetail("application not found"))
		return
	}
	httputil.RespondOK(c, app)
}

func (h *AppHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}

	httputil.RespondNoContent(c)
}

func (h *AppHandler) Scale(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}

	var input struct {
		Replicas int32 `json:"replicas" binding:"required,min=0,max=100"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}

	app, err := h.svc.Scale(c.Request.Context(), id, input.Replicas)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}

	httputil.RespondOK(c, app)
}

func (h *AppHandler) UpdateEnv(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var input struct {
		EnvVars map[string]string `json:"env_vars" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	app, err := h.svc.UpdateEnvVars(c.Request.Context(), id, input.EnvVars)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, app)
}

func (h *AppHandler) GetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	status, err := h.svc.GetStatus(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, status)
}

func (h *AppHandler) GetPods(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	pods, err := h.svc.GetPods(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	// Record real-time metric point for in-memory ring buffer
	if h.metrics != nil && len(pods) > 0 {
		var cpuUsed, cpuLimit, memUsed, memLimit float64
		for _, p := range pods {
			cpuUsed += parseMillicores(p.Resources.CPUUsed)
			cpuLimit += parseMillicores(p.Resources.CPUTotal)
			memUsed += parseMiB(p.Resources.MemUsed)
			memLimit += parseMiB(p.Resources.MemTotal)
		}
		h.metrics.RecordAppMetric(id, service.AppMetricPoint{
			Time:     time.Now(),
			CPUUsed:  cpuUsed,
			CPULimit: cpuLimit,
			MemUsed:  memUsed,
			MemLimit: memLimit,
		})
	}

	httputil.RespondList(c, pods)
}

func (h *AppHandler) GetMetrics(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if h.metrics == nil {
		httputil.RespondOK(c, []struct{}{})
		return
	}
	httputil.RespondOK(c, h.metrics.GetAppMetrics(id))
}

func (h *AppHandler) Restart(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.Restart(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"message": "restart triggered"})
}

func (h *AppHandler) ClearBuildCache(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.ClearBuildCache(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "build cache cleared"})
}

func (h *AppHandler) Stop(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.Stop(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, gin.H{"message": "stopped"})
}

func (h *AppHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var input service.UpdateAppInput
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	app, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, app)
}

func (h *AppHandler) EnableWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	config, err := h.svc.EnableWebhook(c.Request.Context(), id, baseURL)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, config)
}

func (h *AppHandler) DisableWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	if err := h.svc.DisableWebhook(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"auto_deploy": false})
}

func (h *AppHandler) RegenerateWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	config, err := h.svc.RegenerateWebhookSecret(c.Request.Context(), id, baseURL)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, config)
}

func (h *AppHandler) GetWebhookConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	config, err := h.svc.GetWebhookConfig(c.Request.Context(), id, baseURL)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, config)
}

func (h *AppHandler) GetSecrets(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	keys, err := h.svc.GetSecretKeys(c.Request.Context(), id)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, keys)
}

func (h *AppHandler) UpdateSecrets(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	var secrets map[string]string
	if err := c.ShouldBindJSON(&secrets); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	keys, err := h.svc.UpdateSecrets(c.Request.Context(), id, secrets)
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	httputil.RespondOK(c, keys)
}

func (h *AppHandler) GetPodEvents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid app ID"))
		return
	}
	podName := c.Param("podName")
	if podName == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("pod name required"))
		return
	}
	events, err := h.svc.GetPodEvents(c.Request.Context(), id, podName)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondList(c, events)
}
