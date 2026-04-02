package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// AlertRule defines a single alert evaluation rule.
type AlertRule struct {
	Name       string
	Severity   string // "critical" | "warning"
	SourceType string // "node" | "app"
	// Evaluate checks the current state and returns (firing, sourceName, message).
	// May return multiple results for multi-source rules (e.g. per-node).
	Evaluate func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring
}

type AlertFiring struct {
	SourceName string
	Message    string
}

// AlertEvaluator evaluates built-in alert rules and persists results.
type AlertEvaluator struct {
	metricsStore store.MetricsStore
	orch         orchestrator.Orchestrator
	appStore     store.Store
	logger       *slog.Logger
	rules        []AlertRule
	notifSvc     *NotificationService
}

func NewAlertEvaluator(
	metricsStore store.MetricsStore,
	orch orchestrator.Orchestrator,
	appStore store.Store,
	logger *slog.Logger,
	notifSvc *NotificationService,
) *AlertEvaluator {
	ae := &AlertEvaluator{
		metricsStore: metricsStore,
		orch:         orch,
		appStore:     appStore,
		logger:       logger,
		notifSvc:     notifSvc,
	}
	ae.rules = ae.builtinRules()
	return ae
}

func (ae *AlertEvaluator) builtinRules() []AlertRule {
	return []AlertRule{
		{
			Name: "node_cpu_high", Severity: "warning", SourceType: "node",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				nodes, err := orch.GetNodeMetrics(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, n := range nodes {
					used := parseMillis(n.CPUUsed)
					total := parseMillis(n.CPUTotal)
					if total > 0 && float64(used)/float64(total) > 0.90 {
						firings = append(firings, AlertFiring{
							SourceName: n.Name,
							Message:    fmt.Sprintf("Node %s CPU usage at %.0f%%", n.Name, float64(used)/float64(total)*100),
						})
					}
				}
				return firings
			},
		},
		{
			Name: "node_mem_high", Severity: "warning", SourceType: "node",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				nodes, err := orch.GetNodeMetrics(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, n := range nodes {
					used := parseBytes(n.MemUsed)
					total := parseBytes(n.MemTotal)
					if total > 0 && float64(used)/float64(total) > 0.90 {
						firings = append(firings, AlertFiring{
							SourceName: n.Name,
							Message:    fmt.Sprintf("Node %s memory usage at %.0f%%", n.Name, float64(used)/float64(total)*100),
						})
					}
				}
				return firings
			},
		},
		{
			Name: "node_not_ready", Severity: "critical", SourceType: "node",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				nodes, err := orch.GetNodes(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, n := range nodes {
					if n.Status != "Ready" {
						firings = append(firings, AlertFiring{
							SourceName: n.Name,
							Message:    fmt.Sprintf("Node %s is %s", n.Name, n.Status),
						})
					}
				}
				return firings
			},
		},
		{
			Name: "pod_crashloop", Severity: "critical", SourceType: "app",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				pods, err := orch.GetAllPods(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, pod := range pods {
					for _, c := range pod.Containers {
						if c.Reason == "CrashLoopBackOff" {
							firings = append(firings, AlertFiring{
								SourceName: pod.Name,
								Message:    fmt.Sprintf("Pod %s container %s in CrashLoopBackOff", pod.Name, c.Name),
							})
						}
					}
				}
				return firings
			},
		},
		{
			Name: "pod_oom_killed", Severity: "critical", SourceType: "app",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				pods, err := orch.GetAllPods(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, pod := range pods {
					for _, c := range pod.Containers {
						if c.Reason == "OOMKilled" {
							firings = append(firings, AlertFiring{
								SourceName: pod.Name,
								Message:    fmt.Sprintf("Pod %s container %s was OOM killed", pod.Name, c.Name),
							})
						}
					}
				}
				return firings
			},
		},
		{
			Name: "pod_image_pull_error", Severity: "warning", SourceType: "app",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				pods, err := orch.GetAllPods(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, pod := range pods {
					for _, c := range pod.Containers {
						if c.Reason == "ImagePullBackOff" || c.Reason == "ErrImagePull" {
							firings = append(firings, AlertFiring{
								SourceName: pod.Name,
								Message:    fmt.Sprintf("Pod %s failed to pull image (%s)", pod.Name, c.Reason),
							})
						}
					}
				}
				return firings
			},
		},
		{
			Name: "pod_scheduling_failed", Severity: "warning", SourceType: "app",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				pods, err := orch.GetAllPods(ctx)
				if err != nil {
					return nil
				}
				var firings []AlertFiring
				for _, pod := range pods {
					if pod.Phase != "Pending" {
						continue
					}
					// Only alert on truly stuck pods — skip normal transient states
					for _, c := range pod.Containers {
						if c.Reason == "Unschedulable" {
							firings = append(firings, AlertFiring{
								SourceName: pod.Name,
								Message:    fmt.Sprintf("Pod %s cannot be scheduled (%s)", pod.Name, c.Reason),
							})
							break
						}
					}
				}
				return firings
			},
		},
		{
			Name: "k8s_warning_events", Severity: "warning", SourceType: "node",
			Evaluate: func(ctx context.Context, orch orchestrator.Orchestrator) []AlertFiring {
				events, err := orch.GetClusterEvents(ctx, 50)
				if err != nil {
					return nil
				}
				cutoff := time.Now().Add(-5 * time.Minute)

				// Skip noisy/normal K8s operational events.
				// Only skip events that are guaranteed harmless. Anything that
				// *could* indicate a real problem stays out of this list.
				ignoredReasons := map[string]bool{
					// Lifecycle — every deploy/scale triggers these
					"Pulling":          true,
					"Pulled":           true,
					"Created":          true,
					"Started":          true,
					"Killing":          true,
					"Scheduled":        true,
					"SuccessfulCreate": true,

					// Node info events (not failures)
					"NodeReady":      true,
					"RegisteredNode": true,
					"Starting":       true, // kubelet/kube-proxy starting

					// GC & scheduler (self-resolving)
					"FreeDiskSpaceFailed": true,
					"Evicted":             true, // handled by aggregation below
					"Preempting":          true,

					// DNS (benign on VPS with >3 nameservers)
					"DNSConfigForming": true,

					// Volume attach success (not failure)
					"SuccessfulAttachVolume": true,
				}

				var firings []AlertFiring
				seen := make(map[string]bool)
				evictedCount := 0
				evictedNode := ""
				for _, e := range events {
					if e.Type != "Warning" {
						continue
					}
					if e.LastSeen.Before(cutoff) {
						continue
					}
					// Count evictions for aggregation
					if e.Reason == "Evicted" {
						evictedCount++
						if e.Message != "" {
							// Extract node info from message
							evictedNode = e.InvolvedObject
						}
						continue
					}
					if ignoredReasons[e.Reason] {
						continue
					}
					key := e.InvolvedObject + "/" + e.Reason
					if seen[key] {
						continue
					}
					seen[key] = true
					firings = append(firings, AlertFiring{
						SourceName: e.InvolvedObject,
						Message:    fmt.Sprintf("[%s] %s: %s", e.Namespace, e.Reason, e.Message),
					})
				}
				// Aggregate evictions into one alert
				if evictedCount > 0 {
					node := evictedNode
					if node == "" {
						node = "cluster"
					}
					firings = append(firings, AlertFiring{
						SourceName: node,
						Message:    fmt.Sprintf("%d pods evicted due to DiskPressure — free disk space on affected nodes", evictedCount),
					})
				}
				return firings
			},
		},
	}
}

