package adminui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/tmdb"
)

// resolveSnapshot enriches title-only entries before they enter the canonical
// catalog. Automatic matches must be unique exact title/year matches; an
// explicit TMDb ID is the editor's manual selection and supplies the IMDb ID.
func (h *Handler) resolveSnapshot(ctx context.Context, snapshot *crawling.Snapshot) error {
	if snapshot == nil {
		return errors.New("snapshot is required")
	}
	if err := crawling.ValidateSourceSnapshot(*snapshot); err != nil {
		return err
	}
	needsLookup := false
	for _, entry := range snapshot.Entries {
		if entry.TMDbID > 0 || entry.IMDbID == "" {
			needsLookup = true
			break
		}
	}
	if !needsLookup {
		return crawling.ValidateSnapshot(*snapshot)
	}
	if !h.metadata.Configured() {
		return errors.New("TMDb ist nicht konfiguriert. Hinterlege TMDB_API_KEY oder TMDB_READ_ACCESS_TOKEN im Backend, um Titel aufzulösen.")
	}

	type resolution struct {
		index int
		entry crawling.Entry
		err   error
	}
	indices := make([]int, 0, len(snapshot.Entries))
	for index, entry := range snapshot.Entries {
		if entry.TMDbID > 0 || entry.IMDbID == "" {
			indices = append(indices, index)
		}
	}
	results := make(chan resolution, len(indices))
	jobs := make(chan int)
	var workers sync.WaitGroup
	workerCount := min(4, len(indices))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				entry := snapshot.Entries[index]
				err := h.resolveEntry(ctx, &entry)
				results <- resolution{index: index, entry: entry, err: err}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, index := range indices {
			select {
			case jobs <- index:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	resolutionErrors := make([]error, len(snapshot.Entries))
	for result := range results {
		if result.err != nil {
			resolutionErrors[result.index] = result.err
			continue
		}
		snapshot.Entries[result.index] = result.entry
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var failures []string
	for index, err := range resolutionErrors {
		if err != nil {
			entry := snapshot.Entries[index]
			failures = append(failures, fmt.Sprintf("Rang %d (%q): %v", entry.Position, entry.SourceTitle, err))
		}
	}
	if len(failures) > 0 {
		if len(failures) > 12 {
			failures = append(failures[:12], fmt.Sprintf("… und %d weitere nicht aufgelöste Filme", len(failures)-12))
		}
		return errors.New(strings.Join(failures, "\n"))
	}
	return crawling.ValidateSnapshot(*snapshot)
}

func (h *Handler) resolveEntry(ctx context.Context, entry *crawling.Entry) error {
	if entry.TMDbID == 0 && entry.IMDbID != "" {
		return nil
	}
	if !h.metadata.Configured() {
		return errors.New("TMDb ist nicht konfiguriert. Hinterlege TMDB_API_KEY oder TMDB_READ_ACCESS_TOKEN im Backend, um Titel aufzulösen.")
	}
	var details tmdb.Details
	var err error
	if entry.TMDbID > 0 {
		details, err = h.metadata.LookupByTMDbID(ctx, entry.TMDbID)
	} else {
		details, err = h.metadata.LookupByTitle(ctx, entry.SourceTitle, entry.ReleaseYear)
	}
	if err != nil {
		return err
	}
	if !isIMDbID(details.IMDbID) {
		return fmt.Errorf("TMDb %d für %q enthält keine gültige IMDb-ID", details.ID, entry.SourceTitle)
	}
	entry.IMDbID = details.IMDbID
	entry.TMDbID = details.ID
	if entry.ReleaseYear == nil {
		entry.ReleaseYear = releaseYear(details.ReleaseDate)
	}
	return nil
}

func parseOptionalTMDbID(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid TMDb ID")
	}
	return id, nil
}

func isIMDbID(id string) bool {
	if len(id) < 9 || len(id) > 12 || !strings.HasPrefix(id, "tt") {
		return false
	}
	for _, digit := range id[2:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func releaseYear(date string) *int {
	if len(date) < 4 {
		return nil
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil || year < 1888 || year > 2100 {
		return nil
	}
	return &year
}
