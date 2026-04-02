package pg

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type metricsStoreImpl struct {
	snapshots *snapshotStore
	events    *eventStore
	alerts    *alertStore
}

func NewMetricsStore(db *bun.DB) store.MetricsStore {
	return &metricsStoreImpl{
		snapshots: &snapshotStore{db: db},
		events:    &eventStore{db: db},
		alerts:    &alertStore{db: db},
	}
}

func (s *metricsStoreImpl) Snapshots() store.MetricSnapshotStore { return s.snapshots }
func (s *metricsStoreImpl) Events() store.MetricEventStore       { return s.events }
func (s *metricsStoreImpl) Alerts() store.MetricAlertStore       { return s.alerts }

// ── Snapshots ───────────────────────────────────────────────────

type snapshotStore struct{ db *bun.DB }

func (s *snapshotStore) InsertBatch(ctx context.Context, snapshots []model.MetricSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	_, err := s.db.NewInsert().Model(&snapshots).Exec(ctx)
	return err
}

func (s *snapshotStore) Query(ctx context.Context, q store.SnapshotQuery) ([]model.MetricSnapshot, error) {
	result := make([]model.MetricSnapshot, 0)
	query := s.db.NewSelect().Model(&result)

	if q.SourceType != "" {
		query = query.Where("source_type = ?", q.SourceType)
	}
	if q.SourceName != "" {
		query = query.Where("source_name = ?", q.SourceName)
	}
	if !q.From.IsZero() {
		query = query.Where("collected_at >= ?", q.From)
	}
	if !q.To.IsZero() {
		query = query.Where("collected_at <= ?", q.To)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}
	err := query.Order("collected_at ASC").Limit(limit).Scan(ctx)
	return result, err
}

func (s *snapshotStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.NewDelete().Model((*model.MetricSnapshot)(nil)).Where("collected_at < ?", before).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ── Events ──────────────────────────────────────────────────────

type eventStore struct{ db *bun.DB }

func (s *eventStore) UpsertBatch(ctx context.Context, events []model.MetricEvent) error {
	if len(events) == 0 {
		return nil
	}
	// Deduplicate by conflict key to prevent "cannot affect row a second time"
	seen := make(map[string]bool)
	deduped := make([]model.MetricEvent, 0, len(events))
	for _, e := range events {
		key := e.Namespace + "/" + e.InvolvedObject + "/" + e.Reason
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, e)
		}
	}
	events = deduped
	_, err := s.db.NewInsert().Model(&events).
		On("CONFLICT (namespace, involved_object, reason) DO UPDATE").
		Set("last_seen = EXCLUDED.last_seen").
		Set("count = EXCLUDED.count").
		Set("message = EXCLUDED.message").
		Exec(ctx)
	return err
}

func (s *eventStore) List(ctx context.Context, q store.EventQuery) ([]model.MetricEvent, int, error) {
	page, perPage := q.Page, q.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}

	addFilters := func(qb *bun.SelectQuery) *bun.SelectQuery {
		if q.EventType != "" {
			qb = qb.Where("event_type = ?", q.EventType)
		}
		if q.Namespace != "" {
			qb = qb.Where("namespace = ?", q.Namespace)
		}
		if !q.From.IsZero() {
			qb = qb.Where("last_seen >= ?", q.From)
		}
		if !q.To.IsZero() {
			qb = qb.Where("last_seen <= ?", q.To)
		}
		return qb
	}

	count, err := addFilters(s.db.NewSelect().Model((*model.MetricEvent)(nil))).Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var result []model.MetricEvent
	err = addFilters(s.db.NewSelect().Model(&result)).
		Order("last_seen DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Scan(ctx)
	return result, count, err
}

func (s *eventStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.NewDelete().Model((*model.MetricEvent)(nil)).Where("last_seen < ?", before).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ── Alerts ──────────────────────────────────────────────────────

type alertStore struct{ db *bun.DB }

func (s *alertStore) Insert(ctx context.Context, alert *model.MetricAlert) error {
	_, err := s.db.NewInsert().Model(alert).Exec(ctx)
	return err
}

func (s *alertStore) GetActiveByRuleAndSource(ctx context.Context, ruleName, sourceName string) (*model.MetricAlert, error) {
	alert := new(model.MetricAlert)
	err := s.db.NewSelect().Model(alert).
		Where("rule_name = ?", ruleName).
		Where("source_name = ?", sourceName).
		Where("resolved_at IS NULL").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return alert, nil
}

func (s *alertStore) Resolve(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewUpdate().Model((*model.MetricAlert)(nil)).
		Set("resolved_at = NOW()").
		Where("id = ?", id).
		Where("resolved_at IS NULL").
		Exec(ctx)
	return err
}

func (s *alertStore) MarkNotified(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.NewUpdate().Model((*model.MetricAlert)(nil)).
		Set("notified = TRUE").
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (s *alertStore) ListActive(ctx context.Context) ([]model.MetricAlert, error) {
	var result []model.MetricAlert
	err := s.db.NewSelect().Model(&result).
		Where("resolved_at IS NULL").
		Order("fired_at DESC").
		Scan(ctx)
	return result, err
}

func (s *alertStore) List(ctx context.Context, q store.AlertQuery) ([]model.MetricAlert, int, error) {
	page, perPage := q.Page, q.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}

	addFilters := func(qb *bun.SelectQuery) *bun.SelectQuery {
		if q.ActiveOnly {
			qb = qb.Where("resolved_at IS NULL")
		}
		if q.Severity != "" {
			qb = qb.Where("severity = ?", q.Severity)
		}
		if !q.From.IsZero() {
			qb = qb.Where("fired_at >= ?", q.From)
		}
		if !q.To.IsZero() {
			qb = qb.Where("fired_at <= ?", q.To)
		}
		return qb
	}

	count, err := addFilters(s.db.NewSelect().Model((*model.MetricAlert)(nil))).Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	var result []model.MetricAlert
	err = addFilters(s.db.NewSelect().Model(&result)).
		Order("fired_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Scan(ctx)
	return result, count, err
}

func (s *alertStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.NewDelete().Model((*model.MetricAlert)(nil)).
		Where("fired_at < ?", before).
		Where("resolved_at IS NOT NULL").
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
