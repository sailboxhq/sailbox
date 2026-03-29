package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type DomainService struct {
	store      store.Store
	orch       orchestrator.Orchestrator
	logger     *slog.Logger
	settingSvc *SettingService
}

func NewDomainService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger, settingSvc *SettingService) *DomainService {
	return &DomainService{store: s, orch: orch, logger: logger, settingSvc: settingSvc}
}

type CreateDomainInput struct {
	Host     string `json:"host" binding:"required"`
	TLS      bool   `json:"tls"`
	AutoCert bool   `json:"auto_cert"`
}

func normalizeDomainHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.ToLower(host)
	host = strings.TrimRight(host, ".")
	return host
}

func validateDomainHost(host string) error {
	if host == "" {
		return errors.New("hostname cannot be empty")
	}
	if len(host) > 253 {
		return errors.New("hostname must be 253 characters or fewer")
	}
	// Each label must be 1-63 chars, alphanumeric or hyphens, not start/end with hyphen
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("each label must be 1-63 characters, got %q", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("label %q must not start or end with a hyphen", label)
		}
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return fmt.Errorf("invalid character %q in hostname", c)
			}
		}
	}
	if len(labels) < 2 && host != "localhost" {
		return errors.New("hostname must have at least two labels (e.g. app.example.com)")
	}
	return nil
}

func (s *DomainService) Create(ctx context.Context, appID uuid.UUID, input CreateDomainInput) (*model.Domain, error) {
	input.Host = normalizeDomainHost(input.Host)
	if err := validateDomainHost(input.Host); err != nil {
		return nil, err
	}

	app, err := s.store.Applications().GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	domain := &model.Domain{
		AppID:    appID,
		Host:     input.Host,
		TLS:      input.TLS,
		AutoCert: input.AutoCert,
	}

	// DB insert relies on partial unique index (idx_domains_host_active) as the
	// authoritative duplicate check, avoiding TOCTOU race conditions.
	if err := s.store.Domains().Create(ctx, domain); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, errors.New("domain already in use")
		}
		return nil, err
	}

	if err := s.orch.CreateIngress(ctx, domain, app); err != nil {
		// Rollback DB record if Ingress creation fails
		_ = s.store.Domains().Delete(ctx, domain.ID)
		return nil, fmt.Errorf("create ingress failed: %w", err)
	}

	// Sync initial ingress status
	if status, sErr := s.orch.GetIngressStatus(ctx, domain, app); sErr == nil {
		domain.IngressReady = status.Ready
		_ = s.store.Domains().Update(ctx, domain)
	}

	s.logger.Info("domain created", slog.String("host", domain.Host), slog.String("app", app.Name))
	return domain, nil
}

// GenerateTraefikDomain creates an auto-generated <app>-<id>.baseDomain domain.
// If the app already has an auto-generated domain for the current base domain, it is returned instead.
func (s *DomainService) GenerateTraefikDomain(ctx context.Context, appID uuid.UUID) (*model.Domain, error) {
	baseDomain := s.settingSvc.GetBaseDomain(ctx)
	if baseDomain == "" {
		return nil, errors.New("base domain not configured — go to Settings to set it up")
	}

	app, err := s.store.Applications().GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	// Check if app already has an auto-generated domain for this base domain
	existing, _ := s.store.Domains().ListByApp(ctx, appID)
	for _, d := range existing {
		if strings.HasSuffix(d.Host, "."+baseDomain) {
			// Ensure Ingress exists (may have been cleaned up)
			_ = s.orch.CreateIngress(ctx, &d, app)
			return &d, nil
		}
	}

	// Sanitize app name for DNS label: only a-z, 0-9, hyphens
	name := strings.ToLower(app.Name)
	var sanitized []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			sanitized = append(sanitized, c)
		} else {
			sanitized = append(sanitized, '-')
		}
	}
	name = strings.Trim(string(sanitized), "-")
	if name == "" {
		name = "app"
	}
	// Limit prefix length so full host stays under 63-char label limit
	if len(name) > 40 {
		name = name[:40]
	}
	suffix := randomShort(4)
	host := fmt.Sprintf("%s-%s.%s", name, suffix, baseDomain)

	// Dev/wildcard DNS domains don't need TLS (Let's Encrypt won't work for them)
	isDev := strings.Contains(baseDomain, "nip.io") ||
		strings.Contains(baseDomain, "sslip.io") ||
		strings.Contains(baseDomain, "traefik.me") ||
		strings.HasSuffix(baseDomain, ".localhost") ||
		strings.HasSuffix(baseDomain, ".local") ||
		strings.HasSuffix(baseDomain, ".test") ||
		baseDomain == "localhost"
	useTLS := !isDev
	s.logger.Info("generate domain", slog.String("baseDomain", baseDomain), slog.Bool("isDev", isDev), slog.Bool("useTLS", useTLS))

	domain := &model.Domain{
		AppID:    appID,
		Host:     host,
		TLS:      useTLS,
		AutoCert: useTLS,
	}

	if err := s.store.Domains().Create(ctx, domain); err != nil {
		return nil, err
	}

	if err := s.orch.CreateIngress(ctx, domain, app); err != nil {
		_ = s.store.Domains().Delete(ctx, domain.ID)
		return nil, fmt.Errorf("create ingress failed: %w", err)
	}

	// Sync ingress status and cert secret
	if status, sErr := s.orch.GetIngressStatus(ctx, domain, app); sErr == nil {
		domain.IngressReady = status.Ready
	}
	_ = s.store.Domains().Update(ctx, domain)

	s.logger.Info("generated traefik domain", slog.String("host", host))
	return domain, nil
}

