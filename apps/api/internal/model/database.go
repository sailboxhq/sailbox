package model

import (
	"github.com/google/uuid"
)

// Database engine type.
type DBEngine string

const (
	DBPostgres DBEngine = "postgres"
	DBMySQL    DBEngine = "mysql"
	DBMariaDB  DBEngine = "mariadb"
	DBRedis    DBEngine = "redis"
	DBMongo    DBEngine = "mongo"
)

// ManagedDatabase represents a database instance managed by Sailbox.
type ManagedDatabase struct {
	BaseModel `bun:"table:managed_databases,alias:mdb"`

	ProjectID uuid.UUID `bun:"project_id,notnull,type:uuid" json:"project_id"`
	Project   *Project  `bun:"rel:belongs-to,join:project_id=id" json:"-"`

	Name         string   `bun:"name,notnull" json:"name"`
	DatabaseName string   `bun:"database_name,default:''" json:"database_name"`
	Engine       DBEngine `bun:"engine,notnull" json:"engine"`
	Version      string   `bun:"version,notnull" json:"version"`

	// Resources
	StorageSize string `bun:"storage_size,default:'1Gi'" json:"storage_size"`
	CPULimit    string `bun:"cpu_limit,default:'500m'" json:"cpu_limit"`
	MemLimit    string `bun:"mem_limit,default:'512Mi'" json:"mem_limit"`

	// Credentials (stored as K8s secret name)
	CredentialsSecret string `bun:"credentials_secret" json:"-"`

	// K8s mapping
	Namespace string    `bun:"namespace" json:"namespace"`
	K8sName   string    `bun:"k8s_name" json:"k8s_name"`
	Status    AppStatus `bun:"status,default:'idle'" json:"status"`

	// External access
	ExternalPort    int32 `bun:"external_port,default:0" json:"external_port"`
	ExternalEnabled bool  `bun:"external_enabled,default:false" json:"external_enabled"`

	// Backup configuration
	BackupEnabled  bool       `bun:"backup_enabled,default:false" json:"backup_enabled"`
	BackupSchedule string     `bun:"backup_schedule,default:''" json:"backup_schedule"`
	BackupS3ID     *uuid.UUID `bun:"backup_s3_id,type:uuid" json:"backup_s3_id,omitempty"`
}

// ExternalPortInfo is a lightweight view of a database's external port.
type ExternalPortInfo struct {
	DatabaseID   uuid.UUID `json:"database_id"`
	DatabaseName string    `json:"database_name"`
	Engine       DBEngine  `json:"engine"`
	Port         int32     `json:"port"`
}
