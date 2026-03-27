package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type databaseBackupStore struct {
	db *bun.DB
}

func (s *databaseBackupStore) Create(ctx context.Context, backup *model.DatabaseBackup) error {
	_, err := s.db.NewInsert().Model(backup).Returning("*").Exec(ctx)
	return err
}

func (s *databaseBackupStore) Update(ctx context.Context, backup *model.DatabaseBackup) error {
	_, err := s.db.NewUpdate().Model(backup).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *databaseBackupStore) GetByID(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error) {
	backup := new(model.DatabaseBackup)
	err := s.db.NewSelect().Model(backup).Where("id = ?", id).Scan(ctx)
	return backup, err
}

func (s *databaseBackupStore) ListByDatabase(ctx context.Context, databaseID uuid.UUID, params store.ListParams) ([]model.DatabaseBackup, int, error) {
	var backups []model.DatabaseBackup
	count, err := s.db.NewSelect().
		Model(&backups).
		Where("database_id = ?", databaseID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return backups, count, err
}
