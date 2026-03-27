package k3s

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureConfigMap creates or updates a ConfigMap in the given namespace.
func (o *Orchestrator) EnsureConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sailbox",
			},
		},
		Data: data,
	}

	existing, err := o.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create configmap: %w", err)
			}
			o.logger.Info("created ConfigMap", slog.String("name", name), slog.String("ns", namespace))
			return nil
		}
		return err
	}

	existing.Data = data
	_, err = o.client.CoreV1().ConfigMaps(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// DeleteConfigMap deletes a ConfigMap from the given namespace.
func (o *Orchestrator) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	err := o.client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
