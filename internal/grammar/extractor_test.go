package grammar

import (
	"testing"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

func TestExtractsUniqueHighConfidenceGrammarPoints(t *testing.T) {
	article := domain.Article{Paragraphs: []domain.Paragraph{{Text: "If the weather improves, farmers will plant the crop tomorrow. Scientists have found a safer method. The workers were building a new system. People can learn the process quickly."}}}
	points := Extract(article)
	if len(points) != 4 {
		t.Fatalf("points = %#v", points)
	}
	want := map[string]string{"conditional": "will", "perfect": "have", "progressive": "were", "modal": "can"}
	for _, point := range points {
		if want[point.Kind] != point.Answer {
			t.Errorf("%s answer = %q", point.Kind, point.Answer)
		}
		if point.Prompt == point.Example || len(point.Options) != 3 {
			t.Errorf("incomplete point: %#v", point)
		}
	}
}

func TestExtractsAtMostOneExamplePerKind(t *testing.T) {
	article := domain.Article{Paragraphs: []domain.Paragraph{{Text: "Students can practice every day. Teachers should offer useful examples. Learners might ask questions."}}}
	points := Extract(article)
	if len(points) != 1 || points[0].Kind != "modal" {
		t.Fatalf("points = %#v", points)
	}
}

func TestIgnoresShortFragments(t *testing.T) {
	points := Extract(domain.Article{Paragraphs: []domain.Paragraph{{Text: "You can go."}}})
	if len(points) != 0 {
		t.Fatalf("points = %#v", points)
	}
}
