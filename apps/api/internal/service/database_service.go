package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type DatabaseService struct {
	store  store.Store
	orch   orchestrator.Orchestrator
	logger *slog.Logger
}

func NewDatabaseService(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger) *DatabaseService {
	return &DatabaseService{store: s, orch: orch, logger: logger}
}

type CreateDatabaseInput struct {
	ProjectID    uuid.UUID      `json:"project_id" binding:"required"`
	Name         string         `json:"name" binding:"required,min=1,max=63"`
	DatabaseName string         `json:"database_name"`
	Engine       model.DBEngine `json:"engine" binding:"required,oneof=postgres mysql mariadb redis mongo"`
	Version      string         `json:"version" binding:"required"`
	StorageSize  string         `json:"storage_size"`
	CPULimit     string         `json:"cpu_limit"`
	MemLimit     string         `json:"mem_limit"`
}

// safeNameRe validates database/service names: alphanumeric, hyphens, underscores only.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func (s *DatabaseService) Create(ctx context.Context, input CreateDatabaseInput) (*model.ManagedDatabase, error) {
	// Validate version
	if !model.IsValidVersion(input.Engine, input.Version) {
		return nil, fmt.Errorf("unsupported version %q for engine %q", input.Version, input.Engine)
	}

	// Validate name characters (prevents shell injection via $SAILBOX_DB)
	if !safeNameRe.MatchString(input.Name) {
		return nil, fmt.Errorf("database name must start with alphanumeric and contain only letters, numbers, hyphens, and underscores")
	}

	dbName := input.DatabaseName
	if dbName == "" {
		dbName = input.Name // default database name = service name
	}
	if !safeNameRe.MatchString(dbName) {
		return nil, fmt.Errorf("database name %q contains invalid characters", dbName)
	}

	// Inherit namespace from project
	project, err := s.store.Projects().GetByID(ctx, input.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if project.Namespace == "" {
		return nil, fmt.Errorf("project has no namespace configured")
	}

	db := &model.ManagedDatabase{
		ProjectID:    input.ProjectID,
		Name:         input.Name,
		DatabaseName: dbName,
		Engine:       input.Engine,
		Version:      input.Version,
		StorageSize:  input.StorageSize,
		CPULimit:     input.CPULimit,
		MemLimit:     input.MemLimit,
		Namespace:    project.Namespace,
		Status:       model.AppStatusIdle,
	}

	if db.StorageSize == "" {
		db.StorageSize = "1Gi"
	}
	if db.CPULimit == "" {
		db.CPULimit = "500m"
	}
	if db.MemLimit == "" {
		db.MemLimit = "512Mi"
	}

	// Check for K8s name conflicts in the same project (apps + databases share namespace)
	k8sName := sanitizeK8sName(db.Name)
	db.K8sName = k8sName
	existingDBs, _, _ := s.store.ManagedDatabases().ListByProject(ctx, input.ProjectID, store.ListParams{Page: 1, PerPage: 1000})
	for _, e := range existingDBs {
		if sanitizeK8sName(e.Name) == k8sName {
			return nil, fmt.Errorf("a database with K8s name %q already exists (from %q)", k8sName, e.Name)
		}
	}
	existingApps, _, _ := s.store.Applications().ListByProject(ctx, input.ProjectID, store.ListParams{Page: 1, PerPage: 10000})
	for _, e := range existingApps {
		if sanitizeK8sName(e.Name) == k8sName {
			return nil, fmt.Errorf("an application with K8s name %q already exists (from %q) — app and database names must not collide", k8sName, e.Name)
		}
	}

	if err := s.store.ManagedDatabases().Create(ctx, db); err != nil {
		return nil, err
	}

	// Deploy to K3s
	if err := s.orch.DeployDatabase(ctx, db); err != nil {
		s.logger.Error("failed to deploy database", slog.Any("error", err))
		_ = s.store.ManagedDatabases().Delete(ctx, db.ID)
		return nil, err
	}

	// Persist K8s metadata set by orchestrator
	if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
		s.logger.Error("failed to update database with k8s metadata", slog.Any("error", err))
	}

	s.logger.Info("managed database created",
		slog.String("name", db.Name),
		slog.String("engine", string(db.Engine)),
	)
	return db, nil
}

func (s *DatabaseService) GetByID(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error) {
	return s.store.ManagedDatabases().GetByID(ctx, id)
}

func (s *DatabaseService) List(ctx context.Context, projectID uuid.UUID, params store.ListParams) ([]model.ManagedDatabase, int, error) {
	dbs, total, err := s.store.ManagedDatabases().ListByProject(ctx, projectID, params)
	if err == nil {
		s.syncLiveStatuses(ctx, dbs)
	}
	return dbs, total, err
}

