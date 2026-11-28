// Переиндексация поискового текста постов: запускается один раз после накатывания
// миграции со search_text (или после изменения правил санитайзинга).
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os/signal"
	"syscall"

	contentconfig "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/config"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/reindex"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
)

func main() {
	var (
		configPath string
		batchSize  int
	)

	flag.StringVar(&configPath, "config-file", contentconfig.DefaultConfigPath, "path to yaml config")
	flag.IntVar(&batchSize, "batch-size", reindex.DefaultBatchSize, "posts per batch")
	flag.Parse()

	if err := run(configPath, batchSize); err != nil {
		log.Fatal(err)
	}
}

func run(configPath string, batchSize int) error {
	cfg, err := contentconfig.Load(configPath)
	if err != nil {
		return err
	}

	appLogger := logger.New(cfg.Log)

	db, err := postgres.NewClient(cfg.Postgres)
	if err != nil {
		return err
	}
	defer db.Close()

	postSanitizer, err := sanitizer.NewSanitizer(cfg.Sanitizer)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	total, err := reindex.NewRunner(db.Pool, postSanitizer, batchSize, appLogger).Run(ctx)
	if err != nil {
		return err
	}

	appLogger.Info("reindex finished", slog.Int64("posts", total))

	return nil
}
