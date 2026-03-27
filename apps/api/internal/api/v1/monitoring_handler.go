package v1

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type MonitoringHandler struct {
	metrics *service.MetricsCollector
}

func NewMonitoringHandler(metrics *service.MetricsCollector) *MonitoringHandler {
	return &MonitoringHandler{metrics: metrics}
}

func (h *MonitoringHandler) GetSnapshots(c *gin.Context) {
	q := store.SnapshotQuery{
		SourceType: c.Query("source_type"),
		SourceName: c.Query("source_name"),
		Limit:      500,
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q.From = t
		}
	} else {
		q.From = time.Now().Add(-1 * time.Hour)
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			q.To = t
		}
	}

	data, err := h.metrics.GetSnapshots(c.Request.Context(), q)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, data)
}

func (h *MonitoringHandler) GetEvents(c *gin.Context) {
	q := store.EventQuery{
		EventType:  c.Query("event_type"),
		Namespace:  c.Query("namespace"),
		ListParams: bindListParams(c),
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			q.From = t
		}
	}

	events, total, err := h.metrics.GetEvents(c.Request.Context(), q)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(events, q.Page, q.PerPage, total))
}

func (h *MonitoringHandler) GetAlerts(c *gin.Context) {
	q := store.AlertQuery{
		ActiveOnly: c.Query("active") == "true",
		Severity:   c.Query("severity"),
		ListParams: bindListParams(c),
	}

	alerts, total, err := h.metrics.GetAlerts(c.Request.Context(), q)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, httputil.NewListResponse(alerts, q.Page, q.PerPage, total))
}

func (h *MonitoringHandler) GetActiveAlerts(c *gin.Context) {
	alerts, err := h.metrics.GetActiveAlerts(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{
		"count":  len(alerts),
		"alerts": alerts,
	})
}

func (h *MonitoringHandler) ResolveAlert(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("invalid alert ID"))
		return
	}
	if err := h.metrics.ResolveAlert(c.Request.Context(), id); err != nil {
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"message": "alert resolved"})
}
