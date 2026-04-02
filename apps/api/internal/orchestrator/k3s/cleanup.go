package k3s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

const maxCleanupNames = 50

// GetCleanupStats returns counts and names of resources eligible for cleanup.
func (o *Orchestrator) GetCleanupStats(ctx context.Context) (*orchestrator.CleanupStats, error) {
	stats := &orchestrator.CleanupStats{}

	// ── Pods ────────────────────────────────────────────────────
	pods, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	for _, p := range pods.Items {
		if p.Namespace == "kube-system" {
			continue
		}
		ns := p.Namespace
		name := ns + "/" + p.Name

		switch p.Status.Phase {
		case corev1.PodFailed:
			if podIsEvicted(&p) {
				stats.EvictedPods++
				if len(stats.EvictedPodNames) < maxCleanupNames {
					stats.EvictedPodNames = append(stats.EvictedPodNames, name)
				}
			} else {
				stats.FailedPods++
				if len(stats.FailedPodNames) < maxCleanupNames {
					stats.FailedPodNames = append(stats.FailedPodNames, name)
				}
			}
		case corev1.PodSucceeded:
			stats.CompletedPods++
			if len(stats.CompletedPodNames) < maxCleanupNames {
				stats.CompletedPodNames = append(stats.CompletedPodNames, name)
			}
		}
	}

	// ── ReplicaSets ─────────────────────────────────────────────
	rsList, err := o.client.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replicasets: %w", err)
	}
	for _, rs := range rsList.Items {
		if rs.Namespace == "kube-system" {
			continue
		}
		if rs.Spec.Replicas != nil && *rs.Spec.Replicas == 0 && rs.Status.Replicas == 0 {
			stats.StaleReplicaSets++
			if len(stats.StaleRSNames) < maxCleanupNames {
				stats.StaleRSNames = append(stats.StaleRSNames, rs.Namespace+"/"+rs.Name)
			}
		}
	}

	// ── Jobs ────────────────────────────────────────────────────
	jobs, err := o.client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	jobCutoff := time.Now().Add(-1 * time.Hour)
	for _, j := range jobs.Items {
		if j.Namespace == "kube-system" {
			continue
		}
		if !jobIsComplete(&j) {
			continue
		}
		ct := jobCompletionTime(&j)
		if ct == nil || ct.After(jobCutoff) {
			continue // too recent, won't be cleaned
		}
		stats.CompletedJobs++
		if len(stats.CompletedJobNames) < maxCleanupNames {
			stats.CompletedJobNames = append(stats.CompletedJobNames, j.Namespace+"/"+j.Name)
		}
	}

	// ── PVCs ────────────────────────────────────────────────────
	pvcs, err := o.client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcs.Items {
		if pvc.Namespace == "kube-system" {
			continue
		}
		if pvc.Status.Phase != corev1.ClaimBound {
			stats.UnboundPVCs++
			if len(stats.UnboundPVCNames) < maxCleanupNames {
				stats.UnboundPVCNames = append(stats.UnboundPVCNames, pvc.Namespace+"/"+pvc.Name)
			}
		}
	}

	// Ensure nil slices become empty arrays in JSON
	if stats.EvictedPodNames == nil {
		stats.EvictedPodNames = []string{}
	}
	if stats.FailedPodNames == nil {
		stats.FailedPodNames = []string{}
	}
	if stats.CompletedPodNames == nil {
		stats.CompletedPodNames = []string{}
	}
	if stats.StaleRSNames == nil {
		stats.StaleRSNames = []string{}
	}
	if stats.CompletedJobNames == nil {
		stats.CompletedJobNames = []string{}
	}
	if stats.UnboundPVCNames == nil {
		stats.UnboundPVCNames = []string{}
	}
	if stats.OrphanIngressNames == nil {
		stats.OrphanIngressNames = []string{}
	}

	return stats, nil
}

