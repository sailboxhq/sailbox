package k3s

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) CreateIngress(ctx context.Context, domain *model.Domain, app *model.Application) error {
	ns := appNamespace(app)
	name := ingressName(appK8sName(app), domain.Host)

	annotations := map[string]string{
		"kubernetes.io/ingress.class": "traefik",
	}

	// TLS with Let's Encrypt (skip for localhost/dev domains)
	if domain.TLS && domain.AutoCert && !isDevDomain(domain.Host) {
		annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
		annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"] = "letsencrypt"
	}
	// Always listen on both HTTP and HTTPS so browsers can reach the domain
	// regardless of which protocol the user types. Force HTTPS (below) handles redirect.
	annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "web,websecure"

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
	// Note: no SecretName — Traefik's certresolver manages certs in acme.json,
	// not via K8s Secrets. Setting SecretName would cause Traefik to look for a
	// non-existent secret and fall back to its default self-signed cert.
	if domain.TLS && !isDevDomain(domain.Host) {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts: []string{domain.Host},
			},
		}
	}

	existing, err := o.client.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// Check for legacy ingress name (pre-hash scheme, truncated at 63)
		legacy := legacyIngressName(appK8sName(app), domain.Host)
		if legacy != name {
			_, legacyErr := o.client.NetworkingV1().Ingresses(ns).Get(ctx, legacy, metav1.GetOptions{})
			if legacyErr == nil {
				// Legacy ingress exists — migrate: create new first, then delete legacy
				if _, createErr := o.client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{}); createErr == nil {
					if delErr := o.client.NetworkingV1().Ingresses(ns).Delete(ctx, legacy, metav1.DeleteOptions{}); delErr != nil && !k8serrors.IsNotFound(delErr) {
						o.logger.Warn("legacy ingress migration: failed to delete old ingress, duplicate may remain",
							slog.String("legacy", legacy), slog.Any("error", delErr))
					}
					o.logger.Info("migrated legacy ingress", slog.String("old", legacy), slog.String("new", name))
					err = nil
				} else {
					// New name create failed — keep legacy ingress working, update it in-place
					legacyIng, _ := o.client.NetworkingV1().Ingresses(ns).Get(ctx, legacy, metav1.GetOptions{})
					if legacyIng != nil {
						legacyIng.Spec = ingress.Spec
						legacyIng.Annotations = annotations
						_, err = o.client.NetworkingV1().Ingresses(ns).Update(ctx, legacyIng, metav1.UpdateOptions{})
					} else {
						err = createErr
					}
				}
			} else if k8serrors.IsNotFound(legacyErr) {
				// No legacy ingress — create normally
				_, err = o.client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
			} else {
				// Transient K8s API error — don't mask it
				return fmt.Errorf("check legacy ingress %s: %w", legacy, legacyErr)
			}
		} else {
			_, err = o.client.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
		}
	} else if err != nil {
		return err
	} else {
		// Update existing hashed ingress, and clean up any leftover legacy ingress
		legacy := legacyIngressName(appK8sName(app), domain.Host)
		if legacy != name {
			if delErr := o.client.NetworkingV1().Ingresses(ns).Delete(ctx, legacy, metav1.DeleteOptions{}); delErr == nil {
				o.logger.Info("cleaned up leftover legacy ingress during update", slog.String("legacy", legacy))
			}
		}
		existing.Spec = ingress.Spec
		existing.Annotations = annotations
		_, err = o.client.NetworkingV1().Ingresses(ns).Update(ctx, existing, metav1.UpdateOptions{})
	}

	if err != nil {
		return fmt.Errorf("create ingress: %w", err)
	}

	// Mark that TLS is managed by Traefik ACME (cert stored in acme.json, not K8s Secret)
	if domain.TLS && !isDevDomain(domain.Host) {
		domain.CertSecret = "traefik-acme"
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
	// Ensure the panel namespace exists
	if err := o.ensureNamespace(ctx, panelNamespace); err != nil {
		return fmt.Errorf("panel ingress: %w", err)
	}

	// Ensure K8s Service + Endpoints pointing to the host (Sailbox runs in Docker, not K8s)
	if err := o.ensurePanelService(ctx); err != nil {
		return fmt.Errorf("panel service: %w", err)
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class": "traefik",
	}

	if !isDevDomain(domain) {
		annotations["traefik.ingress.kubernetes.io/router.tls"] = "true"
		annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"] = "letsencrypt"
		// Panel must always force HTTPS in production — no plaintext admin access
		annotations["traefik.ingress.kubernetes.io/router.middlewares"] = "default-redirect-https@kubernetescrd"
	}
	annotations["traefik.ingress.kubernetes.io/router.entrypoints"] = "web,websecure"

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

	// Add TLS block when HTTPS is enabled (Traefik ACME manages the cert)
	if httpsEmail != "" {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{Hosts: []string{domain}},
		}
	}

	existing, err := o.client.NetworkingV1().Ingresses(panelNamespace).Get(ctx, panelIngressName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		_, err = o.client.NetworkingV1().Ingresses(panelNamespace).Create(ctx, ingress, metav1.CreateOptions{})
	} else if err == nil {
		existing.Annotations = annotations
		existing.Spec.TLS = ingress.Spec.TLS
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

// ensurePanelService creates a headless Service + Endpoints in the panel namespace
// pointing to the host's IP:3000 (Sailbox runs as a Docker container, not a K8s Pod).
func (o *Orchestrator) ensurePanelService(ctx context.Context) error {
	svcName := "sailbox"
	port := int32(3000)

	// Get node IP for the endpoint
	nodes, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return fmt.Errorf("no nodes found")
	}
	var nodeIP string
	for _, addr := range nodes.Items[0].Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			nodeIP = addr.Address
			break
		}
	}
	if nodeIP == "" {
		return fmt.Errorf("cannot determine node IP")
	}

	// Ensure Service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: panelNamespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "sailbox"},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{Port: port, TargetPort: intstr.FromInt32(port)}},
		},
	}
	_, err = o.client.CoreV1().Services(panelNamespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("create panel service: %w", err)
	}

	// Ensure Endpoints
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: panelNamespace,
		},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{IP: nodeIP}},
			Ports:     []corev1.EndpointPort{{Port: port}},
		}},
	}
	_, err = o.client.CoreV1().Endpoints(panelNamespace).Create(ctx, ep, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		_, err = o.client.CoreV1().Endpoints(panelNamespace).Update(ctx, ep, metav1.UpdateOptions{})
	}
	if err != nil {
		return fmt.Errorf("create panel endpoints: %w", err)
	}
	return nil
}

