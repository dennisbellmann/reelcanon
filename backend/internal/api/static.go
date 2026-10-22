package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// WithFrontend serves the production SPA from the API origin, including deep links.
// Admin handlers are never registered on this listener.
func WithFrontend(api http.Handler, directory string) http.Handler {
	files := http.FileServer(http.Dir(directory))
	basePath := strings.TrimSpace(os.Getenv("APP_BASE_PATH"))
	if basePath != "" && basePath != "/" {
		basePath = "/" + strings.Trim(basePath, "/")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && basePath != "" && basePath != "/" {
			http.Redirect(w, r, basePath+"/", http.StatusPermanentRedirect)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", 405)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			files.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/favicon.svg" || r.URL.Path == "/robots.txt" {
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/artwork/") {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			files.ServeHTTP(w, r)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/canons" && r.URL.Path != "/backup" && r.URL.Path != "/account" && !strings.HasPrefix(r.URL.Path, "/canons/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, filepath.Join(directory, "index.html"))
	})
}

func ValidateFrontend(directory string) error {
	_, err := os.Stat(filepath.Join(directory, "index.html"))
	return err
}
