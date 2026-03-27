package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type sharedResourceStore struct {
	db *bun.DB
}

func (s *sharedResourceStore) GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
	resource := new(model.SharedResource)
	err := s.db.NewSelect().Model(resource).Where("id = ?", id).Scan(ctx)
	return resource, err
}

func (s *sharedResourceStore) Create(ctx context.Context, resource *model.SharedResource) error {
	_, err := s.db.NewInsert().Model(resource).Exec(ctx)
	return err
}

func (s *sharedResourceStore) Update(ctx context.Context, resource *model.SharedResource) error {
	_, err := s.db.NewUpdate().Model(resource).WherePK().Exec(ctx)
	return err
}

func (s *sharedResourceStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.SharedResource)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *sharedResourceStore) ListByOrg(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error) {
	var resources []model.SharedResource
	q := s.db.NewSelect().Model(&resources).Where("org_id = ?", orgID)
	if resourceType != "" {
		q = q.Where("type = ?", resourceType)
	}
	err := q.OrderExpr("created_at DESC").Scan(ctx)
	return resources, err
}
