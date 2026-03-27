package k3s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) CreateVolume(ctx context.Context, opts orchestrator.VolumeOpts) (string, error) {
	qty, err := resource.ParseQuantity(opts.Size)
	if err != nil {
		return "", fmt.Errorf("invalid size %q: %w", opts.Size, err)
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    map[string]string{"managed-by": "sailbox"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}

	if opts.StorageClass != "" {
		pvc.Spec.StorageClassName = &opts.StorageClass
	}

	_, err = o.client.CoreV1().PersistentVolumeClaims(opts.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", fmt.Errorf("create PVC: %w", err)
	}

	return opts.Name, nil
}

func (o *Orchestrator) DeleteVolume(ctx context.Context, name, namespace string) error {
	if err := o.client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete PVC %s/%s: %w", namespace, name, err)
	}
	return nil
}

func (o *Orchestrator) ExpandVolume(ctx context.Context, name, namespace, newSize string) error {
	pvc, err := o.client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newQty, err := resource.ParseQuantity(newSize)
	if err != nil {
		return fmt.Errorf("invalid size %q: %w", newSize, err)
	}

	currentQty := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if newQty.Cmp(currentQty) <= 0 {
		return fmt.Errorf("new size %s must be larger than current size %s", newSize, currentQty.String())
	}

	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newQty
	_, err = o.client.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("expand PVC %s/%s: %w", namespace, name, err)
	}
	return nil
}
