package k3s

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

const sailboxQuotaName = "sailbox-quota"

// EnsureResourceQuota creates or updates a ResourceQuota for the namespace.
func (o *Orchestrator) EnsureResourceQuota(ctx context.Context, namespace string, quota model.ResourceQuotaConfig) error {
	hard := corev1.ResourceList{}

	if quota.CPULimit != "" {
		if q, err := resource.ParseQuantity(quota.CPULimit); err == nil {
			hard[corev1.ResourceLimitsCPU] = q
		}
	}
	if quota.MemLimit != "" {
		if q, err := resource.ParseQuantity(quota.MemLimit); err == nil {
			hard[corev1.ResourceLimitsMemory] = q
		}
	}
	if quota.PodLimit > 0 {
		hard[corev1.ResourcePods] = *resource.NewQuantity(int64(quota.PodLimit), resource.DecimalSI)
	}
	if quota.PVCLimit > 0 {
		hard[corev1.ResourcePersistentVolumeClaims] = *resource.NewQuantity(int64(quota.PVCLimit), resource.DecimalSI)
	}
	if quota.StorageLimit != "" {
		if q, err := resource.ParseQuantity(quota.StorageLimit); err == nil {
			hard[corev1.ResourceRequestsStorage] = q
		}
	}

	if len(hard) == 0 {
		// No quotas set — remove existing quota if any
		return o.DeleteResourceQuota(ctx, namespace)
	}

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sailboxQuotaName,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "sailbox"},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	existing, err := o.client.CoreV1().ResourceQuotas(namespace).Get(ctx, sailboxQuotaName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.CoreV1().ResourceQuotas(namespace).Create(ctx, rq, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create resource quota: %w", err)
			}
			o.logger.Info("created ResourceQuota", slog.String("ns", namespace))
			return nil
		}
		return err
	}

	existing.Spec.Hard = hard
	_, err = o.client.CoreV1().ResourceQuotas(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// DeleteResourceQuota removes the sailbox ResourceQuota from the namespace.
func (o *Orchestrator) DeleteResourceQuota(ctx context.Context, namespace string) error {
	err := o.client.CoreV1().ResourceQuotas(namespace).Delete(ctx, sailboxQuotaName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
