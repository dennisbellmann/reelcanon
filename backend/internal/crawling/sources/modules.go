// Package sources wires operator-configured sources into the shared crawler runner.
package sources

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
	"github.com/reelcanon/reelcanon/backend/internal/crawling/imdbtop250"
	"github.com/reelcanon/reelcanon/backend/internal/crawling/movies1001"
)

// Definition is deployment configuration, not an upload accepted by the cockpit.
// File paths resolve relative to the manifest. Remote endpoints cannot be changed
// by a submitted form; redirects are rejected to retain that boundary.
type Definition struct {
	ID     string `json:"key"`
	Name   string `json:"name"`
	Parser string `json:"parser"`
	File   string `json:"file,omitempty"`
	URL    string `json:"url,omitempty"`
}

type Module struct{ Definition }

func (m Module) Key() string { return m.ID }
func (m Module) Kind() string {
	if m.File != "" {
		return "Gespeicherte JSON-Datei"
	}
	return "Online-Quelle"
}

func Load(path string) ([]Module, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1024*1024))
	decoder.DisallowUnknownFields()
	var definitions []Definition
	if err := decoder.Decode(&definitions); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("module manifest must contain one JSON value")
	}
	modules := make([]Module, 0, len(definitions))
	seen := map[string]bool{}
	for _, d := range definitions {
		if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.Name) == "" || seen[d.ID] {
			return nil, errors.New("modules require unique keys and names")
		}
		seen[d.ID] = true
		if d.Parser != "snapshot" && d.Parser != "imdb-top-250" && d.Parser != "1001-movies" {
			return nil, fmt.Errorf("module %s: unknown parser %q", d.ID, d.Parser)
		}
		if (d.File == "") == (d.URL == "") {
			return nil, fmt.Errorf("module %s requires either file or url", d.ID)
		}
		if d.File != "" {
			if !filepath.IsAbs(d.File) {
				d.File = filepath.Join(filepath.Dir(path), d.File)
			}
			info, err := os.Stat(d.File)
			if err != nil {
				return nil, fmt.Errorf("module %s: %w", d.ID, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("module %s: source must be a file", d.ID)
			}
		} else {
			u, err := url.Parse(d.URL)
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
				return nil, fmt.Errorf("module %s requires an absolute HTTPS endpoint without credentials or fragment", d.ID)
			}
		}
		modules = append(modules, Module{d})
	}
	return modules, nil
}

func (m Module) Fetch(ctx context.Context, _ string) (crawling.SourceDocument, error) {
	var input io.ReadCloser
	if m.File != "" {
		file, err := os.Open(m.File)
		if err != nil {
			return crawling.SourceDocument{}, err
		}
		input = file
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
		if err != nil {
			return crawling.SourceDocument{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "ReelCanon-Cockpit/1.0")
		client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(req)
		if err != nil {
			return crawling.SourceDocument{}, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return crawling.SourceDocument{}, fmt.Errorf("source returned HTTP %d", response.StatusCode)
		}
		input = response.Body
	}
	defer input.Close()
	body, err := io.ReadAll(io.LimitReader(input, 2*1024*1024+1))
	if err != nil {
		return crawling.SourceDocument{}, err
	}
	if len(body) > 2*1024*1024 {
		return crawling.SourceDocument{}, errors.New("source exceeds 2 MB")
	}
	if err := ctx.Err(); err != nil {
		return crawling.SourceDocument{}, err
	}
	hash := sha256.Sum256(body)
	return crawling.SourceDocument{URL: m.URL, Body: body, RetrievedAt: time.Now().UTC(), ContentHash: hex.EncodeToString(hash[:])}, nil
}

func (m Module) Parse(document crawling.SourceDocument) (crawling.Snapshot, error) {
	var snapshot crawling.Snapshot
	var err error
	switch m.Parser {
	case "imdb-top-250":
		snapshot, err = imdbtop250.NewModule(nil).Parse(document)
	case "1001-movies":
		snapshot, err = movies1001.NewModule(nil).Parse(document)
	default:
		snapshot, err = crawling.ParseSnapshot(bytes.NewReader(document.Body))
	}
	if err != nil {
		return crawling.Snapshot{}, err
	}
	snapshot.SourceContentHash = document.ContentHash
	return snapshot, nil
}
