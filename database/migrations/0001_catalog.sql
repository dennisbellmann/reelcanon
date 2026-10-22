CREATE TABLE canons (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    is_live BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    current_edition_id BIGINT
);

CREATE TABLE canon_editions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    canon_id BIGINT NOT NULL REFERENCES canons(id) ON DELETE RESTRICT,
    label TEXT NOT NULL,
    source_retrieved_at TIMESTAMPTZ,
    membership_hash TEXT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (canon_id, label)
);

ALTER TABLE canons
    ADD CONSTRAINT canons_current_edition_fk
    FOREIGN KEY (current_edition_id) REFERENCES canon_editions(id) ON DELETE RESTRICT;

CREATE TABLE movies (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    imdb_id TEXT NOT NULL UNIQUE,
    tmdb_id BIGINT UNIQUE,
    title TEXT NOT NULL,
    original_title TEXT,
    release_year SMALLINT,
    runtime_minutes SMALLINT,
    overview TEXT NOT NULL DEFAULT '',
    poster_path TEXT,
    imdb_rating NUMERIC(3,1),
    rating_source TEXT,
    rating_retrieved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (imdb_rating IS NULL OR (imdb_rating >= 0 AND imdb_rating <= 10))
);

CREATE TABLE canon_movies (
    edition_id BIGINT NOT NULL REFERENCES canon_editions(id) ON DELETE RESTRICT,
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE RESTRICT,
    position INTEGER NOT NULL CHECK (position > 0),
    PRIMARY KEY (edition_id, movie_id),
    UNIQUE (edition_id, position)
);

CREATE TABLE canon_crawler_configs (
    canon_id BIGINT PRIMARY KEY REFERENCES canons(id) ON DELETE RESTRICT,
    crawler_key TEXT NOT NULL,
    refresh_policy TEXT NOT NULL DEFAULT 'manual' CHECK (refresh_policy IN ('manual', 'interval', 'static')),
    refresh_interval_months SMALLINT CHECK (refresh_interval_months IS NULL OR refresh_interval_months BETWEEN 1 AND 36),
    enabled BOOLEAN NOT NULL DEFAULT false,
    auto_publish BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (refresh_policy <> 'interval' OR refresh_interval_months IS NOT NULL)
);

CREATE TABLE canon_crawler_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    canon_id BIGINT NOT NULL REFERENCES canons(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('running', 'succeeded', 'failed', 'unchanged')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    error_summary TEXT,
    source_content_hash TEXT,
    published_edition_id BIGINT REFERENCES canon_editions(id) ON DELETE RESTRICT
);

CREATE TABLE canon_source_snapshots (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    canon_id BIGINT NOT NULL REFERENCES canons(id) ON DELETE RESTRICT,
    retrieved_at TIMESTAMPTZ NOT NULL,
    source_url TEXT NOT NULL,
    source_content_hash TEXT NOT NULL,
    normalized_payload JSONB NOT NULL,
    validation_status TEXT NOT NULL CHECK (validation_status IN ('valid', 'invalid')),
    diagnostics JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE canon_editions
    ADD COLUMN source_snapshot_id BIGINT REFERENCES canon_source_snapshots(id) ON DELETE RESTRICT;

CREATE TABLE canon_manual_overrides (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    canon_id BIGINT NOT NULL REFERENCES canons(id) ON DELETE RESTRICT,
    imdb_id TEXT NOT NULL,
    override_type TEXT NOT NULL CHECK (override_type IN ('include', 'exclude', 'position', 'metadata_mapping')),
    override_value JSONB NOT NULL,
    base_snapshot_id BIGINT REFERENCES canon_source_snapshots(id) ON DELETE RESTRICT,
    reason TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_at TIMESTAMPTZ
);

CREATE TABLE directors (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE movie_directors (
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    director_id BIGINT NOT NULL REFERENCES directors(id) ON DELETE RESTRICT,
    position SMALLINT NOT NULL DEFAULT 1 CHECK (position > 0),
    PRIMARY KEY (movie_id, director_id),
    UNIQUE (movie_id, position)
);

CREATE TABLE genres (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE countries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    code TEXT NOT NULL UNIQUE
);

CREATE TABLE movie_genres (
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    genre_id BIGINT NOT NULL REFERENCES genres(id) ON DELETE RESTRICT,
    PRIMARY KEY (movie_id, genre_id)
);

CREATE TABLE movie_countries (
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    country_id BIGINT NOT NULL REFERENCES countries(id) ON DELETE RESTRICT,
    PRIMARY KEY (movie_id, country_id)
);

CREATE INDEX canon_editions_canon_published_idx ON canon_editions (canon_id, published_at DESC);
CREATE INDEX canon_movies_position_idx ON canon_movies (edition_id, position);
CREATE INDEX movies_release_year_idx ON movies (release_year);
CREATE INDEX canon_crawler_runs_canon_started_idx ON canon_crawler_runs (canon_id, started_at DESC);
CREATE INDEX canon_source_snapshots_canon_retrieved_idx ON canon_source_snapshots (canon_id, retrieved_at DESC);
CREATE INDEX canon_manual_overrides_active_idx ON canon_manual_overrides (canon_id, imdb_id) WHERE active;

INSERT INTO canons (slug, name, description, source_url)
VALUES ('imdb-top-250', 'IMDb Top 250', 'Die 250 am höchsten bewerteten Filme auf IMDb – eine sich wandelnde Sammlung der Filmgeschichte.', 'https://www.imdb.com/chart/top/');

INSERT INTO canon_crawler_configs (canon_id, crawler_key, refresh_policy, refresh_interval_months, enabled, auto_publish)
SELECT id, 'imdb-top-250', 'interval', 2, false, false FROM canons WHERE slug = 'imdb-top-250';
