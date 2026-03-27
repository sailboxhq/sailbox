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

func (s *DomainService) Create(ctx context.Context, appID uuid.UUID, input CreateDomainInput) (*model.Domain, error) {
	existing, _ := s.store.Domains().GetByHost(ctx, input.Host)
	if existing != nil && existing.ID != uuid.Nil {
		return nil, errors.New("domain already in use")
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

	if err := s.store.Domains().Create(ctx, domain); err != nil {
		return nil, err
	}

	if err := s.orch.CreateIngress(ctx, domain, app); err != nil {
		// Rollback DB record if Ingress creation fails
		_ = s.store.Domains().Delete(ctx, domain.ID)
		return nil, fmt.Errorf("create ingress failed: %w", err)
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

	name := strings.ToLower(strings.ReplaceAll(app.Name, "_", "-"))
	name = strings.ReplaceAll(name, " ", "-")
	suffix := randomShort(4)
	host := fmt.Sprintf("%s-%s.%s", name, suffix, baseDomain)

	// Dev/wildcard DNS domains don't need TLS (Let's Encrypt won't work for them)
	isDev := strings.Contains(baseDomain, "nip.io") ||
		strings.Contains(baseDomain, "sslip.io") ||
		strings.Contains(baseDomain, "traefik.me") ||
		strings.HasSuffix(baseDomain, ".localhost") ||
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

	hostChanged := false
	oldHost := domain.Host
	if host != nil && *host != "" && *host != domain.Host {
		domain.Host = *host
		hostChanged = true
	}

	if forceHTTPS != nil {
		domain.ForceHTTPS = *forceHTTPS
	}

	if err := s.store.Domains().Update(ctx, domain); err != nil {
		return nil, err
	}

	if hostChanged {
		// Create new Ingress first, then delete old one (safer order)
		if err := s.orch.CreateIngress(ctx, domain, app); err != nil {
			// Rollback: restore old host in DB
			domain.Host = oldHost
			_ = s.store.Domains().Update(ctx, domain)
			return nil, fmt.Errorf("failed to create ingress for new host: %w", err)
		}
		// New Ingress created successfully, now safe to delete old one
		oldDomain := *domain
		oldDomain.Host = oldHost
		if err := s.orch.DeleteIngress(ctx, &oldDomain); err != nil {
			s.logger.Warn("failed to delete old ingress", slog.Any("error", err))
		}
	} else {
		// Just update in-place
		if err := s.orch.UpdateIngress(ctx, domain, app); err != nil {
			s.logger.Error("failed to update ingress", slog.Any("error", err))
		}
	}

	s.logger.Info("domain updated", slog.String("host", domain.Host))
	return domain, nil
}

func (s *DomainService) ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
	return s.store.Domains().ListByApp(ctx, appID)
}

func (s *DomainService) Delete(ctx context.Context, id uuid.UUID) error {
	domain, err := s.store.Domains().GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.orch.DeleteIngress(ctx, domain); err != nil {
		s.logger.Error("failed to delete ingress", slog.Any("error", err))
	}

	return s.store.Domains().Delete(ctx, id)
}

// randomShort generates a short hex string (e.g. 4 bytes → "a3f1b2c0", n=4 → "a3f1").
func randomShort(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
