package pipeline

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/oldj/english-learning/content-pipeline/internal/classify"
	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

type memoryReclassificationStore struct {
	mu   sync.RWMutex
	data map[string][]byte
	puts int
}

func (m *memoryReclassificationStore) ListKeys(_ context.Context, prefix, suffix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := []string{}
	for key := range m.data {
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *memoryReclassificationStore) Get(_ context.Context, key string) ([]byte, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]byte(nil), m.data[key]...), "application/json", nil
}

func (m *memoryReclassificationStore) Put(_ context.Context, key, _ string, data []byte) (domain.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = append([]byte(nil), data...)
	m.puts++
	return domain.Object{Key: key}, nil
}

func TestReclassifyDryRunApplyAndIdempotency(t *testing.T) {
	articleKey := "candidates/source/item/article.json"
	manifestKey := "candidates/source/item/manifest.json"
	article := domain.Article{
		ContentID: "item", Title: "Grammar practice", Series: "Ask a Teacher",
		Description: "A useful lesson", PublishedAt: "2026-01-01", WordCount: 400,
		Paragraphs: []domain.Paragraph{{Text: "One."}, {Text: "Two."}, {Text: "Three."}, {Text: "Four."}, {Text: "Five."}},
	}
	manifest := domain.Manifest{
		SchemaVersion: 2, ContentID: "item", Revision: 1, Source: "source",
		Objects: map[string]domain.Object{"article": {Key: articleKey}},
	}
	articleData, _ := json.Marshal(article)
	manifestData, _ := json.Marshal(manifest)
	objects := &memoryReclassificationStore{data: map[string][]byte{articleKey: articleData, manifestKey: manifestData}}

	dryRun, err := Reclassify(t.Context(), objects, classify.Rules{}, 0, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Changed != 1 || dryRun.Updated != 0 || dryRun.Quality80To89 != 1 || objects.puts != 0 {
		t.Fatalf("unexpected dry run: %#v, puts=%d", dryRun, objects.puts)
	}

	applied, err := Reclassify(t.Context(), objects, classify.Rules{}, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Updated != 1 || objects.puts != 1 {
		t.Fatalf("unexpected apply: %#v, puts=%d", applied, objects.puts)
	}
	var updated domain.Manifest
	if err = json.Unmarshal(objects.data[manifestKey], &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Classification.RuleVersion != classify.RulesVersion {
		t.Fatalf("updated manifest = %#v", updated)
	}

	again, err := Reclassify(t.Context(), objects, classify.Rules{}, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Changed != 0 || again.Unchanged != 1 || objects.puts != 1 {
		t.Fatalf("reclassification is not idempotent: %#v, puts=%d", again, objects.puts)
	}
}