func (s *DatabaseService) syncLiveStatuses(ctx context.Context, dbs []model.ManagedDatabase) {
	for i := range dbs {
		if dbs[i].Status == model.AppStatusIdle {
			continue
		}
		status, err := s.orch.GetDatabaseStatus(ctx, &dbs[i])
		if err != nil {
			continue
		}
		var live model.AppStatus
		switch status.Phase {
		case "running":
			live = model.AppStatusRunning
		case "stopped":
			live = model.AppStatusStopped
		case "not deployed":
			if dbs[i].Status != model.AppStatusIdle {
				live = model.AppStatusError
			}
		case "pending":
			live = model.AppStatusDeploying
		}
		if live != "" && live != dbs[i].Status {
			dbs[i].Status = live
			go func(db model.ManagedDatabase, s2 model.AppStatus) {
				db.Status = s2
				_ = s.store.ManagedDatabases().Update(context.Background(), &db)
			}(dbs[i], live)
		}
	}
}

func (s *DatabaseService) Delete(ctx context.Context, id uuid.UUID) error {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.orch.DeleteDatabase(ctx, db); err != nil {
		s.logger.Error("failed to delete database from orchestrator", slog.Any("error", err))
		return fmt.Errorf("failed to delete database resources: %w — delete manually from K8s before retrying", err)
	}

	// Also clean up external access service if enabled
	if db.ExternalEnabled {
		if err := s.orch.DisableExternalAccess(ctx, db); err != nil {
			s.logger.Warn("failed to cleanup external access", slog.Any("error", err))
		}
	}

	return s.store.ManagedDatabases().Delete(ctx, id)
}

func (s *DatabaseService) GetCredentials(ctx context.Context, id uuid.UUID) (*orchestrator.DatabaseCredentials, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.orch.GetDatabaseCredentials(ctx, db)
}

func (s *DatabaseService) GetStatus(ctx context.Context, id uuid.UUID) (*orchestrator.AppStatus, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	status, err := s.orch.GetDatabaseStatus(ctx, db)
	if err != nil {
		return nil, err
	}

	// Reconcile: update DB record if K8s state differs
	newStatus := db.Status
	switch status.Phase {
	case "running":
		newStatus = model.AppStatusRunning
	case "not deployed":
		if db.Status == model.AppStatusIdle {
			// Fresh database that was never deployed — keep idle
			newStatus = model.AppStatusIdle
		} else {
			// Previously deployed database with missing resources — flag as error
			newStatus = model.AppStatusError
		}
	case "pending":
		newStatus = "pending"
	case "stopped":
		newStatus = model.AppStatusStopped
	}
	if newStatus != db.Status {
		db.Status = newStatus
		if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
			s.logger.Error("failed to reconcile database status", slog.Any("error", err))
		}
	}
	return status, nil
}

func (s *DatabaseService) GetPods(ctx context.Context, id uuid.UUID) ([]orchestrator.PodInfo, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.orch.GetDatabasePods(ctx, db)
}

func (s *DatabaseService) UpdateExternalAccess(ctx context.Context, id uuid.UUID, enabled bool, port int32) (*model.ManagedDatabase, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if enabled {
		if port > 0 && (port < 30000 || port > 32767) {
			return nil, fmt.Errorf("NodePort must be between 30000–32767, got %d", port)
		}
		if port > 0 {
			// Check port conflict with other databases
			conflict, err := s.store.ManagedDatabases().FindByExternalPort(ctx, port)
			if err == nil && conflict != nil && conflict.ID != id {
				return nil, fmt.Errorf("port %d is already used by database %q", port, conflict.Name)
			}
			db.ExternalPort = port
		}
		nodePort, err := s.orch.EnableExternalAccess(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("enable external access: %w", err)
		}
		db.ExternalEnabled = true
		db.ExternalPort = nodePort
	} else {
		if err := s.orch.DisableExternalAccess(ctx, db); err != nil {
			return nil, fmt.Errorf("disable external access: %w", err)
		}
		db.ExternalEnabled = false
		db.ExternalPort = 0
	}

	if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
		return nil, fmt.Errorf("update database: %w", err)
	}

	s.logger.Info("database external access updated",
		slog.String("database", db.Name),
		slog.Bool("enabled", enabled),
		slog.Int("port", int(db.ExternalPort)),
	)
	return db, nil
}

// UpdateBackupInput holds the configuration for database backup settings.
type UpdateBackupInput struct {
	Enabled  bool       `json:"enabled"`
	Schedule string     `json:"schedule"`
	S3ID     *uuid.UUID `json:"s3_id"`
}

