package grammar

import (
	"regexp"
	"strings"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

const Version = 1

type rule struct {
	kind, title, explanation string
	pattern                  *regexp.Regexp
	choices                  []string
}

var rules = []rule{
	{
		kind: "conditional", title: "Conditional result clause",
		explanation: "A conditional sentence uses an if-clause for the condition and a modal such as will or would in the result clause.",
		pattern:     regexp.MustCompile(`(?i)\bif\b[^.!?]{3,160},[^.!?]{0,120}\b(will|would)\b`),
		choices:     []string{"will", "would", "might"},
	},
	{
		kind: "perfect", title: "Perfect tense",
		explanation: "Have, has, or had combines with a past participle to connect an action with another time or event.",
		pattern:     regexp.MustCompile(`(?i)\b(has|have|had)\s+(?:already\s+|also\s+|never\s+|recently\s+)?([a-z]+(?:ed|en)|been|begun|broken|brought|built|bought|chosen|come|done|driven|eaten|fallen|felt|found|given|gone|grown|heard|held|kept|known|left|lost|made|met|paid|read|run|said|seen|sent|shown|spoken|spent|stood|taken|taught|thought|told|understood|won|written)\b`),
		choices:     []string{"has", "have", "had"},
	},
	{
		kind: "progressive", title: "Progressive aspect",
		explanation: "A form of be followed by an -ing verb presents an action as ongoing around a particular time.",
		pattern:     regexp.MustCompile(`(?i)\b(am|is|are|was|were)\s+(?:still\s+|currently\s+|now\s+)?([a-z]{3,}ing)\b`),
		choices:     []string{"is", "are", "was", "were"},
	},
	{
		kind: "modal", title: "Modal verb",
		explanation: "A modal verb comes before the base form of another verb to express ability, advice, possibility, necessity, or prediction.",
		pattern:     regexp.MustCompile(`(?i)\b(can|could|may|might|must|should|will|would)\s+([a-z]{2,})\b`),
		choices:     []string{"can", "could", "might", "must", "should", "will", "would"},
	},
}

var sentencePattern = regexp.MustCompile(`[^.!?\n]+[.!?]+|[^.!?\n]+$`)

func Extract(article domain.Article) []domain.GrammarPoint {
	var points []domain.GrammarPoint
	seen := map[string]bool{}
	for _, paragraph := range article.Paragraphs {
		for _, raw := range sentencePattern.FindAllString(paragraph.Text, -1) {
			sentence := strings.Join(strings.Fields(raw), " ")
			if len(sentence) < 25 || len(sentence) > 300 {
				continue
			}
			conditionalSentence := rules[0].pattern.MatchString(sentence)
			for _, candidate := range rules {
				if seen[candidate.kind] {
					continue
				}
				if candidate.kind == "modal" && conditionalSentence {
					continue
				}
				match := candidate.pattern.FindStringSubmatchIndex(sentence)
				if len(match) < 4 {
					continue
				}
				answer := strings.ToLower(sentence[match[2]:match[3]])
				points = append(points, domain.GrammarPoint{
					Kind: candidate.kind, Title: candidate.title, Explanation: candidate.explanation,
					Example: sentence, Prompt: sentence[:match[2]] + "_____" + sentence[match[3]:],
					Answer: answer, Options: options(answer, candidate.choices),
				})
				seen[candidate.kind] = true
			}
		}
	}
	return points
}

func options(answer string, candidates []string) []string {
	values := []string{answer}
	for _, candidate := range candidates {
		if candidate == answer {
			continue
		}
		values = append(values, candidate)
		if len(values) == 3 {
			break
		}
	}
	return values
}
