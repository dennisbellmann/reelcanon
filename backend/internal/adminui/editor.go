package adminui

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/canonadmin"
	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/tmdb"
)

type movieMetadata struct {
	TMDbID sql.NullInt64
	Error  sql.NullString
}

type overrideRow struct {
	IMDbID string
	Kind   string
	Reason string
}
type editorData struct {
	Draft      canonadmin.Draft
	JSON       string
	CSRF       string
	Validation string
	Changes    []catalogimport.Change
	History    []canonadmin.Edition
	Metadata   map[string]movieMetadata
	Message    string
	Interval   int
	Policy     string
	Enabled    bool
	Overrides  []overrideRow
	Origins    map[string]string
	ModuleName string
}

func (h *Handler) csrf(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie("reelcanon_admin_csrf"); err == nil && len(cookie.Value) == 64 {
		return cookie.Value, nil
	}
	token, err := newCSRFToken()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: "reelcanon_admin_csrf", Value: token, Path: "/canons", MaxAge: 3600, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	return token, nil
}

func (h *Handler) protectForm(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || (media != "application/x-www-form-urlencoded" && media != "multipart/form-data") {
			http.Error(w, "Ungültiges Formular", 415)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host != r.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				http.Error(w, "Ungültige Anfrageherkunft", 403)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
		if media == "multipart/form-data" {
			err = r.ParseMultipartForm(2 * 1024 * 1024)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			http.Error(w, "Ungültiges oder zu großes Formular (maximal 2 MB)", 400)
			return
		}
		cookie, err := r.Cookie("reelcanon_admin_csrf")
		if err != nil || len(cookie.Value) != 64 || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.PostForm.Get("_csrf"))) != 1 {
			http.Error(w, "Anfrage abgelaufen. Bitte Seite neu laden.", 403)
			return
		}
		next(w, r)
	}
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, catalogimport.ErrConflict) {
		status = http.StatusConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
	}
	h.logger.Warn("admin action failed", "error", err)
	http.Error(w, err.Error(), status)
}

