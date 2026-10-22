package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("catalog item not found")

type Canon struct {
	Slug                string   `json:"slug"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Source              string   `json:"source"`
	FilmCount           int      `json:"filmCount"`
	CurrentEdition      *string  `json:"currentEdition"`
	IMDbIDs             []string `json:"imdbIds,omitempty"`
	DisplayPosition     int      `json:"displayPosition"`
	PosterURL           string   `json:"posterUrl"`
	PosterTitleEmbedded bool     `json:"posterTitleEmbedded"`
}

type Movie struct {
	IMDbID         string   `json:"imdbId"`
	TMDbID         *int64   `json:"tmdbId"`
	Title          string   `json:"title"`
	OriginalTitle  *string  `json:"originalTitle"`
	ReleaseYear    *int     `json:"releaseYear"`
	RuntimeMinutes *int     `json:"runtimeMinutes"`
	Overview       string   `json:"overview"`
	PosterURL      *string  `json:"posterUrl"`
	IMDbRating     *float64 `json:"imdbRating"`
	Directors      []string `json:"directors"`
	Genres         []string `json:"genres"`
	Countries      []string `json:"countries"`
	Position       int      `json:"position"`
}

type CanonCatalog struct {
	Canon  Canon   `json:"canon"`
	Movies []Movie `json:"movies"`
}

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) MovieCanons(ctx context.Context, imdbID string) ([]Canon, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.slug,COALESCE(c.display_name,c.name),COALESCE(c.display_description,c.description),c.source_url,COUNT(cm.movie_id)::int,e.label,c.display_position,c.poster_url,c.poster_title_embedded
 FROM canons c JOIN canon_editions e ON e.id=c.current_edition_id JOIN canon_movies cm ON cm.edition_id=e.id
 WHERE c.is_live AND EXISTS (SELECT 1 FROM canon_movies membership JOIN movies m ON m.id=membership.movie_id WHERE membership.edition_id=e.id AND m.imdb_id=$1)
 GROUP BY c.id,e.id ORDER BY c.display_position, COALESCE(c.display_name,c.name), c.slug`, imdbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Canon, 0)
	for rows.Next() {
		var canon Canon
		if err := rows.Scan(&canon.Slug, &canon.Name, &canon.Description, &canon.Source, &canon.FilmCount, &canon.CurrentEdition, &canon.DisplayPosition, &canon.PosterURL, &canon.PosterTitleEmbedded); err != nil {
			return nil, err
		}
		result = append(result, canon)
	}
	return result, rows.Err()
}

