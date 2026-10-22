-- ENTWURF, KEINE AKTIVE MIGRATION. Stand: 2026-10-06.
-- Kostenlose Bibliotheken im Go-Backend. E-Mail/Google zuerst, Apple spaeter.
-- Voraussetzung: Katalog aus Migrationen 0001 bis 0004.
-- make migrate liest database/migrations, nicht diesen Vorschlagsordner.
-- UUIDs erzeugt das Backend; keine weitere PostgreSQL-Extension erforderlich.

CREATE TABLE users (
    id UUID PRIMARY KEY,
    display_name TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    session_version BIGINT NOT NULL DEFAULT 1 CHECK (session_version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

CREATE TABLE user_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL UNIQUE CHECK (length(email) BETWEEN 3 AND 320),
    password_hash TEXT NOT NULL CHECK (length(password_hash) > 0),
    confirmed BOOLEAN NOT NULL DEFAULT false,
    confirm_selector TEXT NOT NULL DEFAULT '',
    confirm_verifier_hash TEXT NOT NULL DEFAULT '',
    confirm_expires_at TIMESTAMPTZ,
    recover_selector TEXT NOT NULL DEFAULT '',
    recover_verifier_hash TEXT NOT NULL DEFAULT '',
    recover_expires_at TIMESTAMPTZ,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_attempt_at TIMESTAMPTZ,
    locked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((confirm_selector = '' AND confirm_verifier_hash = '' AND confirm_expires_at IS NULL)
        OR (confirm_selector <> '' AND confirm_verifier_hash <> '' AND confirm_expires_at IS NOT NULL)),
    CHECK ((recover_selector = '' AND recover_verifier_hash = '' AND recover_expires_at IS NULL)
        OR (recover_selector <> '' AND recover_verifier_hash <> '' AND recover_expires_at IS NOT NULL))
);
-- Authboss-Adapter: UUID-/E-Mail-Suche, User-/Storer-Interfaces, Tokenverbrauch.
-- Hash- und Verifierformate erzeugt die Bibliothek, kein Klartext.
-- E-Mail-Normalisierung fest im Backend; kein Social-Account-Matching per E-Mail.
-- Passwortwechsel/Reset erhoehen users.session_version in derselben Transaktion.
CREATE UNIQUE INDEX user_credentials_confirm_selector_idx
    ON user_credentials(confirm_selector) WHERE confirm_selector <> '';
CREATE UNIQUE INDEX user_credentials_recover_selector_idx
    ON user_credentials(recover_selector) WHERE recover_selector <> '';

CREATE TABLE user_auth_identities (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('google', 'apple')),
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL CHECK (length(subject) > 0),
    email TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    is_private_email BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    refresh_token_ciphertext BYTEA,
    encryption_key_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, issuer, subject),
    UNIQUE (user_id, provider),
    CHECK ((provider = 'google' AND issuer = 'https://accounts.google.com')
        OR (provider = 'apple' AND issuer = 'https://appleid.apple.com')),
    CHECK ((refresh_token_ciphertext IS NULL) = (encryption_key_id IS NULL))
);
-- Google-Issuer erst nach erfolgreicher Tokenpruefung kanonisieren.
-- Apple-Refresh-Token verschluesselt fuer Widerruf; Google-Login verwahrt keinen.
-- Apple-Provider ist aktuell nicht aktiv; die Spalten bereiten ihn nur vor.

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE
);
-- token/data/expiry entsprechen dem SCS-PostgreSQL-Store.
-- Store-Huelle hasht Cookie-Tokens vor Store-Aufruf.
-- user_id nach SCS-Save fuer neue Login-Session setzen, vor Cookie-Ausgabe.
-- Anonyme Sessions: user_id NULL. Jeder Zugriff prueft Besitzer/Session-Version.
CREATE INDEX sessions_expiry_idx ON sessions(expiry);
CREATE INDEX sessions_user_id_idx ON sessions(user_id) WHERE user_id IS NOT NULL;

