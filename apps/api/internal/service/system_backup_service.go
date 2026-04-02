package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// SystemBackupService manages backups of the Sailbox PostgreSQL database to S3.
type SystemBackupService struct {
	store    store.Store
	settings *SettingService
	dbURL    string
	logger   *slog.Logger
}

// SystemBackupConfig holds the configuration for system backups.
type SystemBackupConfig struct {
	Enabled   bool   `json:"enabled"`
	S3ID      string `json:"s3_id"`
	Schedule  string `json:"schedule"`
	Path      string `json:"path"`
	Retention int    `json:"retention"`
}

// NewSystemBackupService creates a new SystemBackupService.
func NewSystemBackupService(s store.Store, settings *SettingService, dbURL string, logger *slog.Logger) *SystemBackupService {
	svc := &SystemBackupService{
		store:    s,
		settings: settings,
		dbURL:    dbURL,
		logger:   logger,
	}
	go svc.startScheduler()
	return svc
}

// GetConfig reads system backup configuration from settings.
func (s *SystemBackupService) GetConfig(ctx context.Context) (*SystemBackupConfig, error) {
	enabled, _ := s.store.Settings().Get(ctx, "system_backup_enabled")
	s3ID, _ := s.store.Settings().Get(ctx, "system_backup_s3_id")
	schedule, _ := s.store.Settings().Get(ctx, "system_backup_schedule")
	path, _ := s.store.Settings().Get(ctx, "system_backup_path")
	retentionStr, _ := s.store.Settings().Get(ctx, "system_backup_retention")

	retention := 30
	if retentionStr != "" {
		if v, err := strconv.Atoi(retentionStr); err == nil && v > 0 {
			retention = v
		}
	}

	return &SystemBackupConfig{
		Enabled:   enabled == "true",
		S3ID:      s3ID,
		Schedule:  schedule,
		Path:      path,
		Retention: retention,
	}, nil
}

// SaveConfig writes system backup configuration to settings.
func (s *SystemBackupService) SaveConfig(ctx context.Context, cfg *SystemBackupConfig) error {
	// When enabling, require S3 and schedule
	if cfg.Enabled {
		if cfg.S3ID == "" {
			return fmt.Errorf("S3 storage resource is required when backups are enabled")
		}
		if cfg.Schedule == "" {
			return fmt.Errorf("backup schedule is required when backups are enabled")
		}
		fields := strings.Fields(cfg.Schedule)
		if len(fields) != 5 {
			return fmt.Errorf("invalid cron schedule %q: expected 5 fields", cfg.Schedule)
		}
	}

	// Validate S3 resource exists and is correct type
	if cfg.S3ID != "" {
		s3UUID, parseErr := uuid.Parse(cfg.S3ID)
		if parseErr != nil {
			return fmt.Errorf("invalid S3 resource ID: %w", parseErr)
		}
		resource, resErr := s.store.SharedResources().GetByID(ctx, s3UUID)
		if resErr != nil {
			return fmt.Errorf("S3 resource not found: %w", resErr)
		}
		if resource.Type != model.ResourceObjectStorage {
			return fmt.Errorf("resource %q is not an object storage resource", resource.Name)
		}
	}

	pairs := map[string]string{
		"system_backup_s3_id":     cfg.S3ID,
		"system_backup_schedule":  cfg.Schedule,
		"system_backup_path":      cfg.Path,
		"system_backup_retention": strconv.Itoa(cfg.Retention),
	}
	if cfg.Enabled {
		pairs["system_backup_enabled"] = "true"
	} else {
		pairs["system_backup_enabled"] = "false"
	}
	for k, v := range pairs {
		if err := s.store.Settings().Set(ctx, k, v); err != nil {
			return err
		}
	}
	s.logger.Info("system backup config updated")
	return nil
}

