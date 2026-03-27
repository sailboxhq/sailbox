package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

const (
	collectInterval   = 30 * time.Second
	cleanupInterval   = 1 * time.Hour
	snapshotRetention = 7 * 24 * time.Hour
	eventRetention    = 30 * 24 * time.Hour
	alertRetention    = 30 * 24 * time.Hour
)

type MetricsCollector struct {
	metricsStore store.MetricsStore
	appStore     store.Store
	orch         orchestrator.Orchestrator
	logger       *slog.Logger
	evaluator    *AlertEvaluator

	cancel context.CancelFunc
	wg     sync.WaitGroup

	// In-memory ring buffer for real-time app charts (not persisted)
	mu      sync.RWMutex
	appRing map[uuid.UUID][]AppMetricPoint
}

type AppMetricPoint struct {
	Time     time.Time `json:"time"`
	CPUUsed  float64   `json:"cpu_used"`
	CPULimit float64   `json:"cpu_limit"`
	MemUsed  float64   `json:"mem_used"`
	MemLimit float64   `json:"mem_limit"`
}

const appRingMax = 120

func NewMetricsCollector(
	metricsStore store.MetricsStore,
	appStore store.Store,
	orch orchestrator.Orchestrator,
	logger *slog.Logger,
	notifSvc *NotificationService,
) *MetricsCollector {
	mc := &MetricsCollector{
		metricsStore: metricsStore,
		appStore:     appStore,
		orch:         orch,
		logger:       logger,
		appRing:      make(map[uuid.UUID][]AppMetricPoint),
	}
	mc.evaluator = NewAlertEvaluator(metricsStore, orch, appStore, logger, notifSvc)
	return mc
}

// Start launches background collection, evaluation, and cleanup goroutines.
func (c *MetricsCollector) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	c.wg.Add(3)
	go c.collectLoop(ctx)
	go c.evaluateLoop(ctx)
	go c.cleanupLoop(ctx)

	c.logger.Info("metrics collector started",
		slog.Duration("collect_interval", collectInterval),
		slog.Duration("cleanup_interval", cleanupInterval),
	)
}

// Stop gracefully shuts down all background goroutines.
func (c *MetricsCollector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *MetricsCollector) collectLoop(ctx context.Context) {
	defer c.wg.Done()
	// Collect immediately on start
	c.collect(ctx)
	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *MetricsCollector) evaluateLoop(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.evaluator.Evaluate(ctx)
		}
	}
}

func (c *MetricsCollector) cleanupLoop(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanup(ctx)
		}
	}
}

func (c *MetricsCollector) collect(ctx context.Context) {
	now := time.Now()
	var snapshots []model.MetricSnapshot

	// Collect node metrics
	nodeMetrics, err := c.orch.GetNodeMetrics(ctx)
	if err == nil {
		for _, n := range nodeMetrics {
			pc := n.PodCount
			snapshots = append(snapshots, model.MetricSnapshot{
				CollectedAt:    now,
				SourceType:     "node",
				SourceName:     n.Name,
				CPUUsedMillis:  parseMillis(n.CPUUsed),
				CPUTotalMillis: parseMillis(n.CPUTotal),
				MemUsedBytes:   parseBytes(n.MemUsed),
				MemTotalBytes:  parseBytes(n.MemTotal),
				PodCount:       &pc,
			})
		}
	}

	// Collect pod metrics aggregated by app
	allPods, err := c.orch.GetAllPods(ctx)
	if err == nil {
		appMetrics := make(map[string]*model.MetricSnapshot)
		for _, pod := range allPods {
			if pod.AppID == "" {
				continue
			}
			m, ok := appMetrics[pod.AppID]
			if !ok {
				m = &model.MetricSnapshot{
					CollectedAt: now,
					SourceType:  "app",
					SourceName:  pod.AppID,
				}
				appMetrics[pod.AppID] = m
			}
			m.CPUUsedMillis += parseMillis(pod.Resources.CPUUsed)
			m.CPUTotalMillis += parseMillis(pod.Resources.CPUTotal)
			m.MemUsedBytes += parseBytes(pod.Resources.MemUsed)
			m.MemTotalBytes += parseBytes(pod.Resources.MemTotal)
		}
		for _, m := range appMetrics {
			snapshots = append(snapshots, *m)
		}
	}

	if len(snapshots) > 0 {
		if err := c.metricsStore.Snapshots().InsertBatch(ctx, snapshots); err != nil {
			c.logger.Warn("failed to insert snapshots", slog.Any("error", err))
		}
	}

	// Persist warning K8s events
	clusterEvents, err := c.orch.GetClusterEvents(ctx, 100)
	if err == nil {
		var metricsEvents []model.MetricEvent
		for _, e := range clusterEvents {
			if e.Type != "Warning" {
				continue
			}
			metricsEvents = append(metricsEvents, model.MetricEvent{
				EventType:      e.Type,
				Reason:         e.Reason,
				Message:        e.Message,
				Namespace:      e.Namespace,
				InvolvedObject: e.InvolvedObject,
				FirstSeen:      e.FirstSeen,
				LastSeen:       e.LastSeen,
				Count:          e.Count,
			})
		}
		if len(metricsEvents) > 0 {
			if err := c.metricsStore.Events().UpsertBatch(ctx, metricsEvents); err != nil {
				c.logger.Warn("failed to upsert events", slog.Any("error", err))
			}
		}
	}
}