CREATE TABLE auth_login_requests (
    id UUID PRIMARY KEY,
    session_token TEXT NOT NULL REFERENCES sessions(token) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('google', 'apple')),
    purpose TEXT NOT NULL DEFAULT 'login' CHECK (purpose IN ('login', 'link')),
    link_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    state_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
    nonce_hash BYTEA NOT NULL CHECK (octet_length(nonce_hash) = 32),
    browser_binding_hash BYTEA NOT NULL CHECK (octet_length(browser_binding_hash) = 32),
    pkce_verifier_ciphertext BYTEA,
    encryption_key_id TEXT,
    return_path TEXT NOT NULL DEFAULT '/',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    CHECK ((purpose = 'login' AND link_user_id IS NULL)
        OR (purpose = 'link' AND link_user_id IS NOT NULL)),
    CHECK ((pkce_verifier_ciphertext IS NULL) = (encryption_key_id IS NULL)),
    CHECK (provider <> 'google' OR pkce_verifier_ciphertext IS NOT NULL),
    CHECK (expires_at > created_at),
    CHECK (expires_at <= created_at + INTERVAL '10 minutes'),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at),
    CHECK (left(return_path, 1) = '/' AND left(return_path, 2) <> '//')
);
-- Zusaetzliche Backend-Allowlist fuer return_path; kein freies Redirect.
-- Einmalverbrauch atomar per UPDATE WHERE consumed_at IS NULL AND expires_at > now()
-- AND state_hash = $state_hash AND browser_binding_hash = $binding_hash RETURNING ...
-- Bei Link: Besitzer, Gueligkeit und Session-Version der urspruenglichen Session pruefen.
-- Spaeterer Apple-POST prueft separate SameSite=None-Bindung statt Lax-App-Cookie.
CREATE INDEX auth_login_requests_expiry_idx ON auth_login_requests(expires_at);
CREATE INDEX auth_login_requests_session_idx ON auth_login_requests(session_token);
CREATE INDEX auth_login_requests_link_user_idx ON auth_login_requests(link_user_id)
    WHERE link_user_id IS NOT NULL;

CREATE TABLE user_movie_progress (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE RESTRICT,
    watched BOOLEAN NOT NULL DEFAULT false,
    watched_at TIMESTAMPTZ,
    watchlist BOOLEAN NOT NULL DEFAULT false,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, movie_id),
    CHECK ((watched AND watched_at IS NOT NULL) OR
           (NOT watched AND watched_at IS NULL))
);
-- Fortschritt gilt in allen Canons, ohne Canon-/Edition-Spalte.
-- Uncheck: watched=false, watched_at=NULL, Watchlist unveraendert.
-- Gewoehnliches Entfernen behaelt die Zeile und erhoeht revision statt DELETE.
-- Backend prueft erwartete Revision und setzt updated_at/revision atomar.
CREATE INDEX user_movie_progress_watched_idx
    ON user_movie_progress(user_id, movie_id) WHERE watched;
CREATE INDEX user_movie_progress_watchlist_idx
    ON user_movie_progress(user_id, movie_id) WHERE watchlist;
CREATE INDEX user_movie_progress_movie_idx ON user_movie_progress(movie_id);

CREATE TABLE oauth_token_revocation_jobs (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider = 'apple'),
    client_id TEXT NOT NULL,
    token_ciphertext BYTEA NOT NULL,
    encryption_key_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'manual_review')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error_code TEXT,
    CHECK ((status = 'processing' AND lease_until IS NOT NULL) OR
           (status <> 'processing' AND lease_until IS NULL))
);
-- Absichtlich KEIN Nutzer-FK: Auftrag ueberlebt Kontoloeschung.
-- Token beim Uebertragen mit Job-ID/Provider/Client-ID als Kontext neu verschluesseln.
-- Keine Tokens/Profile/Filmlisten in Logs/last_error_code.
-- Faellige Jobs atomar beanspruchen; abgelaufene Leases erneut bearbeiten.
-- Erfolgreiche Jobs entfernen; laengere Fehler alarmieren, nicht still verwerfen.
CREATE INDEX oauth_token_revocation_jobs_due_idx ON oauth_token_revocation_jobs(next_attempt_at)
    WHERE status = 'pending';
CREATE INDEX oauth_token_revocation_jobs_lease_idx ON oauth_token_revocation_jobs(lease_until)
    WHERE status = 'processing';

-- Beispiel: Fortschritt fuer den aktuell veroeffentlichten Canon.
-- SELECT count(*) AS total, count(*) FILTER (WHERE p.watched) AS watched
-- FROM canons c
-- JOIN canon_movies cm ON cm.edition_id = c.current_edition_id
-- LEFT JOIN user_movie_progress p ON p.movie_id = cm.movie_id AND p.user_id = $1
-- WHERE c.slug = $2 AND c.is_live;
