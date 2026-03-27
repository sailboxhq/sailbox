package k3s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (o *Orchestrator) EnsureServiceAccount(ctx context.Context, namespace, name string) error {
	// Create ServiceAccount if it doesn't exist
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"managed-by": "sailbox"},
		},
	}
	_, err := o.client.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create service account: %w", err)
	}

	// Create RoleBinding binding the edit ClusterRole to this SA
	rbName := name + "-edit"
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: namespace,
			Labels:    map[string]string{"managed-by": "sailbox"},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "edit",
		},
	}
	_, err = o.client.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("create role binding: %w", err)
	}

	return nil
}

func (o *Orchestrator) DeleteServiceAccount(ctx context.Context, namespace, name string) error {
	// Delete RoleBinding
	rbName := name + "-edit"
	err := o.client.RbacV1().RoleBindings(namespace).Delete(ctx, rbName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete role binding: %w", err)
	}

	// Delete ServiceAccount
	err = o.client.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete service account: %w", err)
	}

	return nil
}
