// mailbox displays local development mail only; it must never be exposed publicly.
package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/accounts"
	"github.com/reelcanon/reelcanon/backend/internal/config"
)

var filePattern = regexp.MustCompile(`^[0-9a-f-]{36}\.json$`)
var mailboxTemplate = template.Must(template.New("mailbox").Parse(`<!doctype html><html lang="de"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Reel Canon · Entwicklungs-Mails</title><style>body{font:16px system-ui;max-width:900px;margin:40px auto;padding:20px;background:#faf8f2}article{border:1px solid #ccc;padding:20px;margin:20px 0}pre{white-space:pre-wrap;overflow-wrap:anywhere}a{color:#b4442b}</style><h1>Entwicklungs-Mails</h1><p>Lokaler Mailfänger für Reel Canon. <a href="/">Aktualisieren</a></p>{{range .}}<article><h2>{{.Subject}}</h2><p>An: {{range .To}}{{.}} {{end}} · {{.CreatedAt}}</p><pre>{{.Body}}</pre>{{if .Link}}<a href="{{.Link}}" rel="noreferrer">Aktion in Reel Canon öffnen</a>{{end}}</article>{{else}}<p>Noch keine Mails. Registriere ein Konto in Reel Canon.</p>{{end}}</html>`))

type item struct {
	accounts.CapturedMail
	Link string
}

func main() {
	if err := config.LoadDotEnv(); err != nil {
		log.Fatal("cannot load configuration")
	}
	directory := os.Getenv("AUTH_MAIL_DIR")
	if directory == "" {
		directory = "../.local/mail"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// Resist DNS rebinding; this server is strictly loopback development tooling.
		if r.Host != "127.0.0.1:8025" && r.Host != "localhost:8025" {
			http.Error(w, "invalid host", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
		files, err := os.ReadDir(directory)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, "mailbox unavailable", 503)
			return
		}
		mails := []item{}
		for _, file := range files {
			if file.IsDir() || !filePattern.MatchString(file.Name()) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(directory, file.Name()))
			if err != nil {
				continue
			}
			var message accounts.CapturedMail
			if json.Unmarshal(data, &message) != nil {
				continue
			}
			entry := item{CapturedMail: message}
			prefix := strings.TrimRight(os.Getenv("AUTH_PUBLIC_URL"), "/") + "/account#"
			for _, line := range strings.Split(message.Body, "\n") {
				if strings.HasPrefix(line, prefix) {
					entry.Link = line
					break
				}
			}
			mails = append(mails, entry)
		}
		sort.Slice(mails, func(i, j int) bool { return mails[i].CreatedAt.After(mails[j].CreatedAt) })
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = mailboxTemplate.Execute(w, mails)
	})
	server := &http.Server{Addr: "127.0.0.1:8025", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Print("development mailbox: http://127.0.0.1:8025")
	log.Fatal(server.ListenAndServe())
}
