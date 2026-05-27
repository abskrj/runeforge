package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/runeforge/control-plane/internal/api"
	"github.com/runeforge/control-plane/internal/config"
	"github.com/runeforge/control-plane/internal/executor/remote"
	"github.com/runeforge/control-plane/internal/observability"
	"github.com/runeforge/control-plane/internal/scheduler"
	"github.com/runeforge/control-plane/internal/store/postgres"
	redisstore "github.com/runeforge/control-plane/internal/store/redis"
	"github.com/runeforge/control-plane/internal/worker"
	"go.uber.org/zap"
)

func main() {
	// --- Logger ---
	log, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync() //nolint:errcheck

	// --- Config ---
	cfg := config.Load()
	log.Info("starting runeforge control-plane",
		zap.String("port", cfg.Port),
		zap.String("bun_executor", cfg.BunExecutorURL),
		zap.String("python_executor", cfg.PythonExecutorURL),
		zap.String("redis_url", cfg.RedisURL),
		zap.Int("worker_count", cfg.WorkerCount),
	)

	// --- Encryption key ---
	encKey := cfg.EncryptionKeyBytes(log)

	// --- Context (cancelled on shutdown signal) ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Postgres (with retry for docker-compose startup ordering) ---
	store, err := connectWithRetry(ctx, cfg.DatabaseURL, log, 10, 2*time.Second)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer store.Close()
	log.Info("postgres connected and migrations applied")

	// --- Executor ---
	exec := remote.New(cfg.BunExecutorURL, cfg.PythonExecutorURL)

	// --- Redis (optional — if unreachable we fall back to sync-only mode) ---
	var redisClient *redisstore.Client
	var sched *scheduler.Scheduler
	observer := observability.NewPipelineObserver(nil, nil, nil)

	rc, err := redisstore.New(cfg.RedisURL)
	if err != nil {
		log.Warn("redis not available, running in sync-only mode (no async/queue)",
			zap.String("redis_url", cfg.RedisURL),
			zap.Error(err),
		)
		sched = scheduler.New(store, exec, encKey)
	} else {
		redisClient = rc
		log.Info("redis connected", zap.String("redis_url", cfg.RedisURL))
		sched = scheduler.NewWithQueue(store, exec, redisClient, encKey)
	}
	sched.SetObserver(observer)

	// --- Background worker (only if Redis is available) ---
	if redisClient != nil {
		w := worker.New(redisClient, store, exec, log, cfg.WorkerCount)
		w.SetObserver(observer)
		go w.Run(ctx)
		log.Info("background worker started", zap.Int("workers", cfg.WorkerCount))
	}

	// --- Router ---
	router := api.NewRouter(store, sched, log, encKey)

	// --- HTTP server ---
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // allow long invocations and streams
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("http server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("shutting down...")

	// Cancel ctx first so background workers stop pulling new jobs.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}

	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			log.Warn("redis close", zap.Error(err))
		}
	}

	log.Info("server stopped")
}

// connectWithRetry attempts to open a Postgres connection up to maxAttempts
// times, sleeping between each attempt. This handles the race where the control
// plane container starts before Postgres is fully ready.
func connectWithRetry(ctx context.Context, dsn string, log *zap.Logger, maxAttempts int, delay time.Duration) (*postgres.Store, error) {
	var lastErr error
	for i := 1; i <= maxAttempts; i++ {
		store, err := postgres.New(ctx, dsn)
		if err == nil {
			return store, nil
		}
		lastErr = err
		log.Warn("postgres not ready, retrying",
			zap.Int("attempt", i),
			zap.Int("max", maxAttempts),
			zap.Error(err),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("gave up after %d attempts: %w", maxAttempts, lastErr)
}
