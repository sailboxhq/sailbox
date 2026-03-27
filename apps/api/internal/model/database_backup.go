package model

import (
	"time"

	"github.com/google/uuid"
)

// DatabaseBackup represents a point-in-time backup of a managed database.
type DatabaseBackup struct {
	BaseModel `bun:"table:database_backups,alias:dbb"`

	DatabaseID    uuid.UUID  `bun:"database_id,notnull,type:uuid" json:"database_id"`
	Status        string     `bun:"status,notnull,default:'pending'" json:"status"`
	RestoreStatus string     `bun:"restore_status,default:''" json:"restore_status,omitempty"` // "", "running", "completed", "failed"
	SizeBytes     int64      `bun:"size_bytes,default:0" json:"size_bytes"`
	FilePath      string     `bun:"file_path,default:''" json:"file_path"`
	StartedAt     *time.Time `bun:"started_at,nullzero" json:"started_at,omitempty"`
	FinishedAt    *time.Time `bun:"finished_at,nullzero" json:"finished_at,omitempty"`
}
