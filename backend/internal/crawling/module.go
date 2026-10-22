package crawling

import (
	"context"
	"time"
)

type SourceDocument struct {
	URL         string
	RetrievedAt time.Time
	ContentHash string
	Body        []byte
}

type Fetcher interface {
	Fetch(context.Context, string) (SourceDocument, error)
}

// Crawler separates source-specific retrieval and parsing from catalog writes.
type Crawler interface {
	Key() string
	Fetch(context.Context, string) (SourceDocument, error)
	Parse(SourceDocument) (Snapshot, error)
}
