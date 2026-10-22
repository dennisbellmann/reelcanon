package adminui

import (
	"context"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAdminRequiresAuthAndRejectsCSRFBeforeDatabaseAccess(t *testing.T) {
	h := New(nil, "admin", "test-password", slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, path := range []string{"/canons", "/canons/imdb-top-250", "/canons/imdb-top-250/export"} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s without auth: %d", path, w.Code)
		}
	}
	for _, action := range []string{"draft", "presentation", "entry", "publish", "restore", "config", "metadata", "mapping", "import", "visibility", "override"} {
		r := httptest.NewRequest("POST", "/canons/imdb-top-250/"+action, strings.NewReader("_csrf=&live=true"))
		r.SetBasicAuth("admin", "test-password")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(&http.Cookie{Name: "reelcanon_admin_csrf", Value: ""})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s accepted empty CSRF: %d", action, w.Code)
		}
	}
	token := strings.Repeat("a", 64)
	r := httptest.NewRequest("POST", "/canons/imdb-top-250/publish", strings.NewReader("_csrf="+token))
	r.SetBasicAuth("admin", "test-password")
	r.AddCookie(&http.Cookie{Name: "reelcanon_admin_csrf", Value: token})
	r.Header.Set("Origin", "https://untrusted.example")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin form accepted")
	}
}

func TestAdminRendersEditorCreatesDraftAndKeepsIncompleteTop250Hidden(t *testing.T) {
	db := testdb.Open(t)
	h := New(db, "admin", "test-password", slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := httptest.NewRequest("GET", "/canons/imdb-top-250", nil)
	r.SetBasicAuth("admin", "test-password")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Noch nicht veröffentlichbar") || !strings.Contains(w.Body.String(), "Film hinzufügen") {
		t.Fatalf("editor render: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	values := url.Values{"_csrf": {cookie.Value}, "base": {"0"}, "revision": {"0"}, "imdbId": {"tt0111161"}, "title": {"Test A"}, "position": {"1"}, "year": {"1994"}, "reason": {"Test draft"}}
	r = httptest.NewRequest("POST", "/canons/imdb-top-250/entry", strings.NewReader(values.Encode()))
	r.SetBasicAuth("admin", "test-password")
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatalf("save entry: %d %s", w.Code, w.Body.String())
	}
	var revision int64
	if err := db.QueryRowContext(context.Background(), `SELECT revision FROM canon_drafts`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest("POST", "/canons/imdb-top-250/visibility", strings.NewReader(url.Values{"_csrf": {cookie.Value}, "live": {"true"}}.Encode()))
	r.SetBasicAuth("admin", "test-password")
	r.AddCookie(cookie)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("incomplete Top250 enabled")
	}
	r = httptest.NewRequest("GET", "/canons", nil)
	r.SetBasicAuth("admin", "test-password")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `href="/canons/imdb-top-250"`) {
		t.Fatalf("overview: %d %s", w.Code, w.Body.String())
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("CSRF rotated and broke another open tab")
	}
}
