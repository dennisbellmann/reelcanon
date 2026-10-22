package adminui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"github.com/reelcanon/reelcanon/backend/internal/canonadmin"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/crawling/sources"
	"github.com/reelcanon/reelcanon/backend/internal/tmdb"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type canonRow struct {
	Slug           string
	Name           string
	Source         string
	CurrentEdition sql.NullString
	FilmCount      int
	HasEdition     bool
	IsLive         bool
	LastCheck      sql.NullTime
	LastError      sql.NullString
	Interval       sql.NullInt64
	Enabled        bool
	Policy         sql.NullString
	Status         sql.NullString
	NextDue        string
}

type data struct {
	Canons  []canonRow
	CSRF    string
	Error   string
	Now     string
	Modules []sources.Module
}

type Handler struct {
	db       *sql.DB
	username string
	password string
	logger   *slog.Logger
	template *template.Template
	service  canonadmin.Service
	metadata *tmdb.Client
	modules  []sources.Module
	runner   *crawling.Runner
}

func New(db *sql.DB, username, password string, logger *slog.Logger, modules ...sources.Module) http.Handler {
	tmpl := template.Must(template.New("layout").Parse(pageTemplate))
	h := &Handler{db: db, username: username, password: password, logger: logger, template: tmpl, service: canonadmin.Service{DB: db}, metadata: tmdb.New(os.Getenv("TMDB_API_KEY"), os.Getenv("TMDB_READ_ACCESS_TOKEN"))}
	registry := crawling.NewRegistry()
	for _, module := range modules {
		if err := registry.Register(module); err != nil {
			panic(err) // Invalid wiring is a startup error, never a partially working cockpit.
		}
	}
	h.modules, h.runner = modules, crawling.NewRunner(registry, 0)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/canons", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /canons", h.canons)
	mux.HandleFunc("POST /canons", h.protectForm(h.createCanon))
	mux.HandleFunc("GET /canons/template", h.snapshotTemplate)
	mux.HandleFunc("POST /canons/{slug}/crawl", h.protectForm(h.crawl))
	mux.HandleFunc("POST /canons/{slug}/visibility", h.setVisibility)
	mux.HandleFunc("GET /canons/{slug}", h.editor)
	mux.HandleFunc("GET /canons/{slug}/export", h.export)
	mux.HandleFunc("POST /canons/{slug}/draft", h.protectForm(h.saveDraft))
	mux.HandleFunc("POST /canons/{slug}/resolve", h.protectForm(h.resolveDraft))
	mux.HandleFunc("POST /canons/{slug}/presentation", h.protectForm(h.savePresentation))
	mux.HandleFunc("POST /canons/{slug}/entry", h.protectForm(h.editEntry))
	mux.HandleFunc("POST /canons/{slug}/import", h.protectForm(h.importDraft))
	mux.HandleFunc("POST /canons/{slug}/publish", h.protectForm(h.publish))
	mux.HandleFunc("POST /canons/{slug}/restore", h.protectForm(h.restore))
	mux.HandleFunc("POST /canons/{slug}/config", h.protectForm(h.configure))
	mux.HandleFunc("POST /canons/{slug}/metadata", h.protectForm(h.enrich))
	mux.HandleFunc("POST /canons/{slug}/mapping", h.protectForm(h.mapping))
	mux.HandleFunc("POST /canons/{slug}/override", h.protectForm(h.clearOverride))
	return h.basicAuth(mux)
}

