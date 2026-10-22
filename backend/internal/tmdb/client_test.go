package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLookupMatchesIMDbAndLoadsDetailsWithConfiguredImages(t *testing.T) {
	var configCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Error("key missing")
		}
		switch r.URL.Path {
		case "/find/tt0111161":
			if r.URL.Query().Get("external_source") != "imdb_id" {
				t.Error("wrong identity source")
			}
			fmt.Fprint(w, `{"movie_results":[{"id":278}]}`)
		case "/movie/278":
			if r.URL.Query().Get("language") != "de-DE" || r.URL.Query().Get("append_to_response") != "credits" {
				t.Error("missing metadata options")
			}
			fmt.Fprint(w, `{"id":278,"imdb_id":"tt0111161","title":"Die Verurteilten","runtime":142,"poster_path":"/poster.jpg","credits":{"crew":[{"job":"Director","name":"Frank Darabont"}]}}`)
		case "/configuration":
			configCalls.Add(1)
			fmt.Fprint(w, `{"images":{"secure_base_url":"https://image.tmdb.org/t/p/","poster_sizes":["w500"]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := New("test-key", "")
	client.BaseURL = server.URL
	for range 2 {
		movie, err := client.Lookup(context.Background(), "tt0111161", nil)
		if err != nil || movie.ID != 278 || movie.Title != "Die Verurteilten" || movie.ImageBase != "https://image.tmdb.org/t/p/" {
			t.Fatalf("lookup: %+v %v", movie, err)
		}
	}
	if configCalls.Load() != 1 {
		t.Fatal("image configuration was not cached")
	}
}

func TestLookupRejectsIdentityMismatchAndAmbiguousFind(t *testing.T) {
	for _, body := range []string{`{"movie_results":[]}`, `{"movie_results":[{"id":278},{"id":279}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		client := New("secret-key", "")
		client.BaseURL = server.URL
		if _, err := client.Lookup(context.Background(), "tt0111161", nil); err == nil {
			t.Fatal("unresolved match accepted")
		}
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":278,"imdb_id":"tt0068646","title":"Wrong movie"}`)
	}))
	defer server.Close()
	client := New("secret-key", "")
	client.BaseURL = server.URL
	id := int64(278)
	if _, err := client.Lookup(context.Background(), "tt0111161", &id); err == nil {
		t.Fatal("wrong IMDb mapping accepted")
	}
}

func TestStatusRetriesCancellationAndSecretRedaction(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"movie_results":[]}`)
	}))
	defer server.Close()
	client := New("secret-key", "")
	client.BaseURL = server.URL
	_, err := client.Lookup(context.Background(), "tt0111161", nil)
	if calls.Load() != 3 || err == nil {
		t.Fatalf("retry count: %d %v", calls.Load(), err)
	}
	calls.Store(0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = client.Lookup(ctx, "tt0111161", nil)
	if err == nil || strings.Contains(err.Error(), "secret-key") || calls.Load() > 1 {
		t.Fatalf("cancellation or redaction failed: %v", err)
	}
	client.BaseURL = "http://127.0.0.1:1"
	_, err = client.Lookup(ctx, "tt0111161", nil)
	if err == nil || strings.Contains(err.Error(), "secret-key") {
		t.Fatal("secret leaked in error")
	}
}
