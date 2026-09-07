package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"reflect"
	"sync"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
	"github.com/oldj/english-learning/content-pipeline/internal/grammar"
)

type GrammarEnrichmentReport struct {
	Scanned, Eligible, Changed, Updated, Unchanged, Points int
	Modal, Perfect, Progressive, Conditional               int
}

type grammarEnrichmentResult struct {
	eligible, changed, updated, unchanged    bool
	points                                   int
	modal, perfect, progressive, conditional int
	err                                      error
}

func EnrichGrammar(ctx context.Context, objects ReclassificationStore, limit int, apply bool, logger *slog.Logger) (GrammarEnrichmentReport, error) {
	keys, err := objects.ListKeys(ctx, "candidates/", "/manifest.json")
	if err != nil {
		return GrammarEnrichmentReport{}, err
	}
	if limit > 0 {
		return enrichGrammarSequential(ctx, keys, objects, limit, apply, logger)
	}
	return enrichGrammarConcurrent(ctx, keys, objects, apply, logger)
}

func enrichGrammarSequential(ctx context.Context, keys []string, objects ReclassificationStore, limit int, apply bool, logger *slog.Logger) (GrammarEnrichmentReport, error) {
	report := GrammarEnrichmentReport{}
	for _, key := range keys {
		if report.Changed >= limit {
			break
		}
		result := enrichGrammarOne(ctx, key, objects, apply)
		report.add(result)
		if result.err != nil {
			return report, result.err
		}
		logGrammarProgress(logger, report, result.updated)
	}
	return report, nil
}

func enrichGrammarConcurrent(ctx context.Context, keys []string, objects ReclassificationStore, apply bool, logger *slog.Logger) (GrammarEnrichmentReport, error) {
	workerCount := 16
	if len(keys) < workerCount {
		workerCount = len(keys)
	}
	jobs := make(chan string)
	results := make(chan grammarEnrichmentResult, workerCount)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for key := range jobs {
				results <- enrichGrammarOne(ctx, key, objects, apply)
			}
		}()
	}
	go func() {
		for _, key := range keys {
			jobs <- key
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	report := GrammarEnrichmentReport{}
	var firstErr error
	for result := range results {
		report.add(result)
		if firstErr == nil && result.err != nil {
			firstErr = result.err
		}
		logGrammarProgress(logger, report, result.updated)
	}
	return report, firstErr
}

func enrichGrammarOne(ctx context.Context, manifestKey string, objects ReclassificationStore, apply bool) grammarEnrichmentResult {
	manifestData, _, err := objects.Get(ctx, manifestKey)
	if err != nil {
		return grammarEnrichmentResult{err: fmt.Errorf("read manifest %s: %w", manifestKey, err)}
	}
	var manifest domain.Manifest
	if err = json.Unmarshal(manifestData, &manifest); err != nil {
		return grammarEnrichmentResult{err: fmt.Errorf("decode manifest %s: %w", manifestKey, err)}
	}
	if !contains(manifest.Classification.LearningGoals, "grammar") {
		return grammarEnrichmentResult{}
	}
	result := grammarEnrichmentResult{eligible: true}
	articleObject, ok := manifest.Objects["article"]
	if !ok || articleObject.Key == "" {
		result.err = fmt.Errorf("manifest %s has no article object", manifestKey)
		return result
	}
	articleData, _, err := objects.Get(ctx, articleObject.Key)
	if err != nil {
		result.err = fmt.Errorf("read article %s: %w", articleObject.Key, err)
		return result
	}
	var article domain.Article
	if err = json.Unmarshal(articleData, &article); err != nil {
		result.err = fmt.Errorf("decode article %s: %w", articleObject.Key, err)
		return result
	}
	points := grammar.Extract(article)
	result.points = len(points)
	for _, point := range points {
		switch point.Kind {
		case "modal":
			result.modal++
		case "perfect":
			result.perfect++
		case "progressive":
			result.progressive++
		case "conditional":
			result.conditional++
		}
	}
	if article.GrammarVersion == grammar.Version && reflect.DeepEqual(article.GrammarPoints, points) {
		result.unchanged = true
		return result
	}
	result.changed = true
	if !apply {
		return result
	}
	article.GrammarVersion = grammar.Version
	article.GrammarPoints = points
	updatedArticle, err := json.MarshalIndent(article, "", "  ")
	if err != nil {
		result.err = fmt.Errorf("encode article %s: %w", articleObject.Key, err)
		return result
	}
	versionedArticleKey := path.Join(path.Dir(articleObject.Key), fmt.Sprintf("article.grammar-v%d.json", grammar.Version))
	updatedObject, err := objects.Put(ctx, versionedArticleKey, "application/json", updatedArticle)
	if err != nil {
		result.err = fmt.Errorf("write article %s: %w", versionedArticleKey, err)
		return result
	}
	manifest.Objects["article"] = updatedObject
	manifest.Revision++
	if manifest.Revision < 1 {
		manifest.Revision = 1
	}
	updatedManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		result.err = fmt.Errorf("encode manifest %s: %w", manifestKey, err)
		return result
	}
	if _, err = objects.Put(ctx, manifestKey, "application/json", updatedManifest); err != nil {
		result.err = fmt.Errorf("write manifest %s: %w", manifestKey, err)
		return result
	}
	result.updated = true
	return result
}

func (r *GrammarEnrichmentReport) add(result grammarEnrichmentResult) {
	r.Scanned++
	if result.eligible {
		r.Eligible++
	}
	if result.changed {
		r.Changed++
	}
	if result.updated {
		r.Updated++
	}
	if result.unchanged {
		r.Unchanged++
	}
	r.Points += result.points
	r.Modal += result.modal
	r.Perfect += result.perfect
	r.Progressive += result.progressive
	r.Conditional += result.conditional
}

func logGrammarProgress(logger *slog.Logger, report GrammarEnrichmentReport, justUpdated bool) {
	if logger != nil && justUpdated && report.Updated%100 == 0 {
		logger.Info("grammar enrichment progress", "updated", report.Updated, "points", report.Points, "grammar_version", grammar.Version)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
