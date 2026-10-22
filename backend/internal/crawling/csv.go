package crawling

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSV carries entries; Canon identity and provenance are provided by the editor.
func DecodeCSV(input io.Reader, snapshot Snapshot) (Snapshot, error) {
	reader := csv.NewReader(input)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return Snapshot{}, err
	}
	columns := map[string]int{}
	for i, key := range header {
		key = strings.TrimSpace(key)
		switch key {
		case "position", "imdbId", "tmdbId", "sourceTitle", "releaseYear", "imdbRating":
		default:
			return Snapshot{}, fmt.Errorf("CSV has unsupported column %q", key)
		}
		if _, exists := columns[key]; exists {
			return Snapshot{}, fmt.Errorf("CSV column %q appears more than once", key)
		}
		columns[key] = i
	}
	if _, ok := columns["position"]; !ok {
		return Snapshot{}, errors.New("CSV must include position and sourceTitle columns")
	}
	if _, ok := columns["sourceTitle"]; !ok {
		return Snapshot{}, errors.New("CSV must include position and sourceTitle columns")
	}
	snapshot.Entries = []Entry{}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Snapshot{}, fmt.Errorf("CSV line %d: %w", line, err)
		}
		if len(row) != len(header) {
			return Snapshot{}, fmt.Errorf("CSV line %d: expected %d columns, received %d", line, len(header), len(row))
		}
		value := func(name string) string {
			if index, ok := columns[name]; ok {
				return strings.TrimSpace(row[index])
			}
			return ""
		}
		position, err := strconv.Atoi(value("position"))
		if err != nil {
			return Snapshot{}, fmt.Errorf("CSV line %d: invalid position", line)
		}
		entry := Entry{Position: position, IMDbID: value("imdbId"), SourceTitle: value("sourceTitle")}
		if raw := value("tmdbId"); raw != "" {
			entry.TMDbID, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || entry.TMDbID < 1 {
				return Snapshot{}, fmt.Errorf("CSV line %d: invalid TMDb ID", line)
			}
		}
		if raw := value("releaseYear"); raw != "" {
			year, err := strconv.Atoi(raw)
			if err != nil {
				return Snapshot{}, fmt.Errorf("CSV line %d: invalid year", line)
			}
			entry.ReleaseYear = &year
		}
		if raw := value("imdbRating"); raw != "" {
			rating, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return Snapshot{}, fmt.Errorf("CSV line %d: invalid rating", line)
			}
			entry.IMDbRating = &rating
		}
		snapshot.Entries = append(snapshot.Entries, entry)
		if len(snapshot.Entries) > 10000 {
			return Snapshot{}, errors.New("CSV exceeds 10000 entries")
		}
	}
	if err := ValidateSourceSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func WriteCSV(output io.Writer, snapshot Snapshot) error {
	writer := csv.NewWriter(output)
	if err := writer.Write([]string{"position", "imdbId", "tmdbId", "sourceTitle", "releaseYear", "imdbRating"}); err != nil {
		return err
	}
	for _, entry := range snapshot.Entries {
		year, rating, tmdbID := "", "", ""
		if entry.TMDbID > 0 {
			tmdbID = strconv.FormatInt(entry.TMDbID, 10)
		}
		if entry.ReleaseYear != nil {
			year = strconv.Itoa(*entry.ReleaseYear)
		}
		if entry.IMDbRating != nil {
			rating = strconv.FormatFloat(*entry.IMDbRating, 'f', 1, 64)
		}
		if err := writer.Write([]string{strconv.Itoa(entry.Position), entry.IMDbID, tmdbID, entry.SourceTitle, year, rating}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