func (h *Handler) editor(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	draft, err := h.service.Load(ctx, r.PathValue("slug"))
	if err != nil {
		h.fail(w, err)
		return
	}
	token, err := h.csrf(w, r)
	if err != nil {
		h.fail(w, err)
		return
	}
	payload, err := json.MarshalIndent(draft.Snapshot, "", "  ")
	if err != nil {
		h.fail(w, err)
		return
	}
	page := editorData{Draft: draft, JSON: string(payload), CSRF: token, Metadata: map[string]movieMetadata{}, Message: r.URL.Query().Get("message"), Interval: 2, Policy: "manual", Origins: map[string]string{}}
	if err := crawling.ValidateSnapshot(draft.Snapshot); err != nil {
		page.Validation = err.Error()
	}
	if draft.BaseEdition != 0 {
		snapshot, err := h.service.Snapshot(ctx, r.PathValue("slug"), draft.BaseEdition)
		if err != nil {
			h.fail(w, err)
			return
		}
		page.Changes = catalogimport.Changes(snapshot.Entries, draft.Snapshot.Entries)
	} else {
		page.Changes = catalogimport.Changes(nil, draft.Snapshot.Entries)
	}
	page.History, err = h.service.History(ctx, r.PathValue("slug"))
	if err != nil {
		h.fail(w, err)
		return
	}
	for _, entry := range draft.Snapshot.Entries {
		page.Origins[entry.IMDbID] = "Quelle"
	}
	overrides, err := h.db.QueryContext(ctx, `SELECT imdb_id,override_type,reason FROM canon_manual_overrides WHERE canon_id=(SELECT id FROM canons WHERE slug=$1) AND active ORDER BY id`, r.PathValue("slug"))
	if err != nil {
		h.fail(w, err)
		return
	}
	for overrides.Next() {
		var row overrideRow
		if err := overrides.Scan(&row.IMDbID, &row.Kind, &row.Reason); err != nil {
			overrides.Close()
			h.fail(w, err)
			return
		}
		page.Overrides = append(page.Overrides, row)
		page.Origins[row.IMDbID] = "Manuell"
	}
	err = overrides.Err()
	overrides.Close()
	if err != nil {
		h.fail(w, err)
		return
	}
	for _, change := range page.Changes {
		page.Origins[change.IMDbID] = "Manuell (Entwurf)"
	}
	rows, err := h.db.QueryContext(ctx, `SELECT imdb_id,tmdb_id,metadata_error FROM movies WHERE imdb_id IN (SELECT jsonb_array_elements(payload->'entries')->>'imdbId' FROM canon_drafts WHERE canon_id=(SELECT id FROM canons WHERE slug=$1)) OR id IN (SELECT cm.movie_id FROM canon_movies cm JOIN canons c ON c.current_edition_id=cm.edition_id WHERE c.slug=$1)`, r.PathValue("slug"))
	if err != nil {
		h.fail(w, err)
		return
	}
	for rows.Next() {
		var id string
		var movie movieMetadata
		if err := rows.Scan(&id, &movie.TMDbID, &movie.Error); err != nil {
			rows.Close()
			h.fail(w, err)
			return
		}
		page.Metadata[id] = movie
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		h.fail(w, err)
		return
	}
	var interval sql.NullInt64
	err = h.db.QueryRowContext(ctx, `SELECT refresh_policy,refresh_interval_months,enabled FROM canon_crawler_configs WHERE canon_id=(SELECT id FROM canons WHERE slug=$1)`, r.PathValue("slug")).Scan(&page.Policy, &interval, &page.Enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.fail(w, err)
		return
	}
	if interval.Valid {
		page.Interval = int(interval.Int64)
	}
	var moduleKey string
	if err := h.db.QueryRowContext(ctx, `SELECT crawler_key FROM canon_crawler_configs WHERE canon_id=(SELECT id FROM canons WHERE slug=$1)`, r.PathValue("slug")).Scan(&moduleKey); err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.fail(w, err)
		return
	}
	for _, module := range h.modules {
		if module.Key() == moduleKey {
			page.ModuleName = module.Name
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := editorTemplate.Execute(w, page); err != nil {
		h.logger.Error("render editor", "error", err)
	}
}

func (h *Handler) postedDraft(r *http.Request) (canonadmin.Draft, error) {
	draft, err := h.service.Load(r.Context(), r.PathValue("slug"))
	if err != nil {
		return draft, err
	}
	revision, err := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if err != nil {
		return draft, errors.New("invalid draft revision")
	}
	base, err := strconv.ParseInt(r.PostForm.Get("base"), 10, 64)
	if err != nil {
		return draft, errors.New("invalid base edition")
	}
	if revision != draft.Revision || base != draft.BaseEdition {
		return draft, catalogimport.ErrConflict
	}
	draft.Reason = strings.TrimSpace(r.PostForm.Get("reason"))
	return draft, nil
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, message string) {
	http.Redirect(w, r, "/canons/"+url.PathEscape(r.PathValue("slug"))+"?message="+url.QueryEscape(message), http.StatusSeeOther)
}
func (h *Handler) saveDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	snapshot, err := crawling.DecodeSnapshot(strings.NewReader(r.PostForm.Get("snapshot")))
	if err != nil {
		h.fail(w, err)
		return
	}
	draft.Snapshot = snapshot
	if err := h.service.Save(r.Context(), r.PathValue("slug"), draft); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Entwurf gespeichert. Öffentliche Edition unverändert.")
}

