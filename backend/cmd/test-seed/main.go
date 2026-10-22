// test-seed only operates on explicitly named test databases and isolated schemas.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/database"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

func main() {
	schema := flag.String("schema", "", "isolated e2e schema")
	cleanup := flag.Bool("cleanup", false, "remove this test schema")
	flag.Parse()
	connection := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(connection)
	if err != nil || !strings.HasSuffix(strings.Trim(parsed.Path, "/"), "_test") || !regexp.MustCompile(`^e2e_[0-9]+$`).MatchString(*schema) {
		log.Fatal("TEST_DATABASE_URL must name a database ending in _test and schema must match e2e_<digits>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := database.Open(ctx, connection)
	if err != nil {
		log.Fatal(err)
	}
	defer admin.Close()
	if *cleanup {
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+*schema+" CASCADE"); err != nil {
			log.Fatal(err)
		}
		return
	}
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+*schema); err != nil {
		log.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", *schema)
	parsed.RawQuery = query.Encode()
	db, err := database.Open(ctx, parsed.String())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.ApplyMigrations(ctx, db, "../database/migrations"); err != nil {
		log.Fatal(err)
	}
	years := []int{1994, 1972, 2008}
	ids := []string{"tt0111161", "tt0068646", "tt0468569"}
	entries := make([]crawling.Entry, 3)
	for i := range entries {
		rating := 9.0 + float64(i)/10
		entries[i] = crawling.Entry{IMDbID: ids[i], SourceTitle: fmt.Sprintf("Fixture Film %c", 'A'+i), Position: i + 1, ReleaseYear: &years[i], IMDbRating: &rating}
	}
	source := crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: "fixture-films", Name: "Fixture Films", Description: "Isolated end-to-end test collection", SourceURL: "https://example.com/test-films"}, Edition: "fixture-1", RetrievedAt: time.Now().UTC(), Entries: entries}
	if _, err := catalogimport.Import(ctx, db, source); err != nil {
		log.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE canons SET is_live=true WHERE slug='fixture-films'; INSERT INTO genres (name) VALUES ('Drama'); INSERT INTO movie_genres (movie_id,genre_id) SELECT movies.id,genres.id FROM movies CROSS JOIN genres; UPDATE movies SET overview='Fixture synopsis',runtime_minutes=142`); err != nil {
		log.Fatal(err)
	}
}
