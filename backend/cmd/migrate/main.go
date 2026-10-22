package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/config"
	"github.com/reelcanon/reelcanon/backend/internal/database"
)

func main() {
	directory := flag.String("dir", "../database/migrations", "directory containing ordered SQL migration files")
	flag.Parse()
	if err := config.LoadDotEnv(); err != nil {
		log.Printf("could not load .env: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.ApplyMigrations(ctx, db, *directory); err != nil {
		log.Fatal(err)
	}
	log.Println("database migrations are up to date")
}
