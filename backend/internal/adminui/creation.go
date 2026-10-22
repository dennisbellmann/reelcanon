package adminui

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

func (h *Handler) snapshotTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="neue-sammlung.json"`)
	snapshot := crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: "meine-film-liste", Name: "Meine Film-Liste", Description: "Eine eigene Sammlung.", SourceURL: "https://example.com/meine-film-liste", Presentation: &crawling.CanonPresentation{PosterURL: "/artwork/collection-03-filmwelten.png"}}, Edition: "erste-ausgabe", RetrievedAt: time.Now().UTC(), Entries: []crawling.Entry{{IMDbID: "tt0111161", Position: 1, SourceTitle: "The Shawshank Redemption"}}}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		h.logger.Error("write collection template", "error", err)
	}
}

func (h *Handler) createCanon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	kind := r.PostForm.Get("kind")
	var snapshot crawling.Snapshot
	key, policy := "", "manual"
	var err error
	switch kind {
	case "json":
		file, _, fileErr := r.FormFile("file")
		if fileErr != nil {
			h.fail(w, errors.New("Bitte eine JSON-Datei auswählen"))
			return
		}
		defer file.Close()
		snapshot, err = crawling.DecodeSnapshot(file)
		key, policy = "static-json", "static"
	case "module":
		key = r.PostForm.Get("module")
		snapshot, err = h.runner.Update(ctx, key, "")
	case "", "manual":
		snapshot.Canon = crawling.CanonSource{Slug: strings.TrimSpace(r.PostForm.Get("slug")), Name: strings.TrimSpace(r.PostForm.Get("name")), SourceURL: strings.TrimSpace(r.PostForm.Get("source")), Description: strings.TrimSpace(r.PostForm.Get("description"))}
	default:
		h.fail(w, errors.New("Unbekanntes Verfahren zur Listenanlage"))
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	if kind == "module" || kind == "json" {
		if name := strings.TrimSpace(r.PostForm.Get("name")); name != "" {
			snapshot.Canon.Name = name
		}
		if slug := strings.TrimSpace(r.PostForm.Get("slug")); slug != "" {
			snapshot.Canon.Slug = slug
		}
	}
	parsed, err := url.ParseRequestURI(snapshot.Canon.SourceURL)
	if !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(snapshot.Canon.Slug) || strings.TrimSpace(snapshot.Canon.Name) == "" || err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		h.fail(w, errors.New("Gültiger Slug, Name und eine HTTP(S)-Quellen-URL sind erforderlich"))
		return
	}
	if kind == "json" || kind == "module" {
		if len(snapshot.Entries) > 10000 {
			h.fail(w, errors.New("Sammlung überschreitet 10000 Filme"))
			return
		}
		if err := h.resolveSnapshot(ctx, &snapshot); err != nil {
			h.fail(w, err)
			return
		}
		if err := crawling.ValidateSnapshot(snapshot); err != nil {
			h.fail(w, err)
			return
		}
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.fail(w, err)
		return
	}
	defer tx.Rollback()
	var id int64
	c := snapshot.Canon
	err = tx.QueryRowContext(ctx, `INSERT INTO canons (slug,name,source_url,description) VALUES ($1,$2,$3,$4) ON CONFLICT (slug) DO NOTHING RETURNING id`, c.Slug, c.Name, c.SourceURL, c.Description).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		h.fail(w, errors.New("Eine Sammlung mit diesem Slug existiert bereits. Öffne sie zum Bearbeiten oder verwende einen anderen Slug."))
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO canon_crawler_configs (canon_id,crawler_key,refresh_policy,refresh_interval_months,enabled,auto_publish) VALUES ($1,$2,$3,2,false,false)`, id, key, policy); err != nil {
		h.fail(w, err)
		return
	}
	if kind == "json" || kind == "module" {
		if _, err = catalogimport.ImportTx(ctx, tx, snapshot, catalogimport.Options{Origin: "source", ReviewOnly: true}); err != nil {
			h.fail(w, err)
			return
		}
		if _, err = tx.ExecContext(ctx, `UPDATE canon_drafts SET reason=$2 WHERE canon_id=$1`, id, "Neue Sammlung aus "+map[string]string{"json": "JSON-Datei", "module": "Quellenmodul"}[kind]); err != nil {
			h.fail(w, err)
			return
		}
		if kind == "module" {
			if _, err = tx.ExecContext(ctx, `INSERT INTO canon_crawler_runs (canon_id,status,finished_at,source_content_hash) VALUES ($1,'succeeded',now(),NULLIF($2,''))`, id, snapshot.SourceContentHash); err != nil {
				h.fail(w, err)
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		h.fail(w, err)
		return
	}
	http.Redirect(w, r, "/canons/"+url.PathEscape(c.Slug)+"?message="+url.QueryEscape("Sammlung angelegt. Entwurf prüfen, Poster und Position gestalten, anschließend veröffentlichen und live schalten."), http.StatusSeeOther)
}

func (h *Handler) crawl(w http.ResponseWriter, r *http.Request) {
	draft, err := h.postedDraft(r)
	if err != nil {
		h.fail(w, err)
		return
	}
	if draft.Revision != 0 {
		h.fail(w, errors.New("Bitte den vorhandenen Entwurf zuerst prüfen und veröffentlichen. Das Modul überschreibt keine offenen Entwürfe."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	var key string
	if err := h.db.QueryRowContext(ctx, `SELECT crawler_key FROM canon_crawler_configs WHERE canon_id=(SELECT id FROM canons WHERE slug=$1)`, r.PathValue("slug")).Scan(&key); err != nil {
		h.fail(w, err)
		return
	}
	// Verify registration before recording a run. Fetch always uses deployed module configuration.
	found := false
	for _, module := range h.modules {
		if module.Key() == key {
			found = true
		}
	}
	if !found {
		h.fail(w, errors.New("Das konfigurierte Quellenmodul ist nicht registriert"))
		return
	}
	var runID int64
	if err := h.db.QueryRowContext(ctx, `INSERT INTO canon_crawler_runs (canon_id,status) SELECT id,'running' FROM canons WHERE slug=$1 RETURNING id`, r.PathValue("slug")).Scan(&runID); err != nil {
		h.fail(w, err)
		return
	}
	var hash string
	defer func() {
		status, summary := "succeeded", ""
		if err != nil {
			status, summary = "failed", err.Error()
		}
		finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer finishCancel()
		if _, recordErr := h.db.ExecContext(finishCtx, `UPDATE canon_crawler_runs SET status=$2,finished_at=now(),error_summary=NULLIF($3,''),source_content_hash=NULLIF($4,'') WHERE id=$1`, runID, status, summary, hash); recordErr != nil {
			h.logger.Error("record module run", "error", recordErr)
		}
	}()
	var snapshot crawling.Snapshot
	snapshot, err = h.runner.Update(ctx, key, "")
	if err != nil {
		h.fail(w, err)
		return
	}
	hash = snapshot.SourceContentHash
	snapshot.Canon.Slug = draft.Snapshot.Canon.Slug
	if err = h.resolveSnapshot(ctx, &snapshot); err != nil {
		h.fail(w, err)
		return
	}
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		h.fail(w, err)
		return
	}
	defer tx.Rollback()
	_, err = catalogimport.ImportTx(ctx, tx, snapshot, catalogimport.Options{Origin: "source", ReviewOnly: true, ExpectedEdition: &draft.BaseEdition})
	if err != nil {
		h.fail(w, err)
		return
	}
	err = tx.Commit()
	if err != nil {
		h.fail(w, err)
		return
	}
	h.redirect(w, r, "Quelle eingelesen und als Entwurf vorgemerkt. Änderungen prüfen und veröffentlichen.")
}
