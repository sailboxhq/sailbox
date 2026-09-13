package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type applicationStore struct {
	db bun.IDB
}

func (s *applicationStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	app := new(model.Application)
	err := s.db.NewSelect().
		Model(app).
		Where("id = ?", id).
		Scan(ctx)
	return app, err
}

func (s *applicationStore) Create(ctx context.Context, app *model.Application) error {
	_, err := s.db.NewInsert().Model(app).Returning("*").Exec(ctx)
	return err
}

func (s *applicationStore) Update(ctx context.Context, app *model.Application) error {
	_, err := s.db.NewUpdate().Model(app).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *applicationStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.AppStatus) error {
	_, err := s.db.NewUpdate().
		Model((*model.Application)(nil)).
		Set("status = ?", status).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *applicationStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Application)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *applicationStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.Application, int, error) {
	var apps []model.Application
	count, err := s.db.NewSelect().
		Model(&apps).
		Where("project_id = ?", projectID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return apps, count, err
}

func (s *applicationStore) ListAll(ctx context.Context, params store.ListParams, filter store.AppListFilter) ([]model.Application, int, error) {
	var apps []model.Application
	q := s.db.NewSelect().Model(&apps)

	// Scope to the caller's org, and skip apps whose project has been deleted —
	// this listing is what feeds the dashboard and global search.
	if filter.OrgID != uuid.Nil {
		q = q.Where("project_id IN (SELECT id FROM projects WHERE org_id = ? AND deleted_at IS NULL)", filter.OrgID)
	}
	if filter.ProjectIDs != nil {
		if len(filter.ProjectIDs) == 0 {
			return []model.Application{}, 0, nil
		}
		q = q.Where("project_id IN (?)", bun.List(filter.ProjectIDs))
	}
	if filter.Search != "" {
		q = q.Where("LOWER(name) LIKE LOWER(?)", "%"+filter.Search+"%")
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	count, err := q.
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return apps, count, err
}

func (s *applicationStore) DeleteByProject(ctx context.Context, projectID uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Application)(nil)).Where("project_id = ?", projectID).Exec(ctx)
	return err
}