// Evaluate runs all rules and updates alert state.
func (ae *AlertEvaluator) Evaluate(ctx context.Context) {
	for _, rule := range ae.rules {
		firings := rule.Evaluate(ctx, ae.orch)
		firingSet := make(map[string]AlertFiring)
		for _, f := range firings {
			firingSet[f.SourceName] = f
		}

		// Check existing active alerts for this rule
		// If still firing: do nothing. If cleared: resolve.
		active, _ := ae.metricsStore.Alerts().ListActive(ctx)
		for _, alert := range active {
			if alert.RuleName != rule.Name {
				continue
			}
			if _, stillFiring := firingSet[alert.SourceName]; !stillFiring {
				// Auto-resolve
				if err := ae.metricsStore.Alerts().Resolve(ctx, alert.ID); err == nil {
					ae.logger.Info("alert resolved",
						slog.String("rule", rule.Name),
						slog.String("source", alert.SourceName),
					)
					if ae.notifSvc != nil {
						msg := fmt.Sprintf("Resolved: %s on %s", rule.Name, alert.SourceName)
						ae.notifSvc.NotifyAllOrgsAsync(model.EventAlertResolved, rule.Name+" resolved", msg)
					}
				}
			} else {
				// Still firing, remove from set to avoid duplicate insert
				delete(firingSet, alert.SourceName)
			}
		}

		// Insert new alerts for newly firing sources (with 30-min cooldown after resolve)
		for _, firing := range firingSet {
			// Check no active alert exists
			_, err := ae.metricsStore.Alerts().GetActiveByRuleAndSource(ctx, rule.Name, firing.SourceName)
			if err == nil {
				continue // already active
			}
			if !errors.Is(err, sql.ErrNoRows) {
				continue
			}
			// Check cooldown: don't re-fire within 30 min of last resolve
			if ae.isInCooldown(ctx, rule.Name, firing.SourceName) {
				continue
			}

			alert := &model.MetricAlert{
				RuleName:   rule.Name,
				Severity:   rule.Severity,
				SourceType: rule.SourceType,
				SourceName: firing.SourceName,
				Message:    firing.Message,
			}
			if err := ae.metricsStore.Alerts().Insert(ctx, alert); err != nil {
				ae.logger.Warn("failed to insert alert", slog.Any("error", err))
				continue
			}
			ae.logger.Warn("alert fired",
				slog.String("rule", rule.Name),
				slog.String("severity", rule.Severity),
				slog.String("source", firing.SourceName),
				slog.String("message", firing.Message),
			)

			// Send notification to all enabled channels (fire-and-forget)
			if ae.notifSvc != nil {
				ae.notifSvc.NotifyAllOrgsAsync(model.EventAlertFired, alert.RuleName, alert.Message)
			}
		}
	}
}

// isInCooldown checks if an alert for this rule+source was resolved within the last 30 minutes.
func (ae *AlertEvaluator) isInCooldown(ctx context.Context, ruleName, sourceName string) bool {
	cooldown := 30 * time.Minute
	// Query recent alerts (last 30 min) with enough results to find our rule+source
	alerts, _, err := ae.metricsStore.Alerts().List(ctx, store.AlertQuery{
		From:       time.Now().Add(-cooldown),
		ListParams: store.ListParams{Page: 1, PerPage: 200},
	})
	if err != nil {
		return false
	}
	for _, a := range alerts {
		if a.RuleName == ruleName && a.SourceName == sourceName && a.ResolvedAt != nil {
			if time.Since(*a.ResolvedAt) < cooldown {
				return true
			}
		}
	}
	return false
}
