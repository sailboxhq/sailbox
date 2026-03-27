package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type domainStore struct {
	db *bun.DB
}

func (s *domainStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Domain, error) {
	domain := new(model.Domain)
	err := s.db.NewSelect().Model(domain).Where("id = ?", id).Scan(ctx)
	return domain, err
}

func (s *domainStore) Create(ctx context.Context, domain *model.Domain) error {
	_, err := s.db.NewInsert().Model(domain).Returning("*").Exec(ctx)
	return err
}

func (s *domainStore) Update(ctx context.Context, domain *model.Domain) error {
	_, err := s.db.NewUpdate().Model(domain).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *domainStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.Domain)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *domainStore) ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error) {
	var domains []model.Domain
	err := s.db.NewSelect().Model(&domains).Where("app_id = ?", appID).Scan(ctx)
	return domains, err
}

func (s *domainStore) GetByHost(ctx context.Context, host string) (*model.Domain, error) {
	domain := new(model.Domain)
	err := s.db.NewSelect().Model(domain).Where("host = ?", host).Scan(ctx)
	return domain, err
}
