ALTER TABLE movies ADD COLUMN metadata_updated_at TIMESTAMPTZ;
ALTER TABLE movies ADD COLUMN metadata_error TEXT;
ALTER TABLE movies ADD COLUMN poster_base_url TEXT NOT NULL DEFAULT 'https://image.tmdb.org/t/p/';
ALTER TABLE canon_source_snapshots ADD COLUMN origin TEXT NOT NULL DEFAULT 'source' CHECK (origin IN ('source', 'manual'));

CREATE TABLE canon_drafts (
    canon_id BIGINT PRIMARY KEY REFERENCES canons(id) ON DELETE RESTRICT,
    base_edition_id BIGINT REFERENCES canon_editions(id) ON DELETE RESTRICT,
    revision BIGINT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    conflicts JSONB NOT NULL DEFAULT '[]'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