func (o *Orchestrator) DeletePanelIngress(ctx context.Context) error {
	err := o.client.NetworkingV1().Ingresses(panelNamespace).Delete(ctx, panelIngressName, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
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

func (o *Orchestrator) IngressName(app *model.Application, host string) string {
	return ingressName(appK8sName(app), host)
}

func (o *Orchestrator) LegacyIngressName(app *model.Application, host string) string {
	return legacyIngressName(appK8sName(app), host)
}

func ingressName(appName, host string) string {
	sanitized := fmt.Sprintf("%s-%s", appName, sanitize(host))
	if len(sanitized) <= 63 {
		return sanitized
	}
	// Hash from the full unsanitized input to avoid collisions when sanitize() truncates
	full := fmt.Sprintf("%s-%s", appName, host)
	h := sha256.Sum256([]byte(full))
	suffix := hex.EncodeToString(h[:4]) // 8 chars
	return sanitized[:63-9] + "-" + suffix
}

// legacyIngressName returns the old truncate-at-63 name used before the hash scheme.
// Used as fallback when looking up existing ingresses from older installations.
func legacyIngressName(appName, host string) string {
	name := fmt.Sprintf("%s-%s", appName, sanitize(host))
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func (o *Orchestrator) DeleteIngressByName(ctx context.Context, app *model.Application, name string) error {
	ns := appNamespace(app)
	err := o.client.NetworkingV1().Ingresses(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete ingress %s: %w", name, err)
	}
	o.logger.Info("ingress deleted by name", slog.String("name", name), slog.String("ns", ns))
	return nil
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

	var lastErr error
	for _, ns := range nsList.Items {
		ingresses, err := o.client.NetworkingV1().Ingresses(ns.Name).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("sailbox/domain-id=%s", domain.ID.String()),
		})
		if err != nil {
			continue
		}
		for _, ing := range ingresses.Items {
			if err := o.client.NetworkingV1().Ingresses(ns.Name).Delete(ctx, ing.Name, metav1.DeleteOptions{}); err != nil {
				if !k8serrors.IsNotFound(err) {
					lastErr = fmt.Errorf("delete ingress %s: %w", ing.Name, err)
					o.logger.Error("failed to delete ingress", slog.String("name", ing.Name), slog.Any("error", err))
				}
			} else {
				o.logger.Info("ingress deleted", slog.String("name", ing.Name))
			}
		}
	}

	return lastErr
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
	// Extract TLS secret name if present
	certSecret := ""
	if len(ing.Spec.TLS) > 0 {
		certSecret = ing.Spec.TLS[0].SecretName
	}
	return &orchestrator.IngressStatus{Ready: ready, CertSecret: certSecret}, nil
}

func (o *Orchestrator) GetCertExpiry(ctx context.Context, domain *model.Domain, app *model.Application) (*time.Time, error) {
	if !domain.TLS || domain.CertSecret == "" {
		return nil, nil
	}

	// For Traefik ACME-managed certs, check via TLS handshake (cert is in acme.json, not K8s Secret)
	if domain.CertSecret == "traefik-acme" {
		return getCertExpiryViaTLS(domain.Host)
	}

	// Fallback: check K8s Secret (for cert-manager or manual TLS)
	ns := appNamespace(app)
	secret, err := o.client.CoreV1().Secrets(ns).Get(ctx, domain.CertSecret, metav1.GetOptions{})
	if err != nil {
		return nil, nil
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

// getCertExpiryViaTLS reads the cert expiry via TLS handshake.
// Tries localhost first (bypasses CDN), falls back to public hostname.
func getCertExpiryViaTLS(host string) (*time.Time, error) {
	for _, addr := range []string{"127.0.0.1:443", host + ":443"} {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 3 * time.Second},
			"tcp", addr,
			&tls.Config{InsecureSkipVerify: true, ServerName: host},
		)
		if err != nil {
			continue
		}
		certs := conn.ConnectionState().PeerCertificates
		conn.Close()
		if len(certs) == 0 {
			continue
		}
		// Skip Traefik default self-signed cert (ACME not yet issued)
		if certs[0].Issuer.CommonName == "TRAEFIK DEFAULT CERT" {
			return nil, nil
		}
		// Skip Cloudflare edge certs — they don't reflect origin cert state
		if len(certs[0].Issuer.Organization) > 0 && strings.Contains(certs[0].Issuer.Organization[0], "Cloudflare") {
			return nil, nil
		}
		return &certs[0].NotAfter, nil
	}
	return nil, nil
}
