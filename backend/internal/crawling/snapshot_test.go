package crawling

import (
	"fmt"
	"testing"
	"time"
)

func snapshotWithCount(slug string, count int) Snapshot {
	entries := make([]Entry, count)
	for index := range count {
		entries[index] = Entry{
			IMDbID:      fmt.Sprintf("tt%07d", index+1),
			Position:    index + 1,
			SourceTitle: fmt.Sprintf("Film %d", index+1),
		}
	}
	return Snapshot{
		Format:      SnapshotFormat,
		Canon:       CanonSource{Slug: slug, Name: slug, SourceURL: "https://example.com/canon"},
		Edition:     "test-edition",
		RetrievedAt: time.Date(2026, time.October, 5, 0, 0, 0, 0, time.UTC),
		Entries:     entries,
	}
}

func TestValidateSnapshotRequiresCompleteKnownCanons(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		slug  string
		count int
	}{
		{slug: "imdb-top-250", count: 250},
		{slug: "1001-movies", count: 1001},
	} {
		t.Run(testCase.slug, func(t *testing.T) {
			snapshot := snapshotWithCount(testCase.slug, testCase.count)
			if err := ValidateSnapshot(snapshot); err != nil {
				t.Fatalf("valid complete snapshot rejected: %v", err)
			}
			snapshot.Entries = snapshot.Entries[:len(snapshot.Entries)-1]
			if err := ValidateSnapshot(snapshot); err == nil {
				t.Fatal("incomplete snapshot was accepted")
			}
		})
	}
}

func TestValidateSnapshotAllowsGenericCanon(t *testing.T) {
	t.Parallel()
	snapshot := snapshotWithCount("curated-sci-fi", 3)
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("generic snapshot rejected: %v", err)
	}
}

func TestValidateSnapshotRejectsDuplicateIdentityOrPosition(t *testing.T) {
	t.Parallel()
	snapshot := snapshotWithCount("curated-sci-fi", 2)
	snapshot.Entries[1].IMDbID = snapshot.Entries[0].IMDbID
	if err := ValidateSnapshot(snapshot); err == nil {
		t.Fatal("duplicate IMDb ID was accepted")
	}
	snapshot = snapshotWithCount("curated-sci-fi", 2)
	snapshot.Entries[1].Position = snapshot.Entries[0].Position
	if err := ValidateSnapshot(snapshot); err == nil {
		t.Fatal("duplicate position was accepted")
	}
}
func TestPresentationRejectsUnsafeImages(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "//other.example/poster.png", "/artwork/../secret", "http://example.com/poster.png", "https://user:pass@example.com/poster.png", "/artwork/%2e%2e/secret"} {
		if err := ValidatePresentation(&CanonPresentation{PosterURL: value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{"/artwork/collection-03-filmwelten.png", "https://example.com/poster.png"} {
		if err := ValidatePresentation(&CanonPresentation{PosterURL: value}); err != nil {
			t.Errorf("rejected %q: %v", value, err)
		}
	}
	if err := ValidatePresentation(&CanonPresentation{Position: -1, PosterURL: "https://example.com/poster.png"}); err == nil {
		t.Fatal("accepted negative position")
	}
}
