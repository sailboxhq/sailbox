package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type organizationStore struct {
	db *bun.DB
}

func (s *organizationStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	org := new(model.Organization)
	err := s.db.NewSelect().Model(org).Where("id = ?", id).Scan(ctx)
	return org, err
}

func (s *organizationStore) Create(ctx context.Context, org *model.Organization) error {
	_, err := s.db.NewInsert().Model(org).Returning("*").Exec(ctx)
	return err
}

func (s *organizationStore) Update(ctx context.Context, org *model.Organization) error {
	_, err := s.db.NewUpdate().Model(org).WherePK().Returning("*").Exec(ctx)
	return err
}
