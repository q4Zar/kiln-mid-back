package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/application"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/infrastructure/postgres"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/infrastructure/tzkt"
	httpHandler "github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/internal/interfaces/http"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/config"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/logger"
	"github.com/q4ZAr/kiln-mid-back/tezos-delegation-service/pkg/metrics"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Environment)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Tezos Delegation Service...")

	db, err := postgres.NewConnection(&cfg.Database, log)
	if err != nil {
		log.Fatalw("Failed to connect to database", "error", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db, log); err != nil {
		log.Fatalw("Failed to run migrations", "error", err)
	}

	repo := postgres.NewRepository(db, log)

	tzktClient := tzkt.NewClient(
		cfg.TzktAPI.BaseURL,
		cfg.TzktAPI.RequestTimeout,
		cfg.TzktAPI.MaxRetries,
		cfg.TzktAPI.RetryDelay,
		log,
	)

	service := application.NewService(repo, tzktClient, &cfg.TzktAPI, log)

	// Initialize metrics with existing data
	initializeMetrics(repo, log)

	if err := service.StartPolling(); err != nil {
		log.Fatalw("Failed to start polling", "error", err)
	}
	defer service.StopPolling()

	router := httpHandler.NewRouter(service, log)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.Server.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	if cfg.Metrics.Enabled {
		go func() {
			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.Handler())
			metricsServer := &http.Server{
				Addr:    ":" + cfg.Metrics.Port,
				Handler: metricsMux,
			}
			log.Infow("Starting metrics server", "port", cfg.Metrics.Port)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Errorw("Metrics server error", "error", err)
			}
		}()
	}

	go func() {
		log.Infow("Starting HTTP server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalw("Failed to start server", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorw("Server forced to shutdown", "error", err)
	}

	log.Info("Server shutdown complete")
}

func initializeMetrics(repo *postgres.Repository, log *logger.Logger) {
	// Get total count of delegations from database
	delegations, err := repo.FindAll(nil)
	if err != nil {
		log.Errorw("Failed to get delegation count for metrics", "error", err)
		return
	}

	// Initialize the counter with the existing count
	if len(delegations) > 0 {
		metrics.DelegationsStored.Add(float64(len(delegations)))
		log.Infow("Initialized metrics", "existing_delegations", len(delegations))
	}

	// Get last indexed level
	lastLevel, err := repo.GetLastIndexedLevel()
	if err == nil {
		metrics.UpdateLastIndexedLevel(lastLevel)
	}
}
