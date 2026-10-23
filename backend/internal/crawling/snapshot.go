package crawling

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

const SnapshotFormat = "canon-snapshot/v1"

type Snapshot struct {
	Format            string      `json:"format"`
	Canon             CanonSource `json:"canon"`
	Edition           string      `json:"editionLabel"`
	RetrievedAt       time.Time   `json:"retrievedAt"`
	SourceContentHash string      `json:"sourceContentHash,omitempty"`
	Entries           []Entry     `json:"entries"`
}

type CanonSource struct {
	Slug         string             `json:"slug"`
	Name         string             `json:"name"`
	Description  string             `json:"description,omitempty"`
	SourceURL    string             `json:"sourceUrl"`
	Presentation *CanonPresentation `json:"presentation,omitempty"`
}

// CanonPresentation is editorial configuration, published only through the cockpit.
type CanonPresentation struct {
	Position      int    `json:"position"`
	PosterURL     string `json:"posterUrl"`
	TitleEmbedded bool   `json:"titleEmbedded"`
}

func ValidatePresentation(p *CanonPresentation) error {
	if p == nil {
		return nil
	}
	if p.Position < 0 {
		return errors.New("Sammlungsposition muss mindestens 0 sein")
	}
	parsed, err := url.Parse(p.PosterURL)
	if err != nil || strings.ContainsAny(p.PosterURL, "\\\r\n") {
		return errors.New("Ungültige Poster-URL")
	}
	if strings.HasPrefix(p.PosterURL, "/artwork/") && parsed.Host == "" && !strings.Contains(parsed.Path, "..") {
		return nil
	}
	if parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil {
		return nil
	}
	return errors.New("Poster muss einen /artwork/-Pfad oder eine absolute HTTPS-URL verwenden")
}

type Entry struct {
	IMDbID      string   `json:"imdbId"`
	TMDbID      int64    `json:"tmdbId,omitempty"`
	Position    int      `json:"position"`
	SourceTitle string   `json:"sourceTitle"`
	ReleaseYear *int     `json:"releaseYear,omitempty"`
	IMDbRating  *float64 `json:"imdbRating,omitempty"`
}

func ParseSnapshot(input io.Reader) (Snapshot, error) {
	snapshot, err := DecodeSnapshot(input)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ValidateSourceSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// DecodeSnapshot accepts incomplete drafts, but rejects malformed JSON and unknown fields.
func DecodeSnapshot(input io.Reader) (Snapshot, error) {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("snapshot must contain one JSON value")
	}
	return snapshot, nil
}

func ValidateSnapshot(snapshot Snapshot) error {
	if err := ValidateSourceSnapshot(snapshot); err != nil {
		return err
	}
	for _, entry := range snapshot.Entries {
		if !imdbPattern.MatchString(entry.IMDbID) {
			return fmt.Errorf("IMDb-ID fehlt oder ist ungültig für %q", entry.SourceTitle)
		}
	}
	return nil
}

// ValidateSourceSnapshot accepts title-only source entries before the Cockpit
// resolves their canonical IMDb and TMDb identities. Published snapshots still
// pass through ValidateSnapshot and therefore always have IMDb IDs.
func ValidateSourceSnapshot(snapshot Snapshot) error {
	return validateSourceSnapshot(snapshot, true)
}

// ValidateDraftSnapshot validates an editable source snapshot without requiring
// collection canons to have their full publication size yet.
func ValidateDraftSnapshot(snapshot Snapshot) error {
	return validateSourceSnapshot(snapshot, false)
}