func (h *Handler) resolveDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	snapshot, err := crawling.DecodeSnapshot(strings.NewReader(r.PostForm.Get("snapshot")))
	if err != nil {
		h.fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	if err := h.resolveSnapshot(ctx, &snapshot); err != nil {
		h.fail(w, err)
		return
	}
	draft.Snapshot = snapshot
	if err := h.service.Save(ctx, r.PathValue("slug"), draft); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "IMDb- und TMDb-Zuordnungen aufgelöst und im Entwurf gespeichert. Bitte Vorschau prüfen.")
}
func (h *Handler) savePresentation(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	position, err := strconv.Atoi(r.PostForm.Get("position"))
	if err != nil {
		h.fail(w, errors.New("Ungültige Sammlungsposition"))
		return
	}
	draft.Snapshot.Canon.Name = strings.TrimSpace(r.PostForm.Get("name"))
	draft.Snapshot.Canon.Description = strings.TrimSpace(r.PostForm.Get("description"))
	draft.Snapshot.Canon.Presentation = &crawling.CanonPresentation{Position: position, PosterURL: strings.TrimSpace(r.PostForm.Get("posterUrl")), TitleEmbedded: r.PostForm.Get("titleEmbedded") == "true"}
	if err := h.service.Save(r.Context(), r.PathValue("slug"), draft); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Darstellung im Entwurf gespeichert. Nach Prüfung veröffentlichen.")
}
func (h *Handler) importDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		h.fail(w, err)
		return
	}
	defer file.Close()
	if strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		draft.Snapshot, err = crawling.DecodeCSV(file, draft.Snapshot)
	} else {
		presentation := draft.Snapshot.Canon.Presentation
		draft.Snapshot, err = crawling.DecodeSnapshot(file)
		if draft.Snapshot.Canon.Presentation == nil {
			draft.Snapshot.Canon.Presentation = presentation
		}
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	if err := h.resolveSnapshot(ctx, &draft.Snapshot); err != nil {
		h.fail(w, err)
		return
	}
	draft.Snapshot.Edition = "import-" + time.Now().UTC().Format("20060102-150405.000000000")
	if err := h.service.Save(ctx, r.PathValue("slug"), draft); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Datei als Entwurf importiert. Bitte Vorschau prüfen.")
}
func (h *Handler) editEntry(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	oldPosition := 0
	if raw := r.PostForm.Get("originalPosition"); raw != "" {
		oldPosition, err = strconv.Atoi(raw)
		if err != nil || oldPosition < 1 {
			h.fail(w, errors.New("invalid original rank"))
			return
		}
	}
	entries := make([]crawling.Entry, 0, len(draft.Snapshot.Entries)+1)
	var previous *crawling.Entry
	var edited *crawling.Entry
	for _, entry := range draft.Snapshot.Entries {
		if oldPosition != 0 && entry.Position == oldPosition {
			copy := entry
			previous = &copy
			continue
		}
		entries = append(entries, entry)
	}
	if r.PostForm.Get("action") != "remove" {
		position, err := strconv.Atoi(r.PostForm.Get("position"))
		if err != nil || position < 1 {
			h.fail(w, errors.New("invalid rank"))
			return
		}
		entry := crawling.Entry{IMDbID: strings.TrimSpace(r.PostForm.Get("imdbId")), SourceTitle: strings.TrimSpace(r.PostForm.Get("title")), Position: position}
		entry.TMDbID, err = parseOptionalTMDbID(r.PostForm.Get("tmdbId"))
		if err != nil {
			h.fail(w, err)
			return
		}
		if raw := r.PostForm.Get("year"); raw != "" {
			year, err := strconv.Atoi(raw)
			if err != nil {
				h.fail(w, err)
				return
			}
			entry.ReleaseYear = &year
		}
		if previous != nil {
			entry.IMDbRating = previous.IMDbRating
			if entry.TMDbID == 0 {
				entry.TMDbID = previous.TMDbID
			}
		}
		edited = &entry
		entries = append(entries, entry)
	}
	draft.Snapshot.Entries = entries
	if len(entries) > 0 {
		if err := crawling.ValidateSourceSnapshot(draft.Snapshot); err != nil {
			h.fail(w, err)
			return
		}
		needsResolution := edited != nil && (edited.IMDbID == "" || (edited.TMDbID > 0 && (previous == nil || edited.TMDbID != previous.TMDbID)))
		if needsResolution {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
			defer cancel()
			if err := h.resolveEntry(ctx, edited); err != nil {
				h.fail(w, err)
				return
			}
			draft.Snapshot.Entries[len(draft.Snapshot.Entries)-1] = *edited
			if err := crawling.ValidateSnapshot(draft.Snapshot); err != nil {
				h.fail(w, err)
				return
			}
		}
	}
	if err := h.service.Save(r.Context(), r.PathValue("slug"), draft); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Film im Entwurf geändert.")
}
func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	revision, err := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if err != nil {
		h.fail(w, err)
		return
	}
	if _, err := h.service.Publish(r.Context(), r.PathValue("slug"), revision); err != nil {
		h.fail(w, err)
		return
	}
	slug := r.PathValue("slug")
	if h.metadata.Configured() {
		go h.enrichPublishedCanon(slug)
		h.redirect(w, r, "Edition veröffentlicht. Der TMDb-Abgleich läuft im Hintergrund; nicht verfügbare Treffer bleiben in der Metadatenübersicht sichtbar.")
		return
	}
	h.redirect(w, r, "Edition veröffentlicht. TMDb-Metadaten wurden nicht geladen, weil im Backend keine TMDb-Zugangsdaten konfiguriert sind.")
}

func (h *Handler) enrichPublishedCanon(slug string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := tmdb.Enrich(ctx, h.db, h.metadata, slug, false)
	if err != nil {
		h.logger.Error("published collection TMDb enrichment failed", "slug", slug, "error", err)
		return
	}
	if result.Failed > 0 {
		h.logger.Warn("published collection TMDb enrichment completed with unresolved films", "slug", slug, "updated", result.Updated, "failed", result.Failed, "errors", result.Errors)
		return
	}
	h.logger.Info("published collection TMDb enrichment completed", "slug", slug, "updated", result.Updated)
}

