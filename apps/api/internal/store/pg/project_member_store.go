package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type projectMemberStore struct {
	db *bun.DB
}

func (s *projectMemberStore) Create(ctx context.Context, pm *model.ProjectMember) error {
	_, err := s.db.NewInsert().Model(pm).Returning("*").Exec(ctx)
	return err
}

func (s *projectMemberStore) Delete(ctx context.Context, projectID, userID uuid.UUID) error {
	_, err := s.db.NewDelete().
		Model((*model.ProjectMember)(nil)).
		Where("project_id = ?", projectID).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}

func (s *projectMemberStore) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.ProjectMember, error) {
	var members []model.ProjectMember
	err := s.db.NewSelect().
		Model(&members).
		Where("project_id = ?", projectID).
		OrderExpr("created_at ASC").
		Scan(ctx)
	return members, err
}

func (s *projectMemberStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.ProjectMember, error) {
	var members []model.ProjectMember
	err := s.db.NewSelect().
		Model(&members).
		Where("user_id = ?", userID).
		OrderExpr("created_at ASC").
		Scan(ctx)
	return members, err
}

func (s *projectMemberStore) GetByProjectAndUser(ctx context.Context, projectID, userID uuid.UUID) (*model.ProjectMember, error) {
	pm := new(model.ProjectMember)
	err := s.db.NewSelect().
		Model(pm).
		Where("project_id = ?", projectID).
		Where("user_id = ?", userID).
		Scan(ctx)
	return pm, err
}