func (c *MetricsCollector) cleanup(ctx context.Context) {
	now := time.Now()
	if n, err := c.metricsStore.Snapshots().DeleteOlderThan(ctx, now.Add(-snapshotRetention)); err == nil && n > 0 {
		c.logger.Info("cleaned up snapshots", slog.Int64("deleted", n))
	}
	if n, err := c.metricsStore.Events().DeleteOlderThan(ctx, now.Add(-eventRetention)); err == nil && n > 0 {
		c.logger.Info("cleaned up events", slog.Int64("deleted", n))
	}
	if n, err := c.metricsStore.Alerts().DeleteOlderThan(ctx, now.Add(-alertRetention)); err == nil && n > 0 {
		c.logger.Info("cleaned up alerts", slog.Int64("deleted", n))
	}
}

// RecordAppMetric records a real-time metric point for the in-memory ring buffer.
// Called by the app handler when pods are fetched.
func (c *MetricsCollector) RecordAppMetric(appID uuid.UUID, point AppMetricPoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	buf := c.appRing[appID]
	buf = append(buf, point)
	if len(buf) > appRingMax {
		buf = buf[len(buf)-appRingMax:]
	}
	c.appRing[appID] = buf
}

// GetAppMetrics returns the in-memory ring buffer for an app.
func (c *MetricsCollector) GetAppMetrics(appID uuid.UUID) []AppMetricPoint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	buf := c.appRing[appID]
	if buf == nil {
		return []AppMetricPoint{}
	}
	out := make([]AppMetricPoint, len(buf))
	copy(out, buf)
	return out
}

// GetSnapshots returns persisted snapshots from the DB.
func (c *MetricsCollector) GetSnapshots(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error) {
	return c.metricsStore.Snapshots().Query(ctx, q)
}

// GetEvents returns persisted events from the DB.
func (c *MetricsCollector) GetEvents(ctx context.Context, q store.EventQuery) ([]model.MetricEvent, int, error) {
	return c.metricsStore.Events().List(ctx, q)
}

// GetAlerts returns alerts from the DB.
func (c *MetricsCollector) GetAlerts(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error) {
	return c.metricsStore.Alerts().List(ctx, q)
}

// GetActiveAlerts returns unresolved alerts.
func (c *MetricsCollector) GetActiveAlerts(ctx context.Context) ([]model.MetricAlert, error) {
	return c.metricsStore.Alerts().ListActive(ctx)
}

// ResolveAlert manually resolves an alert.
func (c *MetricsCollector) ResolveAlert(ctx context.Context, id uuid.UUID) error {
	return c.metricsStore.Alerts().Resolve(ctx, id)
}

// ── Parse helpers ───────────────────────────────────────────────

func parseMillis(raw string) int64 {
	if raw == "" || raw == "0" {
		return 0
	}
	if s, ok := strings.CutSuffix(raw, "n"); ok {
		v, _ := strconv.ParseFloat(s, 64)
		return int64(v / 1_000_000)
	}
	if s, ok := strings.CutSuffix(raw, "m"); ok {
		v, _ := strconv.ParseFloat(s, 64)
		return int64(v)
	}
	v, _ := strconv.ParseFloat(raw, 64)
	return int64(v * 1000)
}

func parseBytes(raw string) int64 {
	if raw == "" || raw == "0" {
		return 0
	}
	if s, ok := strings.CutSuffix(raw, "Ki"); ok {
		v, _ := strconv.ParseFloat(s, 64)
		return int64(v * 1024)
	}
	if s, ok := strings.CutSuffix(raw, "Mi"); ok {
		v, _ := strconv.ParseFloat(s, 64)
		return int64(v * 1024 * 1024)
	}
	if s, ok := strings.CutSuffix(raw, "Gi"); ok {
		v, _ := strconv.ParseFloat(s, 64)
		return int64(v * 1024 * 1024 * 1024)
	}
	v, _ := strconv.ParseFloat(raw, 64)
	return int64(v)
}
