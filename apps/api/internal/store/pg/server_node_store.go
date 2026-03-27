package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

type serverNodeStore struct {
	db *bun.DB
}

func (s *serverNodeStore) GetByID(ctx context.Context, id uuid.UUID) (*model.ServerNode, error) {
	node := new(model.ServerNode)
	err := s.db.NewSelect().Model(node).Where("id = ?", id).Scan(ctx)
	return node, err
}

func (s *serverNodeStore) Create(ctx context.Context, node *model.ServerNode) error {
	_, err := s.db.NewInsert().Model(node).Exec(ctx)
	return err
}

func (s *serverNodeStore) Update(ctx context.Context, node *model.ServerNode) error {
	_, err := s.db.NewUpdate().Model(node).WherePK().Exec(ctx)
	return err
}

func (s *serverNodeStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.ServerNode)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *serverNodeStore) List(ctx context.Context) ([]model.ServerNode, error) {
	var nodes []model.ServerNode
	err := s.db.NewSelect().Model(&nodes).OrderExpr("created_at DESC").Scan(ctx)
	return nodes, err
}

func (s *serverNodeStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.NodeStatus, msg string) error {
	_, err := s.db.NewUpdate().
		Model((*model.ServerNode)(nil)).
		Set("status = ?", status).
		Set("status_msg = ?", msg).
		Where("id = ?", id).
		Exec(ctx)
	return err
}
