package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"jobqueue/internal/ai"
	"jobqueue/internal/api"
	"jobqueue/internal/config"
	"jobqueue/internal/middleware"
	"jobqueue/internal/models"
	"jobqueue/internal/monitoring"
	"jobqueue/internal/tasks"
	"jobqueue/internal/workers"
)

func main() {
	// Logger
	logger, err := zap.NewDevelopment() // More verbose for local dev
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	// Register Task Processors
	tasks.Register("send_email", &tasks.MockEmailSender{})
	tasks.Register("generate_receipt", &tasks.ReceiptGenerator{})
	tasks.Register("summarize_text", &tasks.MockSummarizer{})

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Database
	db, err := gorm.Open(postgres.Open(cfg.PostgresDSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	if err := db.AutoMigrate(&models.User{}, &models.Project{}, &models.Job{}); err != nil {
		logger.Fatal("auto-migrate failed", zap.Error(err))
	}

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to parse redis url", zap.Error(err))
	}
	rdb := redis.NewClient(redisOpts)
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}

	// Dependencies
	metrics := monitoring.NewMetrics()
	aiClient := ai.New(rdb)
	apiHandler := api.New(db, rdb, logger)
	mw := &middleware.Middleware{
		DB:    db,
		Cache: cache.New(5*time.Minute, 10*time.Minute),
	}

	// Worker Pools & Autoscalers
	highPriorityPool := workers.NewPool(ctx, "queue:high", 1, 10, db, rdb, aiClient, metrics, logger)
	defaultPool := workers.NewPool(ctx, "queue:default", 1, 10, db, rdb, aiClient, metrics, logger)

	highPriorityScaler := workers.NewAutoScaler(highPriorityPool, rdb, metrics, logger)
	defaultScaler := workers.NewAutoScaler(defaultPool, rdb, metrics, logger)

	reaper := workers.NewReaper(db, rdb, metrics, logger)

	go highPriorityScaler.Run(ctx)
	go defaultScaler.Run(ctx)
	go reaper.Run(ctx)

	// API Router
	router := api.NewRouter(
		mw,
		apiHandler.RegisterHandler,
		apiHandler.LoginHandler,
		apiHandler.SubmitHandler,
		apiHandler.StatusHandler,
		apiHandler.ListHandler,
	)

	// Start HTTP Server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
	}

	go func() {
		logger.Info("starting server", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	logger.Info("shutting down server gracefully")

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	// Shutdown worker pools
	highPriorityPool.Shutdown()
	defaultPool.Shutdown()

	logger.Info("shutdown complete")
}