func validateSourceSnapshot(snapshot Snapshot, requireFullCollection bool) error {
	if snapshot.Format != SnapshotFormat {
		return fmt.Errorf("format must be %q", SnapshotFormat)
	}
	if !slugPattern.MatchString(snapshot.Canon.Slug) {
		return errors.New("canon.slug must be lowercase words separated by hyphens")
	}
	if strings.TrimSpace(snapshot.Canon.Name) == "" {
		return errors.New("canon.name is required")
	}
	if err := ValidatePresentation(snapshot.Canon.Presentation); err != nil {
		return err
	}
	parsedURL, err := url.ParseRequestURI(snapshot.Canon.SourceURL)
	if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") || parsedURL.Host == "" {
		return errors.New("canon.sourceUrl must be an absolute HTTP(S) URL")
	}
	if strings.TrimSpace(snapshot.Edition) == "" {
		return errors.New("editionLabel is required")
	}
	if snapshot.RetrievedAt.IsZero() {
		return errors.New("retrievedAt is required")
	}
	if snapshot.SourceContentHash != "" && !contentHashPattern.MatchString(snapshot.SourceContentHash) {
		return errors.New("sourceContentHash must be a lowercase SHA-256 hex digest")
	}
	if len(snapshot.Entries) == 0 {
		return errors.New("entries must not be empty")
	}
	seenIDs := make(map[string]struct{}, len(snapshot.Entries))
	seenTMDbIDs := make(map[int64]struct{}, len(snapshot.Entries))
	seenPositions := make(map[int]struct{}, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if entry.IMDbID != "" && !imdbPattern.MatchString(entry.IMDbID) {
			return fmt.Errorf("invalid IMDb ID %q", entry.IMDbID)
		}
		if entry.IMDbID != "" {
			if _, exists := seenIDs[entry.IMDbID]; exists {
				return fmt.Errorf("duplicate IMDb ID %s", entry.IMDbID)
			}
			seenIDs[entry.IMDbID] = struct{}{}
		}
		if entry.TMDbID < 0 {
			return fmt.Errorf("invalid TMDb ID for %q", entry.SourceTitle)
		}
		if entry.TMDbID > 0 {
			if _, exists := seenTMDbIDs[entry.TMDbID]; exists {
				return fmt.Errorf("duplicate TMDb ID %d", entry.TMDbID)
			}
			seenTMDbIDs[entry.TMDbID] = struct{}{}
		}
		if entry.Position < 1 {
			return fmt.Errorf("invalid position for %q: %d", entry.SourceTitle, entry.Position)
		}
		if _, exists := seenPositions[entry.Position]; exists {
			return fmt.Errorf("duplicate position %d", entry.Position)
		}
		seenPositions[entry.Position] = struct{}{}
		if strings.TrimSpace(entry.SourceTitle) == "" {
			return errors.New("sourceTitle is required for every entry")
		}
		if entry.ReleaseYear != nil && (*entry.ReleaseYear < 1888 || *entry.ReleaseYear > 2100) {
			return fmt.Errorf("invalid release year for %q", entry.SourceTitle)
		}
		if entry.IMDbRating != nil && (math.IsNaN(*entry.IMDbRating) || math.IsInf(*entry.IMDbRating, 0) || *entry.IMDbRating < 0 || *entry.IMDbRating > 10) {
			return fmt.Errorf("invalid IMDb rating for %q", entry.SourceTitle)
		}
	}
	expectedCount := 0
	switch snapshot.Canon.Slug {
	case "imdb-top-250":
		expectedCount = 250
	case "1001-movies":
		expectedCount = 1001
	}
	if requireFullCollection && expectedCount > 0 {
		if len(snapshot.Entries) != expectedCount {
			return fmt.Errorf("%s requires exactly %d entries, received %d", snapshot.Canon.Slug, expectedCount, len(snapshot.Entries))
		}
		for position := 1; position <= expectedCount; position++ {
			if _, exists := seenPositions[position]; !exists {
				return fmt.Errorf("%s is missing position %d", snapshot.Canon.Slug, position)
			}
		}
	}
	return nil
}

func MembershipHash(entries []Entry) string {
	ordered := append([]Entry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Position < ordered[j].Position })
	var canonical bytes.Buffer
	for _, entry := range ordered {
		fmt.Fprintf(&canonical, "%d:%s\n", entry.Position, entry.IMDbID)
	}
	digest := sha256.Sum256(canonical.Bytes())
	return hex.EncodeToString(digest[:])
}