// CleanupEvictedPods deletes all evicted pods across namespaces (excluding kube-system).
func (o *Orchestrator) CleanupEvictedPods(ctx context.Context) (*orchestrator.CleanupResult, error) {
	pods, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Failed",
	})
	if err != nil {
		return nil, fmt.Errorf("list failed pods: %w", err)
	}

	deleted := 0
	failed := 0
	for _, p := range pods.Items {
		if p.Namespace == "kube-system" {
			continue
		}
		if !podIsEvicted(&p) {
			continue
		}
		o.logger.Info("cleanup: deleting evicted pod", slog.String("pod", p.Name), slog.String("namespace", p.Namespace))
		if err := o.client.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
			o.logger.Warn("cleanup: failed to delete evicted pod", slog.String("pod", p.Name), slog.String("namespace", p.Namespace), slog.String("error", err.Error()))
			failed++
			continue
		}
		deleted++
	}

	msg := fmt.Sprintf("Deleted %d evicted pods", deleted)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	return &orchestrator.CleanupResult{Deleted: deleted, Message: msg}, nil
}

// CleanupFailedPods deletes all non-evicted failed pods across namespaces (excluding kube-system).
func (o *Orchestrator) CleanupFailedPods(ctx context.Context) (*orchestrator.CleanupResult, error) {
	pods, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Failed",
	})
	if err != nil {
		return nil, fmt.Errorf("list failed pods: %w", err)
	}

	deleted := 0
	failed := 0
	for _, p := range pods.Items {
		if p.Namespace == "kube-system" {
			continue
		}
		if podIsEvicted(&p) {
			continue
		}
		o.logger.Info("cleanup: deleting failed pod", slog.String("pod", p.Name), slog.String("namespace", p.Namespace))
		if err := o.client.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
			o.logger.Warn("cleanup: failed to delete failed pod", slog.String("pod", p.Name), slog.String("namespace", p.Namespace), slog.String("error", err.Error()))
			failed++
			continue
		}
		deleted++
	}

	msg := fmt.Sprintf("Deleted %d failed pods", deleted)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	return &orchestrator.CleanupResult{Deleted: deleted, Message: msg}, nil
}

// CleanupCompletedPods deletes all succeeded pods across all namespaces.
func (o *Orchestrator) CleanupCompletedPods(ctx context.Context) (*orchestrator.CleanupResult, error) {
	pods, err := o.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Succeeded",
	})
	if err != nil {
		return nil, fmt.Errorf("list completed pods: %w", err)
	}

	deleted := 0
	failed := 0
	for _, p := range pods.Items {
		if p.Namespace == "kube-system" {
			continue
		}
		o.logger.Info("cleanup: deleting completed pod", slog.String("pod", p.Name), slog.String("namespace", p.Namespace))
		if err := o.client.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, metav1.DeleteOptions{}); err != nil {
			o.logger.Warn("cleanup: failed to delete completed pod", slog.String("pod", p.Name), slog.String("namespace", p.Namespace), slog.String("error", err.Error()))
			failed++
			continue
		}
		deleted++
	}

	msg := fmt.Sprintf("Deleted %d completed pods", deleted)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	return &orchestrator.CleanupResult{Deleted: deleted, Message: msg}, nil
}

// CleanupStaleReplicaSets deletes ReplicaSets with 0 desired and 0 actual replicas.
func (o *Orchestrator) CleanupStaleReplicaSets(ctx context.Context) (*orchestrator.CleanupResult, error) {
	rsList, err := o.client.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replicasets: %w", err)
	}

	deleted := 0
	failed := 0
	for _, rs := range rsList.Items {
		if rs.Namespace == "kube-system" {
			continue
		}
		if rs.Spec.Replicas == nil || *rs.Spec.Replicas != 0 || rs.Status.Replicas != 0 {
			continue
		}
		o.logger.Info("cleanup: deleting stale replicaset", slog.String("rs", rs.Name), slog.String("namespace", rs.Namespace))
		if err := o.client.AppsV1().ReplicaSets(rs.Namespace).Delete(ctx, rs.Name, metav1.DeleteOptions{}); err != nil {
			o.logger.Warn("cleanup: failed to delete stale replicaset", slog.String("rs", rs.Name), slog.String("namespace", rs.Namespace), slog.String("error", err.Error()))
			failed++
			continue
		}
		deleted++
	}

	msg := fmt.Sprintf("Deleted %d stale replicasets", deleted)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	return &orchestrator.CleanupResult{Deleted: deleted, Message: msg}, nil
}

