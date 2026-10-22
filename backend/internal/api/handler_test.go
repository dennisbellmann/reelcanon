package api

import (
	"context"
	"encoding/json"
	"github.com/reelcanon/reelcanon/backend/internal/catalog"
	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublicAPINeverRegistersAdminWrites(t *testing.T) {
	h := NewHandler(nil, func(context.Context) error { return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, path := range []string{"/canons", "/api/v1/admin", "/api/v1/canons/imdb-top-250/publish", "/api/v1/canons/imdb-top-250/visibility", "/api/v1/canons/imdb-top-250/draft"} {
		r := httptest.NewRequest("POST", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 404 && w.Code != 405 {
			t.Fatalf("admin write exposed %s: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("health failed")
	}
}
func TestUnpublishedCanonIs404AndPublicAPIWorksIndependently(t *testing.T) {
	db := testdb.Open(t)
	h := NewHandler(catalog.NewRepository(db), db.PingContext, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, path := range []string{"/api/v1/canons", "/api/v1/canons/imdb-top-250"} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		expected := 200
		if path != "/api/v1/canons" {
			expected = 404
		}
		if w.Code != expected {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

func TestCollectionIndexContainsOnlyCurrentPublishedFilmIDs(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	source := crawling.Snapshot{
		Format:  crawling.SnapshotFormat,
		Canon:   crawling.CanonSource{Slug: "visible-films", Name: "Visible Films", SourceURL: "https://example.com/films"},
		Edition: "first", RetrievedAt: time.Now().UTC(),
		Entries: []crawling.Entry{
			{IMDbID: "tt0111161", SourceTitle: "A", Position: 1},
			{IMDbID: "tt0068646", SourceTitle: "B", Position: 2},
		},
	}
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	source.Edition = "second"
	source.Entries[1] = crawling.Entry{IMDbID: "tt0468569", SourceTitle: "C", Position: 2}
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE canons SET is_live=true WHERE slug='visible-films'`); err != nil {
		t.Fatal(err)
	}
	source.Canon.Slug = "private-films"
	source.Canon.Name = "Private Films"
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(catalog.NewRepository(db), db.PingContext, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/canons", nil))
	if w.Code != 200 {
		t.Fatalf("collection index status: %d", w.Code)
	}
	var canons []catalog.Canon
	if err := json.Unmarshal(w.Body.Bytes(), &canons); err != nil {
		t.Fatal(err)
	}
	if len(canons) != 1 || canons[0].Slug != "visible-films" {
		t.Fatalf("private collection leaked: %+v", canons)
	}
	ids := canons[0].IMDbIDs
	if canons[0].FilmCount != 2 || len(ids) != 2 || ids[0] != "tt0111161" || ids[1] != "tt0468569" {
		t.Fatalf("index must contain only current edition identities, in order: %+v", canons[0])
	}
}
