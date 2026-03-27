package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type projectStore struct {
	db *bun.DB
}

func (s *projectStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	project := new(model.Project)
	err := s.db.NewSelect().Model(project).Where("id = ?", id).Scan(ctx)
	return project, err
}

func (s *projectStore) Create(ctx context.Context, project *model.Project) error {
	_, err := s.db.NewInsert().Model(project).Returning("*").Exec(ctx)
	return err
}

func (s *projectStore) Update(ctx context.Context, project *model.Project) error {
	_, err := s.db.NewUpdate().Model(project).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *projectStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Project)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *projectStore) ListByOrg(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.Project, int, error) {
	var projects []model.Project
	count, err := s.db.NewSelect().
		Model(&projects).
		Where("org_id = ?", orgID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return projects, count, err
}
