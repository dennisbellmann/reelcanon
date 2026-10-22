package catalogimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

type Change struct {
	IMDbID string
	Kind   string
	Before int
	After  int
}

func Changes(before, after []crawling.Entry) []Change {
	old := make(map[string]crawling.Entry)
	next := make(map[string]crawling.Entry)
	for _, entry := range before {
		old[entry.IMDbID] = entry
	}
	for _, entry := range after {
		next[entry.IMDbID] = entry
	}
	changes := make([]Change, 0)
	for _, entry := range after {
		previous, exists := old[entry.IMDbID]
		if !exists {
			changes = append(changes, Change{entry.IMDbID, "include", 0, entry.Position})
		} else if previous.Position != entry.Position {
			changes = append(changes, Change{entry.IMDbID, "position", previous.Position, entry.Position})
		}
	}
	for _, entry := range before {
		if _, exists := next[entry.IMDbID]; !exists {
			changes = append(changes, Change{entry.IMDbID, "exclude", entry.Position, 0})
		}
	}
	return changes
}

func applyOverrides(ctx context.Context, tx *sql.Tx, canonID int64, snapshot crawling.Snapshot) (crawling.Snapshot, []string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT imdb_id, override_type, override_value FROM canon_manual_overrides WHERE canon_id=$1 AND active ORDER BY id`, canonID)
	if err != nil {
		return crawling.Snapshot{}, nil, err
	}
	defer rows.Close()
	entries := make(map[string]crawling.Entry)
	for _, entry := range snapshot.Entries {
		entries[entry.IMDbID] = entry
	}
	conflicts := make([]string, 0)
	for rows.Next() {
		var id, kind string
		var payload []byte
		if err := rows.Scan(&id, &kind, &payload); err != nil {
			return crawling.Snapshot{}, nil, err
		}
		switch kind {
		case "exclude":
			delete(entries, id)
		case "include":
			var entry crawling.Entry
			if err := json.Unmarshal(payload, &entry); err != nil {
				return crawling.Snapshot{}, nil, err
			}
			entries[id] = entry
		case "position":
			entry, exists := entries[id]
			if !exists {
				conflicts = append(conflicts, fmt.Sprintf("%s: source removed a film with a manual position", id))
				continue
			}
			if err := json.Unmarshal(payload, &entry.Position); err != nil {
				return crawling.Snapshot{}, nil, err
			}
			entries[id] = entry
		}
	}
	if err := rows.Err(); err != nil {
		return crawling.Snapshot{}, nil, err
	}
	snapshot.Entries = make([]crawling.Entry, 0, len(entries))
	for _, entry := range entries {
		snapshot.Entries = append(snapshot.Entries, entry)
	}
	sort.Slice(snapshot.Entries, func(i, j int) bool { return snapshot.Entries[i].Position < snapshot.Entries[j].Position })
	if err := crawling.ValidateSnapshot(snapshot); err != nil {
		conflicts = append(conflicts, err.Error())
	}
	return snapshot, conflicts, nil
}

// RecordOverrides compares the final manual edition with the latest raw source,
// so subsequent source imports cannot silently undo administrator decisions.
func RecordOverrides(ctx context.Context, tx *sql.Tx, canonID int64, snapshot crawling.Snapshot, reason string) error {
	var raw []byte
	var sourceID int64
	err := tx.QueryRowContext(ctx, `SELECT id, normalized_payload FROM canon_source_snapshots WHERE canon_id=$1 AND origin='source' ORDER BY id DESC LIMIT 1`, canonID).Scan(&sourceID, &raw)
	if err == sql.ErrNoRows {
		// Use the first manual edition as a base if no source import exists yet.
		err = tx.QueryRowContext(ctx, `SELECT ss.id,ss.normalized_payload FROM canon_editions e JOIN canon_source_snapshots ss ON ss.id=e.source_snapshot_id WHERE e.canon_id=$1 ORDER BY e.id LIMIT 1`, canonID).Scan(&sourceID, &raw)
	}
	if err != nil {
		return err
	}
	var source crawling.Snapshot
	if err := json.Unmarshal(raw, &source); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE canon_manual_overrides SET active=false, cleared_at=now() WHERE canon_id=$1 AND active`, canonID); err != nil {
		return err
	}
	byID := make(map[string]crawling.Entry)
	for _, entry := range snapshot.Entries {
		byID[entry.IMDbID] = entry
	}
	for _, change := range Changes(source.Entries, snapshot.Entries) {
		var value any = change.After
		if change.Kind == "include" {
			value = byID[change.IMDbID]
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO canon_manual_overrides (canon_id,imdb_id,override_type,override_value,base_snapshot_id,reason) VALUES ($1,$2,$3,$4::jsonb,$5,$6)`, canonID, change.IMDbID, change.Kind, string(payload), sourceID, reason); err != nil {
			return err
		}
	}
	return nil
}
