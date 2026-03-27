package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type invitationStore struct {
	db *bun.DB
}

func (s *invitationStore) Create(ctx context.Context, inv *model.Invitation) error {
	_, err := s.db.NewInsert().Model(inv).Returning("*").Exec(ctx)
	return err
}

func (s *invitationStore) GetByToken(ctx context.Context, token string) (*model.Invitation, error) {
	inv := new(model.Invitation)
	err := s.db.NewSelect().Model(inv).Where("token = ?", token).Scan(ctx)
	return inv, err
}

func (s *invitationStore) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error) {
	var invitations []model.Invitation
	err := s.db.NewSelect().
		Model(&invitations).
		Where("org_id = ?", orgID).
		OrderExpr("created_at DESC").
		Scan(ctx)
	return invitations, err
}

func (s *invitationStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*model.Invitation)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *invitationStore) Update(ctx context.Context, inv *model.Invitation) error {
	_, err := s.db.NewUpdate().Model(inv).WherePK().Returning("*").Exec(ctx)
	return err
}
