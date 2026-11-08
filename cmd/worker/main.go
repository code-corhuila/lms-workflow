// Command worker is the entry point for lms-workflow — the overdue-
// detection + penalty-application saga (ADR-008-circulation-saga-scope.md).
// No HTTP surface of its own: it's driven by an internal ticker, not
// requests.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/code-corhuila/lms-workflow/internal/application/usecase"
	"github.com/code-corhuila/lms-workflow/internal/config"
	"github.com/code-corhuila/lms-workflow/internal/infrastructure/httpclient"
	"github.com/code-corhuila/lms-workflow/internal/infrastructure/logger"
	"github.com/code-corhuila/lms-workflow/internal/infrastructure/postgres"
	"github.com/code-corhuila/lms-workflow/internal/infrastructure/scheduler"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		println("config error:", err.Error())
		return err
	}

	zapLog, err := logger.New(cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = zapLog.Sync() }()
	log := zapLog.Sugar()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DSN())
	if err != nil {
		log.Errorw("failed to connect to database", "error", err)
		return err
	}
	defer pool.Close()

	runs := postgres.NewRunRepository(pool)
	circulation := httpclient.NewCirculationClient(cfg.CirculationServiceURL, cfg.InternalJWTSecret)
	membership := httpclient.NewMembershipClient(cfg.MembershipServiceURL, cfg.InternalJWTSecret)

	applyPenalty := usecase.NewApplyPenalty(runs, circulation, membership)
	ticker := scheduler.NewTicker(cfg.PollInterval, circulation, applyPenalty, log)

	log.Infow("lms-workflow started", "pollInterval", cfg.PollInterval)
	ticker.Run(ctx)

	log.Info("shutting down")
	return nil
}
