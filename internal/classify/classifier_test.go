package classify

import (
	"slices"
	"testing"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

func TestRulesClassifyLearningUses(t *testing.T) {
	a := domain.Article{
		Title: "How English Verbs Work", Series: "Ask a Teacher", Language: "en",
		WordCount: 700, Description: "A grammar lesson", PublishedAt: "2026-01-01",
		Paragraphs: []domain.Paragraph{{Text: "This lesson explains a verb and gives vocabulary in Words in This Story."}, {Text: "Learners listen and practice the expression."}, {Text: "More examples follow for English students."}, {Text: "The teacher explains each example."}, {Text: "Now answer the questions."}},
	}
	got := (Rules{}).Classify(a)
	for _, want := range []string{"grammar", "vocabulary"} {
		if !slices.Contains(got.Topics, want) {
			t.Errorf("topics %v missing %q", got.Topics, want)
		}
	}
	for _, want := range []string{"listening", "retelling", "vocabulary", "grammar"} {
		if !slices.Contains(got.LearningGoals, want) {
			t.Errorf("goals %v missing %q", got.LearningGoals, want)
		}
	}
	if got.QualityScore < 80 {
		t.Errorf("quality score = %d, want >= 80", got.QualityScore)
	}
}

func TestRulesUsesDeclaredLevel(t *testing.T) {
	got := (Rules{}).Classify(domain.Article{Title: "Lesson", Level: "beginning", WordCount: 300})
	if got.Level != "beginning" || got.LevelConfidence < .9 {
		t.Fatalf("level = %s confidence = %f", got.Level, got.LevelConfidence)
	}
}
