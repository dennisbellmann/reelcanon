package canonadmin

import (
	"context"
	"errors"
	"github.com/reelcanon/reelcanon/backend/internal/catalog"
	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
	"testing"
	"time"
)

func fixture() crawling.Snapshot {
	return crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: "fixture-films", Name: "Test films", SourceURL: "https://example.com/films"}, Edition: "source-1", RetrievedAt: time.Now().UTC(), Entries: []crawling.Entry{{IMDbID: "tt0111161", Position: 1, SourceTitle: "Test A"}, {IMDbID: "tt0068646", Position: 2, SourceTitle: "Test B"}}}
}
func TestDraftPublicationAndSourceOverrides(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	source := fixture()
	result, err := catalogimport.Import(ctx, db, source)
	if err != nil || !result.EditionCreated {
		t.Fatalf("initial import: %+v %v", result, err)
	}
	result, err = catalogimport.Import(ctx, db, source)
	if err != nil || result.EditionCreated {
		t.Fatalf("idempotency: %+v %v", result, err)
	}
	repo := catalog.NewRepository(db)
	canons, err := repo.ListCanons(ctx)
	if err != nil || len(canons) != 0 {
		t.Fatalf("unpublished Canon leaked: %+v %v", canons, err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE canons SET is_live=true WHERE slug='fixture-films'`); err != nil {
		t.Fatal(err)
	}
	service := Service{DB: db}
	draft, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	draft.Snapshot.Entries[0].Position = 2
	draft.Snapshot.Entries[1].Position = 1
	draft.Reason = "Manual ordering"
	if err := service.Save(ctx, source.Canon.Slug, draft); err != nil {
		t.Fatal(err)
	}
	if err := service.Save(ctx, source.Canon.Slug, draft); !errors.Is(err, catalogimport.ErrConflict) {
		t.Fatalf("stale draft accepted: %v", err)
	}
	public, err := repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Movies[0].IMDbID != "tt0111161" {
		t.Fatalf("draft leaked: %+v %v", public, err)
	}
	draft, err = service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, source.Canon.Slug, draft.Revision); err != nil {
		t.Fatal(err)
	}
	public, err = repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Movies[0].IMDbID != "tt0068646" {
		t.Fatalf("publication failed: %+v %v", public, err)
	}
	source.Edition = "source-2"
	source.Canon.Description = "Candidate description must remain private"
	source.Entries[0].SourceTitle = "metadata update"
	result, err = catalogimport.Import(ctx, db, source)
	if err != nil || !result.NeedsReview || result.EditionCreated {
		t.Fatalf("source overrode manual edit: %+v %v", result, err)
	}
	candidate, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Snapshot.Entries[0].IMDbID != "tt0068646" {
		t.Fatal("manual position not carried forward")
	}
	public, err = repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Movies[0].IMDbID != "tt0068646" {
		t.Fatal("source candidate leaked")
	}
	if public.Canon.Description == source.Canon.Description {
		t.Fatal("candidate Canon metadata leaked before publication")
	}
	history, err := service.History(ctx, source.Canon.Slug)
	if err != nil || len(history) != 2 {
		t.Fatalf("history: %+v %v", history, err)
	}
	if err := service.Restore(ctx, source.Canon.Slug, history[1].ID, candidate.Revision); err != nil {
		t.Fatal(err)
	}
	restored, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, source.Canon.Slug, restored.Revision); err != nil {
		t.Fatal(err)
	}
	history, err = service.History(ctx, source.Canon.Slug)
	if err != nil || len(history) != 3 {
		t.Fatalf("restore must append history: %+v %v", history, err)
	}
}

func TestInvalidImportAndDraftRetainPublishedEdition(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	source := fixture()
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	service := Service{DB: db}
	draft, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	base := draft.BaseEdition
	source.Entries[1].IMDbID = source.Entries[0].IMDbID
	if _, err := catalogimport.Import(ctx, db, source); err == nil {
		t.Fatal("duplicate import accepted")
	}
	draft.Snapshot = source
	draft.Snapshot.Edition = "invalid"
	draft.Reason = "Incomplete draft"
	if err := service.Save(ctx, source.Canon.Slug, draft); err != nil {
		t.Fatal(err)
	}
	saved, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, source.Canon.Slug, saved.Revision); err == nil {
		t.Fatal("invalid draft published")
	}
	var actual int64
	if err := db.QueryRowContext(ctx, `SELECT current_edition_id FROM canons WHERE slug=$1`, source.Canon.Slug).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != base {
		t.Fatal("last valid edition changed")
	}
}

func TestStaleBaseAndTransactionRollback(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	source := fixture()
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	service := Service{DB: db}
	draft, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	source.Edition = "source-2"
	source.Entries[0].Position = 2
	source.Entries[1].Position = 1
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	if err := service.Save(ctx, source.Canon.Slug, draft); !errors.Is(err, catalogimport.ErrConflict) {
		t.Fatalf("stale edition accepted: %v", err)
	}
	source.Entries[0].Position = 1
	source.Entries[1].Position = 2
	if _, err := catalogimport.Import(ctx, db, source); err == nil {
		t.Fatal("duplicate edition label accepted")
	}
	history, err := service.History(ctx, source.Canon.Slug)
	if err != nil || len(history) != 2 {
		t.Fatalf("failed transaction damaged history: %+v %v", history, err)
	}
}
func TestPresentationPublishesAtomicallyAndSurvivesSourceRefresh(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	source := fixture()
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE canons SET is_live=true WHERE slug=$1`, source.Canon.Slug); err != nil {
		t.Fatal(err)
	}
	service := Service{DB: db}
	repo := catalog.NewRepository(db)
	draft, err := service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	draft.Snapshot.Canon.Name = "Unser Filmkanon"
	draft.Snapshot.Canon.Presentation = &crawling.CanonPresentation{Position: 7, PosterURL: "/artwork/collection-02-kinosaal.png", TitleEmbedded: false}
	draft.Reason = "Collection design"
	if err := service.Save(ctx, source.Canon.Slug, draft); err != nil {
		t.Fatal(err)
	}
	public, err := repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Canon.Name != source.Canon.Name || public.Canon.DisplayPosition != 0 {
		t.Fatalf("draft leaked: %+v %v", public.Canon, err)
	}
	draft, err = service.Load(ctx, source.Canon.Slug)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Publish(ctx, source.Canon.Slug, draft.Revision)
	if err != nil || !result.EditionCreated {
		t.Fatalf("design publication must create restorable edition: %+v %v", result, err)
	}
	public, err = repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Canon.Name != "Unser Filmkanon" || public.Canon.PosterURL != "/artwork/collection-02-kinosaal.png" || public.Canon.DisplayPosition != 7 {
		t.Fatalf("presentation not published: %+v %v", public.Canon, err)
	}
	memberships, err := repo.MovieCanons(ctx, source.Entries[0].IMDbID)
	if err != nil || len(memberships) != 1 || memberships[0].PosterURL != public.Canon.PosterURL {
		t.Fatalf("membership presentation: %+v %v", memberships, err)
	}
	source.Canon.Name = "Source renamed this list"
	source.Edition = "source-refreshed"
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		t.Fatal(err)
	}
	public, err = repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Canon.Name != "Unser Filmkanon" || public.Canon.DisplayPosition != 7 {
		t.Fatalf("source overwrote editorial configuration: %+v %v", public.Canon, err)
	}
	draft, err = service.Load(ctx, source.Canon.Slug)
	if err != nil || draft.Snapshot.Canon.Name != "Unser Filmkanon" {
		t.Fatalf("draft lost published name: %+v %v", draft, err)
	}
	draft.Snapshot.Canon.Presentation.PosterURL = "/artwork/collection-01-filmrolle.png"
	draft.Snapshot.Canon.Presentation.Position = 2
	draft.Reason = "Second poster"
	if err := service.Save(ctx, source.Canon.Slug, draft); err != nil {
		t.Fatal(err)
	}
	draft, _ = service.Load(ctx, source.Canon.Slug)
	if _, err := service.Publish(ctx, source.Canon.Slug, draft.Revision); err != nil {
		t.Fatal(err)
	}
	history, err := service.History(ctx, source.Canon.Slug)
	if err != nil || len(history) != 3 {
		t.Fatalf("history: %+v %v", history, err)
	}
	if err := service.Restore(ctx, source.Canon.Slug, history[1].ID, 0); err != nil {
		t.Fatal(err)
	}
	draft, _ = service.Load(ctx, source.Canon.Slug)
	if _, err := service.Publish(ctx, source.Canon.Slug, draft.Revision); err != nil {
		t.Fatal(err)
	}
	public, err = repo.GetCanon(ctx, source.Canon.Slug)
	if err != nil || public.Canon.PosterURL != "/artwork/collection-02-kinosaal.png" || public.Canon.DisplayPosition != 7 {
		t.Fatalf("restore lost presentation: %+v %v", public.Canon, err)
	}
	// Ordering is controlled by the cockpit, independently of names.
	other := fixture()
	other.Canon.Slug = "other-films"
	other.Canon.Name = "Z first"
	if _, err := catalogimport.Import(ctx, db, other); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE canons SET is_live=true,display_position=1 WHERE slug=$1`, other.Canon.Slug); err != nil {
		t.Fatal(err)
	}
	index, err := repo.ListCanons(ctx)
	if err != nil || len(index) != 2 || index[0].Slug != other.Canon.Slug || index[1].PosterURL != public.Canon.PosterURL {
		t.Fatalf("index order/presentation: %+v %v", index, err)
	}
}
