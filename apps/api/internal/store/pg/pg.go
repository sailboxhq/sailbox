package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

// Store implements store.Store backed by PostgreSQL using Bun ORM.
//
// A Store is either root-scoped (db non-nil, queries run on the pool) or
// transaction-scoped (db nil, queries run on the enclosing bun.Tx). Both share
// the same sub-store implementations, which only need a bun.IDB.
type Store struct {
	db  *bun.DB
	idb bun.IDB
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
			return &Store{db: db, idb: db}, nil
		}
		if i < 29 {
			time.Sleep(time.Second)
		}
	}

	return nil, fmt.Errorf("database not reachable after 30s: %w", err)
}

// setupLockID is an arbitrary but stable key for the first-run setup advisory lock.
const setupLockID = 8231977

// AcquireSetupLock serialises first-run registration. Without it two requests
// can both observe an empty users table and each create an org plus an owner.
func (s *Store) AcquireSetupLock(ctx context.Context) error {
	_, err := s.idb.NewRaw("SELECT pg_advisory_xact_lock(?)", setupLockID).Exec(ctx)
	return err
}

// DB returns the underlying bun.DB for use in migrations.
func (s *Store) DB() *bun.DB {
	return s.db
}

// RunInTx runs fn inside a single database transaction. Every store obtained
// from the Store handed to fn writes through that transaction, so a returned
// error rolls back the whole unit of work.
//
// Calls nest safely: when the receiver is already transaction-scoped, fn runs
// in the existing transaction rather than opening a second one.
func (s *Store) RunInTx(ctx context.Context, fn func(ctx context.Context, tx store.Store) error) error {
	if s.db == nil {
		return fn(ctx, s)
	}
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, &Store{idb: tx})
	})
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Organizations() store.OrganizationStore { return &organizationStore{db: s.idb} }
func (s *Store) Users() store.UserStore                 { return &userStore{db: s.idb} }
func (s *Store) Projects() store.ProjectStore           { return &projectStore{db: s.idb} }
func (s *Store) Applications() store.ApplicationStore   { return &applicationStore{db: s.idb} }
func (s *Store) Deployments() store.DeploymentStore     { return &deploymentStore{db: s.idb} }
func (s *Store) Domains() store.DomainStore             { return &domainStore{db: s.idb} }
func (s *Store) ManagedDatabases() store.ManagedDatabaseStore {
	return &managedDatabaseStore{db: s.idb}
}
func (s *Store) Templates() store.TemplateStore             { return &templateStore{db: s.idb} }
func (s *Store) Settings() store.SettingStore               { return &settingStore{db: s.idb} }
func (s *Store) ServerNodes() store.ServerNodeStore         { return &serverNodeStore{db: s.idb} }
func (s *Store) SharedResources() store.SharedResourceStore { return &sharedResourceStore{db: s.idb} }
func (s *Store) CronJobs() store.CronJobStore               { return &cronJobStore{db: s.idb} }
func (s *Store) CronJobRuns() store.CronJobRunStore         { return &cronJobRunStore{db: s.idb} }
func (s *Store) DatabaseBackups() store.DatabaseBackupStore { return &databaseBackupStore{db: s.idb} }
func (s *Store) ProjectMembers() store.ProjectMemberStore   { return &projectMemberStore{db: s.idb} }
func (s *Store) Invitations() store.InvitationStore         { return &invitationStore{db: s.idb} }
func (s *Store) NotificationChannels() store.NotificationChannelStore {
	return &notificationChannelStore{db: s.idb}
}
func (s *Store) SystemBackups() store.SystemBackupStore { return &systemBackupStore{db: s.idb} }
