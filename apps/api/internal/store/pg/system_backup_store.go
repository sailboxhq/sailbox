package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type systemBackupStore struct {
	db *bun.DB
}

func (s *systemBackupStore) Create(ctx context.Context, backup *model.SystemBackup) error {
	_, err := s.db.NewInsert().Model(backup).Returning("*").Exec(ctx)
	return err
}

func (s *systemBackupStore) Update(ctx context.Context, backup *model.SystemBackup) error {
	_, err := s.db.NewUpdate().Model(backup).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *systemBackupStore) GetByID(ctx context.Context, id uuid.UUID) (*model.SystemBackup, error) {
	backup := new(model.SystemBackup)
	err := s.db.NewSelect().Model(backup).Where("id = ?", id).Scan(ctx)
	return backup, err
}

func (s *systemBackupStore) List(ctx context.Context, params store.ListParams) ([]model.SystemBackup, int, error) {
	var backups []model.SystemBackup
	count, err := s.db.NewSelect().
		Model(&backups).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return backups, count, err
}
