package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/uptrace/bun/migrate"

	"github.com/sailboxhq/sailbox/apps/api/internal/config"
	"github.com/sailboxhq/sailbox/apps/api/internal/store/pg"
	"github.com/sailboxhq/sailbox/apps/api/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <command> [args]")
		fmt.Println("Commands: up, rollback, create <name>, status, init")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	store, err := pg.New(cfg.Database.URL)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	migrator := migrate.NewMigrator(store.DB(), migrations.Migrations)
	ctx := context.Background()

	command := os.Args[1]

	switch command {
	case "init":
		if err := migrator.Init(ctx); err != nil {
			logger.Error("init failed", slog.Any("error", err))
			os.Exit(1)
		}
		fmt.Println("Migration tables created.")

	case "up":
		if err := migrator.Init(ctx); err != nil {
			logger.Error("init failed", slog.Any("error", err))
			os.Exit(1)
		}
		group, err := migrator.Migrate(ctx)
		if err != nil {
			logger.Error("migration failed", slog.Any("error", err))
			os.Exit(1)
		}
		if group.IsZero() {
			fmt.Println("No new migrations to run.")
		} else {
			fmt.Printf("Migrated: %s\n", group)
		}

	case "rollback":
		group, err := migrator.Rollback(ctx)
		if err != nil {
			logger.Error("rollback failed", slog.Any("error", err))
			os.Exit(1)
		}
		if group.IsZero() {
			fmt.Println("Nothing to rollback.")
		} else {
			fmt.Printf("Rolled back: %s\n", group)
		}

	case "status":
		if err := migrator.Init(ctx); err != nil {
			logger.Error("init failed", slog.Any("error", err))
			os.Exit(1)
		}
		ms, err := migrator.MigrationsWithStatus(ctx)
		if err != nil {
			logger.Error("status failed", slog.Any("error", err))
			os.Exit(1)
		}
		fmt.Printf("migrations: %s\n", ms)
		fmt.Printf("unapplied: %s\n", ms.Unapplied())
		fmt.Printf("last group: %s\n", ms.LastGroup())

	case "create":
		if len(os.Args) < 3 {
			fmt.Println("Usage: migrate create <name>")
			os.Exit(1)
		}
		name := os.Args[2]
		mf, err := migrator.CreateGoMigration(ctx, name)
		if err != nil {
			logger.Error("create failed", slog.Any("error", err))
			os.Exit(1)
		}
		fmt.Printf("Created migration: %s\n", mf.Name)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
