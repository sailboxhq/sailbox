package pg

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// Store implements store.Store backed by PostgreSQL using Bun ORM.
type Store struct {
	db *bun.DB
}

// PoolConfig holds connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// New creates a new PostgreSQL-backed Store with connection retry and pool config.
func New(databaseURL string, pool ...PoolConfig) (*Store, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))

	// Apply pool settings (enables automatic reconnect on broken connections)
	if len(pool) > 0 {
		p := pool[0]
		if p.MaxOpenConns > 0 {
			sqldb.SetMaxOpenConns(p.MaxOpenConns)
		}
		if p.MaxIdleConns > 0 {
			sqldb.SetMaxIdleConns(p.MaxIdleConns)
		}
		if p.ConnMaxLifetime > 0 {
			sqldb.SetConnMaxLifetime(p.ConnMaxLifetime)
		}
	} else {
		sqldb.SetMaxOpenConns(25)
		sqldb.SetMaxIdleConns(5)
		sqldb.SetConnMaxLifetime(5 * time.Minute)
	}

	db := bun.NewDB(sqldb, pgdialect.New())

	// Retry connection — PG may still be starting
	var err error
	for i := range 30 {
		if err = db.Ping(); err == nil {
			return &Store{db: db}, nil
		}
		if i < 29 {
			time.Sleep(time.Second)
		}
	}

	return nil, fmt.Errorf("database not reachable after 30s: %w", err)
}

// DB returns the underlying bun.DB for use in migrations.
func (s *Store) DB() *bun.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Organizations() store.OrganizationStore { return &organizationStore{db: s.db} }
func (s *Store) Users() store.UserStore                 { return &userStore{db: s.db} }
func (s *Store) Projects() store.ProjectStore           { return &projectStore{db: s.db} }
func (s *Store) Applications() store.ApplicationStore   { return &applicationStore{db: s.db} }
func (s *Store) Deployments() store.DeploymentStore     { return &deploymentStore{db: s.db} }
func (s *Store) Domains() store.DomainStore             { return &domainStore{db: s.db} }
func (s *Store) ManagedDatabases() store.ManagedDatabaseStore {
	return &managedDatabaseStore{db: s.db}
}
func (s *Store) Templates() store.TemplateStore             { return &templateStore{db: s.db} }
func (s *Store) Settings() store.SettingStore               { return &settingStore{db: s.db} }
func (s *Store) ServerNodes() store.ServerNodeStore         { return &serverNodeStore{db: s.db} }
func (s *Store) SharedResources() store.SharedResourceStore { return &sharedResourceStore{db: s.db} }
func (s *Store) CronJobs() store.CronJobStore               { return &cronJobStore{db: s.db} }
func (s *Store) CronJobRuns() store.CronJobRunStore         { return &cronJobRunStore{db: s.db} }
func (s *Store) DatabaseBackups() store.DatabaseBackupStore { return &databaseBackupStore{db: s.db} }
func (s *Store) ProjectMembers() store.ProjectMemberStore   { return &projectMemberStore{db: s.db} }
func (s *Store) Invitations() store.InvitationStore         { return &invitationStore{db: s.db} }
func (s *Store) NotificationChannels() store.NotificationChannelStore {
	return &notificationChannelStore{db: s.db}
}
func (s *Store) SystemBackups() store.SystemBackupStore { return &systemBackupStore{db: s.db} }
