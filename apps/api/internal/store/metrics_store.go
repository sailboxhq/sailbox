package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

// MetricsStore is the aggregate interface for all metrics storage.
// Decoupled from the main Store interface for independent backend swapping.
type MetricsStore interface {
	Snapshots() MetricSnapshotStore
	Events() MetricEventStore
	Alerts() MetricAlertStore
}

type SnapshotQuery struct {
	SourceType string
	SourceName string
	From       time.Time
	To         time.Time
	Limit      int
}

type EventQuery struct {
	EventType string // filter by "Warning", "Normal", or "" for all
	Namespace string
	From      time.Time
	To        time.Time
	ListParams
}

type AlertQuery struct {
	ActiveOnly bool
	Severity   string
	From       time.Time
	To         time.Time
	ListParams
}

type MetricSnapshotStore interface {
	InsertBatch(ctx context.Context, snapshots []model.MetricSnapshot) error
	Query(ctx context.Context, q SnapshotQuery) ([]model.MetricSnapshot, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

type MetricEventStore interface {
	UpsertBatch(ctx context.Context, events []model.MetricEvent) error
	List(ctx context.Context, q EventQuery) ([]model.MetricEvent, int, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

type MetricAlertStore interface {
	Insert(ctx context.Context, alert *model.MetricAlert) error
	GetActiveByRuleAndSource(ctx context.Context, ruleName, sourceName string) (*model.MetricAlert, error)
	Resolve(ctx context.Context, id uuid.UUID) error
	MarkNotified(ctx context.Context, id uuid.UUID) error
	ListActive(ctx context.Context) ([]model.MetricAlert, error)
	List(ctx context.Context, q AlertQuery) ([]model.MetricAlert, int, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}
