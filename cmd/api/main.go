package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/roma007/OpenQuizGo/internal/admin"
	"github.com/roma007/OpenQuizGo/internal/battle"
	"github.com/roma007/OpenQuizGo/internal/config"
	"github.com/roma007/OpenQuizGo/internal/daily"
	"github.com/roma007/OpenQuizGo/internal/dashboard"
	"github.com/roma007/OpenQuizGo/internal/familyadmin"
	"github.com/roma007/OpenQuizGo/internal/httpapi"
	"github.com/roma007/OpenQuizGo/internal/identity"
	"github.com/roma007/OpenQuizGo/internal/media"
	"github.com/roma007/OpenQuizGo/internal/membership"
	"github.com/roma007/OpenQuizGo/internal/platform/cache"
	"github.com/roma007/OpenQuizGo/internal/platform/cryptox"
	"github.com/roma007/OpenQuizGo/internal/platform/database"
	"github.com/roma007/OpenQuizGo/internal/platform/logx"
	"github.com/roma007/OpenQuizGo/internal/platform/tracing"
	"github.com/roma007/OpenQuizGo/internal/question"
	"github.com/roma007/OpenQuizGo/internal/quiz"
	"github.com/roma007/OpenQuizGo/internal/report"
	"github.com/roma007/OpenQuizGo/internal/rule"
	"github.com/roma007/OpenQuizGo/internal/topic"
	"github.com/roma007/OpenQuizGo/internal/watch"
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

	shutdownTracing, err := tracing.Init(initCtx, "openquizgo-api", cfg.OTELEndpoint)
	if err != nil {
		logger.Error("init tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	pool, err := database.Open(initCtx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := cache.Open(initCtx, cfg.RedisURL)
	if err != nil {
		logger.Error("connect cache", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	cipher, err := cryptox.New(cfg.EncryptionKey, cfg.PhoneHashKey)
	if err != nil {
		logger.Error("init field cipher", "error", err)
		os.Exit(1)
	}
	identityService := identity.NewService(pool, rdb, cipher, identity.Options{
		JWTSecret:   cfg.JWTSecret,
		AccessTTL:   cfg.AccessTokenTTL,
		RefreshTTL:  cfg.RefreshTokenTTL,
		SMSMode:     cfg.SMSMode,
		SMSMockCode: cfg.SMSMockCode,
		Environment: cfg.Env,
	}, logger)

	ruleService := rule.New(pool, identityService, logger)
	watchService, err := watch.New(pool, ruleService, identityService, cfg.JWTSecret, cfg.OfflineLeaseKey, logger)
	if err != nil {
		logger.Error("init watch service", "error", err)
		os.Exit(1)
	}
	adminService := admin.NewService(pool, rdb, cfg.JWTSecret, 8*time.Hour, logger)
	questionService := question.New(pool, logger)
	reportService := report.New(pool, logger)
	dailyService := daily.New(pool, logger)
	battleService := battle.New(pool, rdb)
	topicService := topic.New(pool)
	membershipService := membership.New(pool, cfg.PaymentCallbackSecret).WithRefundSecret(cfg.RefundCallbackSecret)
	dashboardService := dashboard.New(pool)
	mediaService := media.New(cfg.UploadDir, cfg.UploadMaxMB)
	familyService := familyadmin.New(pool)
	quizService := quiz.New(pool, rdb, ruleService, watchService, logger)

	handler := httpapi.New(httpapi.Deps{
		Logger:     logger,
		DB:         pool,
		Redis:      rdb,
		Identity:   identityService,
		Admin:      adminService,
		Questions:  questionService,
		Quiz:       quizService,
		Rule:       ruleService,
		Watch:      watchService,
		Report:     reportService,
		Daily:      dailyService,
		Battle:     battleService,
		Topic:      topicService,
		Membership: membershipService,
		Dashboard:  dashboardService,
		Media:      mediaService,
		Families:   familyService,
		Version:    cfg.Version,
		Env:        cfg.Env,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("http server starting", "addr", cfg.HTTPAddr, "env", cfg.Env, "version", cfg.Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("bye")
}
