// Package catalogupdate records manual checks and retains the last valid edition.
// No scheduler is started here.
package catalogupdate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"time"
)

func run(ctx context.Context, db *sql.DB, slug string, prepare func(context.Context) (crawling.Snapshot, error)) (result catalogimport.Result, err error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, "canon-update:"+slug).Scan(&locked); err != nil {
		return result, err
	}
	if !locked {
		return result, crawling.ErrUpdateInProgress
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,0))`, "canon-update:"+slug)
	var runID int64
	err = conn.QueryRowContext(ctx, `INSERT INTO canon_crawler_runs (canon_id,status) SELECT id,'running' FROM canons WHERE slug=$1 RETURNING id`, slug).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		snapshot, prepareErr := prepare(ctx)
		if prepareErr != nil {
			return result, prepareErr
		}
		return catalogimport.Import(ctx, db, snapshot)
	}
	if err != nil {
		return result, err
	}
	var hash string
	defer func() {
		status := "unchanged"
		var summary any
		if err != nil {
			status = "failed"
			summary = err.Error()
		} else if result.EditionCreated || result.NeedsReview {
			status = "succeeded"
		}
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer finishCancel()
		_, recordErr := conn.ExecContext(finishCtx, `UPDATE canon_crawler_runs SET status=$1,finished_at=now(),error_summary=$2,source_content_hash=NULLIF($3,''),published_edition_id=CASE WHEN $4 THEN (SELECT current_edition_id FROM canons WHERE slug=$5) ELSE NULL END WHERE id=$6`, status, summary, hash, result.EditionCreated, slug, runID)
		if recordErr != nil && err == nil {
			err = fmt.Errorf("record update status: %w", recordErr)
		}
	}()
	snapshot, err := prepare(ctx)
	if err != nil {
		return result, err
	}
	hash = snapshot.SourceContentHash
	if snapshot.Canon.Slug != slug {
		return result, errors.New("crawler returned a different Canon identity")
	}
	result, err = catalogimport.Import(ctx, db, snapshot)
	return result, err
}
func ImportSnapshot(ctx context.Context, db *sql.DB, snapshot crawling.Snapshot) (catalogimport.Result, error) {
	return run(ctx, db, snapshot.Canon.Slug, func(context.Context) (crawling.Snapshot, error) {
		if err := crawling.ValidateSnapshot(snapshot); err != nil {
			return crawling.Snapshot{}, err
		}
		return snapshot, nil
	})
}
func Update(ctx context.Context, db *sql.DB, runner *crawling.Runner, key, sourceURL string) (catalogimport.Result, error) {
	return run(ctx, db, key, func(ctx context.Context) (crawling.Snapshot, error) { return runner.Update(ctx, key, sourceURL) })
}
