-- Apply as migration owner after 0005. Provision LOGIN/password separately.
-- This role may write account data and read movie identity; no catalog writes.
GRANT USAGE ON SCHEMA public TO reelcanon_accounts;
GRANT SELECT ON movies TO reelcanon_accounts;
GRANT SELECT, INSERT, UPDATE, DELETE ON users, user_credentials,
    user_auth_identities, sessions, auth_login_requests, user_movie_progress TO reelcanon_accounts;
REVOKE ALL ON users, user_credentials, user_auth_identities, sessions,
    auth_login_requests, user_movie_progress FROM PUBLIC;
-- Also explicitly REVOKE these tables from your catalog API role if it has
-- default privileges granting SELECT on newly-created tables.

GRANT SELECT, INSERT, UPDATE, DELETE ON mobile_login_challenges, mobile_sessions, mobile_refresh_tokens, progress_operations TO reelcanon_accounts;
REVOKE ALL ON mobile_login_challenges, mobile_sessions, mobile_refresh_tokens, progress_operations FROM PUBLIC;