func (r *Repository) ListCanons(ctx context.Context) ([]Canon, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.slug, COALESCE(c.display_name,c.name), COALESCE(c.display_description,c.description), c.source_url,
		COUNT(cm.movie_id)::int, e.label, c.display_position, c.poster_url, c.poster_title_embedded,
		COALESCE(json_agg(m.imdb_id ORDER BY cm.position) FILTER (WHERE m.imdb_id IS NOT NULL), '[]'::json)
		FROM canons c
		LEFT JOIN canon_editions e ON e.id = c.current_edition_id
		LEFT JOIN canon_movies cm ON cm.edition_id = e.id
		LEFT JOIN movies m ON m.id = cm.movie_id
		WHERE c.is_live = true AND c.current_edition_id IS NOT NULL
		GROUP BY c.id, e.id ORDER BY c.display_position, COALESCE(c.display_name,c.name), c.slug`)
	if err != nil {
		return nil, fmt.Errorf("list canons: %w", err)
	}
	defer rows.Close()
	canons := make([]Canon, 0)
	for rows.Next() {
		var canon Canon
		var membershipJSON []byte
		if err := rows.Scan(&canon.Slug, &canon.Name, &canon.Description, &canon.Source, &canon.FilmCount, &canon.CurrentEdition, &canon.DisplayPosition, &canon.PosterURL, &canon.PosterTitleEmbedded, &membershipJSON); err != nil {
			return nil, fmt.Errorf("read canon: %w", err)
		}
		if err := json.Unmarshal(membershipJSON, &canon.IMDbIDs); err != nil {
			return nil, fmt.Errorf("decode canon membership: %w", err)
		}
		canons = append(canons, canon)
	}
	return canons, rows.Err()
}

func (r *Repository) GetCanon(ctx context.Context, slug string) (CanonCatalog, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return CanonCatalog{}, err
	}
	defer tx.Rollback()
	var result CanonCatalog
	err = tx.QueryRowContext(ctx, `SELECT c.slug, COALESCE(c.display_name,c.name), COALESCE(c.display_description,c.description), c.source_url,
		COUNT(cm.movie_id)::int, e.label, c.display_position, c.poster_url, c.poster_title_embedded
		FROM canons c
		LEFT JOIN canon_editions e ON e.id = c.current_edition_id
		LEFT JOIN canon_movies cm ON cm.edition_id = e.id
		WHERE c.slug = $1 AND c.is_live = true AND c.current_edition_id IS NOT NULL
		GROUP BY c.id, e.id`, slug).Scan(&result.Canon.Slug, &result.Canon.Name, &result.Canon.Description, &result.Canon.Source, &result.Canon.FilmCount, &result.Canon.CurrentEdition, &result.Canon.DisplayPosition, &result.Canon.PosterURL, &result.Canon.PosterTitleEmbedded)
	if errors.Is(err, sql.ErrNoRows) {
		return CanonCatalog{}, ErrNotFound
	}
	if err != nil {
		return CanonCatalog{}, fmt.Errorf("read canon: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `SELECT m.imdb_id, m.tmdb_id, m.title, m.original_title,
		m.release_year, m.runtime_minutes, m.overview, m.poster_path, m.poster_base_url, m.imdb_rating, cm.position,
		COALESCE((SELECT json_agg(d.name ORDER BY md.position) FROM movie_directors md JOIN directors d ON d.id = md.director_id WHERE md.movie_id = m.id), '[]'::json),
		COALESCE((SELECT json_agg(g.name ORDER BY g.name) FROM movie_genres mg JOIN genres g ON g.id = mg.genre_id WHERE mg.movie_id = m.id), '[]'::json),
		COALESCE((SELECT json_agg(country.name ORDER BY country.name) FROM movie_countries mc JOIN countries country ON country.id = mc.country_id WHERE mc.movie_id = m.id), '[]'::json)
		FROM canon_movies cm JOIN movies m ON m.id = cm.movie_id
		WHERE cm.edition_id = (SELECT current_edition_id FROM canons WHERE slug = $1)
		ORDER BY cm.position`, slug)
	if err != nil {
		return CanonCatalog{}, fmt.Errorf("list canon movies: %w", err)
	}
	defer rows.Close()
	result.Movies = make([]Movie, 0, result.Canon.FilmCount)
	for rows.Next() {
		var movie Movie
		var posterPath sql.NullString
		var posterBase string
		var directorsJSON, genresJSON, countriesJSON []byte
		if err := rows.Scan(&movie.IMDbID, &movie.TMDbID, &movie.Title, &movie.OriginalTitle, &movie.ReleaseYear, &movie.RuntimeMinutes, &movie.Overview, &posterPath, &posterBase, &movie.IMDbRating, &movie.Position, &directorsJSON, &genresJSON, &countriesJSON); err != nil {
			return CanonCatalog{}, fmt.Errorf("read canon movie: %w", err)
		}
		if posterPath.Valid {
			posterURL := strings.TrimRight(posterBase, "/") + "/w500" + posterPath.String
			movie.PosterURL = &posterURL
		}
		if err := json.Unmarshal(directorsJSON, &movie.Directors); err != nil {
			return CanonCatalog{}, fmt.Errorf("decode directors: %w", err)
		}
		if err := json.Unmarshal(genresJSON, &movie.Genres); err != nil {
			return CanonCatalog{}, fmt.Errorf("decode genres: %w", err)
		}
		if err := json.Unmarshal(countriesJSON, &movie.Countries); err != nil {
			return CanonCatalog{}, fmt.Errorf("decode countries: %w", err)
		}
		result.Movies = append(result.Movies, movie)
	}
	if err := rows.Err(); err != nil {
		return CanonCatalog{}, fmt.Errorf("read canon movies: %w", err)
	}
	if err := rows.Close(); err != nil {
		return CanonCatalog{}, err
	}
	if err := tx.Commit(); err != nil {
		return CanonCatalog{}, err
	}
	return result, nil
}
