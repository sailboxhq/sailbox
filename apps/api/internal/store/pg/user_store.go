package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type userStore struct {
	db bun.IDB
}

func (s *userStore) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user := new(model.User)
	err := s.db.NewSelect().Model(user).Where("id = ?", id).Scan(ctx)
	return user, err
}

func (s *userStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := new(model.User)
	err := s.db.NewSelect().Model(user).Where("email = ?", email).Scan(ctx)
	return user, err
}

func (s *userStore) Create(ctx context.Context, user *model.User) error {
	_, err := s.db.NewInsert().Model(user).Returning("*").Exec(ctx)
	return err
}

func (s *userStore) Update(ctx context.Context, user *model.User) error {
	_, err := s.db.NewUpdate().Model(user).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *userStore) ListByOrg(ctx context.Context, orgID uuid.UUID, params store.ListParams) ([]model.User, int, error) {
	var users []model.User
	count, err := s.db.NewSelect().
		Model(&users).
		Where("org_id = ?", orgID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return users, count, err
}

func (s *userStore) UpdateRole(ctx context.Context, userID uuid.UUID, role string) error {
	_, err := s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("role = ?", role).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (s *userStore) Delete(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*model.User)(nil)).
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

// IncrementTokenVersion invalidates the user's issued refresh tokens.
func (s *userStore) IncrementTokenVersion(ctx context.Context, userID uuid.UUID) error {
	_, err := s.db.NewUpdate().
		Model((*model.User)(nil)).
		Set("token_version = token_version + 1").
		Where("id = ?", userID).
		Exec(ctx)
	return err
}

func (s *userStore) Count(ctx context.Context) (int, error) {
	return s.db.NewSelect().Model((*model.User)(nil)).Count(ctx)
}
