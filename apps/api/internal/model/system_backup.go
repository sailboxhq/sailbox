package model

import "time"

// SystemBackup represents a point-in-time backup of the Sailbox PostgreSQL database.
type SystemBackup struct {
	BaseModel `bun:"table:system_backups,alias:sb"`

	Status     string     `bun:"status,notnull,default:'pending'" json:"status"` // pending, running, completed, failed
	SizeBytes  int64      `bun:"size_bytes,default:0" json:"size_bytes"`
	FileName   string     `bun:"file_name,default:''" json:"file_name"`
	S3Bucket   string     `bun:"s3_bucket,default:''" json:"s3_bucket"`
	S3Path     string     `bun:"s3_path,default:''" json:"s3_path"`
	Error      string     `bun:"error_msg,default:''" json:"error,omitempty"`
	StartedAt  *time.Time `bun:"started_at,nullzero" json:"started_at,omitempty"`
	FinishedAt *time.Time `bun:"finished_at,nullzero" json:"finished_at,omitempty"`
}
