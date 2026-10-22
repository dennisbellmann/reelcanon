package imdbtop250

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

// Parser handles the normalized source interchange snapshot. A future IMDb page
// adapter can translate fetched bytes into this format without coupling parsing
// to persistence or TMDb enrichment.
type Parser struct{}

func (Parser) Parse(input io.Reader) (crawling.Snapshot, error) {
	body, err := io.ReadAll(io.LimitReader(input, 2*1024*1024+1))
	if err != nil {
		return crawling.Snapshot{}, err
	}
	if len(body) > 2*1024*1024 {
		return crawling.Snapshot{}, fmt.Errorf("source exceeds 2 MB")
	}
	var header struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return crawling.Snapshot{}, err
	}
	if header.Format == "" {
		return parseArchive(body)
	}
	snapshot, err := crawling.ParseSnapshot(bytes.NewReader(body))
	if err != nil {
		return crawling.Snapshot{}, err
	}
	if snapshot.Canon.Slug != "imdb-top-250" {
		return crawling.Snapshot{}, fmt.Errorf("expected imdb-top-250 snapshot, got %q", snapshot.Canon.Slug)
	}
	return snapshot, nil
}

// parseArchive supports the saved IMDb archive interchange used for bootstrap.
// It never guesses identities by title and never fetches the source.
func parseArchive(body []byte) (crawling.Snapshot, error) {
	var archive struct {
		Updated string `json:"updated_at"`
		Data    []struct {
			Rank   int      `json:"rank"`
			Title  string   `json:"title"`
			URL    string   `json:"url"`
			Rating *float64 `json:"rating"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &archive); err != nil {
		return crawling.Snapshot{}, err
	}
	retrieved, err := time.Parse("January 2, 2006 03:04 PM MST", archive.Updated)
	if err != nil {
		return crawling.Snapshot{}, fmt.Errorf("archive source date invalid: %w", err)
	}
	snapshot := crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: "imdb-top-250", Name: "IMDb Top 250", Description: "Die 250 Filme der IMDb-Topliste.", SourceURL: "https://github.com/sharmadhiraj/free-json-datasets/blob/master/docs/sports-entertainment/imdb_top_movies.json"}, Edition: "archive-" + retrieved.Format("2006-01-02"), RetrievedAt: retrieved, Entries: make([]crawling.Entry, 0, len(archive.Data))}
	identity := regexp.MustCompile(`^/title/(tt[0-9]{7,10})/?$`)
	for _, movie := range archive.Data {
		parsed, err := url.Parse(movie.URL)
		if err != nil || parsed.Host != "www.imdb.com" || parsed.Scheme != "https" {
			return crawling.Snapshot{}, fmt.Errorf("archive movie URL invalid for %s", movie.Title)
		}
		match := identity.FindStringSubmatch(parsed.Path)
		if len(match) != 2 {
			return crawling.Snapshot{}, fmt.Errorf("archive IMDb identity missing for %s", movie.Title)
		}
		snapshot.Entries = append(snapshot.Entries, crawling.Entry{IMDbID: match[1], Position: movie.Rank, SourceTitle: movie.Title, IMDbRating: movie.Rating})
	}
	if err := crawling.ValidateSnapshot(snapshot); err != nil {
		return crawling.Snapshot{}, err
	}
	return snapshot, nil
}
