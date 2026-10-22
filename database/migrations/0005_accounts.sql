-- Account data is writable through a separate connection; catalog access stays read-only.
CREATE TABLE users (
    id UUID PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    session_version BIGINT NOT NULL DEFAULT 1 CHECK (session_version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE user_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL UNIQUE CHECK (length(email) BETWEEN 3 AND 320),
    password_hash TEXT NOT NULL,
    confirmed BOOLEAN NOT NULL DEFAULT false,
    confirm_selector TEXT NOT NULL DEFAULT '',
    confirm_verifier TEXT NOT NULL DEFAULT '',
    confirm_expires_at TIMESTAMPTZ,
    recover_selector TEXT NOT NULL DEFAULT '',
    recover_verifier TEXT NOT NULL DEFAULT '',
    recover_expires_at TIMESTAMPTZ,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    locked_at TIMESTAMPTZ,
    CHECK ((confirm_selector = '' AND confirm_verifier = '' AND confirm_expires_at IS NULL)
        OR (confirm_selector <> '' AND confirm_verifier <> '' AND confirm_expires_at IS NOT NULL)),
    CHECK ((recover_selector = '' AND recover_verifier = '' AND recover_expires_at IS NULL)
        OR (recover_selector <> '' AND recover_verifier <> '' AND recover_expires_at IS NOT NULL))
);
CREATE UNIQUE INDEX credentials_confirm ON user_credentials(confirm_selector) WHERE confirm_selector <> '';
CREATE UNIQUE INDEX credentials_recover ON user_credentials(recover_selector) WHERE recover_selector <> '';
CREATE TABLE user_auth_identities (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider = 'google'),
    issuer TEXT NOT NULL CHECK (issuer = 'https://accounts.google.com'),
    subject TEXT NOT NULL,
    email TEXT,
    PRIMARY KEY (provider, issuer, subject),
    UNIQUE (user_id, provider)
);
CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX sessions_expiry ON sessions(expiry);
CREATE INDEX sessions_user ON sessions(user_id);
-- One-use, browser-bound Google login requests. PKCE verifier is encrypted.
CREATE TABLE auth_login_requests (
    state_hash TEXT PRIMARY KEY,
    session_hash TEXT NOT NULL,
    nonce_hash TEXT NOT NULL,
    verifier_ciphertext BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX login_requests_expiry ON auth_login_requests(expires_at);
CREATE TABLE user_movie_progress (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE RESTRICT,
    watched BOOLEAN NOT NULL DEFAULT false,
    watchlist BOOLEAN NOT NULL DEFAULT false,
    watched_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, movie_id),
    CHECK ((watched AND watched_at IS NOT NULL) OR (NOT watched AND watched_at IS NULL))
);
CREATE INDEX progress_movie ON user_movie_progress(movie_id);
CREATE INDEX progress_watched ON user_movie_progress(user_id) WHERE watched;
CREATE INDEX progress_watchlist ON user_movie_progress(user_id) WHERE watchlist;
