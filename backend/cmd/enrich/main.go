package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/reelcanon/reelcanon/backend/internal/config"
	"github.com/reelcanon/reelcanon/backend/internal/database"
	"github.com/reelcanon/reelcanon/backend/internal/tmdb"
	"log"
	"os"
	"time"
)

func main() {
	slug := flag.String("canon", "imdb-top-250", "Canon slug")
	force := flag.Bool("force", false, "refresh cached metadata")
	flag.Parse()
	if err := config.LoadDotEnv(); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	result, err := tmdb.Enrich(ctx, db, tmdb.New(os.Getenv("TMDB_API_KEY"), os.Getenv("TMDB_READ_ACCESS_TOKEN")), *slug, *force)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Updated: %d; failed: %d\n", result.Updated, result.Failed)
	for _, issue := range result.Errors {
		fmt.Println(issue)
	}
	if result.Failed > 0 {
		os.Exit(1)
	}
}
