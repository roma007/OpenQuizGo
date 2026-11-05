package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/roma007/OpenQuizGo/internal/config"
	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/membership"
	"github.com/roma007/OpenQuizGo/internal/platform/database"
	"github.com/roma007/OpenQuizGo/internal/platform/logx"
	"github.com/roma007/OpenQuizGo/internal/question"
	"github.com/roma007/OpenQuizGo/internal/report"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger := logx.New(cfg.LogLevel, cfg.Env)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Open(initCtx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc := report.New(pool, logger)
	questionSvc := question.New(pool, logger)
	membershipSvc := membership.New(pool, cfg.PaymentCallbackSecret)
	run := func() {
		start := time.Now()
		count, err := svc.GenerateAll(ctx)
		if err != nil {
			logger.Error("generate reports", "error", err)
			return
		}
		logger.Info("reports generated", "count", count, "duration_ms", time.Since(start).Milliseconds())
		if snap, err := questionSvc.CaptureCoverageSnapshot(ctx); err != nil {
			logger.Error("capture coverage snapshot", "error", err)
		} else {
			logger.Info("coverage snapshot captured", "date", snap.CapturedOn, "total", snap.Total)
		}
		if count, err := membershipSvc.ProcessAutoRenew(ctx); err != nil {
			logger.Error("process auto renew", "error", err)
		} else if count > 0 {
			logger.Info("auto renew orders created", "count", count)
		}
		if cfg.DeviceStaleDays > 0 {
			count, err := identity.UnbindStaleDevices(ctx, pool, cfg.DeviceStaleDays)
			if err != nil {
				logger.Error("unbind stale devices", "error", err)
			} else if count > 0 {
				logger.Info("stale devices unbound", "count", count, "days", cfg.DeviceStaleDays)
			}
		}
	}

	run()
	ticker := time.NewTicker(cfg.ReportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutting down")
			return
		case <-ticker.C:
			run()
		}
	}
}
