package canonadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/reelcanon/reelcanon/backend/internal/catalogimport"
	"github.com/reelcanon/reelcanon/backend/internal/crawling"
)

type Draft struct {
	Snapshot    crawling.Snapshot
	BaseEdition int64
	Revision    int64
	Reason      string
	Conflicts   []string
}
type Edition struct {
	ID          int64
	Label       string
	PublishedAt time.Time
}
type Service struct{ DB *sql.DB }

func (s Service) Load(ctx context.Context, slug string) (Draft, error) {
	var id int64
	var name, description, source string
	var edition sql.NullInt64
	var presentation crawling.CanonPresentation
	if err := s.DB.QueryRowContext(ctx, `SELECT id,COALESCE(display_name,name),COALESCE(display_description,description),source_url,current_edition_id,display_position,poster_url,poster_title_embedded FROM canons WHERE slug=$1`, slug).Scan(&id, &name, &description, &source, &edition, &presentation.Position, &presentation.PosterURL, &presentation.TitleEmbedded); err != nil {
		return Draft{}, err
	}
	var draft Draft
	var payload, conflicts []byte
	var base sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT base_edition_id,revision,payload,reason,conflicts FROM canon_drafts WHERE canon_id=$1`, id).Scan(&base, &draft.Revision, &payload, &draft.Reason, &conflicts)
	if err == nil {
		draft.BaseEdition = base.Int64
		if err = json.Unmarshal(payload, &draft.Snapshot); err != nil {
			return Draft{}, err
		}
		if draft.Snapshot.Canon.Presentation == nil {
			draft.Snapshot.Canon.Presentation = &presentation
		}
		err = json.Unmarshal(conflicts, &draft.Conflicts)
		return draft, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Draft{}, err
	}
	draft.BaseEdition = edition.Int64
	if edition.Valid {
		draft.Snapshot, err = s.Snapshot(ctx, slug, edition.Int64)
		if err != nil {
			return Draft{}, err
		}
	} else {
		draft.Snapshot = crawling.Snapshot{Format: crawling.SnapshotFormat, Canon: crawling.CanonSource{Slug: slug, Name: name, Description: description, SourceURL: source}, RetrievedAt: time.Now().UTC(), Entries: []crawling.Entry{}}
	}
	draft.Snapshot.Canon = crawling.CanonSource{Slug: slug, Name: name, Description: description, SourceURL: source, Presentation: &presentation}
	draft.Snapshot.Edition = "manual-" + time.Now().UTC().Format("20060102-150405.000000000")
	return draft, nil
}

func (s Service) Snapshot(ctx context.Context, slug string, editionID int64) (crawling.Snapshot, error) {
	var payload []byte
	err := s.DB.QueryRowContext(ctx, `SELECT ss.normalized_payload FROM canon_editions e JOIN canons c ON c.id=e.canon_id JOIN canon_source_snapshots ss ON ss.id=e.source_snapshot_id WHERE c.slug=$1 AND e.id=$2`, slug, editionID).Scan(&payload)
	if err != nil {
		return crawling.Snapshot{}, err
	}
	var snapshot crawling.Snapshot
	err = json.Unmarshal(payload, &snapshot)
	return snapshot, err
}

func (s Service) Save(ctx context.Context, slug string, draft Draft) error {
	if draft.Snapshot.Canon.Slug != slug || draft.Snapshot.Format != crawling.SnapshotFormat {
		return errors.New("Canon identity and snapshot format must match")
	}
	if strings.TrimSpace(draft.Snapshot.Canon.Name) == "" {
		return errors.New("Sammlungsname ist erforderlich")
	}
	if err := crawling.ValidatePresentation(draft.Snapshot.Canon.Presentation); err != nil {
		return err
	}
	if len(draft.Snapshot.Entries) > 10000 {
		return errors.New("draft exceeds 10000 films")
	}
	payload, err := json.Marshal(draft.Snapshot)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var edition sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT id,current_edition_id FROM canons WHERE slug=$1 FOR UPDATE`, slug).Scan(&id, &edition); err != nil {
		return err
	}
	if edition.Int64 != draft.BaseEdition {
		return catalogimport.ErrConflict
	}
	var currentRevision int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM canon_drafts WHERE canon_id=$1 FOR UPDATE`, id).Scan(&currentRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if currentRevision != draft.Revision {
		return catalogimport.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO canon_drafts (canon_id,base_edition_id,payload,reason) VALUES ($1,$2,$3::jsonb,$4) ON CONFLICT (canon_id) DO UPDATE SET payload=EXCLUDED.payload,reason=EXCLUDED.reason,revision=nextval('canon_draft_revision_seq'),conflicts='[]'::jsonb,updated_at=now()`, id, edition, string(payload), draft.Reason)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) Publish(ctx context.Context, slug string, revision int64) (catalogimport.Result, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return catalogimport.Result{}, err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM canons WHERE slug=$1 FOR UPDATE`, slug).Scan(&id); err != nil {
		return catalogimport.Result{}, err
	}
	var payload, conflicts []byte
	var base sql.NullInt64
	var actual int64
	var reason string
	if err = tx.QueryRowContext(ctx, `SELECT payload,base_edition_id,revision,reason,conflicts FROM canon_drafts WHERE canon_id=$1 FOR UPDATE`, id).Scan(&payload, &base, &actual, &reason, &conflicts); err != nil {
		return catalogimport.Result{}, err
	}
	if actual != revision {
		return catalogimport.Result{}, catalogimport.ErrConflict
	}
	if strings.TrimSpace(reason) == "" {
		return catalogimport.Result{}, errors.New("a reason for manual publication is required")
	}
	var issues []string
	if err = json.Unmarshal(conflicts, &issues); err != nil {
		return catalogimport.Result{}, err
	}
	if len(issues) > 0 {
		return catalogimport.Result{}, errors.New("resolve source conflicts by reviewing and saving the draft first")
	}
	snapshot, err := crawling.ParseSnapshot(strings.NewReader(string(payload)))
	if err != nil {
		return catalogimport.Result{}, err
	}
	if snapshot.Canon.Presentation == nil {
		p := &crawling.CanonPresentation{}
		if err := tx.QueryRowContext(ctx, `SELECT display_position,poster_url,poster_title_embedded FROM canons WHERE id=$1`, id).Scan(&p.Position, &p.PosterURL, &p.TitleEmbedded); err != nil {
			return catalogimport.Result{}, err
		}
		snapshot.Canon.Presentation = p
	}
	result, err := catalogimport.ImportTx(ctx, tx, snapshot, catalogimport.Options{Origin: "manual", ExpectedEdition: &base.Int64})
	if err != nil {
		return catalogimport.Result{}, err
	}
	if snapshot.Canon.Presentation != nil {
		p := snapshot.Canon.Presentation
		if _, err = tx.ExecContext(ctx, `UPDATE canons SET display_name=$2,display_position=$3,poster_url=$4,poster_title_embedded=$5,display_description=$6,updated_at=now() WHERE id=$1`, id, snapshot.Canon.Name, p.Position, p.PosterURL, p.TitleEmbedded, snapshot.Canon.Description); err != nil {
			return catalogimport.Result{}, err
		}
	}
	if err = catalogimport.RecordOverrides(ctx, tx, id, snapshot, reason); err != nil {
		return catalogimport.Result{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM canon_drafts WHERE canon_id=$1`, id); err != nil {
		return catalogimport.Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return catalogimport.Result{}, err
	}
	return result, nil
}

