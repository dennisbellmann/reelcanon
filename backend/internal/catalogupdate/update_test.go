package catalogupdate

import (
	"context"
	"fmt"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
	"testing"
	"time"
)

func TestRunStatusAndLastValidEditionOnFailure(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	entries := make([]crawling.Entry, 250)
	for i := range entries {
		entries[i] = crawling.Entry{IMDbID: fmt.Sprintf("tt%07d", i+1), SourceTitle: fmt.Sprintf("Test %d", i+1), Position: i + 1}
	}
	snapshot := crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: "imdb-top-250", Name: "Test Top250", SourceURL: "https://example.com/test"}, Edition: "test", RetrievedAt: time.Now().UTC(), Entries: entries}
	if _, err := ImportSnapshot(ctx, db, snapshot); err != nil {
		t.Fatal(err)
	}
	if result, err := ImportSnapshot(ctx, db, snapshot); err != nil || result.EditionCreated {
		t.Fatalf("unchanged: %+v %v", result, err)
	}
	var before int64
	if err := db.QueryRowContext(ctx, `SELECT current_edition_id FROM canons WHERE slug='imdb-top-250'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	snapshot.Entries = snapshot.Entries[:249]
	if _, err := ImportSnapshot(ctx, db, snapshot); err == nil {
		t.Fatal("incomplete run succeeded")
	}
	var after int64
	var status string
	var summary string
	if err := db.QueryRowContext(ctx, `SELECT current_edition_id FROM canons WHERE slug='imdb-top-250'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status,error_summary FROM canon_crawler_runs ORDER BY id DESC LIMIT 1`).Scan(&status, &summary); err != nil {
		t.Fatal(err)
	}
	if before != after || status != "failed" || summary == "" {
		t.Fatalf("failure damaged publication or was not recorded: %d %d %s %s", before, after, status, summary)
	}
}
