package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type TemplateService struct {
	store  store.Store
	orch   orchestrator.Orchestrator
	logger *slog.Logger
}

func NewTemplateService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger) *TemplateService {
	return &TemplateService{store: s, orch: orch, logger: logger}
}

func (s *TemplateService) List(ctx context.Context, params store.ListParams) ([]model.Template, int, error) {
	return s.store.Templates().List(ctx, params)
}

func (s *TemplateService) GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error) {
	return s.store.Templates().GetByID(ctx, id)
}
