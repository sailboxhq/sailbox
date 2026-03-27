package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type deploymentStore struct {
	db *bun.DB
}

func (s *deploymentStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error) {
	deploy := new(model.Deployment)
	err := s.db.NewSelect().Model(deploy).Where("id = ?", id).Scan(ctx)
	return deploy, err
}

func (s *deploymentStore) Create(ctx context.Context, deploy *model.Deployment) error {
	_, err := s.db.NewInsert().Model(deploy).Returning("*").Exec(ctx)
	return err
}

func (s *deploymentStore) Update(ctx context.Context, deploy *model.Deployment) error {
	_, err := s.db.NewUpdate().Model(deploy).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *deploymentStore) ListByApp(ctx context.Context, appID uuid.UUID, params store.ListParams) ([]model.Deployment, int, error) {
	var deploys []model.Deployment
	count, err := s.db.NewSelect().
		Model(&deploys).
		Where("app_id = ?", appID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deploys, count, err
}

func (s *deploymentStore) ListAll(ctx context.Context, params store.ListParams, filter store.DeploymentListFilter) ([]model.Deployment, int, error) {
	var deploys []model.Deployment
	q := s.db.NewSelect().Model(&deploys)

	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	count, err := q.
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return deploys, count, err
}

func (s *deploymentStore) GetLatestByApp(ctx context.Context, appID uuid.UUID) (*model.Deployment, error) {
	deploy := new(model.Deployment)
	err := s.db.NewSelect().
		Model(deploy).
		Where("app_id = ?", appID).
		OrderExpr("created_at DESC").
		Limit(1).
		Scan(ctx)
	return deploy, err
}
