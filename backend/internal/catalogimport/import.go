package catalogimport

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

type Result struct {
	EditionCreated bool
	FilmCount      int
	NeedsReview    bool
}

var ErrConflict = errors.New("the published edition or draft has changed; reload before saving")

type Options struct {
	Origin          string
	ExpectedEdition *int64
	ReviewOnly      bool // Retain the source and prepare a draft without publishing.
}

func Import(ctx context.Context, db *sql.DB, snapshot crawling.Snapshot) (Result, error) {
	if err := crawling.ValidateSnapshot(snapshot); err != nil {
		return Result{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin catalog import: %w", err)
	}
	defer tx.Rollback()
	result, err := ImportTx(ctx, tx, snapshot, Options{Origin: "source"})
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit catalog import: %w", err)
	}
	return result, nil
}

// ImportTx lets admin publication and draft removal commit atomically.
func ImportTx(ctx context.Context, tx *sql.Tx, snapshot crawling.Snapshot, options Options) (Result, error) {
	if err := crawling.ValidateSnapshot(snapshot); err != nil {
		return Result{}, err
	}
	hash := crawling.MembershipHash(snapshot.Entries)
	if options.Origin == "" {
		options.Origin = "source"
	}

	_, err := tx.ExecContext(ctx, `INSERT INTO canons (slug, name, description, source_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slug) DO NOTHING`, snapshot.Canon.Slug, snapshot.Canon.Name, snapshot.Canon.Description, snapshot.Canon.SourceURL)
	if err != nil {
		return Result{}, fmt.Errorf("upsert canon: %w", err)
	}
	var canonID int64
	var currentEdition sql.NullInt64
	if err := tx.QueryRowContext(ctx, "SELECT id, current_edition_id FROM canons WHERE slug = $1 FOR UPDATE", snapshot.Canon.Slug).Scan(&canonID, &currentEdition); err != nil {
		return Result{}, fmt.Errorf("read canon: %w", err)
	}
	if options.ExpectedEdition != nil && currentEdition.Int64 != *options.ExpectedEdition {
		return Result{}, ErrConflict
	}
	if options.ReviewOnly {
		var hasDraft bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM canon_drafts WHERE canon_id=$1)`, canonID).Scan(&hasDraft); err != nil {
			return Result{}, err
		}
		if hasDraft {
			return Result{}, ErrConflict
		}
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return Result{}, fmt.Errorf("encode source snapshot: %w", err)
	}
	sourceHash := snapshot.SourceContentHash
	if sourceHash == "" {
		sourceDigest := sha256.Sum256(payload)
		sourceHash = hex.EncodeToString(sourceDigest[:])
	}
	var snapshotID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO canon_source_snapshots
		(canon_id, retrieved_at, source_url, source_content_hash, normalized_payload, validation_status, origin)
		VALUES ($1, $2, $3, $4, $5::jsonb, 'valid', $6) RETURNING id`, canonID, snapshot.RetrievedAt, snapshot.Canon.SourceURL, sourceHash, string(payload), options.Origin).Scan(&snapshotID); err != nil {
		return Result{}, fmt.Errorf("store source snapshot: %w", err)
	}
	if options.Origin == "source" {
		var hasOverrides bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM canon_manual_overrides WHERE canon_id=$1 AND active)", canonID).Scan(&hasOverrides); err != nil {
			return Result{}, err
		}
		if hasOverrides || options.ReviewOnly {
			candidate, conflicts, err := applyOverrides(ctx, tx, canonID, snapshot)
			if err != nil {
				return Result{}, err
			}
			var displayName sql.NullString
			var displayDescription sql.NullString
			p := &crawling.CanonPresentation{}
			if err := tx.QueryRowContext(ctx, `SELECT display_name,display_description,display_position,poster_url,poster_title_embedded FROM canons WHERE id=$1`, canonID).Scan(&displayName, &displayDescription, &p.Position, &p.PosterURL, &p.TitleEmbedded); err != nil {
				return Result{}, err
			}
			if currentEdition.Valid || candidate.Canon.Presentation == nil {
				candidate.Canon.Presentation = p
			}
			if displayName.Valid {
				candidate.Canon.Name = displayName.String
			}
			if displayDescription.Valid {
				candidate.Canon.Description = displayDescription.String
			}
			if options.ReviewOnly {
				candidate.Edition = "source-review-" + time.Now().UTC().Format("20060102-150405.000000000")
			}
			candidatePayload, _ := json.Marshal(candidate)
			conflictPayload, _ := json.Marshal(conflicts)
			if _, err := tx.ExecContext(ctx, `INSERT INTO canon_drafts (canon_id, base_edition_id, payload, reason, conflicts)
				VALUES ($1,$2,$3::jsonb,'Source update with retained manual overrides',$4::jsonb) ON CONFLICT (canon_id) DO NOTHING`, canonID, currentEdition, string(candidatePayload), string(conflictPayload)); err != nil {
				return Result{}, err
			}
			return Result{FilmCount: len(snapshot.Entries), NeedsReview: true}, nil
		}
	}
	if currentEdition.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE canons SET name=$2,description=$3,source_url=$4,updated_at=now() WHERE id=$1`, canonID, snapshot.Canon.Name, snapshot.Canon.Description, snapshot.Canon.SourceURL); err != nil {
			return Result{}, err
		}
		var currentHash string
		if err := tx.QueryRowContext(ctx, "SELECT membership_hash FROM canon_editions WHERE id = $1", currentEdition.Int64).Scan(&currentHash); err != nil {
			return Result{}, fmt.Errorf("read current edition: %w", err)
		}
		if currentHash == hash && options.Origin != "manual" {
			for _, entry := range snapshot.Entries {
				if _, err := upsertMovie(ctx, tx, snapshot, entry, options.Origin); err != nil {
					return Result{}, fmt.Errorf("refresh movie %s: %w", entry.IMDbID, err)
				}
			}
			return Result{EditionCreated: false, FilmCount: len(snapshot.Entries)}, nil
		}
	}

	var editionID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO canon_editions
		(canon_id, label, source_retrieved_at, membership_hash, source_snapshot_id)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, canonID, snapshot.Edition, snapshot.RetrievedAt, hash, snapshotID).Scan(&editionID); err != nil {
		return Result{}, fmt.Errorf("create edition: %w", err)
	}
	for _, entry := range snapshot.Entries {
		movieID, err := upsertMovie(ctx, tx, snapshot, entry, options.Origin)
		if err != nil {
			return Result{}, fmt.Errorf("upsert movie %s: %w", entry.IMDbID, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO canon_movies (edition_id, movie_id, position) VALUES ($1, $2, $3)", editionID, movieID, entry.Position); err != nil {
			return Result{}, fmt.Errorf("insert Canon membership for %s: %w", entry.IMDbID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE canons SET current_edition_id = $1, name=$3,description=$4,source_url=$5,updated_at = now() WHERE id = $2", editionID, canonID, snapshot.Canon.Name, snapshot.Canon.Description, snapshot.Canon.SourceURL); err != nil {
		return Result{}, fmt.Errorf("publish edition: %w", err)
	}
	return Result{EditionCreated: true, FilmCount: len(snapshot.Entries)}, nil
}

func upsertMovie(ctx context.Context, tx *sql.Tx, snapshot crawling.Snapshot, entry crawling.Entry, origin string) (int64, error) {
	var movieID int64
	err := tx.QueryRowContext(ctx, `INSERT INTO movies (imdb_id, tmdb_id, title, release_year, imdb_rating, rating_source, rating_retrieved_at)
		VALUES ($1, NULLIF($2,0), $3, $4, $5, CASE WHEN $5::numeric IS NULL THEN NULL ELSE 'IMDb source snapshot' END, $6)
		ON CONFLICT (imdb_id) DO UPDATE SET
			tmdb_id = COALESCE(movies.tmdb_id, EXCLUDED.tmdb_id),
			title = CASE WHEN $7='source' AND movies.metadata_updated_at IS NULL THEN EXCLUDED.title ELSE movies.title END,
			release_year = CASE WHEN $7='source' THEN COALESCE(EXCLUDED.release_year,movies.release_year) ELSE movies.release_year END,
			imdb_rating = CASE WHEN $7='source' THEN COALESCE(EXCLUDED.imdb_rating,movies.imdb_rating) ELSE movies.imdb_rating END,
			rating_source = CASE WHEN $7<>'source' OR EXCLUDED.imdb_rating IS NULL THEN movies.rating_source ELSE EXCLUDED.rating_source END,
			rating_retrieved_at = CASE WHEN $7<>'source' OR EXCLUDED.imdb_rating IS NULL THEN movies.rating_retrieved_at ELSE EXCLUDED.rating_retrieved_at END,
			updated_at = now()
		WHERE EXCLUDED.tmdb_id IS NULL OR movies.tmdb_id IS NULL OR movies.tmdb_id = EXCLUDED.tmdb_id
		RETURNING id`, entry.IMDbID, entry.TMDbID, entry.SourceTitle, entry.ReleaseYear, entry.IMDbRating, snapshot.RetrievedAt, origin).Scan(&movieID)
	if errors.Is(err, sql.ErrNoRows) && entry.TMDbID > 0 {
		return 0, fmt.Errorf("IMDb %s already has a different TMDb mapping; correct the shared movie mapping before importing this selection", entry.IMDbID)
	}
	return movieID, err
}