// CleanupCompletedJobs deletes completed jobs that are older than 1 hour.
func (o *Orchestrator) CleanupCompletedJobs(ctx context.Context) (*orchestrator.CleanupResult, error) {
	jobs, err := o.client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	cutoff := time.Now().Add(-1 * time.Hour)
	deleted := 0
	failed := 0
	propagation := metav1.DeletePropagationBackground

	for _, j := range jobs.Items {
		if j.Namespace == "kube-system" {
			continue
		}
		if !jobIsComplete(&j) {
			continue
		}
		// Only delete jobs completed more than 1 hour ago
		completionTime := jobCompletionTime(&j)
		if completionTime == nil || completionTime.After(cutoff) {
			continue
		}
		o.logger.Info("cleanup: deleting completed job", slog.String("job", j.Name), slog.String("namespace", j.Namespace))
		if err := o.client.BatchV1().Jobs(j.Namespace).Delete(ctx, j.Name, metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		}); err != nil {
			o.logger.Warn("cleanup: failed to delete completed job", slog.String("job", j.Name), slog.String("namespace", j.Namespace), slog.String("error", err.Error()))
			failed++
			continue
		}
		deleted++
	}

	msg := fmt.Sprintf("Deleted %d completed jobs", deleted)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	return &orchestrator.CleanupResult{Deleted: deleted, Message: msg}, nil
}

// ── helpers ─────────────────────────────────────────────────────

func podIsEvicted(p *corev1.Pod) bool {
	return p.Status.Reason == "Evicted"
}

func jobIsComplete(j *batchv1.Job) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func jobCompletionTime(j *batchv1.Job) *time.Time {
	if j.Status.CompletionTime != nil {
		t := j.Status.CompletionTime.Time
		return &t
	}
	// Fallback: check condition transition time
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			t := c.LastTransitionTime.Time
			return &t
		}
	}
	return nil
}

func (o *Orchestrator) GetOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) ([]string, error) {
	return o.findOrphanIngresses(ctx, validHosts, systemIngresses)
}

func (o *Orchestrator) CleanupOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) (*orchestrator.CleanupResult, error) {
	orphans, err := o.findOrphanIngresses(ctx, validHosts, systemIngresses)
	if err != nil {
		return nil, err
	}

	deleted := 0
	failed := 0
	for _, key := range orphans {
		// key format: "namespace/name"
		parts := splitNS(key)
		o.logger.Info("cleanup: deleting orphan ingress", slog.String("ingress", key))
		if err := o.client.NetworkingV1().Ingresses(parts[0]).Delete(ctx, parts[1], metav1.DeleteOptions{}); err != nil {
			o.logger.Warn("cleanup: failed to delete orphan ingress", slog.String("ingress", key), slog.String("error", err.Error()))
			failed++
		} else {
			deleted++
		}
	}

	msg := fmt.Sprintf("Deleted %d orphan ingresses", deleted)
	if failed > 0 {
		msg += fmt.Sprintf(" (%d failed)", failed)
	}
	return &orchestrator.CleanupResult{Deleted: deleted, Message: msg}, nil
}

func (o *Orchestrator) findOrphanIngresses(ctx context.Context, validHosts map[string]bool, systemIngresses map[string]string) ([]string, error) {
	var allIngresses []networkingv1.Ingress

	// List sailbox-managed ingresses across all namespaces
	nsList, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	for _, ns := range nsList.Items {
		ingresses, err := o.client.NetworkingV1().Ingresses(ns.Name).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=sailbox",
		})
		if err != nil {
			continue
		}
		allIngresses = append(allIngresses, ingresses.Items...)
	}

	var orphans []string
	for _, ing := range allIngresses {
		key := ing.Namespace + "/" + ing.Name
		isOrphan := false

		// System ingresses (e.g. panel) are validated by resource identity:
		// their host must match the currently configured expected host.
		if expectedHost, ok := systemIngresses[key]; ok {
			for _, rule := range ing.Spec.Rules {
				if rule.Host != expectedHost {
					isOrphan = true
					break
				}
			}
		} else {
			// App ingresses are validated against the global valid hosts set.
			for _, rule := range ing.Spec.Rules {
				if !validHosts[rule.Host] {
					isOrphan = true
					break
				}
			}
		}

		if isOrphan {
			orphans = append(orphans, key)
		}
	}
	return orphans, nil
}

func splitNS(key string) [2]string {
	for i, c := range key {
		if c == '/' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{"default", key}
}
