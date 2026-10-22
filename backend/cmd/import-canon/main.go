package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/catalogupdate"
	"github.com/reelcanon/reelcanon/backend/internal/config"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/database"
)

func main() {
	filePath := flag.String("file", "", "path to a normalized canon-snapshot/v1 JSON file")
	flag.Parse()
	if *filePath == "" {
		log.Fatal("pass -file <snapshot.json>")
	}
	if err := config.LoadDotEnv(); err != nil {
		log.Printf("could not load .env: %v", err)
	}
	file, err := os.Open(*filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	snapshot, err := crawling.DecodeSnapshot(file)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	result, err := catalogupdate.ImportSnapshot(ctx, db, snapshot)
	if err != nil {
		log.Fatal(err)
	}
	if result.NeedsReview {
		fmt.Printf("Source snapshot retained; manual overrides require review in the admin. Published edition unchanged (%d source films).\n", result.FilmCount)
	} else if result.EditionCreated {
		fmt.Printf("Published %s (%d films).\n", snapshot.Edition, result.FilmCount)
	} else {
		fmt.Printf("No membership change for %s (%d films); no edition created.\n", snapshot.Canon.Slug, result.FilmCount)
	}
}
