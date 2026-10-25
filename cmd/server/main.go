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
	"jobqueue/internal/jobs"
	"jobqueue/internal/middleware"
	"jobqueue/internal/models"
	"jobqueue/internal/monitoring"
	"jobqueue/internal/tasks"
	"jobqueue/internal/workers"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	tasks.Register(config.JobTypeSendEmail, &tasks.MockEmailSender{})
	tasks.Register(config.JobTypeGenerateReceipt, &tasks.ReceiptGenerator{})
	tasks.Register(config.JobTypeSummarizeText, &tasks.MockSummarizer{})
	tasks.Register(config.JobTypeEcho, tasks.EchoProcessor{})
	tasks.Register(config.JobTypeFailUntilN, tasks.FailUntilNProcessor{})
	tasks.Register(config.JobTypeAlwaysFail, tasks.AlwaysFailProcessor{})
	tasks.Register(config.JobTypeSleep, tasks.SleepProcessor{})

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := gorm.Open(postgres.Open(cfg.PostgresDSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.Job{},
		&models.JobEvent{},
		&models.FailureIncident{},
		&models.FailureEvent{},
	); err != nil {
		logger.Fatal("auto-migrate failed", zap.Error(err))
	}

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to parse redis url", zap.Error(err))
	}
	rdb := redis.NewClient(redisOpts)
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}

	if err := jobs.EnsureConsumerGroups(ctx, rdb, cfg.StreamGroup); err != nil {
		logger.Fatal("failed to create stream groups", zap.Error(err))
	}

	var diagnoser jobs.Diagnoser = jobs.NopDiagnoser{}
	if cfg.GroqAPIKey != "" {
		diagnoser = &ai.GroqDiagnoser{
			APIKey:  cfg.GroqAPIKey,
			Model:   cfg.GroqModel,
			BaseURL: cfg.GroqBaseURL,
		}
		logger.Info("groq diagnoser enabled", zap.String("model", cfg.GroqModel))
	}

	metrics := monitoring.NewMetrics()
	aiClient := ai.New(diagnoser)
	apiHandler := api.New(db, rdb, logger)
	mw := &middleware.Middleware{
		DB:    db,
		Cache: cache.New(5*time.Minute, 10*time.Minute),
	}

	highPriorityPool := workers.NewPool(ctx, config.StreamHigh, cfg.StreamGroup, cfg.JobLease, 1, 10, db, rdb, aiClient, metrics, logger)
	defaultPool := workers.NewPool(ctx, config.StreamDefault, cfg.StreamGroup, cfg.JobLease, 1, 10, db, rdb, aiClient, metrics, logger)

	highPriorityScaler := workers.NewAutoScaler(highPriorityPool, rdb, metrics, logger)
	defaultScaler := workers.NewAutoScaler(defaultPool, rdb, metrics, logger)
	claimer := workers.NewClaimer(db, rdb, aiClient, metrics, logger, cfg.StreamGroup, cfg.JobLease)

	go highPriorityScaler.Run(ctx)
	go defaultScaler.Run(ctx)
	go claimer.Run(ctx)

	router := api.NewRouter(
		mw,
		apiHandler.RegisterHandler,
		apiHandler.LoginHandler,
		apiHandler.CreateProjectHandler,
		apiHandler.ListProjectsHandler,
		apiHandler.SubmitHandler,
		apiHandler.StatusHandler,
		apiHandler.ListHandler,
		apiHandler.ListFailuresHandler,
		apiHandler.GetFailureHandler,
		apiHandler.GetFailedJobHandler,
		apiHandler.RequeueFailedJobHandler,
		apiHandler.DiscardFailedJobHandler,
	)

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

	<-ctx.Done()
	logger.Info("shutting down server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", zap.Error(err))
	}

	highPriorityPool.Shutdown()
	defaultPool.Shutdown()
	logger.Info("shutdown complete")
}
