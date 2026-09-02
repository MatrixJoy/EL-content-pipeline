package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"github.com/oldj/voa-content-pipeline/internal/config"
	"github.com/oldj/voa-content-pipeline/internal/pipeline"
	"github.com/oldj/voa-content-pipeline/internal/source"
	"github.com/oldj/voa-content-pipeline/internal/store"
)

func main() {
	limit := flag.Int("limit", 0, "maximum new URLs to inspect; 0 means all")
	singleURL := flag.String("url", "", "inspect one article URL instead of discovering the sitemap")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	state, err := store.OpenState(cfg.StatePath)
	if err != nil {
		logger.Error("state failed", "error", err)
		os.Exit(1)
	}
	defer state.Close()
	objects, err := store.NewObjects(ctx, cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Bucket, cfg.UseTLS)
	if err != nil {
		logger.Error("object storage failed", "error", err)
		os.Exit(1)
	}
	runner := pipeline.Runner{Source: source.New(cfg.UserAgent, cfg.Delay), State: state, Objects: objects, Logger: logger}
	if *singleURL != "" {
		err = runner.RunURLs(ctx, []string{*singleURL}, 1)
	} else {
		err = runner.Run(ctx, cfg.Sitemap, *limit)
	}
	if err != nil {
		logger.Error("sync failed", "error", err)
		os.Exit(1)
	}
}
