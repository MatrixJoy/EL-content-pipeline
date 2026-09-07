package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/oldj/english-learning/content-pipeline/internal/classify"
	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

type ReclassificationStore interface {
	ListKeys(context.Context, string, string) ([]string, error)
	Get(context.Context, string) ([]byte, string, error)
	Put(context.Context, string, string, []byte) (domain.Object, error)
}

type ReclassificationReport struct {
	Scanned, Changed, Updated, Unchanged       int
	Quality0To49, Quality50To69, Quality70To79 int
	Quality80To89, Quality90To100              int
}

func Reclassify(ctx context.Context, objects ReclassificationStore, classifier classify.Classifier, limit int, apply bool, logger *slog.Logger) (ReclassificationReport, error) {
	keys, err := objects.ListKeys(ctx, "candidates/", "/manifest.json")
	if err != nil {
		return ReclassificationReport{}, err
	}
	if limit > 0 {
		return reclassifySequential(ctx, keys, objects, classifier, limit, apply, logger)
	}
	return reclassifyConcurrent(ctx, keys, objects, classifier, apply, logger)
}

type reclassificationResult struct {
	quality                     int
	changed, updated, unchanged bool
	err                         error
}

func reclassifySequential(ctx context.Context, keys []string, objects ReclassificationStore, classifier classify.Classifier, limit int, apply bool, logger *slog.Logger) (ReclassificationReport, error) {
	report := ReclassificationReport{}
	for _, key := range keys {
		if limit > 0 && report.Changed >= limit {
			break
		}
		result := reclassifyOne(ctx, key, objects, classifier, apply)
		report.add(result)
		if result.err != nil {
			return report, result.err
		}
		logReclassificationProgress(logger, report, classifier.Version())
	}
	return report, nil
}

func reclassifyConcurrent(ctx context.Context, keys []string, objects ReclassificationStore, classifier classify.Classifier, apply bool, logger *slog.Logger) (ReclassificationReport, error) {
	workerCount := 16
	if len(keys) < workerCount {
		workerCount = len(keys)
	}
	jobs := make(chan string)
	results := make(chan reclassificationResult, workerCount)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for key := range jobs {
				results <- reclassifyOne(ctx, key, objects, classifier, apply)
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

	report := ReclassificationReport{}
	var firstErr error
	for result := range results {
		report.add(result)
		if firstErr == nil && result.err != nil {
			firstErr = result.err
		}
		logReclassificationProgress(logger, report, classifier.Version())
	}
	return report, firstErr
}

func reclassifyOne(ctx context.Context, manifestKey string, objects ReclassificationStore, classifier classify.Classifier, apply bool) reclassificationResult {
	manifestData, _, err := objects.Get(ctx, manifestKey)
	if err != nil {
		return reclassificationResult{err: fmt.Errorf("read manifest %s: %w", manifestKey, err)}
	}
	var manifest domain.Manifest
	if err = json.Unmarshal(manifestData, &manifest); err != nil {
		return reclassificationResult{err: fmt.Errorf("decode manifest %s: %w", manifestKey, err)}
	}
	articleObject, ok := manifest.Objects["article"]
	if !ok || articleObject.Key == "" {
		return reclassificationResult{err: fmt.Errorf("manifest %s has no article object", manifestKey)}
	}
	articleData, _, err := objects.Get(ctx, articleObject.Key)
	if err != nil {
		return reclassificationResult{err: fmt.Errorf("read article %s: %w", articleObject.Key, err)}
	}
	var article domain.Article
	if err = json.Unmarshal(articleData, &article); err != nil {
		return reclassificationResult{err: fmt.Errorf("decode article %s: %w", articleObject.Key, err)}
	}
	classification := classifier.Classify(article)
	result := reclassificationResult{quality: classification.QualityScore}
	if classification.RuleVersion != classifier.Version() {
		result.err = fmt.Errorf("classifier returned rule version %d, expected %d", classification.RuleVersion, classifier.Version())
		return result
	}
	if reflect.DeepEqual(manifest.Classification, classification) {
		result.unchanged = true
		return result
	}
	result.changed = true
	if !apply {
		return result
	}
	manifest.Classification = classification
	manifest.Revision++
	if manifest.Revision < 1 {
		manifest.Revision = 1
	}
	updated, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		result.err = fmt.Errorf("encode manifest %s: %w", manifestKey, err)
		return result
	}
	if _, err = objects.Put(ctx, manifestKey, "application/json", updated); err != nil {
		result.err = fmt.Errorf("write manifest %s: %w", manifestKey, err)
		return result
	}
	result.updated = true
	return result
}

func (r *ReclassificationReport) add(result reclassificationResult) {
	r.Scanned++
	if result.err == nil {
		r.addQuality(result.quality)
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
}

func logReclassificationProgress(logger *slog.Logger, report ReclassificationReport, version int) {
	if logger != nil && report.Updated > 0 && report.Updated%500 == 0 {
		logger.Info("reclassification progress", "updated", report.Updated, "rule_version", version)
	}
}

func (r *ReclassificationReport) addQuality(score int) {
	switch {
	case score < 50:
		r.Quality0To49++
	case score < 70:
		r.Quality50To69++
	case score < 80:
		r.Quality70To79++
	case score < 90:
		r.Quality80To89++
	default:
		r.Quality90To100++
	}
}
