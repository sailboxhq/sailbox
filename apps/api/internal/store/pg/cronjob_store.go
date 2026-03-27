package pg

import (
	"context"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type cronJobStore struct {
	db *bun.DB
}

func (s *cronJobStore) GetByID(ctx context.Context, id uuid.UUID) (*model.CronJob, error) {
	cj := new(model.CronJob)
	err := s.db.NewSelect().Model(cj).Where("id = ?", id).Scan(ctx)
	return cj, err
}

func (s *cronJobStore) Create(ctx context.Context, cj *model.CronJob) error {
	_, err := s.db.NewInsert().Model(cj).Returning("*").Exec(ctx)
	return err
}

func (s *cronJobStore) Update(ctx context.Context, cj *model.CronJob) error {
	_, err := s.db.NewUpdate().Model(cj).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *cronJobStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*model.CronJob)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *cronJobStore) ListByProject(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.CronJob, int, error) {
	var jobs []model.CronJob
	count, err := s.db.NewSelect().
		Model(&jobs).
		Where("project_id = ?", projectID).
		OrderExpr("created_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return jobs, count, err
}

// ── CronJobRun store ─────────────────────────────────────────────────

type cronJobRunStore struct {
	db *bun.DB
}

func (s *cronJobRunStore) GetByID(ctx context.Context, id uuid.UUID) (*model.CronJobRun, error) {
	run := new(model.CronJobRun)
	err := s.db.NewSelect().Model(run).Where("id = ?", id).Scan(ctx)
	return run, err
}

func (s *cronJobRunStore) Create(ctx context.Context, run *model.CronJobRun) error {
	_, err := s.db.NewInsert().Model(run).Returning("*").Exec(ctx)
	return err
}

func (s *cronJobRunStore) Update(ctx context.Context, run *model.CronJobRun) error {
	_, err := s.db.NewUpdate().Model(run).WherePK().Returning("*").Exec(ctx)
	return err
}

func (s *cronJobRunStore) ListByCronJob(ctx context.Context, cronJobID uuid.UUID, params store.ListParams) ([]model.CronJobRun, int, error) {
	var runs []model.CronJobRun
	count, err := s.db.NewSelect().
		Model(&runs).
		Where("cron_job_id = ?", cronJobID).
		OrderExpr("started_at DESC").
		Limit(params.Limit()).
		Offset(params.Offset()).
		ScanAndCount(ctx)
	return runs, count, err
}