// TriggerBackup starts a system database backup.
func (s *SystemBackupService) TriggerBackup(ctx context.Context) (*model.SystemBackup, error) {
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup config: %w", err)
	}
	if cfg.S3ID == "" {
		return nil, fmt.Errorf("system backup S3 resource not configured")
	}

	// Load S3 config from shared_resources
	s3ID, err := uuid.Parse(cfg.S3ID)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 resource ID: %w", err)
	}
	resource, err := s.store.SharedResources().GetByID(ctx, s3ID)
	if err != nil {
		return nil, fmt.Errorf("S3 resource not found: %w", err)
	}

	var s3Cfg orchestrator.S3Config
	if err := json.Unmarshal(resource.Config, &s3Cfg); err != nil {
		return nil, fmt.Errorf("invalid S3 config: %w", err)
	}

	now := time.Now()
	backup := &model.SystemBackup{
		Status:    "running",
		StartedAt: &now,
		S3Bucket:  s3Cfg.Bucket,
	}
	if err := s.store.SystemBackups().Create(ctx, backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	// Run backup in background goroutine
	go s.runBackup(backup, cfg, &s3Cfg)

	return backup, nil
}

// ListBackups returns a paginated list of system backups.
func (s *SystemBackupService) ListBackups(ctx context.Context, params store.ListParams) ([]model.SystemBackup, int, error) {
	return s.store.SystemBackups().List(ctx, params)
}

// runBackup performs the actual pg_dump and S3 upload in the background.
func (s *SystemBackupService) runBackup(backup *model.SystemBackup, cfg *SystemBackupConfig, s3Cfg *orchestrator.S3Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	timestamp := time.Now().UTC().Format("20060102-150405")
	fileName := fmt.Sprintf("sailbox-%s.dump", timestamp)

	s3Path := cfg.Path
	if s3Path == "" {
		s3Path = "sailbox-backups"
	}
	fullS3Path := fmt.Sprintf("%s/%s", s3Path, fileName)
	backup.S3Path = fullS3Path
	backup.FileName = fileName

	// Create temp file for dump
	tmpFile, err := os.CreateTemp("", "sailbox-backup-*.dump")
	if err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("failed to create temp file: %v", err))
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// Parse database URL
	host, port, user, password, dbname, parseErr := parseDatabaseURL(s.dbURL)
	if parseErr != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("failed to parse database URL: %v", parseErr))
		return
	}

	// Run pg_dump
	cmd := exec.CommandContext(ctx, "pg_dump",
		"-h", host, "-p", port, "-U", user, "-d", dbname,
		"-F", "c",
		"-f", tmpPath,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	if output, err := cmd.CombinedOutput(); err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("pg_dump failed: %v — %s", err, string(output)))
		return
	}

	// Get file size
	info, err := os.Stat(tmpPath)
	if err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("failed to stat dump file: %v", err))
		return
	}
	backup.SizeBytes = info.Size()

	// Upload to S3 using aws-cli
	s3URI := fmt.Sprintf("s3://%s/%s", s3Cfg.Bucket, fullS3Path)
	uploadCmd := exec.CommandContext(ctx, "aws", "s3", "cp", tmpPath, s3URI,
		"--endpoint-url", s3Cfg.Endpoint,
	)
	uploadCmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+s3Cfg.AccessKey,
		"AWS_SECRET_ACCESS_KEY="+s3Cfg.SecretKey,
		"AWS_DEFAULT_REGION="+s3Region(s3Cfg.Region),
	)

	if output, err := uploadCmd.CombinedOutput(); err != nil {
		s.failBackup(ctx, backup, fmt.Sprintf("S3 upload failed: %v — %s", err, string(output)))
		return
	}

	// Mark backup as completed
	now := time.Now()
	backup.Status = "completed"
	backup.FinishedAt = &now
	if err := s.store.SystemBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to update backup record", slog.Any("error", err))
	}
	s.logger.Info("system backup completed", slog.String("file", fileName), slog.Int64("size", backup.SizeBytes))

	// Enforce retention: delete old backups beyond limit
	s.enforceRetention(ctx, cfg.Retention, s3Cfg, s3Path)
}

