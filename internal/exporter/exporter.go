package exporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ydelafollye/aws-cost-exporter-go/internal/collector"
	"github.com/ydelafollye/aws-cost-exporter-go/internal/config"
	"github.com/ydelafollye/aws-cost-exporter-go/internal/server"
)

type Exporter struct {
	config    *config.Config
	collector *collector.CostCollector
	server    *server.Server
	logger    *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) (*Exporter, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	// Init collector
	coll, err := collector.New(cfg, logger.With("component", "collector"))
	if err != nil {
		return nil, fmt.Errorf("creating collector: %w", err)
	}

	// Record collector into Prometheus
	if err := prometheus.Register(coll); err != nil {
		// Ignore error if already registered
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegistered) {
			return nil, fmt.Errorf("registering collector: %w", err)
		}
	}

	// Create HTTP server
	srv := server.New(cfg.ExporterPort, logger.With("component", "server"))

	return &Exporter{
		config:    cfg,
		collector: coll,
		server:    srv,
		logger:    logger,
	}, nil
}

// Run HTTP server and Poller
func (e *Exporter) Run(ctx context.Context) error {
	e.logger.Info("starting exporter",
		"port", e.config.ExporterPort,
		"polling_interval", e.config.PollingInterval,
		"accounts", len(e.config.TargetAWSAccounts),
		"metrics", len(e.config.Metrics),
	)

	// Channel to capture goroutines error
	errCh := make(chan error, 2)

	// Start HTTP server
	go func() {
		if err := e.server.Start(); err != nil {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Start the poller
	poller := NewPoller(e.collector, e.config.PollingInterval, e.logger.With("component", "poller"))
	go func() {
		if err := poller.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- fmt.Errorf("poller error: %w", err)
		}
	}()

	// Wait for error or shutdown signal
	select {
	case err := <-errCh:
		e.logger.Error("component failed", "error", err)
		e.shutdown()
		return err

	case <-ctx.Done():
		e.logger.Info("shutdown signal received")
		e.shutdown()
		return nil
	}
}

// Shutdown all components
func (e *Exporter) shutdown() {
	e.logger.Info("shutting down exporter")

	// Shutdown timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop HTTP server
	if err := e.server.Shutdown(ctx); err != nil {
		e.logger.Error("server shutdown error", "error", err)
	}

	// Unregister from Prometheus
	prometheus.Unregister(e.collector)

	e.logger.Info("exporter stopped")
}

// Return the collector for testing
func (e *Exporter) Collector() *collector.CostCollector {
	return e.collector
}

// Return exporter config
func (e *Exporter) Config() *config.Config {
	return e.config
}