func (h *Handler) canons(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.db.QueryContext(ctx, `SELECT c.slug, COALESCE(c.display_name,c.name), c.source_url, e.label,
		COUNT(cm.movie_id)::int, c.current_edition_id IS NOT NULL, c.is_live,
		(SELECT MAX(finished_at) FROM canon_crawler_runs WHERE canon_id=c.id AND status IN ('succeeded','unchanged')),
		(SELECT error_summary FROM canon_crawler_runs WHERE canon_id=c.id ORDER BY id DESC LIMIT 1),
		cfg.refresh_interval_months, COALESCE(cfg.enabled,false),cfg.refresh_policy,
		(SELECT status FROM canon_crawler_runs WHERE canon_id=c.id ORDER BY id DESC LIMIT 1)
		FROM canons c
		LEFT JOIN canon_editions e ON e.id = c.current_edition_id
		LEFT JOIN canon_movies cm ON cm.edition_id = e.id
		LEFT JOIN canon_crawler_configs cfg ON cfg.canon_id=c.id
		GROUP BY c.id, e.id, cfg.canon_id ORDER BY c.display_position, COALESCE(c.display_name,c.name), c.slug`)
	page := data{Canons: make([]canonRow, 0), Now: time.Now().UTC().Format("2006-01-02 15:04 UTC"), Modules: h.modules}
	token, tokenErr := h.csrf(w, r)
	if tokenErr != nil {
		h.logger.Error("create admin CSRF token", "error", tokenErr)
		http.Error(w, "Die Admin-Seite konnte nicht geladen werden.", http.StatusInternalServerError)
		return
	}
	page.CSRF = token
	if err != nil {
		h.logger.Error("query admin Canon list", "error", err)
		page.Error = "The catalog database could not be queried. Check the server logs."
	} else {
		defer rows.Close()
		for rows.Next() {
			var item canonRow
			if scanErr := rows.Scan(&item.Slug, &item.Name, &item.Source, &item.CurrentEdition, &item.FilmCount, &item.HasEdition, &item.IsLive, &item.LastCheck, &item.LastError, &item.Interval, &item.Enabled, &item.Policy, &item.Status); scanErr != nil {
				h.logger.Error("read admin Canon row", "error", scanErr)
				page.Error = "The catalog response could not be read. Check the server logs."
				break
			}
			if item.Enabled && item.Policy.String == "interval" && item.LastCheck.Valid && item.Interval.Valid {
				item.NextDue = item.LastCheck.Time.AddDate(0, int(item.Interval.Int64), 0).UTC().Format("2006-01-02")
			}
			page.Canons = append(page.Canons, item)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			h.logger.Error("iterate admin Canon rows", "error", rowsErr)
			page.Error = "The catalog response could not be read. Check the server logs."
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if page.Error != "" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := h.template.Execute(w, page); err != nil {
		h.logger.Error("render admin Canon page", "error", err)
		return
	}
}

func (h *Handler) setVisibility(w http.ResponseWriter, r *http.Request) {
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/x-www-form-urlencoded" {
		http.Error(w, "Ungültiges Formular.", http.StatusUnsupportedMediaType)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != r.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			http.Error(w, "Ungültige Anfrageherkunft.", http.StatusForbidden)
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ungültiges Formular.", http.StatusBadRequest)
		return
	}
	cookie, err := r.Cookie("reelcanon_admin_csrf")
	if err != nil || len(cookie.Value) != 64 || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.PostForm.Get("_csrf"))) != 1 {
		http.Error(w, "Die Anfrage ist abgelaufen. Bitte lade die Admin-Seite neu.", http.StatusForbidden)
		return
	}
	live, err := strconv.ParseBool(r.FormValue("live"))
	if err != nil {
		http.Error(w, "Ungültiger Veröffentlichungsstatus.", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.db.ExecContext(ctx, `UPDATE canons SET is_live = $1, updated_at = now()
		WHERE slug = $2 AND ($1 = false OR current_edition_id IS NOT NULL)`, live, r.PathValue("slug"))
	if err != nil {
		h.logger.Error("change Canon visibility", "slug", r.PathValue("slug"), "error", err)
		http.Error(w, "Der Veröffentlichungsstatus konnte nicht gespeichert werden.", http.StatusInternalServerError)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		http.Error(w, "Canon nicht gefunden oder noch ohne vollständige Edition.", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/canons", http.StatusSeeOther)
}

func newCSRFToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func (h *Handler) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		username, password, ok := r.BasicAuth()
		if !ok || !constantTimeEqual(username, h.username) || !constantTimeEqual(password, h.password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Reel Canon Admin", charset="UTF-8"`)
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func constantTimeEqual(a, b string) bool {
	first, second := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(first[:], second[:]) == 1
}

var pageTemplate = `<!doctype html>
<html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Canon Cockpit · Reel Canon</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f4f1ea;color:#171717;font:15px/1.5 Arial,sans-serif}header,main{max-width:1160px;margin:auto;padding:24px}header{border-bottom:1px solid #d8d3c9;display:flex;justify-content:space-between;align-items:center}header strong{font:24px Georgia,serif;font-weight:400}header span,.muted{color:#625e58}h1{font:48px/1.05 Georgia,serif;font-weight:400;margin:44px 0 8px}.eyebrow{text-transform:uppercase;letter-spacing:.16em;font-size:11px;color:#625e58}table{border-collapse:collapse;width:100%;margin-top:30px;background:#f9f7f2}th,td{text-align:left;padding:15px;border-bottom:1px solid #d8d3c9;vertical-align:top}th{font-size:11px;text-transform:uppercase;letter-spacing:.12em;color:#625e58;font-weight:400}.empty,.error{padding:28px;margin-top:24px;border:1px solid #d8d3c9}.error{border-color:#a03027;color:#78241e}code{font-size:12px}a{color:inherit}.live{color:#49644a}.off{color:#625e58}form{margin:0}button{border:1px solid #171717;background:transparent;padding:8px 11px;font-size:11px;text-transform:uppercase;letter-spacing:.1em;white-space:nowrap}button:hover{background:#171717;color:#f4f1ea}button:disabled{border-color:#d8d3c9;color:#625e58;background:transparent}footer{border-top:1px solid #d8d3c9;margin-top:56px;padding:20px;text-align:center;color:#625e58;font-size:12px}
.table-scroll{overflow-x:auto}table{min-width:650px}label{display:block;margin:12px 0}input{max-width:100%;font:inherit;border:1px solid #d8d3c9;background:transparent;padding:8px}header{flex-wrap:wrap;gap:12px}.creation-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,320px),1fr));gap:20px}.creation-grid section{padding:20px;border:1px solid #d8d3c9;background:#f9f7f2}.creation-grid select{max-width:100%;padding:8px;font:inherit;background:transparent;border:1px solid #d8d3c9}h3{font:24px Georgia,serif;font-weight:400}.creation-grid button{white-space:normal}</style></head><body><header><strong>reel canon<span style="color:#a03027">.</span> <small style="font:11px Arial,sans-serif;color:#625e58">COCKPIT</small></strong><span>Canon Cockpit</span></header><main>
<p class="eyebrow">Canon Cockpit</p><h1>Sammlungen</h1><p class="muted">Importierte Editionen werden erst nach der Freischaltung hier öffentlich angezeigt.</p>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Canons}}<div class="table-scroll"><table><thead><tr><th>Sammlung</th><th>Aktuelle Edition</th><th>Filme</th><th>Öffentlicher Status</th><th>Quelle / Refresh</th></tr></thead><tbody>{{range .Canons}}<tr><td><a href="/canons/{{.Slug}}"><strong>{{.Name}}</strong></a><br><code>{{.Slug}}</code></td><td>{{if .CurrentEdition.Valid}}{{.CurrentEdition.String}}{{else}}<span class="muted">Keine Edition</span>{{end}}</td><td>{{.FilmCount}}</td><td>{{if .IsLive}}<span class="live">Live</span>{{else}}<span class="off">Nicht veröffentlicht</span>{{end}}<br><form method="post" action="/canons/{{.Slug}}/visibility"><input type="hidden" name="_csrf" value="{{$.CSRF}}"><input type="hidden" name="live" value="{{if .IsLive}}false{{else}}true{{end}}"><button type="submit" {{if and (not .IsLive) (not .HasEdition)}}disabled title="Zuerst eine vollständige Edition importieren"{{end}}>{{if .IsLive}}Ausblenden{{else}}Live schalten{{end}}</button></form></td><td>{{if .Source}}<a href="{{.Source}}" rel="noreferrer">Quelle ↗</a>{{else}}—{{end}}<p class="muted">{{if .Enabled}}Vorgemerkt{{else}}Pausiert{{end}}{{if .Interval.Valid}} · alle {{.Interval.Int64}} Monate{{end}}</p><p class="muted">Letzte Prüfung: {{if .LastCheck.Valid}}{{.LastCheck.Time.Format "2006-01-02 15:04"}}{{else}}Noch keine{{end}}</p><p class="muted">Status: {{if .Status.Valid}}{{.Status.String}}{{else}}Noch kein Lauf{{end}} · Nächste Prüfung: {{if .NextDue}}{{.NextDue}}{{else}}Nicht geplant{{end}}</p>{{if .LastError.Valid}}<p class="error">{{.LastError.String}}</p>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty"><strong>Noch keine Canons</strong><p class="muted">Importiere einen normalisierten Snapshot mit <code>make import-canon FILE=…</code>. Danach kannst du die Sammlung hier freischalten.</p></div>{{end}}
<h2>Neue Sammlung</h2><p class="muted">Wähle eine feste JSON-Datei oder ein registriertes Quellenmodul. Titel-only Einträge werden vor dem Entwurf über TMDb abgeglichen. Automatisch übernommen werden nur eindeutige exakte Titel- und Jahrtreffer; mehrdeutige Treffer zeigen TMDb-IDs zur manuellen Auswahl. Poster, Position und Name kannst du anschließend gestalten.</p>
<div class="creation-grid">
<section><h3>Aus JSON-Datei</h3><p class="muted">Name, Slug und Filme werden aus der Datei übernommen. <a href="/canons/template">JSON-Vorlage herunterladen</a> · maximal 2 MB.</p><form method="post" action="/canons" enctype="multipart/form-data"><input type="hidden" name="_csrf" value="{{.CSRF}}"><input type="hidden" name="kind" value="json"><label>JSON-Datei <input name="file" type="file" accept=".json,application/json" required></label><label>Eigener Name (optional) <input name="name" placeholder="Name aus der Datei"></label><label>Eigener Slug (optional) <input name="slug" pattern="[a-z0-9]+(-[a-z0-9]+)*" placeholder="Slug aus der Datei"></label><button>JSON-Sammlung anlegen</button></form></section>
<section><h3>Aus Quellenmodul</h3>{{if .Modules}}<form method="post" action="/canons"><input type="hidden" name="_csrf" value="{{.CSRF}}"><input type="hidden" name="kind" value="module"><label>Quellenmodul <select name="module" required>{{range .Modules}}<option value="{{.Key}}">{{.Name}} · {{.Kind}}</option>{{end}}</select></label><label>Eigener Name (optional) <input name="name" placeholder="Name aus der Quelle"></label><label>Eigener Slug (optional) <input name="slug" pattern="[a-z0-9]+(-[a-z0-9]+)*" placeholder="Slug aus der Quelle"></label><button>Modul einlesen und Sammlung anlegen</button></form><p class="muted">Das Modul liest seine konfigurierte Quelle nur auf deinen Aufruf ein. Verwende einen eigenen Slug, wenn die ursprüngliche Sammlung bereits existiert.</p>{{else}}<p class="muted">Noch keine Quellenmodule registriert. Module werden in der Server-Konfiguration eingebunden und erscheinen anschließend hier.</p>{{end}}</section></div>
<h3 style="margin-top:36px">Leere Sammlung manuell anlegen</h3><form class="manual-create" method="post" action="/canons"><input type="hidden" name="_csrf" value="{{.CSRF}}"><label>Name <input name="name" required></label><label>Slug <input name="slug" pattern="[a-z0-9]+(-[a-z0-9]+)*" required></label><label>Quellen-URL <input name="source" type="url" required></label><label>Beschreibung <input name="description"></label><button>Sammlung anlegen</button></form><p class="muted">Automatische Abrufe bleiben pausiert. Nach dem Prüfen veröffentlichst du die Edition und schaltest die Sammlung separat live.</p></main><footer>Reel Canon · Canon Cockpit · {{.Now}}</footer></body></html>`