// failBackup marks a backup record as failed.
func (s *SystemBackupService) failBackup(ctx context.Context, backup *model.SystemBackup, errMsg string) {
	now := time.Now()
	backup.Status = "failed"
	backup.Error = errMsg
	backup.FinishedAt = &now
	if err := s.store.SystemBackups().Update(ctx, backup); err != nil {
		s.logger.Error("failed to update backup record", slog.Any("error", err))
	}
	s.logger.Error("system backup failed", slog.String("error", errMsg))
}

// enforceRetention deletes old backup records beyond the retention limit.
func (s *SystemBackupService) enforceRetention(ctx context.Context, retention int, s3Cfg *orchestrator.S3Config, s3Path string) {
	if retention <= 0 {
		return
	}
	allBackups, total, err := s.store.SystemBackups().List(ctx, store.ListParams{Page: 1, PerPage: 1000})
	if err != nil || total <= retention {
		return
	}

	// Backups are ordered by created_at DESC, so skip the first 'retention' entries
	for i := retention; i < len(allBackups); i++ {
		old := allBackups[i]
		if old.Status != "completed" {
			continue
		}
		// Delete from S3
		if old.S3Path != "" && s3Cfg != nil {
			s3URI := fmt.Sprintf("s3://%s/%s", old.S3Bucket, old.S3Path)
			delCmd := exec.CommandContext(ctx, "aws", "s3", "rm", s3URI,
				"--endpoint-url", s3Cfg.Endpoint,
			)
			delCmd.Env = append(os.Environ(),
				"AWS_ACCESS_KEY_ID="+s3Cfg.AccessKey,
				"AWS_SECRET_ACCESS_KEY="+s3Cfg.SecretKey,
				"AWS_DEFAULT_REGION="+s3Region(s3Cfg.Region),
			)
			if output, err := delCmd.CombinedOutput(); err != nil {
				s.logger.Warn("failed to delete old backup from S3",
					slog.String("path", s3URI), slog.Any("error", err), slog.String("output", string(output)))
			}
		}
		// Soft-delete the DB record
		now := time.Now()
		old.DeletedAt = &now
		if err := s.store.SystemBackups().Update(ctx, &old); err != nil {
			s.logger.Warn("failed to soft-delete old backup record", slog.String("id", old.ID.String()), slog.Any("error", err))
		}
	}
}

// startScheduler runs a ticker loop to trigger scheduled backups.
func (s *SystemBackupService) startScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cfg, err := s.GetConfig(context.Background())
		if err != nil || !cfg.Enabled || cfg.Schedule == "" {
			continue
		}
		if shouldRunNow(cfg.Schedule) {
			s.logger.Info("triggering scheduled system backup")
			if _, err := s.TriggerBackup(context.Background()); err != nil {
				s.logger.Error("scheduled system backup failed", slog.Any("error", err))
			}
		}
	}
}

// shouldRunNow checks if a cron expression matches the current minute.
func shouldRunNow(cronExpr string) bool {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return false
	}
	now := time.Now()
	return matchField(parts[0], now.Minute()) &&
		matchField(parts[1], now.Hour()) &&
		matchField(parts[2], now.Day()) &&
		matchField(parts[3], int(now.Month())) &&
		matchField(parts[4], int(now.Weekday()))
}

// matchField checks if a single cron field matches a value.
func matchField(pattern string, value int) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*/") {
		step, err := strconv.Atoi(pattern[2:])
		if err != nil || step <= 0 {
			return false
		}
		return value%step == 0
	}
	target, err := strconv.Atoi(pattern)
	if err != nil {
		return false
	}
	return target == value
}

// parseDatabaseURL extracts connection components from a PostgreSQL URL.
func parseDatabaseURL(dbURL string) (host, port, user, password, dbname string, err error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "", "", "", "", "", fmt.Errorf("invalid database URL: %w", err)
	}

	host = u.Hostname()
	port = u.Port()
	if port == "" {
		port = "5432"
	}
	user = u.User.Username()
	password, _ = u.User.Password()
	dbname = strings.TrimPrefix(u.Path, "/")

	return host, port, user, password, dbname, nil
}

