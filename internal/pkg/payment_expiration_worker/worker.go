package paymentexpirationworker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type ExpiredOrderRepository interface {
	CancelExpiredOrders(ctx context.Context, limit int) (int, error)
}

type Config struct {
	Interval  time.Duration `mapstructure:"interval"`
	BatchSize int           `mapstructure:"batch_size"`
}

func DefaultConfig() Config {
	return Config{
		Interval:  30 * time.Second,
		BatchSize: 100,
	}
}

func (c Config) Validate() error {
	var errs []error
	if c.Interval <= 0 {
		errs = append(errs, fmt.Errorf("payment expiration worker: interval must be positive, got %s", c.Interval))
	}
	if c.BatchSize <= 0 {
		errs = append(errs, fmt.Errorf("payment expiration worker: batch_size must be positive, got %d", c.BatchSize))
	}
	return errors.Join(errs...)
}

func RunExpirationWorker(ctx context.Context, repo ExpiredOrderRepository, cfg Config, logger *slog.Logger) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	process := func() {
		count, err := repo.CancelExpiredOrders(ctx, cfg.BatchSize)
		if err != nil {
			logger.ErrorContext(ctx, "failed to cancel expired payment orders",
				"error", err,
			)
			return
		}

		if count > 0 {
			logger.InfoContext(ctx, "expired payment orders canceled",
				"count", count,
			)
		}
	}

	process()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}
