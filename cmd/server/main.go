package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Rassimdou/Real-time-Analytics/internal/config"
	"github.com/Rassimdou/Real-time-Analytics/internal/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger, err := setupLogger(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting real-time analytics engine",
		zap.String("version", "1.0.0"),
		zap.String("config", *configPath),
	)

	// Create event queue (buffered channel)
	eventQueue := make(chan server.Event, cfg.Processing.BufferSize)

	// Determine Gin mode based on log level
	ginMode := "release"
	if cfg.Logging.Level == "debug" {
		ginMode = "debug"
	}

	// Create HTTP server
	srv := server.NewServer(cfg.GetServerAddress(), logger, eventQueue, ginMode)

	// Start worker pool to process events
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	for i := 0; i < cfg.Processing.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			processEvents(ctx, workerID, eventQueue, logger)
		}(i)
	}

	logger.Info("started worker pool",
		zap.Int("workers", cfg.Processing.WorkerCount),
		zap.Int("buffer_size", cfg.Processing.BufferSize),
	)

	// Start HTTP server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- srv.Start()
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("server error", zap.Error(err))
		os.Exit(1)

	case sig := <-quit:
		logger.Info("received shutdown signal",
			zap.String("signal", sig.String()),
		)

		// Graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(),
			cfg.Server.ShutdownTimeout,
		)
		defer shutdownCancel()

		// Shutdown HTTP server
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", zap.Error(err))
		}

		// Stop workers
		cancel()

		// Wait for workers to finish with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("all workers stopped gracefully")
		case <-time.After(10 * time.Second):
			logger.Warn("workers did not stop in time")
		}

		logger.Info("shutdown complete")
	}
}

// setupLogger creates and configures a logger
func setupLogger(level string, format string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	cfg.Level = zap.NewAtomicLevelAt(zapLevel)

	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return logger, nil
}

// processEvents is a worker function that processes events from the queue
func processEvents(ctx context.Context, workerID int, eventQueue <-chan server.Event, logger *zap.Logger) {
	logger.Info("worker started", zap.Int("worker_id", workerID))

	processed := 0
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopping",
				zap.Int("worker_id", workerID),
				zap.Int("processed", processed),
			)
			return

		case event := <-eventQueue:
			// Process the event
			// TODO: Add actual processing logic (aggregation, storage, etc.)

			processed++

			// Log event details (for now)
			logger.Debug("processing event",
				zap.Int("worker_id", workerID),
				zap.String("event_id", event.ID),
				zap.String("event_type", event.Type),
				zap.String("user_id", event.UserID),
				zap.Time("timestamp", event.Timestamp),
			)

		case <-ticker.C:
			// Periodic stats logging
			if processed > 0 {
				logger.Info("worker stats",
					zap.Int("worker_id", workerID),
					zap.Int("processed", processed),
					zap.Float64("rate", float64(processed)/10.0),
				)
				processed = 0
			}
		}
	}
}
