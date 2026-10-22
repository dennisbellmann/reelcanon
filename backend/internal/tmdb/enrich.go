package tmdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type EnrichmentResult struct {
	Updated int
	Failed  int
	Errors  []string
}
type target struct {
	IMDbID  string
	Mapping *int64
}

func Enrich(ctx context.Context, db *sql.DB, client *Client, slug string, force bool) (EnrichmentResult, error) {
	if !client.Configured() {
		return EnrichmentResult{}, errors.New("configure TMDB_API_KEY or TMDB_READ_ACCESS_TOKEN on the backend first")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return EnrichmentResult{}, err
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "metadata:"+slug).Scan(&locked); err != nil {
		return EnrichmentResult{}, err
	}
	if !locked {
		return EnrichmentResult{}, errors.New("metadata update is already running")
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "metadata:"+slug)
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT m.imdb_id,m.tmdb_id FROM movies m JOIN canon_movies cm ON cm.movie_id=m.id JOIN canons c ON c.current_edition_id=cm.edition_id WHERE c.slug=$1 AND ($2 OR m.metadata_updated_at IS NULL OR m.metadata_updated_at<now()-interval '30 days' OR m.metadata_error IS NOT NULL)`, slug, force)
	if err != nil {
		return EnrichmentResult{}, err
	}
	targets := []target{}
	for rows.Next() {
		var movie target
		if err := rows.Scan(&movie.IMDbID, &movie.Mapping); err != nil {
			rows.Close()
			return EnrichmentResult{}, err
		}
		targets = append(targets, movie)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return EnrichmentResult{}, err
	}
	result := EnrichmentResult{Errors: []string{}}
	var mu sync.Mutex
	var workers sync.WaitGroup
	jobs := make(chan target)
	for range 4 {
		workers.Go(func() {
			for movie := range jobs {
				details, lookupErr := client.Lookup(ctx, movie.IMDbID, movie.Mapping)
				if lookupErr == nil {
					lookupErr = Store(ctx, db, movie.IMDbID, details)
				}
				mu.Lock()
				if lookupErr != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", movie.IMDbID, lookupErr.Error()))
				} else {
					result.Updated++
				}
				mu.Unlock()
				if lookupErr != nil {
					db.ExecContext(ctx, `UPDATE movies SET metadata_error=$1 WHERE imdb_id=$2`, lookupErr.Error(), movie.IMDbID)
				}
			}
		})
	}
dispatch:
	for _, movie := range targets {
		select {
		case jobs <- movie:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	workers.Wait()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, nil
}

func Store(ctx context.Context, db *sql.DB, imdbID string, movie Details) error {
	return store(ctx, db, imdbID, movie, "")
}

func StoreMapping(ctx context.Context, db *sql.DB, imdbID string, movie Details, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("mapping reason is required")
	}
	return store(ctx, db, imdbID, movie, reason)
}

func store(ctx context.Context, db *sql.DB, imdbID string, movie Details, reason string) error {
	if movie.IMDbID != imdbID {
		return errors.New("metadata identity mismatch")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if reason != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO movie_mapping_changes (movie_id,previous_tmdb_id,next_tmdb_id,reason) SELECT id,tmdb_id,$2,$3 FROM movies WHERE imdb_id=$1`, imdbID, movie.ID, reason); err != nil {
			return err
		}
	}
	var year *int
	if len(movie.ReleaseDate) >= 4 {
		value, err := strconv.Atoi(movie.ReleaseDate[:4])
		if err == nil && value >= 1888 && value <= 2100 {
			year = &value
		}
	}
	var id int64
	var tmdbID any
	if !movie.IsTV {
		tmdbID = movie.ID
	}
	err = tx.QueryRowContext(ctx, `UPDATE movies SET tmdb_id=$1,title=$2,original_title=$3,overview=$4,release_year=COALESCE($5,release_year),runtime_minutes=NULLIF($6,0),poster_path=$7,poster_base_url=$8,metadata_updated_at=now(),metadata_error=NULL,updated_at=now() WHERE imdb_id=$9 RETURNING id`, tmdbID, movie.Title, movie.OriginalTitle, movie.Overview, year, movie.Runtime, movie.PosterPath, movie.ImageBase, imdbID).Scan(&id)
	if err != nil {
		return err
	}
	for _, table := range []string{"movie_directors", "movie_genres", "movie_countries"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE movie_id=$1", id); err != nil {
			return err
		}
	}
	directors := map[string]bool{}
	position := 0
	for _, person := range movie.Credits.Crew {
		if person.Job != "Director" || strings.TrimSpace(person.Name) == "" || directors[person.Name] {
			continue
		}
		directors[person.Name] = true
		position++
		var directorID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO directors (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, person.Name).Scan(&directorID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO movie_directors (movie_id,director_id,position) VALUES ($1,$2,$3)`, id, directorID, position); err != nil {
			return err
		}
	}
	for _, genre := range movie.Genres {
		if strings.TrimSpace(genre.Name) == "" {
			continue
		}
		var genreID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO genres (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, genre.Name).Scan(&genreID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO movie_genres (movie_id,genre_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, genreID); err != nil {
			return err
		}
	}
	for _, country := range movie.Countries {
		if country.Code == "" || country.Name == "" {
			continue
		}
		var countryID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO countries (code,name) VALUES ($1,$2) ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, country.Code, country.Name).Scan(&countryID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO movie_countries (movie_id,country_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, countryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
