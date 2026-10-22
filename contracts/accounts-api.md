# Account API (development release)

All paths below are relative to `/api/v1` on the web origin. Credentials use the
`reelcanon_session` HttpOnly cookie. Private responses use `Cache-Control: no-store`.
User IDs always come from the server session.

## Session bootstrap and authentication

| Method/path | JSON input | Result |
| --- | --- | --- |
| GET /me | — | 200: enabled, user (profile or null), csrfToken, providers.google; local developmentMailbox |
| POST /auth/register | email, password, confirm_password | Authboss JSON status; confirmation required before login |
| POST /auth/login | email, password | Authboss JSON status; rotates session and CSRF token |
| POST /auth/confirm | cnf | Consume confirmation token (24 hours) |
| POST /auth/confirm/resend | email | Generic success; rotate pending confirmation link |
| POST /auth/recover | email | Generic success; no account enumeration in response |
| POST /auth/recover/end | token, password, confirm_password | Consume reset token (one hour), revoke sessions; manual login required |
| GET /auth/google | — | Provider redirect; 503 if unconfigured |
| GET /auth/google/callback | Google query code/state | Browser redirect to /account; errors use a generic fragment |
| POST /auth/logout | — | Destroy current session |
| POST /auth/logout-all | — | Revoke sessions by increasing session version |
| DELETE /me | — | Delete account and personal rows; requires login in last ten minutes |

All browser writes require an allowed `Origin` and `X-CSRF-Token` from GET /me.
After login/logout refresh GET /me. The frontend sends `X-Account-ID` on authenticated
requests as an additional guard against a tab switching accounts; a mismatch yields
409, and never authorizes access. Authentication POST bodies: maximum 16 KB;
rate limit: 30 auth attempts per source IP per ten minutes (in-memory per process).

Authboss can return HTTP 200 with `status: failure`, `error` or validation data.
Clients must inspect that status; HTTP success alone does not mean login success.
An anonymous GET /me is intentionally **200 with user=null**, so a guest can
bootstrap CSRF without an error. Personal endpoints return 401 without authentication.
Disabled accounts return enabled=false from bootstrap.

Confirmation/reset links point at `/account#confirm=...` or `#reset=...`.
The UI removes the fragment and submits the token only when the user confirms.
Tokens, OAuth codes, cookies and passwords are never written to application logs.

## Progress

| Method/path | Input | Result |
| --- | --- | --- |
| GET /me/progress | — | Array of imdbId, watched, watchlist, watchedAt, updatedAt, revision |
| PUT /me/progress/{imdbId}/watched | value: boolean | Updated row |
| DELETE /me/progress/{imdbId}/watched | — | Set watched=false, retain watchlist |
| PUT /me/progress/{imdbId}/watchlist | value: boolean | Updated row |
| DELETE /me/progress/{imdbId}/watchlist | — | Set watchlist=false, retain watched |
| POST /me/progress/import | reel-canon-progress/v1 backup | added, existing, unknown IMDb IDs |

Progress writes require a strong `If-Match: "N"` revision; use `"0"` for a
missing row. Missing precondition: 428. Invalid precondition: 400.
Stale revision: 412 with `current` row and ETag. Unknown catalog movie: 404.
Successful updates return an ETag. Sending the existing value with a matching
revision does not increment revision. Removing all flags keeps a server row
as a tombstone. Writes serialize per account, including imports.

Import validates the entire file before writing (5 MB, 20,000 unique movie IDs,
booleans and RFC3339 timestamps). Only missing server rows are inserted; existing
rows and tombstones win regardless of device timestamp. Unknown catalog IDs
are skipped and reported. Repeating the import adds no duplicates.
Export uses GET /me/progress and serializes the existing versioned backup format
in the frontend; there is no separate export endpoint in this release.

The first web implementation requires connectivity for account progress.
Native durable caching/outbox are specified separately in the mobile plan.
## Localized errors

Account errors include a stable `code` alongside the existing human-readable
`error` field, for example `{"code":"password_too_short","error":"..."}`.
Clients translate `code` in their active UI language and use a local generic
fallback for unknown codes. The server's `error` text is retained for existing
clients; it is not localized. Conflict responses also retain `current`.


## Native clients

Android uses the separate [mobile API contract](mobile-api.md). Web cookies and
CSRF protections remain required on browser routes. Progress responses also
include watchedRevision and watchlistRevision for safe mobile field rebasing;
the existing row revision and web conflict semantics are unchanged.
