package accounts

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Progress struct {
	ImdbID            string     `json:"imdbId"`
	Watched           bool       `json:"watched"`
	Watchlist         bool       `json:"watchlist"`
	WatchedAt         *time.Time `json:"watchedAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
	Revision          int64      `json:"revision"`
	WatchedRevision   int64      `json:"watchedRevision"`
	WatchlistRevision int64      `json:"watchlistRevision"`
}

var imdbPattern = regexp.MustCompile(`^tt[0-9]{7,10}$`)
var revisionPattern = regexp.MustCompile(`^"[0-9]+"$`)

func (s *Service) listProgress(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.require(w, r)
	if !ok {
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT m.imdb_id,p.watched,p.watchlist,p.watched_at,p.updated_at,p.revision,p.watched_revision,p.watchlist_revision
 FROM user_movie_progress p JOIN movies m ON m.id=p.movie_id WHERE p.user_id=$1 ORDER BY m.imdb_id`, profile.ID)
	if err != nil {
		jsonError(w, 503, "progress_unavailable", "Dein Fortschritt konnte nicht geladen werden.")
		return
	}
	defer rows.Close()
	items := []Progress{}
	for rows.Next() {
		var item Progress
		if err = rows.Scan(&item.ImdbID, &item.Watched, &item.Watchlist, &item.WatchedAt, &item.UpdatedAt, &item.Revision, &item.WatchedRevision, &item.WatchlistRevision); err != nil {
			jsonError(w, 503, "progress_unavailable", "Dein Fortschritt konnte nicht geladen werden.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		jsonError(w, 503, "progress_unavailable", "Dein Fortschritt konnte nicht geladen werden.")
		return
	}
	writeJSON(w, 200, items)
}
func (s *Service) changeProgress(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.require(w, r)
	if !ok {
		return
	}
	imdb, field := r.PathValue("imdbId"), r.PathValue("field")
	if !imdbPattern.MatchString(imdb) || (field != "watched" && field != "watchlist") {
		jsonError(w, 400, "invalid_progress", "Ungültiger Film oder Status.")
		return
	}
	match := r.Header.Get("If-Match")
	if match == "" {
		jsonError(w, 428, "missing_revision", "Die aktuelle Versionsnummer fehlt.")
		return
	}
	if !revisionPattern.MatchString(match) {
		jsonError(w, 400, "invalid_revision", "Ungültige Versionsnummer.")
		return
	}
	expected, err := strconv.ParseInt(strings.Trim(match, "\""), 10, 64)
	if err != nil || expected < 0 {
		jsonError(w, 400, "invalid_revision", "Ungültige Versionsnummer.")
		return
	}
	var input struct {
		Value *bool `json:"value"`
	}
	value := false
	if r.Method == http.MethodPut {
		if err = decodeBody(w, r, 1024, &input); err != nil || input.Value == nil {
			jsonError(w, 400, "invalid_boolean", "Ein boolescher Status ist erforderlich.")
			return
		}
		value = *input.Value
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
		return
	}
	defer tx.Rollback()
	// Serialize all progress writes for this account (including imports). Also
	// prevents an account-deletion race without inventing a catalog-wide lock.
	var version int64
	if err = tx.QueryRowContext(r.Context(), "SELECT session_version FROM users WHERE id=$1 AND status='active' FOR UPDATE", profile.ID).Scan(&version); err != nil || version != s.authVersion(r.Context()) || !s.validMobileWrite(r.Context(), tx) {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	operationID, requestHash := "", ""
	if _, mobile := r.Context().Value(mobileAuthKey{}).(mobileAuth); mobile {
		operationID = r.Header.Get("Idempotency-Key")
		if parsed, e := uuid.Parse(operationID); e != nil {
			jsonError(w, 400, "invalid_operation", "Operations-ID erforderlich.")
			return
		} else {
			operationID = parsed.String()
		}
		requestHash = tokenHash(imdb + ":" + field + ":" + strconv.FormatBool(value) + ":" + strconv.FormatInt(expected, 10))
		var savedHash string
		var saved []byte
		e := tx.QueryRowContext(r.Context(), "SELECT request_hash,result FROM progress_operations WHERE user_id=$1 AND operation_id=$2", profile.ID, operationID).Scan(&savedHash, &saved)
		if e == nil {
			if savedHash != requestHash {
				jsonError(w, 409, "operation_reused", "Operations-ID bereits verwendet.")
				return
			}
			var result Progress
			if json.Unmarshal(saved, &result) != nil {
				jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
				return
			}
			w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(result.Revision, 10)))
			writeJSON(w, 200, result)
			return
		}
		if !errors.Is(e, sql.ErrNoRows) {
			jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
			return
		}
	}
	var movieID int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM movies WHERE imdb_id=$1", imdb).Scan(&movieID); errors.Is(err, sql.ErrNoRows) {
		jsonError(w, 404, "movie_not_found", "Dieser Film ist nicht im Katalog.")
		return
	} else if err != nil {
		jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
		return
	}
	item := Progress{ImdbID: imdb, UpdatedAt: time.Now().UTC()}
	err = tx.QueryRowContext(r.Context(), `SELECT watched,watchlist,watched_at,updated_at,revision,watched_revision,watchlist_revision FROM user_movie_progress WHERE user_id=$1 AND movie_id=$2 FOR UPDATE`, profile.ID, movieID).Scan(&item.Watched, &item.Watchlist, &item.WatchedAt, &item.UpdatedAt, &item.Revision, &item.WatchedRevision, &item.WatchlistRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
		return
	}
	if item.Revision != expected {
		w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(item.Revision, 10)))
		writeJSON(w, 412, map[string]any{"error": "Der Film wurde auf einem anderen Gerät geändert.", "code": "progress_conflict", "current": item})
		return
	}
	current := item.Watched
	if field == "watchlist" {
		current = item.Watchlist
	}
	if current != value {
		now := time.Now().UTC()
		if field == "watched" {
			item.Watched = value
			if value {
				item.WatchedAt = &now
			} else {
				item.WatchedAt = nil
			}
		} else {
			item.Watchlist = value
		}
		item.UpdatedAt = now
		item.Revision++
		if field == "watched" {
			item.WatchedRevision = item.Revision
		} else {
			item.WatchlistRevision = item.Revision
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO user_movie_progress(user_id,movie_id,watched,watchlist,watched_at,updated_at,revision,watched_revision,watchlist_revision)
  VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(user_id,movie_id) DO UPDATE SET watched=excluded.watched,
  watchlist=excluded.watchlist,watched_at=excluded.watched_at,updated_at=excluded.updated_at,revision=excluded.revision,watched_revision=excluded.watched_revision,watchlist_revision=excluded.watchlist_revision`,
			profile.ID, movieID, item.Watched, item.Watchlist, item.WatchedAt, item.UpdatedAt, item.Revision, item.WatchedRevision, item.WatchlistRevision)
		if err != nil {
			jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
			return
		}
	}
	if operationID != "" {
		saved, _ := json.Marshal(item)
		if _, err = tx.ExecContext(r.Context(), "INSERT INTO progress_operations(user_id,operation_id,request_hash,result) VALUES($1,$2,$3,$4)", profile.ID, operationID, requestHash, string(saved)); err != nil {
			jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		jsonError(w, 503, "save_failed", "Speichern fehlgeschlagen.")
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(item.Revision, 10)))
	writeJSON(w, 200, item)
}
func (s *Service) importProgress(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.require(w, r)
	if !ok {
		return
	}
	// Pointer fields distinguish omitted values from explicit false/null.
	type imported struct {
		ImdbID    string     `json:"imdbId"`
		Watched   *bool      `json:"watched"`
		Watchlist *bool      `json:"watchlist"`
		WatchedAt *time.Time `json:"watchedAt"`
		UpdatedAt time.Time  `json:"updatedAt"`
	}
	var input struct {
		Format     string     `json:"format"`
		ExportedAt string     `json:"exportedAt"`
		Progress   []imported `json:"progress"`
	}
	if err := decodeBody(w, r, 5_000_000, &input); err != nil || input.Format != "reel-canon-progress/v1" || input.Progress == nil || len(input.Progress) > 20000 {
		jsonError(w, 400, "invalid_backup", "Ungültige Sicherung (maximal 5 MB / 20.000 Filme).")
		return
	}
	seen := map[string]bool{}
	for _, item := range input.Progress {
		if !imdbPattern.MatchString(item.ImdbID) || seen[item.ImdbID] || item.Watched == nil || item.Watchlist == nil || item.UpdatedAt.IsZero() ||
			(*item.Watched && (item.WatchedAt == nil || item.WatchedAt.IsZero())) || (!*item.Watched && item.WatchedAt != nil) {
			jsonError(w, 400, "invalid_backup_entries", "Ungültige oder doppelte Filme und Zeitangaben.")
			return
		}
		seen[item.ImdbID] = true
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 503, "import_failed", "Import fehlgeschlagen.")
		return
	}
	defer tx.Rollback()
	var version int64
	if err = tx.QueryRowContext(r.Context(), "SELECT session_version FROM users WHERE id=$1 AND status='active' FOR UPDATE", profile.ID).Scan(&version); err != nil || version != s.authVersion(r.Context()) || !s.validMobileWrite(r.Context(), tx) {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	added, existing := int64(0), 0
	unknown := []string{}
	for _, item := range input.Progress {
		var movieID int64
		err = tx.QueryRowContext(r.Context(), "SELECT id FROM movies WHERE imdb_id=$1", item.ImdbID).Scan(&movieID)
		if errors.Is(err, sql.ErrNoRows) {
			unknown = append(unknown, item.ImdbID)
			continue
		}
		if err != nil {
			jsonError(w, 503, "import_failed", "Import fehlgeschlagen.")
			return
		}
		result, err := tx.ExecContext(r.Context(), `INSERT INTO user_movie_progress(user_id,movie_id,watched,watchlist,watched_at,updated_at)
  VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(user_id,movie_id) DO NOTHING`, profile.ID, movieID, *item.Watched, *item.Watchlist, item.WatchedAt, item.UpdatedAt)
		if err != nil {
			jsonError(w, 503, "import_failed", "Import fehlgeschlagen.")
			return
		}
		count, err := result.RowsAffected()
		if err != nil {
			jsonError(w, 503, "import_failed", "Import fehlgeschlagen.")
			return
		}
		added += count
		if count == 0 {
			existing++
		}
	}
	if err = tx.Commit(); err != nil {
		jsonError(w, 503, "import_failed", "Import fehlgeschlagen.")
		return
	}
	writeJSON(w, 200, map[string]any{"added": added, "existing": existing, "unknown": unknown})
}
