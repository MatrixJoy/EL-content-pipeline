package source

import (
	"context"

	"github.com/oldj/english-learning/content-pipeline/internal/domain"
)

// Connector is the only contract the pipeline requires from a content source.
// A new provider implements this interface without changing classification,
// storage, CMS manifests, or crawl state.
type Connector interface {
	ID() string
	ExtractionVersion() int
	Attribution() string
	Discover(context.Context) ([]domain.Item, error)
	FetchCandidate(context.Context, string) (domain.Candidate, error)
	Download(context.Context, string) ([]byte, string, error)
}
