package pipeline

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
	"github.com/oldj/english-learning/content-pipeline/internal/grammar"
)

func TestGrammarEnrichmentDryRunApplyAndIdempotency(t *testing.T) {
	articleKey := "candidates/source/item/article.json"
	manifestKey := "candidates/source/item/manifest.json"
	article := domain.Article{ContentID: "item", Paragraphs: []domain.Paragraph{{Text: "Learners can practice this useful language pattern every day."}}}
	manifest := domain.Manifest{
		SchemaVersion: 2, ContentID: "item", Revision: 2, Source: "source",
		Classification: domain.Classification{LearningGoals: []string{"listening", "grammar"}},
		Objects:        map[string]domain.Object{"article": {Key: articleKey}},
	}
	articleData, _ := json.Marshal(article)
	manifestData, _ := json.Marshal(manifest)
	objects := &memoryReclassificationStore{data: map[string][]byte{articleKey: articleData, manifestKey: manifestData}}

	dryRun, err := EnrichGrammar(t.Context(), objects, 0, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Eligible != 1 || dryRun.Changed != 1 || dryRun.Updated != 0 || dryRun.Points != 1 || dryRun.Modal != 1 || objects.puts != 0 {
		t.Fatalf("unexpected dry run: %#v, puts=%d", dryRun, objects.puts)
	}

	applied, err := EnrichGrammar(t.Context(), objects, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Updated != 1 || objects.puts != 2 {
		t.Fatalf("unexpected apply: %#v, puts=%d", applied, objects.puts)
	}
	var updatedManifest domain.Manifest
	if err = json.Unmarshal(objects.data[manifestKey], &updatedManifest); err != nil {
		t.Fatal(err)
	}
	updatedArticleKey := updatedManifest.Objects["article"].Key
	if updatedManifest.Revision != 3 || !strings.HasSuffix(updatedArticleKey, "article.grammar-v1.json") {
		t.Fatalf("updated manifest = %#v", updatedManifest)
	}
	var updatedArticle domain.Article
	if err = json.Unmarshal(objects.data[updatedArticleKey], &updatedArticle); err != nil {
		t.Fatal(err)
	}
	if updatedArticle.GrammarVersion != grammar.Version || len(updatedArticle.GrammarPoints) != 1 {
		t.Fatalf("updated article = %#v", updatedArticle)
	}

	again, err := EnrichGrammar(t.Context(), objects, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Changed != 0 || again.Unchanged != 1 || objects.puts != 2 {
		t.Fatalf("grammar enrichment is not idempotent: %#v, puts=%d", again, objects.puts)
	}
}

func TestGrammarEnrichmentSkipsNonGrammarContent(t *testing.T) {
	manifestKey := "candidates/source/item/manifest.json"
	manifest := domain.Manifest{ContentID: "item", Classification: domain.Classification{LearningGoals: []string{"listening"}}}
	manifestData, _ := json.Marshal(manifest)
	objects := &memoryReclassificationStore{data: map[string][]byte{manifestKey: manifestData}}
	report, err := EnrichGrammar(t.Context(), objects, 0, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 1 || report.Eligible != 0 || report.Changed != 0 {
		t.Fatalf("report = %#v", report)
	}
}
