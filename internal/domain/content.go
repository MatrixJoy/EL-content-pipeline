package domain

import "time"

type Paragraph struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

type Article struct {
	SchemaVersion int         `json:"schema_version"`
	ContentID     string      `json:"content_id"`
	SourceURL     string      `json:"source_url"`
	Title         string      `json:"title"`
	Description   string      `json:"description,omitempty"`
	Series        string      `json:"series,omitempty"`
	PublishedAt   string      `json:"published_at,omitempty"`
	Level         string      `json:"level,omitempty"`
	Topics        []string    `json:"topics,omitempty"`
	Language      string      `json:"language"`
	WordCount     int         `json:"word_count"`
	Paragraphs    []Paragraph `json:"paragraphs"`
}

type Classification struct {
	Media           string   `json:"media"`
	Language        string   `json:"language"`
	Level           string   `json:"level"`
	LevelConfidence float64  `json:"level_confidence"`
	Series          string   `json:"series,omitempty"`
	Topics          []string `json:"topics"`
	LearningGoals   []string `json:"learning_goals"`
	QualityScore    int      `json:"quality_score"`
}

type Object struct {
	Key       string `json:"key"`
	SHA256    string `json:"sha256"`
	ByteSize  int64  `json:"byte_size"`
	MediaType string `json:"media_type"`
}

type Manifest struct {
	SchemaVersion  int               `json:"schema_version"`
	ContentID      string            `json:"content_id"`
	Revision       int               `json:"revision"`
	Status         string            `json:"status"`
	Source         string            `json:"source"`
	SourceURL      string            `json:"source_url"`
	CapturedAt     time.Time         `json:"captured_at"`
	Classification Classification    `json:"classification"`
	Objects        map[string]Object `json:"objects"`
	Attribution    string            `json:"attribution"`
}

type Candidate struct {
	Article   Article
	HTML      []byte
	AudioURL  string
	AudioType string
}

type Item struct {
	URL          string
	LastModified time.Time
}
