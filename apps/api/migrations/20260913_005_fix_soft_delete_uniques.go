package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// Hard UNIQUE constraints on soft-deleted tables permanently burn the value:
// once a row is soft-deleted the constraint still holds it, so the same email
// or channel type can never be used again. Replace them with partial unique
// indexes that only cover live rows, the same way domains was fixed in 004.
//
// The deployments index is a different kind of guard: it makes "one in-flight
// deployment per app" a database invariant, so two concurrent triggers can no
// longer both pass an application-level status check.
//
// Everything runs in one transaction. These statements are not individually
// idempotent in their effect (dropping a constraint, then failing to add its
// replacement, leaves the table unprotected), and a failure part-way through
// would otherwise leave the schema half-migrated.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// An install that already hit the race this index prevents has two
			// or more in-flight deployments for one app, and CREATE UNIQUE INDEX
			// would fail on them. Settle the older ones first — the same outcome
			// the stale-deployment sweeper produces — keeping the most recent.
			if _, err := tx.ExecContext(ctx, `
				UPDATE deployments d
				SET status = 'failed',
				    finished_at = COALESCE(d.finished_at, NOW()),
				    build_log = COALESCE(d.build_log, '') ||
				                E'\n\n--- Superseded: another deployment for this app was already in progress ---'
				WHERE d.deleted_at IS NULL
				  AND d.status IN ('queued', 'building', 'deploying')
				  AND EXISTS (
				      SELECT 1 FROM deployments newer
				      WHERE newer.app_id = d.app_id
				        AND newer.deleted_at IS NULL
				        AND newer.status IN ('queued', 'building', 'deploying')
				        AND (newer.created_at, newer.id) > (d.created_at, d.id)
				  )`); err != nil {
				return fmt.Errorf("settle duplicate in-flight deployments: %w", err)
			}

			stmts := []string{
				`ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_active ON users(email) WHERE deleted_at IS NULL`,

				`ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_org_id_type_key`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_channels_active ON notification_channels(org_id, type) WHERE deleted_at IS NULL`,

				`ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_name_key`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_name_active ON organizations(name) WHERE deleted_at IS NULL`,

				`ALTER TABLE templates DROP CONSTRAINT IF EXISTS templates_name_key`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idx_templates_name_active ON templates(name) WHERE deleted_at IS NULL`,

				`CREATE UNIQUE INDEX IF NOT EXISTS idx_deployments_one_in_flight ON deployments(app_id)
				 WHERE deleted_at IS NULL AND status IN ('queued', 'building', 'deploying')`,
			}
			for _, q := range stmts {
				if _, err := tx.ExecContext(ctx, q); err != nil {
					return fmt.Errorf("%s: %w", q, err)
				}
			}
			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// Going back to unconditional UNIQUE fails whenever a value was
			// legitimately reused after a soft delete — which is exactly what
			// this migration enabled. Rename the value on the soft-deleted rows
			// first: they are invisible to the application either way, and this
			// keeps the rollback from destroying data or refusing to run.
			dedupe := []string{
				`UPDATE users u SET email = u.email || '.deleted.' || u.id
				 WHERE u.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM users o WHERE o.email = u.email AND o.id <> u.id AND o.deleted_at IS NULL)`,

				`UPDATE organizations o SET name = o.name || ' (deleted ' || o.id || ')'
				 WHERE o.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM organizations x WHERE x.name = o.name AND x.id <> o.id AND x.deleted_at IS NULL)`,

				`UPDATE templates t SET name = t.name || ' (deleted ' || t.id || ')'
				 WHERE t.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM templates x WHERE x.name = t.name AND x.id <> t.id AND x.deleted_at IS NULL)`,

				// notification_channels is keyed on (org_id, type); type is a
				// closed set, so a soft-deleted duplicate is removed outright
				// rather than renamed into an invalid value.
				`DELETE FROM notification_channels c
				 WHERE c.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM notification_channels x
				               WHERE x.org_id = c.org_id AND x.type = c.type AND x.id <> c.id AND x.deleted_at IS NULL)`,

				// Any remaining same-value pairs are soft-deleted on both sides;
				// keep the newest and rename the rest.
				`UPDATE users u SET email = u.email || '.deleted.' || u.id
				 WHERE u.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM users o WHERE o.email = u.email AND o.id <> u.id)`,

				`UPDATE organizations o SET name = o.name || ' (deleted ' || o.id || ')'
				 WHERE o.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM organizations x WHERE x.name = o.name AND x.id <> o.id)`,

				`UPDATE templates t SET name = t.name || ' (deleted ' || t.id || ')'
				 WHERE t.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM templates x WHERE x.name = t.name AND x.id <> t.id)`,

				`DELETE FROM notification_channels c
				 WHERE c.deleted_at IS NOT NULL
				   AND EXISTS (SELECT 1 FROM notification_channels x
				               WHERE x.org_id = c.org_id AND x.type = c.type AND x.id <> c.id)`,
			}
			for _, q := range dedupe {
				if _, err := tx.ExecContext(ctx, q); err != nil {
					return fmt.Errorf("resolve duplicates before restoring constraint: %w", err)
				}
			}

			stmts := []string{
				`DROP INDEX IF EXISTS idx_deployments_one_in_flight`,
				`DROP INDEX IF EXISTS idx_templates_name_active`,
				`ALTER TABLE templates ADD CONSTRAINT templates_name_key UNIQUE (name)`,
				`DROP INDEX IF EXISTS idx_organizations_name_active`,
				`ALTER TABLE organizations ADD CONSTRAINT organizations_name_key UNIQUE (name)`,
				`DROP INDEX IF EXISTS idx_notification_channels_active`,
				`ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_org_id_type_key UNIQUE (org_id, type)`,
				`DROP INDEX IF EXISTS idx_users_email_active`,
				`ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email)`,
			}
			for _, q := range stmts {
				if _, err := tx.ExecContext(ctx, q); err != nil {
					return fmt.Errorf("%s: %w", q, err)
				}
			}
			return nil
		})
	})
}
