package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"github.com/reelcanon/reelcanon/backend/internal/crawling/imdbtop250"
	"log"
	"os"
)

func main() {
	file := flag.String("file", "", "saved IMDb normalized JSON or archive source file")
	output := flag.String("out", "", "normalized snapshot output path")
	flag.Parse()
	if *file == "" || *output == "" {
		log.Fatal("-file and -out are required")
	}
	body, err := os.ReadFile(*file)
	if err != nil {
		log.Fatal(err)
	}
	snapshot, err := (imdbtop250.Parser{}).Parse(bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	hash := sha256.Sum256(body)
	snapshot.SourceContentHash = hex.EncodeToString(hash[:])
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, append(payload, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("Validated %d films; saved %s", len(snapshot.Entries), *output)
}
