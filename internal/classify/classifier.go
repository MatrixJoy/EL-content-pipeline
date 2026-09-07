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
	Version() int
}

type Rules struct{}

const RulesVersion = 2

func (Rules) Version() int { return RulesVersion }

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
	quality := 10
	switch {
	case a.WordCount >= 250 && a.WordCount <= 1200:
		quality += 25
	case a.WordCount >= 150 && a.WordCount < 250:
		quality += 14
	case a.WordCount >= 80 && a.WordCount < 150:
		quality += 5
	case a.WordCount > 1200 && a.WordCount <= 1800:
		quality += 18
	case a.WordCount > 1800:
		quality += 8
	}
	switch {
	case len(a.Paragraphs) >= 5:
		quality += 15
	case len(a.Paragraphs) >= 2:
		quality += 8
	}
	if a.Description != "" {
		quality += 8
	}
	if a.PublishedAt != "" {
		quality += 5
	}
	if a.Series != "" {
		quality += 5
	}
	if len(topics) == 1 && topics[0] == "general-english" {
		quality += 3
	} else {
		quality += 8
	}
	if len(a.FeaturedWords) >= 3 {
		quality += 10
	} else if len(a.FeaturedWords) > 0 {
		quality += 6
	}
	if seen["grammar"] || seen["vocabulary"] || seen["stories"] || seen["everyday-english"] {
		quality += 8
	}
	if a.Level != "" && a.Level != "unclassified" {
		quality += 6
	}
	if quality > 100 {
		quality = 100
	}
	return domain.Classification{RuleVersion: RulesVersion, Media: "audio", Language: "en", Level: level, LevelConfidence: confidence, Series: a.Series, Topics: topics, LearningGoals: goals, QualityScore: quality}
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
