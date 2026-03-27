package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
)

// Store aggregates all repository interfaces.
type Store interface {
	Organizations() OrganizationStore
	Users() UserStore
	Projects() ProjectStore
	Applications() ApplicationStore
	Deployments() DeploymentStore
	Domains() DomainStore
	ManagedDatabases() ManagedDatabaseStore
	Templates() TemplateStore
	Settings() SettingStore
	ServerNodes() ServerNodeStore
	SharedResources() SharedResourceStore
	CronJobs() CronJobStore
	CronJobRuns() CronJobRunStore
	DatabaseBackups() DatabaseBackupStore
	ProjectMembers() ProjectMemberStore
	Invitations() InvitationStore
	NotificationChannels() NotificationChannelStore
	SystemBackups() SystemBackupStore
}

// Pagination request parameters.
type ListParams struct {
	Page    int
	PerPage int
}

func (p ListParams) Offset() int {
	return (p.Page - 1) * p.PerPage
}

func (p ListParams) Limit() int {
	return p.PerPage
}

// DefaultListParams returns sensible defaults.
func DefaultListParams() ListParams {
	return ListParams{Page: 1, PerPage: 20}
}

// ============================================================================
// Repository Interfaces
// ============================================================================

type OrganizationStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error)
	Create(ctx context.Context, org *model.Organization) error
	Update(ctx context.Context, org *model.Organization) error
}

type UserStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, params ListParams) ([]model.User, int, error)
	UpdateRole(ctx context.Context, userID uuid.UUID, role string) error
	RemoveFromOrg(ctx context.Context, userID uuid.UUID) error
	Count(ctx context.Context) (int, error)
}

type ProjectStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	Create(ctx context.Context, project *model.Project) error
	Update(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, params ListParams) ([]model.Project, int, error)
}

// AppListFilter provides optional filters for global app queries.
type AppListFilter struct {
	Search string // name contains
	Status string // exact status match
}

type ApplicationStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error)
	Create(ctx context.Context, app *model.Application) error
	Update(ctx context.Context, app *model.Application) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.AppStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.Application, int, error)
	ListAll(ctx context.Context, params ListParams, filter AppListFilter) ([]model.Application, int, error)
}

// DeploymentListFilter provides optional filters for global deployment queries.
type DeploymentListFilter struct {
	Status string // optional status filter (queued, building, deploying, success, failed, cancelled)
}

type DeploymentStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Deployment, error)
	Create(ctx context.Context, deploy *model.Deployment) error
	Update(ctx context.Context, deploy *model.Deployment) error
	ListByApp(ctx context.Context, appID uuid.UUID, params ListParams) ([]model.Deployment, int, error)
	ListAll(ctx context.Context, params ListParams, filter DeploymentListFilter) ([]model.Deployment, int, error)
	GetLatestByApp(ctx context.Context, appID uuid.UUID) (*model.Deployment, error)
}

type DomainStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Domain, error)
	Create(ctx context.Context, domain *model.Domain) error
	Update(ctx context.Context, domain *model.Domain) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByApp(ctx context.Context, appID uuid.UUID) ([]model.Domain, error)
	GetByHost(ctx context.Context, host string) (*model.Domain, error)
}

type ManagedDatabaseStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.ManagedDatabase, error)
	Create(ctx context.Context, db *model.ManagedDatabase) error
	Update(ctx context.Context, db *model.ManagedDatabase) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.ManagedDatabase, int, error)
	FindByExternalPort(ctx context.Context, port int32) (*model.ManagedDatabase, error)
	ListExternalPorts(ctx context.Context) ([]model.ExternalPortInfo, error)
}

type TemplateStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error)
	GetByName(ctx context.Context, name string) (*model.Template, error)
	List(ctx context.Context, params ListParams) ([]model.Template, int, error)
	Create(ctx context.Context, tpl *model.Template) error
}

type SettingStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) ([]model.Setting, error)
}

type ServerNodeStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.ServerNode, error)
	Create(ctx context.Context, node *model.ServerNode) error
	Update(ctx context.Context, node *model.ServerNode) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]model.ServerNode, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.NodeStatus, msg string) error
}

type SharedResourceStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error)
	Create(ctx context.Context, resource *model.SharedResource) error
	Update(ctx context.Context, resource *model.SharedResource) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByOrg(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error)
}

type CronJobStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.CronJob, error)
	Create(ctx context.Context, cj *model.CronJob) error
	Update(ctx context.Context, cj *model.CronJob) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID, params ListParams) ([]model.CronJob, int, error)
}

type CronJobRunStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.CronJobRun, error)
	Create(ctx context.Context, run *model.CronJobRun) error
	Update(ctx context.Context, run *model.CronJobRun) error
	ListByCronJob(ctx context.Context, cronJobID uuid.UUID, params ListParams) ([]model.CronJobRun, int, error)
}

type DatabaseBackupStore interface {
	Create(ctx context.Context, backup *model.DatabaseBackup) error
	Update(ctx context.Context, backup *model.DatabaseBackup) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.DatabaseBackup, error)
	ListByDatabase(ctx context.Context, databaseID uuid.UUID, params ListParams) ([]model.DatabaseBackup, int, error)
}

type ProjectMemberStore interface {
	Create(ctx context.Context, pm *model.ProjectMember) error
	Delete(ctx context.Context, projectID, userID uuid.UUID) error
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.ProjectMember, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.ProjectMember, error)
	GetByProjectAndUser(ctx context.Context, projectID, userID uuid.UUID) (*model.ProjectMember, error)
}

type InvitationStore interface {
	Create(ctx context.Context, inv *model.Invitation) error
	GetByToken(ctx context.Context, token string) (*model.Invitation, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.Invitation, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Update(ctx context.Context, inv *model.Invitation) error
}

type NotificationChannelStore interface {
	GetByOrgAndType(ctx context.Context, orgID uuid.UUID, channelType string) (*model.NotificationChannel, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]model.NotificationChannel, error)
	ListAllEnabled(ctx context.Context) ([]model.NotificationChannel, error)
	Upsert(ctx context.Context, channel *model.NotificationChannel) error
}

type SystemBackupStore interface {
	Create(ctx context.Context, backup *model.SystemBackup) error
	Update(ctx context.Context, backup *model.SystemBackup) error
	List(ctx context.Context, params ListParams) ([]model.SystemBackup, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.SystemBackup, error)
}
