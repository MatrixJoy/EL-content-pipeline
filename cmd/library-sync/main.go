package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"

	"github.com/oldj/english-learning/content-pipeline/internal/classify"
	"github.com/oldj/english-learning/content-pipeline/internal/config"
	"github.com/oldj/english-learning/content-pipeline/internal/grammar"
	"github.com/oldj/english-learning/content-pipeline/internal/pipeline"
	"github.com/oldj/english-learning/content-pipeline/internal/source"
	"github.com/oldj/english-learning/content-pipeline/internal/store"
)

func main() {
	limit := flag.Int("limit", 0, "maximum new URLs to inspect; 0 means all")
	singleURL := flag.String("url", "", "inspect one article URL instead of discovering the sitemap")
	sourceID := flag.String("source", "voa", "configured source connector")
	reclassify := flag.Bool("reclassify", false, "recompute classifications from stored article objects without crawling")
	enrichGrammar := flag.Bool("enrich-grammar", false, "derive structured grammar practice from stored grammar articles")
	apply := flag.Bool("apply", false, "persist the selected offline operation; otherwise report a dry run")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	objects, err := store.NewObjects(ctx, cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Bucket, cfg.UseTLS)
	if err != nil {
		logger.Error("object storage failed", "error", err)
		os.Exit(1)
	}
	classifier := classify.Rules{}
	if *reclassify && *enrichGrammar {
		logger.Error("choose only one offline operation")
		os.Exit(2)
	}
	if *reclassify {
		report, reclassifyErr := pipeline.Reclassify(ctx, objects, classifier, *limit, *apply, logger)
		if reclassifyErr != nil {
			logger.Error("reclassification failed", "error", reclassifyErr, "scanned", report.Scanned, "changed", report.Changed, "updated", report.Updated)
			os.Exit(1)
		}
		logger.Info("reclassification complete", "apply", *apply, "rule_version", classifier.Version(), "scanned", report.Scanned, "changed", report.Changed, "updated", report.Updated, "unchanged", report.Unchanged,
			"quality_0_49", report.Quality0To49, "quality_50_69", report.Quality50To69, "quality_70_79", report.Quality70To79, "quality_80_89", report.Quality80To89, "quality_90_100", report.Quality90To100)
		return
	}
	if *enrichGrammar {
		report, enrichmentErr := pipeline.EnrichGrammar(ctx, objects, *limit, *apply, logger)
		if enrichmentErr != nil {
			logger.Error("grammar enrichment failed", "error", enrichmentErr, "scanned", report.Scanned, "eligible", report.Eligible, "changed", report.Changed, "updated", report.Updated)
			os.Exit(1)
		}
		logger.Info("grammar enrichment complete", "apply", *apply, "grammar_version", grammar.Version, "scanned", report.Scanned, "eligible", report.Eligible, "changed", report.Changed, "updated", report.Updated, "unchanged", report.Unchanged, "points", report.Points,
			"modal", report.Modal, "perfect", report.Perfect, "progressive", report.Progressive, "conditional", report.Conditional)
		return
	}
	state, err := store.OpenState(cfg.StatePath)
	if err != nil {
		logger.Error("state failed", "error", err)
		os.Exit(1)
	}
	defer state.Close()
	var connector source.Connector
	switch *sourceID {
	case "voa":
		connector = source.NewVOA(cfg.Sitemap, cfg.UserAgent, cfg.Delay, cfg.VOAMaxArticleID)
	default:
		logger.Error("unknown source", "source", *sourceID)
		os.Exit(2)
	}
	runner := pipeline.Runner{Source: connector, Classifier: classifier, State: state, Objects: objects, Logger: logger}
	if *singleURL != "" {
		err = runner.RunURLs(ctx, []string{*singleURL}, 1)
	} else {
		err = runner.Run(ctx, *limit)
	}
	if err != nil {
		logger.Error("sync failed", "error", err)
		os.Exit(1)
	}
}
