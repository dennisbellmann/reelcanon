package imdbtop250

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSavedFullSnapshotAndIncompleteSource(t *testing.T) {
	file, err := os.Open("../../../../data/imdb-top-250-2025-08-20.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	snapshot, err := (Parser{}).Parse(file)
	if err != nil || len(snapshot.Entries) != 250 || snapshot.Entries[0].IMDbID != "tt0111161" {
		t.Fatalf("saved snapshot: %v", err)
	}
	snapshot.Entries = snapshot.Entries[:249]
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Parser{}).Parse(strings.NewReader(string(body))); err == nil {
		t.Fatal("incomplete snapshot accepted")
	}
	if _, err := (Parser{}).Parse(strings.NewReader(`{"updated_at":"August 20, 2025 09:42 AM UTC","data":[{"rank":1,"title":"A","url":"https://www.imdb.com/title/tt0111161/","rating":9.3}]}`)); err == nil {
		t.Fatal("incomplete archive accepted")
	}
}
