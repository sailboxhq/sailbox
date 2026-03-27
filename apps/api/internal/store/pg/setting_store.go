package pg

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type settingStore struct {
	db *bun.DB
}

func (s *settingStore) Get(ctx context.Context, key string) (string, error) {
	setting := new(model.Setting)
	err := s.db.NewSelect().Model(setting).Where("key = ?", key).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return setting.Value, nil
}

func (s *settingStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.NewInsert().
		Model(&model.Setting{Key: key, Value: value}).
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_at = NOW()").
		Exec(ctx)
	return err
}

func (s *settingStore) GetAll(ctx context.Context) ([]model.Setting, error) {
	var settings []model.Setting
	err := s.db.NewSelect().Model(&settings).OrderExpr("key ASC").Scan(ctx)
	return settings, err
}
