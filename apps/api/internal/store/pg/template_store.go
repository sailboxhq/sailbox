package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type templateStore struct {
	db *bun.DB
}

func (s *templateStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error) {
	tpl := new(model.Template)
	err := s.db.NewSelect().Model(tpl).Where("id = ?", id).Scan(ctx)
	return tpl, err
}

func (s *templateStore) GetByName(ctx context.Context, name string) (*model.Template, error) {
	tpl := new(model.Template)
	err := s.db.NewSelect().Model(tpl).Where("name = ?", name).Scan(ctx)
	return tpl, err
}

func (s *templateStore) List(ctx context.Context, params store.ListParams) ([]model.Template, int, error) {
	var templates []model.Template
	count, err := s.db.NewSelect().
		Model(&templates).
		OrderExpr("name ASC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return templates, count, err
}

func (s *templateStore) Create(ctx context.Context, tpl *model.Template) error {
	_, err := s.db.NewInsert().Model(tpl).Returning("*").Exec(ctx)
	return err
}