func (s *DomainService) Update(ctx context.Context, id uuid.UUID, host *string, forceHTTPS *bool) (*model.Domain, error) {
	domain, err := s.store.Domains().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	app, err := s.store.Applications().GetByID(ctx, domain.AppID)
	if err != nil {
		return nil, err
	}

	// Save original state for rollback before any mutation
	oldHost := domain.Host
	oldForceHTTPS := domain.ForceHTTPS

	hostChanged := false
	if host != nil && *host != "" {
		normalized := normalizeDomainHost(*host)
		if err := validateDomainHost(normalized); err != nil {
			return nil, err
		}
		host = &normalized
	}
	if host != nil && *host != "" && *host != domain.Host {
		// Block rename for manual-cert domains (tls=true, auto_cert=false)
		if domain.TLS && !domain.AutoCert {
			return nil, errors.New("cannot rename a domain with a manually configured TLS certificate")
		}
		domain.Host = *host
		hostChanged = true
	}

	if forceHTTPS != nil {
		domain.ForceHTTPS = *forceHTTPS
	}

	if hostChanged {
		// Order: update DB first (uniqueness check) → create new ingress → delete old ingress.
		// DB-first prevents creating a K8s ingress for a host that's already taken.

		// Step 1: Update DB (partial unique index enforces no duplicate active hosts)
		if err := s.store.Domains().Update(ctx, domain); err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				return nil, errors.New("domain already in use")
			}
			return nil, err
		}

		// Step 2: Create new ingress
		if err := s.orch.CreateIngress(ctx, domain, app); err != nil {
			// Rollback DB: restore all changed fields
			domain.Host = oldHost
			domain.ForceHTTPS = oldForceHTTPS
			if rbErr := s.store.Domains().Update(ctx, domain); rbErr != nil {
				s.logger.Error("CRITICAL: failed to rollback domain after ingress creation failure — DB may be inconsistent",
					slog.String("domain_id", domain.ID.String()),
					slog.String("stuck_host", *host),
					slog.String("original_host", oldHost),
					slog.Any("rollback_error", rbErr),
				)
				return nil, fmt.Errorf("ingress creation failed and rollback failed — manual fix required for domain %s", domain.ID)
			}
			return nil, fmt.Errorf("failed to create ingress for new host: %w", err)
		}

		// Step 3: Delete old ingress (safe — DB + new ingress already committed)
		// Try current naming scheme first, then legacy (pre-hash truncated) name
		oldIngressName := s.orch.IngressName(app, oldHost)
		if err := s.orch.DeleteIngressByName(ctx, app, oldIngressName); err != nil {
			s.logger.Warn("failed to delete old ingress by current name",
				slog.String("old_host", oldHost), slog.Any("error", err))
		}
		legacyName := s.orch.LegacyIngressName(app, oldHost)
		if legacyName != oldIngressName {
			if err := s.orch.DeleteIngressByName(ctx, app, legacyName); err != nil {
				s.logger.Warn("failed to delete legacy ingress",
					slog.String("old_host", oldHost), slog.Any("error", err))
			}
		}

		// Sync ingress status
		if status, sErr := s.orch.GetIngressStatus(ctx, domain, app); sErr == nil {
			domain.IngressReady = status.Ready
			_ = s.store.Domains().Update(ctx, domain)
		}
	} else {
		// Non-host changes (e.g. force_https toggle)
		if err := s.store.Domains().Update(ctx, domain); err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				return nil, errors.New("domain already in use")
			}
			return nil, err
		}
		// Just update in-place
		if err := s.orch.UpdateIngress(ctx, domain, app); err != nil {
			s.logger.Error("failed to update ingress", slog.Any("error", err))
			return nil, fmt.Errorf("domain saved, but ingress not updated: %w", err)
		}
	}

	s.logger.Info("domain updated", slog.String("host", domain.Host))
	return domain, nil
}

func (s *DomainService) ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
	domains, err := s.store.Domains().ListByApp(ctx, appID)
	if err != nil {
		return nil, err
	}

	// Sync live ingress/cert status from K8s
	app, appErr := s.store.Applications().GetByID(ctx, appID)
	if appErr != nil {
		return domains, nil // return stale data if app lookup fails
	}
	for i := range domains {
		changed := false

		// Check ingress ready
		status, sErr := s.orch.GetIngressStatus(ctx, &domains[i], app)
		if sErr == nil && status.Ready != domains[i].IngressReady {
			domains[i].IngressReady = status.Ready
			changed = true
		}

		// Migrate CertSecret to traefik-acme (Traefik manages certs, not K8s Secrets)
		if domains[i].TLS && domains[i].CertSecret != "traefik-acme" {
			domains[i].CertSecret = "traefik-acme"
			changed = true
		}

		// Check cert expiry — only update if actually changed
		if domains[i].TLS && domains[i].CertSecret != "" {
			expiry, cErr := s.orch.GetCertExpiry(ctx, &domains[i], app)
			if cErr == nil && expiry != nil {
				// Only mark changed if expiry is new or different
				if domains[i].CertExpiry == nil || !expiry.Equal(*domains[i].CertExpiry) {
					domains[i].CertExpiry = expiry
					changed = true
				}
			}
		}

		if changed {
			_ = s.store.Domains().Update(ctx, &domains[i])
		}
	}

	return domains, nil
}

func (s *DomainService) Delete(ctx context.Context, id uuid.UUID) error {
	domain, err := s.store.Domains().GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.orch.DeleteIngress(ctx, domain); err != nil {
		s.logger.Error("failed to delete ingress", slog.Any("error", err))
		return fmt.Errorf("failed to remove ingress from cluster: %w — domain not deleted", err)
	}

	return s.store.Domains().Delete(ctx, id)
}

// randomShort generates a short hex string (e.g. 4 bytes → "a3f1b2c0", n=4 → "a3f1").
func randomShort(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
