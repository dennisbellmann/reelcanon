package adminui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/canonadmin"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/crawling/sources"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
)

func creationSnapshot(slug string) crawling.Snapshot {
	return crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: slug, Name: "Festival-Filme", SourceURL: "https://example.com/festival", Presentation: &crawling.CanonPresentation{Position: 4, PosterURL: "/artwork/collection-03-filmwelten.png"}}, Edition: "2026", RetrievedAt: time.Now().UTC(), Entries: []crawling.Entry{{IMDbID: "tt0111161", Position: 1, SourceTitle: "First"}, {IMDbID: "tt0068646", Position: 2, SourceTitle: "Second"}}}
}

func creationRequest(t *testing.T, handler http.Handler, path string, values url.Values, upload []byte) *httptest.ResponseRecorder {
	t.Helper()
	values.Set("_csrf", strings.Repeat("a", 64))
	var body io.Reader
	contentType := "application/x-www-form-urlencoded"
	if upload != nil {
		buffer := new(bytes.Buffer)
		writer := multipart.NewWriter(buffer)
		for key, entries := range values {
			for _, value := range entries {
				if err := writer.WriteField(key, value); err != nil {
					t.Fatal(err)
				}
			}
		}
		file, err := writer.CreateFormFile("file", "films.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(upload); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		body, contentType = buffer, writer.FormDataContentType()
	} else {
		body = strings.NewReader(values.Encode())
	}
	r := httptest.NewRequest(http.MethodPost, path, body)
	r.SetBasicAuth("admin", "test-password")
	r.Header.Set("Content-Type", contentType)
	r.AddCookie(&http.Cookie{Name: "reelcanon_admin_csrf", Value: strings.Repeat("a", 64)})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestJSONCreationIsAtomicAndStartsHiddenWithPresentation(t *testing.T) {
	db := testdb.Open(t)
	h := New(db, "admin", "test-password", slog.New(slog.NewTextHandler(io.Discard, nil)))
	snapshot := creationSnapshot("festival")
	invalid := snapshot
	invalid.Entries = append(invalid.Entries, invalid.Entries[0])
	payload, _ := json.Marshal(invalid)
	w := creationRequest(t, h, "/canons", url.Values{"kind": {"json"}}, payload)
	if w.Code != 400 {
		t.Fatalf("invalid upload: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM canons WHERE slug='festival'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid upload left a collection: %d %v", count, err)
	}
	payload, _ = json.Marshal(snapshot)
	w = creationRequest(t, h, "/canons", url.Values{"kind": {"json"}}, payload)
	if w.Code != 303 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	service := canonadmin.Service{DB: db}
	draft, err := service.Load(context.Background(), "festival")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Revision == 0 || len(draft.Snapshot.Entries) != 2 || draft.Snapshot.Canon.Presentation.Position != 4 {
		t.Fatalf("uploaded draft: %+v", draft)
	}
	var live, edition, static bool
	if err := db.QueryRow(`SELECT is_live,current_edition_id IS NOT NULL,cfg.refresh_policy='static' FROM canons c JOIN canon_crawler_configs cfg ON cfg.canon_id=c.id WHERE slug='festival'`).Scan(&live, &edition, &static); err != nil {
		t.Fatal(err)
	}
	if live || edition || !static {
		t.Fatal("file upload published or misconfigured collection")
	}
	w = creationRequest(t, h, "/canons", url.Values{"kind": {"json"}}, payload)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "existiert bereits") {
		t.Fatalf("duplicate: %d %s", w.Code, w.Body.String())
	}
	var actualRevision int64
	if err := db.QueryRow(`SELECT revision FROM canon_drafts WHERE canon_id=(SELECT id FROM canons WHERE slug='festival')`).Scan(&actualRevision); err != nil || actualRevision != draft.Revision {
		t.Fatal("duplicate upload changed existing draft")
	}
	w = creationRequest(t, h, "/canons/festival/publish", url.Values{"revision": {strconv.FormatInt(draft.Revision, 10)}}, nil)
	if w.Code != 303 {
		t.Fatalf("publish: %d %s", w.Code, w.Body.String())
	}
	var position int
	if err := db.QueryRow(`SELECT display_position,is_live FROM canons WHERE slug='festival'`).Scan(&position, &live); err != nil || position != 4 || live {
		t.Fatalf("publication presentation/visibility: %d %v %v", position, live, err)
	}
}

func TestModuleCreationRefreshRetainsOverridesAndRecordsFailures(t *testing.T) {
	db := testdb.Open(t)
	snapshot := creationSnapshot("source-festival")
	path := filepath.Join(t.TempDir(), "source.json")
	payload, _ := json.Marshal(snapshot)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	module := sources.Module{Definition: sources.Definition{ID: "festival-module", Name: "Festival", Parser: "snapshot", File: path}}
	h := New(db, "admin", "test-password", slog.New(slog.NewTextHandler(io.Discard, nil)), module)
	w := creationRequest(t, h, "/canons", url.Values{"kind": {"module"}, "module": {"festival-module"}, "slug": {"my-festival"}, "name": {"Mein Festival"}}, nil)
	if w.Code != 303 {
		t.Fatalf("module creation: %d %s", w.Code, w.Body.String())
	}
	service := canonadmin.Service{DB: db}
	draft, err := service.Load(context.Background(), "my-festival")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Snapshot.Canon.Name != "Mein Festival" || len(draft.Snapshot.Entries) != 2 {
		t.Fatal("module identity override failed")
	}
	// An open draft must not be overwritten, including before its first publication.
	w = creationRequest(t, h, "/canons/my-festival/crawl", url.Values{"revision": {strconv.FormatInt(draft.Revision, 10)}, "base": {"0"}}, nil)
	if w.Code != 400 {
		t.Fatalf("open draft accepted: %d", w.Code)
	}
	if _, err := service.Publish(context.Background(), "my-festival", draft.Revision); err != nil {
		t.Fatal(err)
	}
	draft, err = service.Load(context.Background(), "my-festival")
	if err != nil {
		t.Fatal(err)
	}
	draft.Snapshot.Entries = append(draft.Snapshot.Entries, crawling.Entry{IMDbID: "tt0468569", Position: 3, SourceTitle: "Manual inclusion"})
	draft.Reason = "Keep this extra film"
	if err := service.Save(context.Background(), "my-festival", draft); err != nil {
		t.Fatal(err)
	}
	draft, err = service.Load(context.Background(), "my-festival")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), "my-festival", draft.Revision); err != nil {
		t.Fatal(err)
	}
	draft, err = service.Load(context.Background(), "my-festival")
	if err != nil {
		t.Fatal(err)
	}
	base := draft.BaseEdition
	w = creationRequest(t, h, "/canons/my-festival/crawl", url.Values{"revision": {"0"}, "base": {strconv.FormatInt(base, 10)}}, nil)
	if w.Code != 303 {
		t.Fatalf("refresh: %d %s", w.Code, w.Body.String())
	}
	draft, err = service.Load(context.Background(), "my-festival")
	if err != nil {
		t.Fatal(err)
	}
	if draft.BaseEdition != base || len(draft.Snapshot.Entries) != 3 || draft.Snapshot.Canon.Name != "Mein Festival" {
		t.Fatalf("refresh lost override/presentation: %+v", draft)
	}
	var actualBase int64
	if err := db.QueryRow(`SELECT current_edition_id FROM canons WHERE slug='my-festival'`).Scan(&actualBase); err != nil || actualBase != base {
		t.Fatal("refresh replaced edition")
	}
	if _, err := service.Publish(context.Background(), "my-festival", draft.Revision); err != nil {
		t.Fatal(err)
	}
	draft, err = service.Load(context.Background(), "my-festival")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"broken":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	w = creationRequest(t, h, "/canons/my-festival/crawl", url.Values{"revision": {"0"}, "base": {strconv.FormatInt(draft.BaseEdition, 10)}}, nil)
	if w.Code != 400 {
		t.Fatalf("broken source accepted: %d", w.Code)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM canon_crawler_runs WHERE canon_id=(SELECT id FROM canons WHERE slug='my-festival') ORDER BY id DESC LIMIT 1`).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("run status: %s %v", status, err)
	}
	if err := db.QueryRow(`SELECT current_edition_id FROM canons WHERE slug='my-festival'`).Scan(&actualBase); err != nil || actualBase != draft.BaseEdition {
		t.Fatal("failure changed edition")
	}
}

func TestCreationTemplateIsValidAndProtected(t *testing.T) {
	h := New(nil, "admin", "test-password", slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := httptest.NewRequest("GET", "/canons/template", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("template accessible without authentication")
	}
	r.SetBasicAuth("admin", "test-password")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("template: %d", w.Code)
	}
	if _, err := crawling.ParseSnapshot(w.Body); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/canons", "/canons/example/crawl"} {
		r := httptest.NewRequest("POST", path, strings.NewReader("kind=json"))
		r.SetBasicAuth("admin", "test-password")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("CSRF protection: %d", w.Code)
		}
	}
}
