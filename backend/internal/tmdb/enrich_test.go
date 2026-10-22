package tmdb

import (
	"context"
	"fmt"
	"github.com/reelcanon/reelcanon/backend/internal/catalog"
	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEnrichmentCachesMetadataKeepsIMDbRatingsAndRetainsMissingMatches(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	rating := 9.3
	source := crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: "metadata-fixture", Name: "Metadata fixture", SourceURL: "https://example.com"}, Edition: "test", RetrievedAt: time.Now().UTC(), Entries: []crawling.Entry{{IMDbID: "tt0111161", SourceTitle: "Source A", Position: 1, IMDbRating: &rating}, {IMDbID: "tt0068646", SourceTitle: "Source B", Position: 2}}}
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/find/tt0111161":
			fmt.Fprint(w, `{"movie_results":[{"id":278}]}`)
		case "/find/tt0068646":
			fmt.Fprint(w, `{"movie_results":[]}`)
		case "/movie/278":
			fmt.Fprint(w, `{"id":278,"imdb_id":"tt0111161","title":"Die Verurteilten","original_title":"The Shawshank Redemption","overview":"Test overview","runtime":142,"release_date":"1994-09-23","poster_path":"/poster.jpg","vote_average":1.0,"genres":[{"name":"Drama"}],"production_countries":[{"iso_3166_1":"US","name":"United States"}],"credits":{"crew":[{"name":"Frank Darabont","job":"Director"}]}}`)
		case "/configuration":
			fmt.Fprint(w, `{"images":{"secure_base_url":"https://image.tmdb.org/t/p/","poster_sizes":["w500"]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := New("test-key", "")
	client.BaseURL = server.URL
	result, err := Enrich(ctx, db, client, source.Canon.Slug, false)
	if err != nil || result.Updated != 1 || result.Failed != 1 {
		t.Fatalf("enrichment: %+v %v", result, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE canons SET is_live=true WHERE slug=$1`, source.Canon.Slug); err != nil {
		t.Fatal(err)
	}
	public, err := catalog.NewRepository(db).GetCanon(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(public.Movies) != 2 || public.Movies[0].Title != "Die Verurteilten" || *public.Movies[0].IMDbRating != 9.3 || *public.Movies[0].PosterURL != "https://image.tmdb.org/t/p/w500/poster.jpg" || len(public.Movies[0].Directors) != 1 {
		t.Fatalf("public metadata: %+v", public)
	}
	result, err = Enrich(ctx, db, client, source.Canon.Slug, false)
	if err != nil || result.Updated != 0 || result.Failed != 1 {
		t.Fatalf("cache or failure retry: %+v %v", result, err)
	}
}