// S3BackupFile represents a backup file found in S3.
type S3BackupFile struct {
	Key          string `json:"key"`
	FileName     string `json:"file_name"`
	SizeBytes    int64  `json:"size_bytes"`
	LastModified string `json:"last_modified"`
}

// ScanS3Backups connects to S3 with provided credentials and lists available .dump files.
// Only allowed on fresh installations (no users registered).
func (s *SystemBackupService) ScanS3Backups(ctx context.Context, s3 orchestrator.S3Config, path string) ([]S3BackupFile, error) {
	// Safety check: only allow scan on fresh installation (no users)
	count, err := s.store.Users().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check user count: %w", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("restore scan is only available on fresh installations")
	}

	if path == "" {
		path = "sailbox-backups"
	}

	cmd := exec.CommandContext(ctx, "aws", "s3", "ls",
		fmt.Sprintf("s3://%s/%s/", s3.Bucket, path),
		"--endpoint-url", s3.Endpoint,
	)
	cmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+s3.AccessKey,
		"AWS_SECRET_ACCESS_KEY="+s3.SecretKey,
		"AWS_DEFAULT_REGION="+s3Region(s3.Region),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 bucket: %v — %s", err, string(output))
	}

	var files []S3BackupFile
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ".dump") {
			continue
		}
		// Format: 2026-03-27 03:00:00    2100000 sailbox-20260327.dump
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		dateStr := parts[0] + " " + parts[1]
		sizeStr := parts[2]
		fileName := parts[3]

		var sizeBytes int64
		if v, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			sizeBytes = v
		}

		files = append(files, S3BackupFile{
			Key:          fmt.Sprintf("%s/%s", path, fileName),
			FileName:     fileName,
			SizeBytes:    sizeBytes,
			LastModified: dateStr,
		})
	}

	// Sort by date descending (most recent first)
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}

	return files, nil
}

// RestoreFromS3 downloads a backup from S3 and restores it via pg_restore.
func (s *SystemBackupService) RestoreFromS3(ctx context.Context, s3 orchestrator.S3Config, s3Key string) error {
	// Safety check: only allow restore on fresh installation (no users)
	count, err := s.store.Users().Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check user count: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("restore only available on fresh installation")
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "sailbox-restore-*.dump")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// Download from S3
	dlCmd := exec.CommandContext(ctx, "aws", "s3", "cp",
		fmt.Sprintf("s3://%s/%s", s3.Bucket, s3Key),
		tmpPath,
		"--endpoint-url", s3.Endpoint,
	)
	dlCmd.Env = append(os.Environ(),
		"AWS_ACCESS_KEY_ID="+s3.AccessKey,
		"AWS_SECRET_ACCESS_KEY="+s3.SecretKey,
		"AWS_DEFAULT_REGION="+s3Region(s3.Region),
	)

	if output, err := dlCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("S3 download failed: %v — %s", err, string(output))
	}

	// Parse database URL
	host, port, user, password, dbname, parseErr := parseDatabaseURL(s.dbURL)
	if parseErr != nil {
		return fmt.Errorf("failed to parse database URL: %w", parseErr)
	}

	// Run pg_restore
	restoreCmd := exec.CommandContext(ctx, "pg_restore",
		"--clean", "--if-exists",
		"--no-owner", "--no-privileges",
		"-h", host, "-p", port, "-U", user, "-d", dbname,
		tmpPath,
	)
	restoreCmd.Env = append(os.Environ(), "PGPASSWORD="+password)

	if output, err := restoreCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pg_restore failed: %v — %s", err, string(output))
	}

	s.logger.Info("system restore completed", slog.String("s3_key", s3Key))
	return nil
}

// s3Region returns the region or a sensible default.
func s3Region(region string) string {
	if region == "" {
		return "auto"
	}
	return region
}
