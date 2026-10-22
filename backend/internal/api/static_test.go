package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionDeepLinksAndAPIBoundary(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<html>SPA fixture</html>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "artwork"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "artwork", "cover.png"), []byte("cover fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	handler := WithFrontend(api, directory)
	for _, path := range []string{"/canons/fixture-films", "/canons/fixture-films/movies/tt0111161", "/backup"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "SPA fixture") {
			t.Fatalf("deep link %s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/artwork/cover.png", nil))
	if w.Code != 200 || w.Body.String() != "cover fixture" || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("artwork should be served as an image, not the SPA: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	for _, path := range []string{"/api/v1/admin", "/api/v1/canons/fixture-films/draft", "/assets/missing.js", "/artwork/missing.png"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("unknown endpoint returned SPA: %s %d", path, w.Code)
		}
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/canons/fixture-films/publish", nil))
	if w.Code != 405 {
		t.Fatal("SPA accepted mutation")
	}
}
