package k3s

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) CreateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	ns := appNamespace(app)
	name := fmt.Sprintf("%s-%s", appK8sName(app), sanitize(domain.Host))
	if len(name) > 63 {
		name = name[:63]
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class": "traefik",
	}

	// TLS with Let's Encrypt (skip for localhost/dev domains)
	if domain.TLS && domain.AutoCert && !isDevDomain(domain.Host) {
		annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
		annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"] = "letsencrypt"
		annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "websecure"
	} else {
		annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "web,websecure"
	}

	// Force HTTPS redirect
	if domain.ForceHTTPS {
		annotations["traefik.ingress.kubernetes.io/router.middlewares"] = "default-redirect-https@kubernetescrd"
	}

	// Backend port = first service port (what the K8s Service exposes)
	backendPort := int32(80)
	if len(app.Ports) > 0 {
		backendPort = int32(app.Ports[0].ServicePort)
	}

	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sailbox",
				"sailbox/app-id":               app.ID.String(),
				"sailbox/domain-id":            domain.ID.String(),
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: domain.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: appK8sName(app),
											Port: networkingv1.ServiceBackendPort{
												Number: backendPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Add TLS spec (skip for dev domains)
	if domain.TLS && !isDevDomain(domain.Host) {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{domain.Host},
				SecretName: fmt.Sprintf("%s-tls", name),
			},
		}
	}

	existing, err := o.client.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = o.client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
		} else {
			return err
		}
	} else {
		existing.Spec = ingress.Spec
		existing.Annotations = annotations
		_, err = o.client.NetworkingV1().Ingresses(ns).Update(ctx, existing, metav1.UpdateOptions{})
	}

	if err != nil {
		return fmt.Errorf("create ingress: %w", err)
	}

	o.logger.Info("ingress created", slog.String("host", domain.Host), slog.String("ns", ns))
	return nil
}

// SyncIngressPorts updates the backend port on all ingresses for an app.
// Called during deploy when ports may have changed.
func (o *Orchestrator) SyncIngressPorts(ctx context.Context, app *model.Application) error {
	ns := appNamespace(app)
	ingresses, err := o.client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("sailbox/app-id=%s", app.ID.String()),
	})
	if err != nil || len(ingresses.Items) == 0 {
		return nil
	}

	backendPort := int32(80)
	if len(app.Ports) > 0 {
		bp := int32(app.Ports[0].ServicePort)
		if bp == 0 {
			bp = int32(app.Ports[0].ContainerPort)
		}
		if bp > 0 {
			backendPort = bp
		}
	}

	svcName := appK8sName(app)
	for _, ing := range ingresses.Items {
		updated := false
		for i, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for j, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil && path.Backend.Service.Name == svcName &&
					path.Backend.Service.Port.Number != backendPort {
					ing.Spec.Rules[i].HTTP.Paths[j].Backend.Service.Port.Number = backendPort
					updated = true
				}
			}
		}
		if updated {
			_, err := o.client.NetworkingV1().Ingresses(ns).Update(ctx, &ing, metav1.UpdateOptions{})
			if err != nil {
				o.logger.Warn("failed to sync ingress port",
					slog.String("ingress", ing.Name),
					slog.Any("error", err),
				)
			}
		}
	}
	return nil
}

// ── Panel Ingress ───────────────────────────────────────────────

const panelIngressName = "sailbox-panel"
const panelNamespace = "sailbox"

func (o *Orchestrator) EnsurePanelIngress(ctx context.Context, domain, httpsEmail string) error {
	annotations := map[string]string{
		"kubernetes.io/ingress.class": "traefik",
	}

	if !isDevDomain(domain) {
		annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
		annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"] = "letsencrypt"
		annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "websecure"
	} else {
		annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "web,websecure"
	}

	pathType := networkingv1.PathTypePrefix
	port := int32(3000)

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        panelIngressName,
			Namespace:   panelNamespace,
			Annotations: annotations,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sailbox",
				"app.kubernetes.io/component":  "panel",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "sailbox",
											Port: networkingv1.ServiceBackendPort{Number: port},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	existing, err := o.client.NetworkingV1().Ingresses(panelNamespace).Get(ctx, panelIngressName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = o.client.NetworkingV1().Ingresses(panelNamespace).Create(ctx, ingress, metav1.CreateOptions{})
	} else if err == nil {
		existing.Annotations = annotations
		existing.Labels = ingress.Labels
		existing.Spec = ingress.Spec
		_, err = o.client.NetworkingV1().Ingresses(panelNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	}

	if err != nil {
		return fmt.Errorf("panel ingress: %w", err)
	}

	o.logger.Info("panel ingress applied", slog.String("domain", domain))
	return nil
}

func (o *Orchestrator) DeletePanelIngress(ctx context.Context) error {
	err := o.client.NetworkingV1().Ingresses(panelNamespace).Delete(ctx, panelIngressName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete panel ingress: %w", err)
	}
	o.logger.Info("panel ingress deleted")
	return nil
}

func isDevDomain(host string) bool {
	return strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".traefik.me") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".test") ||
		strings.Contains(host, ".nip.io") ||
		strings.Contains(host, ".sslip.io")
}

func (o *Orchestrator) UpdateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	return o.CreateIngress(ctx, domain, app) // Upsert
}

func (o *Orchestrator) DeleteIngress(ctx context.Context, domain *model.Domain) error {
	// Find and delete ingress by domain label
	nsList, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: "managed-by=sailbox",
	})
	if err != nil {
		return err
	}

	for _, ns := range nsList.Items {
		ingresses, err := o.client.NetworkingV1().Ingresses(ns.Name).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("sailbox/domain-id=%s", domain.ID.String()),
		})
		if err != nil {
			continue
		}
		for _, ing := range ingresses.Items {
			_ = o.client.NetworkingV1().Ingresses(ns.Name).Delete(ctx, ing.Name, metav1.DeleteOptions{})
			o.logger.Info("ingress deleted", slog.String("name", ing.Name))
		}
	}

	return nil
}

func (o *Orchestrator) GetIngressStatus(ctx context.Context, domain *model.Domain, app *model.Application) (*orchestrator.IngressStatus, error) {
	ns := appNamespace(app)
	ingresses, err := o.client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("sailbox/domain-id=%s", domain.ID.String()),
	})
	if err != nil {
		return &orchestrator.IngressStatus{Ready: false, Message: err.Error()}, nil
	}
	if len(ingresses.Items) == 0 {
		return &orchestrator.IngressStatus{Ready: false, Message: "ingress not found"}, nil
	}
	ing := ingresses.Items[0]
	ready := len(ing.Status.LoadBalancer.Ingress) > 0
	return &orchestrator.IngressStatus{Ready: ready}, nil
}

func (o *Orchestrator) GetCertExpiry(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error) {
	if !domain.TLS || domain.CertSecret == "" {
		return nil, nil
	}
	ns := appNamespace(app)
	secret, err := o.client.CoreV1().Secrets(ns).Get(ctx, domain.CertSecret, metav1.GetOptions{})
	if err != nil {
		return nil, nil // cert secret not found, not critical
	}
	certPEM, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, nil
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil
	}
	return &cert.NotAfter, nil
}
