package k3s

import (
	"context"
	"fmt"
	"log/slog"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const sailboxNetPolName = "sailbox-isolation"

// EnsureNetworkPolicy creates or removes a default-deny NetworkPolicy for namespace isolation.
func (o *Orchestrator) EnsureNetworkPolicy(ctx context.Context, namespace string, enabled bool) error {
	if !enabled {
		// Remove network policy if it exists
		err := o.client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, sailboxNetPolName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		o.logger.Info("removed NetworkPolicy", slog.String("ns", namespace))
		return nil
	}

	// Default deny ingress from other namespaces, allow within namespace
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sailboxNetPolName,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "sailbox"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			// Apply to all pods in this namespace
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow traffic from same namespace
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": namespace,
								},
							},
						},
					},
				},
				{
					// Allow traffic from kube-system (for metrics, DNS, etc.)
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
						},
					},
				},
			},
		},
	}

	existing, err := o.client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, sailboxNetPolName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.NetworkingV1().NetworkPolicies(namespace).Create(ctx, netpol, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create network policy: %w", err)
			}
			o.logger.Info("created NetworkPolicy", slog.String("ns", namespace))
			return nil
		}
		return err
	}

	existing.Spec = netpol.Spec
	_, err = o.client.NetworkingV1().NetworkPolicies(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}
