package migrations

import "github.com/uptrace/bun/migrate"

// Migrations is the collection of all database migrations.
// Each migration file registers itself via init().
var Migrations = migrate.NewMigrations()
