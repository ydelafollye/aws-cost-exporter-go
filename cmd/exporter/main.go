package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ydelafollye/aws-cost-exporter-go/internal/config"
	"github.com/ydelafollye/aws-cost-exporter-go/internal/exporter"
)

func main() {
	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Init logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config file", "error", err)
		os.Exit(1)
	}

	// Create exporter
	exp, err := exporter.New(cfg, logger)
	if err != nil {
		slog.Error("failed to create exporter", "error", err)
		os.Exit(1)
	}

	// Run exporter
	if err := exp.Run(ctx); err != nil {
		slog.Error("exporter error", "error", err)
		os.Exit(1)
	}
}