func (s *DatabaseService) UpdateBackupConfig(ctx context.Context, id uuid.UUID, input UpdateBackupInput) (*model.ManagedDatabase, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Validate cron schedule if backup is being enabled
	if input.Enabled && input.Schedule != "" {
		fields := strings.Fields(input.Schedule)
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid cron schedule %q: expected 5 fields (minute hour dom month dow)", input.Schedule)
		}
	}

	// Validate S3 resource exists, is correct type, and belongs to same org
	if input.S3ID != nil {
		resource, err := s.store.SharedResources().GetByID(ctx, *input.S3ID)
		if err != nil {
			return nil, fmt.Errorf("S3 resource not found: %w", err)
		}
		if resource.Type != model.ResourceObjectStorage {
			return nil, fmt.Errorf("resource %s is not an object storage resource", resource.Name)
		}
		project, projErr := s.store.Projects().GetByID(ctx, db.ProjectID)
		if projErr != nil {
			return nil, fmt.Errorf("project not found: %w", projErr)
		}
		if resource.OrgID != project.OrgID {
			return nil, fmt.Errorf("S3 resource does not belong to this organization")
		}
	}

	db.BackupEnabled = input.Enabled
	db.BackupSchedule = input.Schedule
	db.BackupS3ID = input.S3ID

	if err := s.store.ManagedDatabases().Update(ctx, db); err != nil {
		return nil, fmt.Errorf("update database: %w", err)
	}

	s.logger.Info("database backup config updated",
		slog.String("database", db.Name),
		slog.Bool("enabled", input.Enabled),
		slog.String("schedule", input.Schedule),
	)
	return db, nil
}

// UsedExternalPorts returns all ports currently in use for external access.
func (s *DatabaseService) UsedExternalPorts(ctx context.Context) ([]model.ExternalPortInfo, error) {
	return s.store.ManagedDatabases().ListExternalPorts(ctx)
}

// dbBackupExt returns the file extension for a database engine backup.
func dbBackupExt(engine model.DBEngine) string {
	switch engine {
	case model.DBPostgres, model.DBMySQL, model.DBMariaDB:
		return "sql"
	case model.DBMongo:
		return "archive"
	case model.DBRedis:
		return "rdb"
	default:
		return "dump"
	}
}

// dbSafeName returns the K8s name or database name, preferring K8sName.
func dbSafeName(db *model.ManagedDatabase) string {
	if db.K8sName != "" {
		return db.K8sName
	}
	return db.Name
}

func (s *DatabaseService) loadS3Config(ctx context.Context, s3ID *uuid.UUID) (*orchestrator.S3Config, error) {
	if s3ID == nil {
		return nil, nil
	}

	resource, err := s.store.SharedResources().GetByID(ctx, *s3ID)
	if err != nil {
		return nil, fmt.Errorf("load S3 resource: %w", err)
	}

	var cfg orchestrator.S3Config
	if err := json.Unmarshal(resource.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse S3 config: %w", err)
	}
	return &cfg, nil
}

