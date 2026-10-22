package crawling

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestSnapshotAndCSVRoundTrip(t *testing.T) {
	snapshot := snapshotWithCount("fixture", 3)
	snapshot.Entries[0].SourceTitle = "A, \"quoted\" title"
	year, rating := 1994, 9.3
	snapshot.Entries[0].ReleaseYear = &year
	snapshot.Entries[0].IMDbRating = &rating
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSnapshot(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if MembershipHash(snapshot.Entries) != MembershipHash(parsed.Entries) {
		t.Fatal("JSON round-trip changed membership")
	}
	var csv bytes.Buffer
	if err := WriteCSV(&csv, snapshot); err != nil {
		t.Fatal(err)
	}
	parsed, err = DecodeCSV(&csv, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Entries[0].SourceTitle != snapshot.Entries[0].SourceTitle || *parsed.Entries[0].IMDbRating != 9.3 {
		t.Fatal("CSV corrupted metadata")
	}
	if _, err := ParseSnapshot(strings.NewReader(string(payload) + " {}")); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, err := ParseSnapshot(strings.NewReader(strings.Replace(string(payload), `"format":`, `"unknown":true,"format":`, 1))); err == nil {
		t.Fatal("unknown field accepted")
	}
}
func TestMembershipIgnoresMetadataAndInputOrdering(t *testing.T) {
	snapshot := snapshotWithCount("fixture", 2)
	hash := MembershipHash(snapshot.Entries)
	snapshot.Entries[0].SourceTitle = "changed"
	snapshot.Entries[0], snapshot.Entries[1] = snapshot.Entries[1], snapshot.Entries[0]
	if MembershipHash(snapshot.Entries) != hash {
		t.Fatal("metadata created a false edition change")
	}
	snapshot.Entries[0].Position, snapshot.Entries[1].Position = snapshot.Entries[1].Position, snapshot.Entries[0].Position
	if MembershipHash(snapshot.Entries) == hash {
		t.Fatal("rank change not detected")
	}
	rating := math.NaN()
	snapshot.Entries[0].IMDbRating = &rating
	if err := ValidateSnapshot(snapshot); err == nil {
		t.Fatal("non-finite rating accepted")
	}
}