func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PostForm.Get("edition"), 10, 64)
	if err != nil {
		h.fail(w, err)
		return
	}
	revision, err := strconv.ParseInt(r.PostForm.Get("revision"), 10, 64)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Restore(r.Context(), r.PathValue("slug"), id, revision); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Frühere Edition als neuer Entwurf geladen. Bitte prüfen und veröffentlichen.")
}
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	draft, err := h.service.Load(r.Context(), r.PathValue("slug"))
	if err != nil {
		h.fail(w, err)
		return
	}
	format := r.URL.Query().Get("format")
	if format != "csv" {
		format = "json"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, draft.Snapshot.Canon.Slug, format))
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		err = crawling.WriteCSV(w, draft.Snapshot)
	} else {
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(draft.Snapshot)
	}
	if err != nil {
		h.logger.Error("export snapshot", "error", err)
	}
}
func (h *Handler) configure(w http.ResponseWriter, r *http.Request) {
	interval, err := strconv.Atoi(r.PostForm.Get("interval"))
	if err != nil || interval < 1 || interval > 36 {
		h.fail(w, errors.New("interval must be 1–36 months"))
		return
	}
	policy := r.PostForm.Get("policy")
	if policy != "manual" && policy != "interval" && policy != "static" {
		h.fail(w, errors.New("invalid refresh policy"))
		return
	}
	result, err := h.db.ExecContext(r.Context(), `INSERT INTO canon_crawler_configs (canon_id,crawler_key,refresh_policy,refresh_interval_months,enabled) SELECT id,slug,$2,$3,$4 FROM canons WHERE slug=$1 ON CONFLICT (canon_id) DO UPDATE SET refresh_policy=EXCLUDED.refresh_policy,refresh_interval_months=EXCLUDED.refresh_interval_months,enabled=EXCLUDED.enabled,updated_at=now()`, r.PathValue("slug"), policy, interval, r.PostForm.Get("enabled") == "true")
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		h.fail(w, sql.ErrNoRows)
		return
	}
	h.redirect(w, r, "Refresh-Konfiguration gespeichert. Automatische Quellenabrufe bleiben deaktiviert.")
}
func (h *Handler) enrich(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	result, err := tmdb.Enrich(ctx, h.db, h.metadata, r.PathValue("slug"), false)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, fmt.Sprintf("Metadaten: %d aktualisiert, %d fehlgeschlagen. Fehler sind in der Filmliste sichtbar und erneut abrufbar.", result.Updated, result.Failed))
}
func (h *Handler) mapping(w http.ResponseWriter, r *http.Request) {
	id := r.PostForm.Get("imdbId")
	mapping, err := strconv.ParseInt(r.PostForm.Get("tmdbId"), 10, 64)
	if err != nil || mapping < 1 {
		h.fail(w, errors.New("invalid TMDb ID"))
		return
	}
	if strings.TrimSpace(r.PostForm.Get("reason")) == "" {
		h.fail(w, errors.New("mapping correction requires a reason"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	movie, err := h.metadata.Lookup(ctx, id, &mapping)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := tmdb.StoreMapping(ctx, h.db, id, movie, r.PostForm.Get("reason")); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Gemeinsame Filmzuordnung korrigiert. IMDb-Fortschritt bleibt erhalten.")
}

var editorTemplate = template.Must(template.New("editor").Parse(editorHTML))

func (h *Handler) clearOverride(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	id := r.PostForm.Get("imdbId")
	var payload []byte
	err = h.db.QueryRowContext(r.Context(), `SELECT normalized_payload FROM canon_source_snapshots WHERE canon_id=(SELECT id FROM canons WHERE slug=$1) AND origin='source' ORDER BY id DESC LIMIT 1`, r.PathValue("slug")).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		err = h.db.QueryRowContext(r.Context(), `SELECT ss.normalized_payload FROM canon_editions e JOIN canons c ON c.id=e.canon_id JOIN canon_source_snapshots ss ON ss.id=e.source_snapshot_id WHERE c.slug=$1 ORDER BY e.id LIMIT 1`, r.PathValue("slug")).Scan(&payload)
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	var source crawling.Snapshot
	if err := json.Unmarshal(payload, &source); err != nil {
		h.fail(w, err)
		return
	}
	entries := []crawling.Entry{}
	for _, entry := range draft.Snapshot.Entries {
		if entry.IMDbID != id {
			entries = append(entries, entry)
		}
	}
	for _, entry := range source.Entries {
		if entry.IMDbID == id {
			entries = append(entries, entry)
		}
	}
	draft.Snapshot.Entries = entries
	draft.Reason = "Return " + id + " to source: " + draft.Reason
	if err := h.service.Save(r.Context(), r.PathValue("slug"), draft); err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Quelleneintrag im Entwurf wiederhergestellt. Nach Prüfung veröffentlichen, um die manuelle Änderung zu löschen.")
}
