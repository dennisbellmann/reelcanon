package imdbtop250

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

type Fetcher = crawling.Fetcher

type Module struct {
	fetcher Fetcher
	parser  Parser
}

func NewModule(fetcher Fetcher) Module {
	return Module{fetcher: fetcher, parser: Parser{}}
}

func (Module) Key() string { return "imdb-top-250" }

func (m Module) Fetch(ctx context.Context, sourceURL string) (crawling.SourceDocument, error) {
	if m.fetcher == nil {
		return crawling.SourceDocument{}, errors.New("IMDb Top 250 source fetcher is not configured")
	}
	return m.fetcher.Fetch(ctx, sourceURL)
}

func (m Module) Parse(document crawling.SourceDocument) (crawling.Snapshot, error) {
	snapshot, err := m.parser.Parse(bytes.NewReader(document.Body))
	if err != nil {
		return crawling.Snapshot{}, err
	}
	if snapshot.Canon.SourceURL == "" {
		snapshot.Canon.SourceURL = document.URL
	}
	if snapshot.RetrievedAt.IsZero() {
		snapshot.RetrievedAt = document.RetrievedAt
	}
	if snapshot.SourceContentHash == "" {
		snapshot.SourceContentHash = document.ContentHash
	}
	if snapshot.RetrievedAt.IsZero() {
		snapshot.RetrievedAt = time.Now().UTC()
	}
	return snapshot, nil
}
