CREATE TABLE mobile_login_challenges (
    id UUID PRIMARY KEY,
    nonce_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE mobile_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_version BIGINT NOT NULL,
    access_hash TEXT NOT NULL UNIQUE,
    access_expires_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE mobile_refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES mobile_sessions(id) ON DELETE CASCADE,
    spent BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX mobile_sessions_user ON mobile_sessions(user_id);
CREATE INDEX mobile_sessions_expiry ON mobile_sessions(expires_at);
CREATE INDEX mobile_refresh_session ON mobile_refresh_tokens(session_id);
CREATE TABLE progress_operations (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    operation_id UUID NOT NULL,
    request_hash TEXT NOT NULL,
    result JSONB NOT NULL,
    PRIMARY KEY (user_id, operation_id)
);
REVOKE ALL ON mobile_login_challenges, mobile_sessions, mobile_refresh_tokens, progress_operations FROM PUBLIC;

-- Field revisions distinguish a concurrent change to the other flag from a conflict.
ALTER TABLE user_movie_progress ADD COLUMN watched_revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE user_movie_progress ADD COLUMN watchlist_revision BIGINT NOT NULL DEFAULT 1;
UPDATE user_movie_progress SET watched_revision=revision,watchlist_revision=revision;
