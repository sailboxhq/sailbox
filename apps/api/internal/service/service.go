package service

import (
	"log/slog"

	"github.com/sailboxhq/sailbox/apps/api/internal/auth"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// Container holds all services with their dependencies.
type Container struct {
	Auth         *AuthService
	Project      *ProjectService
	App          *AppService
	Deploy       *DeployService
	Build        *BuildService
	Database     *DatabaseService
	Template     *TemplateService
	Domain       *DomainService
	Setting      *SettingService
	Node         *NodeService
	Resource     *ResourceService
	Metrics      *MetricsCollector
	CronJob      *CronJobService
	Team         *TeamService
	Notification *NotificationService
	Version      *VersionService
	SystemBackup *SystemBackupService
}

// NewContainer creates all services with shared dependencies.
func NewContainer(
	s store.Store,
	metricsStore store.MetricsStore,
	orch orchestrator.Orchestrator,
	jwtManager *auth.JWTManager,
	logger *slog.Logger,
	dbURL string,
) *Container {
	settingSvc := NewSettingService(s, orch, logger)
	notifSvc := NewNotificationService(s, settingSvc, logger)
	domainSvc := NewDomainService(s, orch, logger, settingSvc)
	buildSvc := NewBuildService(s, orch, logger)

	return &Container{
		Auth:         NewAuthService(s, jwtManager, logger),
		Project:      NewProjectService(s, orch, logger),
		App:          NewAppService(s, orch, logger, domainSvc),
		Deploy:       NewDeployService(s, orch, logger, buildSvc, notifSvc),
		Build:        buildSvc,
		Database:     NewDatabaseService(s, orch, logger),
		Template:     NewTemplateService(s, orch, logger),
		Domain:       domainSvc,
		Setting:      settingSvc,
		Node:         NewNodeService(s, orch, logger),
		Resource:     NewResourceService(s, logger),
		Metrics:      NewMetricsCollector(metricsStore, s, orch, logger, notifSvc),
		CronJob:      NewCronJobService(s, orch, logger),
		Team:         NewTeamService(s, logger),
		Notification: notifSvc,
		Version:      NewVersionService(logger),
		SystemBackup: NewSystemBackupService(s, settingSvc, dbURL, logger),
	}
}
