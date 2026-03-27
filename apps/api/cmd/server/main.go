package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/uptrace/bun/migrate"

	"github.com/sailboxhq/sailbox/apps/api/internal/api/ws"
	"github.com/sailboxhq/sailbox/apps/api/internal/auth"
	"github.com/sailboxhq/sailbox/apps/api/internal/config"
	"github.com/sailboxhq/sailbox/apps/api/internal/event"
	orch_iface "github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator/k3s"
	"github.com/sailboxhq/sailbox/apps/api/internal/server"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
	"github.com/sailboxhq/sailbox/apps/api/internal/store/pg"
	"github.com/sailboxhq/sailbox/apps/api/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// Database
	store, err := pg.New(cfg.Database.URL, pg.PoolConfig{
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer store.Close()
	logger.Info("connected to database")

	// Auto-migrate database
	logger.Info("running database migrations...")
	ctx := context.Background()

	// Acquire advisory lock to prevent concurrent migrations
	if _, err := store.DB().ExecContext(ctx, "SELECT pg_advisory_lock(1)"); err != nil {
		logger.Error("failed to acquire migration lock", slog.Any("error", err))
		os.Exit(1)
	}

	migrator := migrate.NewMigrator(store.DB(), migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		logger.Error("failed to init migrations", slog.Any("error", err))
		store.DB().ExecContext(ctx, "SELECT pg_advisory_unlock(1)") //nolint:errcheck
		os.Exit(1)
	}
	group, err := migrator.Migrate(ctx)
	if err != nil {
		logger.Error("failed to run migrations", slog.Any("error", err))
		store.DB().ExecContext(ctx, "SELECT pg_advisory_unlock(1)") //nolint:errcheck
		os.Exit(1)
	}
	// Release migration lock after successful migration
	store.DB().ExecContext(ctx, "SELECT pg_advisory_unlock(1)") //nolint:errcheck

	if group.IsZero() {
		logger.Info("no new migrations to run")
	} else {
		logger.Info("migrations applied", slog.String("group", group.String()))
	}

	// JWT
	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry, cfg.Auth.RefreshExpiry)

	// Orchestrator — real K3s if kubeconfig available, otherwise noop fallback
	var orch orch_iface.Orchestrator
	if cfg.K8s.InCluster || cfg.K8s.Kubeconfig != "" {
		orch, err = k3s.New(cfg.K8s, logger)
		if err != nil {
			logger.Warn("K3s connection failed, falling back to noop", slog.Any("error", err))
			orch = orch_iface.NewNoop(logger)
		}
	} else {
		logger.Info("no KUBECONFIG set, using noop orchestrator")
		orch = orch_iface.NewNoop(logger)
	}

	// Real-time events: PG LISTEN/NOTIFY → SSE broker → browser
	subscriber, err := event.NewPGSubscriber(store.DB(), logger)
	if err != nil {
		logger.Warn("PG LISTEN failed, SSE events disabled", slog.Any("error", err))
	}
	sseBroker := ws.NewSSEBroker(logger)
	if subscriber != nil {
		go sseBroker.Run(subscriber)
		defer subscriber.Close()
		logger.Info("real-time events enabled (PG LISTEN/NOTIFY → SSE)")
	}

	// Metrics store (independent from business store, same DB)
	metricsStore := pg.NewMetricsStore(store.DB())

	// Services
	services := service.NewContainer(store, metricsStore, orch, jwtManager, logger, cfg.Database.URL)

	// Start background metrics collector
	services.Metrics.Start()
	defer services.Metrics.Stop()

	// Auto-initialize settings (detect server IP, set default base domain)
	if err := services.Setting.InitDefaults(context.Background()); err != nil {
		logger.Error("failed to init settings", slog.Any("error", err))
	}

	// Router
	router := server.NewRouter(&server.RouterDeps{
		Services:   services,
		JWTManager: jwtManager,
		Orch:       orch,
		Store:      store,
		SSEBroker:  sseBroker,
		AppURL:     cfg.Server.AppURL,
		Logger:     logger,
	})

	// HTTP server
	srv := &http.Server{
		Addr:    cfg.ListenAddr(),
		Handler: router,
	}

	go func() {
		logger.Info("starting sailbox api server", slog.String("addr", cfg.ListenAddr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	// Wait for pending async notifications to finish
	services.Notification.Shutdown()

	logger.Info("server exited gracefully")
}
