package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Println("  [up] initializing sailbox database schema...")

		queries := []string{
			// ────────────────────────────────────────────────────────
			// Functions
			// ────────────────────────────────────────────────────────

			`CREATE OR REPLACE FUNCTION update_updated_at()
			RETURNS TRIGGER AS $$
			BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
			$$ LANGUAGE plpgsql`,

			`CREATE OR REPLACE FUNCTION notify_change()
			RETURNS TRIGGER AS $$
			DECLARE
				payload JSONB;
				rec RECORD;
			BEGIN
				IF TG_OP = 'DELETE' THEN rec := OLD; ELSE rec := NEW; END IF;
				payload := jsonb_build_object('table', TG_TABLE_NAME, 'op', TG_OP, 'id', rec.id);
				PERFORM pg_notify('sailbox_changes', payload::text);
				RETURN rec;
			END;
			$$ LANGUAGE plpgsql`,

			// ────────────────────────────────────────────────────────
			// Core tables
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS organizations (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(255) NOT NULL UNIQUE,
				description TEXT DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,

			`CREATE TABLE IF NOT EXISTS users (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				email VARCHAR(255) NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				display_name VARCHAR(255) DEFAULT '',
				role VARCHAR(50) NOT NULL DEFAULT 'member',
				avatar_url TEXT DEFAULT '',
				first_name VARCHAR(100) DEFAULT '',
				last_name VARCHAR(100) DEFAULT '',
				two_fa_secret VARCHAR(255) DEFAULT '',
				two_fa_enabled BOOLEAN DEFAULT false,
				token_version INTEGER DEFAULT 0,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id)`,
			`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,

			`CREATE TABLE IF NOT EXISTS settings (
				key VARCHAR(255) PRIMARY KEY,
				value TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,

			// ────────────────────────────────────────────────────────
			// Projects
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS projects (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				namespace VARCHAR(255) DEFAULT '',
				description TEXT DEFAULT '',
				env_vars JSONB DEFAULT '{}',
				service_account VARCHAR(255) DEFAULT '',
				resource_quota JSONB DEFAULT '{}',
				network_policy_enabled BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_projects_org_id ON projects(org_id)`,

			// ────────────────────────────────────────────────────────
			// Applications
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS applications (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				description TEXT DEFAULT '',
				source_type VARCHAR(50) NOT NULL DEFAULT 'git',
				git_repo TEXT DEFAULT '',
				git_branch VARCHAR(255) DEFAULT 'main',
				git_provider_id UUID,
				docker_image TEXT DEFAULT '',
				compose_file TEXT DEFAULT '',
				build_type VARCHAR(50) DEFAULT 'dockerfile',
				dockerfile VARCHAR(255) DEFAULT 'Dockerfile',
				build_args JSONB DEFAULT '{}',
				build_context VARCHAR(255) DEFAULT '.',
				build_env_vars JSONB DEFAULT '{}',
				watch_paths JSONB DEFAULT '[]',
				replicas INTEGER DEFAULT 1,
				cpu_limit VARCHAR(50) DEFAULT '500m',
				cpu_request VARCHAR(20) DEFAULT '50m',
				mem_limit VARCHAR(50) DEFAULT '512Mi',
				mem_request VARCHAR(20) DEFAULT '64Mi',
				env_vars JSONB DEFAULT '{}',
				ports JSONB DEFAULT '[]',
				volumes JSONB DEFAULT '[]',
				health_check JSONB DEFAULT '{}',
				autoscaling JSONB DEFAULT '{}',
				deploy_strategy VARCHAR(20) DEFAULT 'rolling',
				deploy_strategy_config JSONB DEFAULT '{}',
				termination_grace_period INTEGER DEFAULT 30,
				namespace VARCHAR(255) DEFAULT '',
				k8s_name VARCHAR(255) DEFAULT '',
				status VARCHAR(50) DEFAULT 'idle',
				secrets JSONB DEFAULT '{}',
				no_cache BOOLEAN DEFAULT false,
				node_pool VARCHAR(255) DEFAULT '',
				webhook_secret VARCHAR(255) DEFAULT '',
				auto_deploy BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_applications_project_id ON applications(project_id)`,

			// ────────────────────────────────────────────────────────
			// Deployments
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS deployments (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
				app_name VARCHAR(255) DEFAULT '',
				project_id UUID DEFAULT NULL REFERENCES projects(id) ON DELETE SET NULL,
				status VARCHAR(50) NOT NULL DEFAULT 'queued',
				commit_sha VARCHAR(255) DEFAULT '',
				image TEXT DEFAULT '',
				build_log TEXT DEFAULT '',
				started_at TIMESTAMPTZ,
				finished_at TIMESTAMPTZ,
				trigger_type VARCHAR(50) DEFAULT 'manual',
				triggered_by UUID,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_deployments_app_id ON deployments(app_id)`,
			`CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status)`,
			`CREATE INDEX IF NOT EXISTS idx_deployments_created_at ON deployments(created_at DESC)`,

			// ────────────────────────────────────────────────────────
			// Domains
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS domains (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
				host VARCHAR(255) NOT NULL,
				tls BOOLEAN NOT NULL DEFAULT TRUE,
				auto_cert BOOLEAN NOT NULL DEFAULT TRUE,
				cert_secret VARCHAR(255) DEFAULT '',
				force_https BOOLEAN DEFAULT false,
				cert_expiry TIMESTAMPTZ DEFAULT NULL,
				ingress_ready BOOLEAN DEFAULT false,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_domains_host_active ON domains(host) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS idx_domains_app_id ON domains(app_id)`,

			// ────────────────────────────────────────────────────────
			// Managed Databases
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS shared_resources (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				type VARCHAR(50) NOT NULL,
				provider VARCHAR(50) DEFAULT '',
				config JSONB DEFAULT '{}',
				status VARCHAR(20) DEFAULT 'active',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_shared_resources_org_type ON shared_resources(org_id, type)`,

			`CREATE TABLE IF NOT EXISTS managed_databases (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				database_name VARCHAR(255) DEFAULT '',
				engine VARCHAR(50) NOT NULL,
				version VARCHAR(50) NOT NULL,
				storage_size VARCHAR(50) DEFAULT '1Gi',
				cpu_limit VARCHAR(50) DEFAULT '500m',
				mem_limit VARCHAR(50) DEFAULT '512Mi',
				credentials_secret VARCHAR(255) DEFAULT '',
				namespace VARCHAR(255) DEFAULT '',
				k8s_name VARCHAR(255) DEFAULT '',
				status VARCHAR(50) DEFAULT 'idle',
				external_port INT DEFAULT 0,
				external_enabled BOOLEAN DEFAULT false,
				backup_enabled BOOLEAN DEFAULT false,
				backup_schedule VARCHAR(100) DEFAULT '',
				backup_s3_id UUID DEFAULT NULL REFERENCES shared_resources(id) ON DELETE SET NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_managed_databases_project_id ON managed_databases(project_id)`,

			`CREATE TABLE IF NOT EXISTS database_backups (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				database_id UUID NOT NULL REFERENCES managed_databases(id) ON DELETE CASCADE,
				status VARCHAR(50) NOT NULL DEFAULT 'pending',
				restore_status VARCHAR(50) DEFAULT '',
				size_bytes BIGINT DEFAULT 0,
				file_path TEXT DEFAULT '',
				started_at TIMESTAMPTZ,
				finished_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_database_backups_database_id ON database_backups(database_id)`,

			// ────────────────────────────────────────────────────────
			// Cron Jobs
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS cron_jobs (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				name VARCHAR(255) NOT NULL,
				description TEXT DEFAULT '',
				cron_expression VARCHAR(100) NOT NULL,
				timezone VARCHAR(100) DEFAULT 'UTC',
				command TEXT NOT NULL,
				image VARCHAR(500) DEFAULT '',
				source_type VARCHAR(20) DEFAULT 'image',
				git_repo VARCHAR(500) DEFAULT '',
				git_branch VARCHAR(100) DEFAULT 'main',
				env_vars JSONB DEFAULT '{}',
				cpu_limit VARCHAR(20) DEFAULT '500m',
				mem_limit VARCHAR(20) DEFAULT '512Mi',
				namespace VARCHAR(63) DEFAULT '',
				k8s_name VARCHAR(63) DEFAULT '',
				enabled BOOLEAN DEFAULT true,
				concurrency_policy VARCHAR(20) DEFAULT 'Forbid',
				restart_policy VARCHAR(20) DEFAULT 'OnFailure',
				backoff_limit INT DEFAULT 3,
				active_deadline_seconds INT DEFAULT 0,
				last_run_at TIMESTAMPTZ,
				status VARCHAR(20) DEFAULT 'idle',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_cron_jobs_project_id ON cron_jobs(project_id)`,

			`CREATE TABLE IF NOT EXISTS cron_job_runs (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				cron_job_id UUID NOT NULL REFERENCES cron_jobs(id) ON DELETE CASCADE,
				status VARCHAR(20) NOT NULL DEFAULT 'running',
				started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				finished_at TIMESTAMPTZ,
				exit_code INT,
				logs TEXT DEFAULT '',
				trigger_type VARCHAR(20) DEFAULT 'scheduled',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,
			`CREATE INDEX IF NOT EXISTS idx_cron_job_runs_cron_job_id ON cron_job_runs(cron_job_id)`,
			`CREATE INDEX IF NOT EXISTS idx_cron_job_runs_status ON cron_job_runs(status)`,

			// ────────────────────────────────────────────────────────
			// Templates
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS templates (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(255) NOT NULL UNIQUE,
				description TEXT DEFAULT '',
				logo_url TEXT DEFAULT '',
				category VARCHAR(100) DEFAULT '',
				compose_yaml TEXT NOT NULL,
				env_schema JSONB DEFAULT '{}',
				min_cpu VARCHAR(50) DEFAULT '250m',
				min_memory VARCHAR(50) DEFAULT '256Mi',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,

			// ────────────────────────────────────────────────────────
			// Infrastructure: Nodes
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS server_nodes (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(255) NOT NULL,
				host VARCHAR(255) NOT NULL,
				port INTEGER DEFAULT 22,
				ssh_user VARCHAR(100) DEFAULT 'root',
				auth_type VARCHAR(20) DEFAULT 'password',
				ssh_key_id UUID,
				password TEXT DEFAULT '',
				role VARCHAR(20) DEFAULT 'worker',
				status VARCHAR(20) DEFAULT 'pending',
				status_msg TEXT DEFAULT '',
				k8s_node_name VARCHAR(255) DEFAULT '',
				host_key_fingerprint TEXT DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,

			// ────────────────────────────────────────────────────────
			// Team
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS project_members (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
				user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				role VARCHAR(20) NOT NULL DEFAULT 'viewer',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE(project_id, user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id)`,

			`CREATE TABLE IF NOT EXISTS invitations (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				email VARCHAR(255) NOT NULL,
				role VARCHAR(20) NOT NULL DEFAULT 'member',
				token VARCHAR(255) NOT NULL UNIQUE,
				invited_by UUID REFERENCES users(id),
				expires_at TIMESTAMPTZ NOT NULL,
				accepted_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE INDEX IF NOT EXISTS idx_invitations_token ON invitations(token)`,
			`CREATE INDEX IF NOT EXISTS idx_invitations_org ON invitations(org_id)`,

			// ────────────────────────────────────────────────────────
			// Notifications
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS notification_channels (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
				type VARCHAR(50) NOT NULL,
				enabled BOOLEAN DEFAULT false,
				config JSONB DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				UNIQUE(org_id, type)
			)`,

			// ────────────────────────────────────────────────────────
			// System Backups
			// ────────────────────────────────────────────────────────

			`CREATE TABLE IF NOT EXISTS system_backups (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				status VARCHAR(50) NOT NULL DEFAULT 'pending',
				size_bytes BIGINT DEFAULT 0,
				file_name VARCHAR(500) DEFAULT '',
				s3_bucket VARCHAR(255) DEFAULT '',
				s3_path VARCHAR(500) DEFAULT '',
				error_msg TEXT DEFAULT '',
				started_at TIMESTAMPTZ,
				finished_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)`,

			// ────────────────────────────────────────────────────────
			// Metrics (separate schema)
			// ────────────────────────────────────────────────────────

			`CREATE SCHEMA IF NOT EXISTS metrics`,

			`CREATE TABLE IF NOT EXISTS metrics.snapshots (
				id BIGSERIAL PRIMARY KEY,
				collected_at TIMESTAMPTZ NOT NULL,
				source_type VARCHAR(20) NOT NULL,
				source_name VARCHAR(255) NOT NULL,
				cpu_used_millis BIGINT DEFAULT 0,
				cpu_total_millis BIGINT DEFAULT 0,
				mem_used_bytes BIGINT DEFAULT 0,
				mem_total_bytes BIGINT DEFAULT 0,
				disk_used_bytes BIGINT,
				disk_total_bytes BIGINT,
				pod_count INT
			)`,
			`CREATE INDEX IF NOT EXISTS idx_metrics_snapshots_time ON metrics.snapshots(collected_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_metrics_snapshots_source ON metrics.snapshots(source_type, source_name)`,

			`CREATE TABLE IF NOT EXISTS metrics.events (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				event_type VARCHAR(20) NOT NULL,
				reason VARCHAR(255) NOT NULL,
				message TEXT DEFAULT '',
				namespace VARCHAR(255) DEFAULT '',
				involved_object VARCHAR(255) DEFAULT '',
				source_component VARCHAR(255) DEFAULT '',
				first_seen TIMESTAMPTZ,
				last_seen TIMESTAMPTZ,
				count INT DEFAULT 1
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_events_conflict ON metrics.events(namespace, involved_object, reason)`,
			`CREATE INDEX IF NOT EXISTS idx_metrics_events_time ON metrics.events(recorded_at DESC)`,

			`CREATE TABLE IF NOT EXISTS metrics.alerts (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				rule_name VARCHAR(255) NOT NULL,
				severity VARCHAR(20) NOT NULL,
				source_type VARCHAR(50) NOT NULL,
				source_name VARCHAR(255) NOT NULL,
				message TEXT DEFAULT '',
				fired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				resolved_at TIMESTAMPTZ,
				notified BOOLEAN DEFAULT false
			)`,
			`CREATE INDEX IF NOT EXISTS idx_metrics_alerts_active ON metrics.alerts(fired_at DESC) WHERE resolved_at IS NULL`,
		}

		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("migration failed: %w\nquery: %s", err, q)
			}
		}

		// ── Triggers ────────────────────────────────────────────

		// updated_at triggers
		updatedAtTables := []string{
			"organizations", "users", "projects", "applications",
			"deployments", "domains", "managed_databases", "templates",
			"server_nodes", "shared_resources",
		}
		for _, t := range updatedAtTables {
			q := fmt.Sprintf(
				`CREATE OR REPLACE TRIGGER trigger_%s_updated_at
				BEFORE UPDATE ON %s
				FOR EACH ROW EXECUTE FUNCTION update_updated_at()`, t, t)
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("updated_at trigger for %s: %w", t, err)
			}
		}

		// NOTIFY triggers for real-time UI updates
		notifyTables := []string{
			"applications", "deployments", "domains", "managed_databases",
			"projects", "server_nodes", "shared_resources",
			"cron_jobs", "cron_job_runs",
		}
		for _, t := range notifyTables {
			q := fmt.Sprintf(
				`CREATE OR REPLACE TRIGGER trigger_%s_notify
				AFTER INSERT OR UPDATE OR DELETE ON %s
				FOR EACH ROW EXECUTE FUNCTION notify_change()`, t, t)
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("notify trigger for %s: %w", t, err)
			}
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Println("  [down] dropping all sailbox tables...")

		// Metrics schema
		_, _ = db.ExecContext(ctx, "DROP SCHEMA IF EXISTS metrics CASCADE")

		// Tables in reverse dependency order
		tables := []string{
			"system_backups", "notification_channels",
			"invitations", "project_members",
			"cron_job_runs", "cron_jobs",
			"database_backups", "managed_databases",
			"server_nodes", "shared_resources",
			"templates", "domains", "deployments",
			"applications", "projects", "users", "organizations",
			"settings",
		}
		for _, t := range tables {
			_, _ = db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
		}
		_, _ = db.ExecContext(ctx, "DROP FUNCTION IF EXISTS update_updated_at() CASCADE")
		_, _ = db.ExecContext(ctx, "DROP FUNCTION IF EXISTS notify_change() CASCADE")
		return nil
	})
}
