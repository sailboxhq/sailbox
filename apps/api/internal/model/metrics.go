package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// MetricSnapshot stores a time-series data point from metrics-server.
type MetricSnapshot struct {
	bun.BaseModel  `bun:"table:metrics.snapshots"`
	ID             int64     `bun:"id,pk,autoincrement" json:"-"`
	CollectedAt    time.Time `bun:"collected_at,notnull" json:"collected_at"`
	SourceType     string    `bun:"source_type,notnull" json:"source_type"` // "node" | "app"
	SourceName     string    `bun:"source_name,notnull" json:"source_name"`
	CPUUsedMillis  int64     `bun:"cpu_used_millis" json:"cpu_used"`
	CPUTotalMillis int64     `bun:"cpu_total_millis" json:"cpu_total"`
	MemUsedBytes   int64     `bun:"mem_used_bytes" json:"mem_used"`
	MemTotalBytes  int64     `bun:"mem_total_bytes" json:"mem_total"`
	DiskUsedBytes  *int64    `bun:"disk_used_bytes" json:"disk_used,omitempty"`
	DiskTotalBytes *int64    `bun:"disk_total_bytes" json:"disk_total,omitempty"`
	PodCount       *int      `bun:"pod_count" json:"pod_count,omitempty"`
}

// MetricEvent stores a persisted K8s cluster event.
type MetricEvent struct {
	bun.BaseModel   `bun:"table:metrics.events"`
	ID              uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	RecordedAt      time.Time `bun:"recorded_at,notnull,default:current_timestamp" json:"recorded_at"`
	EventType       string    `bun:"event_type,notnull" json:"event_type"` // "Normal" | "Warning"
	Reason          string    `bun:"reason,notnull" json:"reason"`
	Message         string    `bun:"message" json:"message"`
	Namespace       string    `bun:"namespace" json:"namespace"`
	InvolvedObject  string    `bun:"involved_object" json:"involved_object"`
	SourceComponent string    `bun:"source_component" json:"source_component"`
	FirstSeen       time.Time `bun:"first_seen" json:"first_seen"`
	LastSeen        time.Time `bun:"last_seen" json:"last_seen"`
	Count           int32     `bun:"count" json:"count"`
}

// MetricAlert stores a fired alert instance.
type MetricAlert struct {
	bun.BaseModel `bun:"table:metrics.alerts"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	RuleName      string     `bun:"rule_name,notnull" json:"rule_name"`
	Severity      string     `bun:"severity,notnull" json:"severity"` // "critical" | "warning" | "info"
	SourceType    string     `bun:"source_type,notnull" json:"source_type"`
	SourceName    string     `bun:"source_name,notnull" json:"source_name"`
	Message       string     `bun:"message" json:"message"`
	FiredAt       time.Time  `bun:"fired_at,notnull,default:current_timestamp" json:"fired_at"`
	ResolvedAt    *time.Time `bun:"resolved_at" json:"resolved_at,omitempty"`
	Notified      bool       `bun:"notified,default:false" json:"notified"`
}
