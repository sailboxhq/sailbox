package pg

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type notificationChannelStore struct {
	db *bun.DB
}

func (s *notificationChannelStore) GetByOrgAndType(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error) {
	ch := new(model.NotificationChannel)
	err := s.db.NewSelect().Model(ch).
		Where("org_id = ?", orgID).
		Where("type = ?", channelType).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *notificationChannelStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	err := s.db.NewSelect().Model(&channels).
		Where("org_id = ?", orgID).
		OrderExpr("type ASC").
		Scan(ctx)
	return channels, err
}

func (s *notificationChannelStore) ListAllEnabled(ctx context.Context) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	err := s.db.NewSelect().Model(&channels).
		Where("enabled = true").
		Scan(ctx)
	return channels, err
}

func (s *notificationChannelStore) Upsert(ctx context.Context, channel *model.NotificationChannel) error {
	existing := new(model.NotificationChannel)
	err := s.db.NewSelect().Model(existing).
		Where("org_id = ?", channel.OrgID).
		Where("type = ?", channel.Type).
		Scan(ctx)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// Insert new
		_, err = s.db.NewInsert().Model(channel).Returning("*").Exec(ctx)
		return err
	}

	// Update existing
	existing.Enabled = channel.Enabled
	existing.Config = channel.Config
	_, err = s.db.NewUpdate().Model(existing).
		Set("enabled = ?", channel.Enabled).
		Set("config = ?", channel.Config).
		Set("updated_at = NOW()").
		WherePK().
		Returning("*").
		Exec(ctx)
	return err
}
