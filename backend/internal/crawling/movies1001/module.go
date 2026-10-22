package movies1001

import (
	"bytes"
	"context"
	"errors"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

// Module reuses the shared snapshot contract. A source-specific Fetcher can be
// added later without changing the catalog schema or public application.
type Module struct {
	fetcher crawling.Fetcher
}

func NewModule(fetcher crawling.Fetcher) Module {
	return Module{fetcher: fetcher}
}

func (Module) Key() string { return "1001-movies" }

func (m Module) Fetch(ctx context.Context, sourceURL string) (crawling.SourceDocument, error) {
	if m.fetcher == nil {
		return crawling.SourceDocument{}, errors.New("1001 Movies source fetcher is not configured")
	}
	return m.fetcher.Fetch(ctx, sourceURL)
}

func (Module) Parse(document crawling.SourceDocument) (crawling.Snapshot, error) {
	snapshot, err := crawling.ParseSnapshot(bytes.NewReader(document.Body))
	if err != nil {
		return crawling.Snapshot{}, err
	}
	if snapshot.Canon.Slug != "1001-movies" {
		return crawling.Snapshot{}, errors.New("expected a 1001-movies snapshot")
	}
	if snapshot.SourceContentHash == "" {
		snapshot.SourceContentHash = document.ContentHash
	}
	return snapshot, crawling.ValidateSourceSnapshot(snapshot)
}