func (s Service) History(ctx context.Context, slug string) ([]Edition, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT e.id,e.label,e.published_at FROM canon_editions e JOIN canons c ON c.id=e.canon_id WHERE c.slug=$1 ORDER BY e.id DESC`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	editions := make([]Edition, 0)
	for rows.Next() {
		var edition Edition
		if err := rows.Scan(&edition.ID, &edition.Label, &edition.PublishedAt); err != nil {
			return nil, err
		}
		editions = append(editions, edition)
	}
	return editions, rows.Err()
}

func (s Service) Restore(ctx context.Context, slug string, editionID, revision int64) error {
	draft, err := s.Load(ctx, slug)
	if err != nil {
		return err
	}
	if draft.Revision != revision {
		return catalogimport.ErrConflict
	}
	snapshot, err := s.Snapshot(ctx, slug, editionID)
	if err != nil {
		return err
	}
	snapshot.Edition = "restore-" + time.Now().UTC().Format("20060102-150405.000000000")
	if snapshot.Canon.Presentation == nil {
		snapshot.Canon.Presentation = draft.Snapshot.Canon.Presentation
	}
	draft.Snapshot = snapshot
	draft.Reason = fmt.Sprintf("Restore edition %d", editionID)
	return s.Save(ctx, slug, draft)
}
