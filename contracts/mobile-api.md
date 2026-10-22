# Mobile account and progress API

Paths are relative to `/api/v1`. JSON only, private responses `no-store`.
HTTPS is mandatory except the debug emulator's `10.0.2.2` loopback bridge.
Mobile endpoints never accept browser cookies as authorization and never set
browser cookies. Web routes retain their cookie, Origin and CSRF contract.

| Method/path | Input | Result |
| --- | --- | --- |
| POST /auth/mobile/google/challenge | empty object | challengeId, nonce, serverClientId, expiresIn (300 seconds) |
| POST /auth/mobile/google | challengeId, idToken | accessToken, refreshToken, expiresIn (900), user {id,email} |
| POST /auth/mobile/refresh | refreshToken | rotated token pair and user |
| POST /auth/mobile/logout | refreshToken | ok; revoke the entire session family, including its access token |
| GET /mobile/me | Bearer accessToken | user {id,email} |
| POST /mobile/me/progress/import | Bearer; v1 guest backup | added, existing, unknown IMDb IDs; same missing-row import rules as web |
| GET /mobile/me/progress | Bearer accessToken | full progress array, including tombstones |
| PUT /mobile/me/progress/{imdbId}/{watched or watchlist} | value: boolean; Bearer, If-Match, Idempotency-Key | progress row and ETag |

Google login is enabled only when `GOOGLE_MOBILE_CLIENT_IDS` is an explicit
comma-separated allowlist of authorized-party (`azp`) client IDs. Audience is
the existing `GOOGLE_CLIENT_ID` web/server client ID, validated by the OIDC
verifier with Google's issuer, signature and expiration checks. Tokens without
`azp` are accepted only when their single audience itself is allowlisted.
The server issues a random, one-use nonce challenge, validates the signed nonce,
then consumes the challenge and creates the app session atomically. Invalid
tokens cannot consume another client's challenge. Login requires verified email
and a nonempty subject. Identity is exactly the existing web Google provider,
issuer and subject; unknown subjects create an independent account with the same
rules as web. Password accounts are never merged by email. Blocked identities
cannot recreate an account.

Opaque 256-bit access and refresh tokens are stored on the server only as SHA-256
hashes. Access expires after 15 minutes. A session family expires absolutely
after 30 days (no sliding extension). Refresh is single-use and serialized;
rotation invalidates the old access token immediately. Spent refresh hashes are
retained until family expiry: any reuse revokes the entire family and returns
401, including concurrent refresh requests. Clients serialize refresh and never
blindly retry a refresh after an ambiguous network failure; reauthentication is
required. Logout can use a spent refresh token to revoke the family. Account
block, deletion, password reset and logout-all invalidate mobile sessions through
the same server session-version check. Logout is idempotent, including unknown
tokens. Auth requests are limited to 16 KB and 30 attempts/IP/10 minutes; refresh
and challenge/login share that limit. No tokens are logged.

Progress preserves the web revision rules (428/400/412/404, independent fields,
no revision increment for a no-op). Responses also include `watchedRevision` and `watchlistRevision`, the last row
revision that changed each field; legacy rows conservatively initialize both
to their current revision. A client rebases only when its field revision has
not advanced, or acknowledges an already satisfied target. Each mobile write requires a UUID operation
ID in `Idempotency-Key`. Successful results and the request fingerprint (movie,
field, target value, expected revision) are persisted in the same transaction as
progress. Retrying the identical operation returns its original result even
after subsequent writes. A different request using that ID yields 409. 412
responses are not committed operations: a conflict decision/rebase must use a
new ID. Receipts are retained for the lifetime of the account, so offline retry
has no expiry window. Deletion cascades receipts and sessions. Account write
serialization also covers web updates/imports and mobile session revocation.

Local progress and outbox keys include the server account ID, or the distinct
`guest` owner. Existing v1 Room progress migrates to guest. Guest backups/imports
stay guest; explicit guest import into an account inserts only missing server
rows and keeps server rows/tombstones. Local writes and durable outbox creation
are atomic. Outbox entries retain stable IDs and expected revisions until an
acknowledgment is stored atomically. Full pulls preserve pending local overlays.
When a newer revision changes only another field, an operation can rebase with
a new ID. If its own field changed, present server/local choices; no device-time
last-write-wins. If the target already holds, the operation is satisfied.

Android uses Credential Manager and a Keystore AES-GCM protected atomic file in
`noBackupFilesDir`. Tokens and Google ID tokens never enter Room, backups or APK
constants. Only public client IDs are build configuration. iOS can implement the
same common platform-auth/session-store interfaces with Keychain and a provider
adapter; iOS login is not activated by this Android implementation.
