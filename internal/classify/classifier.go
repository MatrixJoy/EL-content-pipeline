package classify

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

type Classifier interface {
	Classify(domain.Article) domain.Classification
}

type Rules struct{}

var topicRules = []struct {
	slug  string
	terms []string
}{
	{"grammar", []string{"grammar", "ask a teacher", "verb", "noun", "adjective", "preposition", "pronunciation"}},
	{"vocabulary", []string{"words and their stories", "vocabulary", "idiom", "expression"}},
	{"stories", []string{"american stories", "short story", "fiction", "literature"}},
	{"science-technology", []string{"science", "technology", "health", "space", "environment", "internet"}},
	{"world-news", []string{"news", "economics", "education", "agriculture", "united states"}},
	{"culture", []string{"culture", "music", "arts", "history", "people in america"}},
	{"everyday-english", []string{"everyday grammar", "let's learn english", "english in a minute", "how to pronounce"}},
}

var sentenceEnd = regexp.MustCompile(`[.!?]+`)

func (Rules) Classify(a domain.Article) domain.Classification {
	var body strings.Builder
	metadata := strings.ToLower(a.Title + " " + a.Series + " " + strings.Join(a.Topics, " "))
	for _, p := range a.Paragraphs {
		body.WriteByte(' ')
		body.WriteString(p.Text)
	}
	text := strings.ToLower(body.String())
	seen := map[string]bool{}
	for _, rule := range topicRules {
		for _, term := range rule.terms {
			if strings.Contains(metadata, term) {
				seen[rule.slug] = true
				break
			}
		}
	}
	if strings.Contains(text, "words in this story") {
		seen["vocabulary"] = true
	}
	if len(seen) == 0 {
		seen["general-english"] = true
	}
	topics := make([]string, 0, len(seen))
	for slug := range seen {
		topics = append(topics, slug)
	}
	sort.Strings(topics)
	goals := []string{"listening", "retelling"}
	if seen["vocabulary"] || strings.Contains(text, "words in this story") {
		goals = append(goals, "vocabulary")
	}
	if seen["grammar"] || seen["everyday-english"] {
		goals = append(goals, "grammar")
	}
	level, confidence := estimateLevel(a, text)
	quality := 45
	if a.WordCount >= 250 {
		quality += 15
	}
	if a.WordCount >= 600 {
		quality += 10
	}
	if len(a.Paragraphs) >= 5 {
		quality += 10
	}
	if a.Description != "" {
		quality += 5
	}
	if a.PublishedAt != "" {
		quality += 5
	}
	if len(topics) > 0 {
		quality += 5
	}
	if quality > 100 {
		quality = 100
	}
	return domain.Classification{Media: "audio", Language: "en", Level: level, LevelConfidence: confidence, Series: a.Series, Topics: topics, LearningGoals: goals, QualityScore: quality}
}

func estimateLevel(a domain.Article, text string) (string, float64) {
	if a.Level != "" && a.Level != "unclassified" {
		return a.Level, .95
	}
	words := strings.Fields(text)
	sentences := len(sentenceEnd.FindAllString(text, -1))
	if sentences < 1 {
		sentences = 1
	}
	long := 0
	for _, w := range words {
		if len(strings.Trim(w, ",.;:!?\"'()")) >= 9 {
			long++
		}
	}
	avg := float64(len(words)) / float64(sentences)
	ratio := float64(long) / math.Max(1, float64(len(words)))
	switch {
	case avg <= 13 && ratio < .08:
		return "beginning", .62
	case avg >= 21 || ratio > .16:
		return "advanced", .62
	default:
		return "intermediate", .58
	}
}