func (s *DatabaseService) TriggerBackup(ctx context.Context, databaseID uuid.UUID) (*model.DatabaseBackup, error) {
	db, err := s.store.ManagedDatabases().GetByID(ctx, databaseID)
	if err != nil {
		return nil, err
	}

	// Verify the database is actually deployed and running in K8s before backup.
	// If pending, wait up to 30s for it to become ready (covers freshly-created databases).
	var status *orchestrator.AppStatus
	for attempt := 0; attempt < 6; attempt++ {
		status, err = s.orch.GetDatabaseStatus(ctx, db)
		if err != nil {
			return nil, fmt.Errorf("cannot check database status: %w", err)
		}
		if status.Phase == "running" {
			break
		}
		if status.Phase == "not deployed" {
			return nil, fmt.Errorf("database %q is not deployed — deploy it first before backing up", db.Name)
		}
		// pending/other — wait 5s and retry
		if attempt < 5 {
			s.logger.Info("backup waiting for database to become ready",
				slog.String("db", db.Name), slog.String("phase", status.Phase), slog.Int("attempt", attempt+1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	if status.Phase != "running" {
		return nil, fmt.Errorf("database %q is not ready after 30s (status: %s) — please try again later", db.Name, status.Phase)
	}

	// Load S3 config if configured
	s3Config, err := s.loadS3Config(ctx, db.BackupS3ID)
	if err != nil {
		return nil, fmt.Errorf("load S3 config: %w", err)
	}

	now := time.Now()
	backup := &model.DatabaseBackup{
		DatabaseID: databaseID,
		Status:     "running",
		StartedAt:  &now,
	}

	if err := s.store.DatabaseBackups().Create(ctx, backup); err != nil {
		return nil, fmt.Errorf("create backup record: %w", err)
	}

	// Generate a stable S3 key: {engine}-{name}/{timestamp}-{id8}.{ext}
	backupExt := dbBackupExt(db.Engine)
	ts := now.UTC().Format("20060102-150405")
	s3Key := fmt.Sprintf("sailbox/db-backups/%s-%s/%s-%s.%s",
		db.Engine, dbSafeName(db), ts, backup.ID.String()[:8], backupExt)
	backup.FilePath = s3Key

	if err := s.orch.RunDatabaseBackup(ctx, db, backup.ID, s3Config, s3Key); err != nil {
		backup.Status = "failed"
		finished := time.Now()
		backup.FinishedAt = &finished
		_ = s.store.DatabaseBackups().Update(ctx, backup)
		return nil, fmt.Errorf("run backup: %w", err)
	}

	// Save FilePath
	_ = s.store.DatabaseBackups().Update(ctx, backup)

	s.logger.Info("database backup triggered",
		slog.String("database", db.Name),
		slog.String("backup_id", backup.ID.String()),
		slog.String("s3_key", s3Key),
	)
	return backup, nil
}

func (s *DatabaseService) ListBackups(ctx context.Context, databaseID uuid.UUID, params store.ListParams) ([]model.DatabaseBackup, int, error) {
	backups, total, err := s.store.DatabaseBackups().ListByDatabase(ctx, databaseID, params)
	if err != nil {
		return nil, 0, err
	}

	// Reconcile: check K8s Job status for any "running" backups
	for i := range backups {
		if backups[i].Status != "running" {
			continue
		}
		jobStatus := s.orch.GetBackupJobStatus(ctx, backups[i].ID)
		if jobStatus == "" {
			continue
		}
		now := time.Now()
		switch jobStatus {
		case "completed":
			backups[i].Status = "completed"
			backups[i].FinishedAt = &now
		case "failed":
			backups[i].Status = "failed"
			backups[i].FinishedAt = &now
		}
		_ = s.store.DatabaseBackups().Update(ctx, &backups[i])
	}

	// Also reconcile restore job statuses
	for i := range backups {
		if backups[i].RestoreStatus != "running" {
			continue
		}
		jobStatus := s.orch.GetRestoreJobStatus(ctx, backups[i].ID)
		if jobStatus == "" {
			continue
		}
		switch jobStatus {
		case "completed":
			backups[i].RestoreStatus = "completed"
		case "failed":
			backups[i].RestoreStatus = "failed"
		}
		_ = s.store.DatabaseBackups().Update(ctx, &backups[i])
	}

	return backups, total, nil
}

func (s *DatabaseService) RestoreBackup(ctx context.Context, databaseID, backupID uuid.UUID) error {
	db, err := s.store.ManagedDatabases().GetByID(ctx, databaseID)
	if err != nil {
		return err
	}

	// Verify the database is running
	status, err := s.orch.GetDatabaseStatus(ctx, db)
	if err != nil {
		return fmt.Errorf("cannot check database status: %w", err)
	}
	if status.Phase != "running" {
		return fmt.Errorf("database %q must be running to restore (current: %s)", db.Name, status.Phase)
	}

	// Get the backup record
	backup, err := s.store.DatabaseBackups().GetByID(ctx, backupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}
	if backup.DatabaseID != databaseID {
		return fmt.Errorf("backup does not belong to this database")
	}
	if backup.Status != "completed" {
		return fmt.Errorf("can only restore from a completed backup (current: %s)", backup.Status)
	}
	if backup.RestoreStatus == "running" {
		return fmt.Errorf("a restore is already in progress for this backup")
	}

	// Load S3 config
	s3Config, err := s.loadS3Config(ctx, db.BackupS3ID)
	if err != nil {
		return fmt.Errorf("load S3 config: %w", err)
	}
	if s3Config == nil {
		return fmt.Errorf("S3 storage is not configured for this database — configure backup settings first")
	}

	// Use saved S3 key from backup record; fall back to legacy path for old backups
	s3Key := backup.FilePath
	if s3Key == "" {
		ext := dbBackupExt(db.Engine)
		s3Key = fmt.Sprintf("sailbox/%s/%s.%s", db.Name, backupID.String(), ext)
	}

	// Launch restore job
	if err := s.orch.RestoreDatabaseBackup(ctx, db, backupID, s3Config, s3Key); err != nil {
		return fmt.Errorf("start restore job: %w", err)
	}

	// Update backup record with restore status
	backup.RestoreStatus = "running"
	if err := s.store.DatabaseBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to update backup restore status", slog.Any("error", err))
	}

	s.logger.Info("database restore started",
		slog.String("database", db.Name),
		slog.String("backup_id", backupID.String()),
	)
	return nil
}
