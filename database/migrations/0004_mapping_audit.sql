CREATE TABLE movie_mapping_changes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE RESTRICT,
    previous_tmdb_id BIGINT,
    next_tmdb_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
