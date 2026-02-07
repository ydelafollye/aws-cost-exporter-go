package exporter

import (
	"context"
	"log/slog"
	"time"

	"github.com/ydelafollye/aws-cost-exporter-go/internal/collector"
)

type Poller struct {
	collector *collector.CostCollector
	interval  time.Duration
	logger    *slog.Logger
}

func NewPoller(c *collector.CostCollector, interval time.Duration, logger *slog.Logger) *Poller {
	return &Poller{
		collector: c,
		interval:  interval,
		logger:    logger,
	}
}

func (p *Poller) Run(ctx context.Context) error {
	p.logger.Info("performing initial cost data fetch")
	if err := p.collector.Refresh(ctx); err != nil {
		p.logger.Warn("initial fetch had errors", "error", err)
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("poller shutting down")
			return ctx.Err()

		case <-ticker.C:
			p.logger.Info("refreshing cost data")
			if err := p.collector.Refresh(ctx); err != nil {
				p.logger.Error("refresh failed", "error", err)
			}
		}
	}
}
