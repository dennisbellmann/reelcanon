# Reel Canon — Current MVP decisions

Date: 2026-10-05

Financing clarification, 2026-10-06: this is an ambitious hobby project, not a
full-time business. Covering running costs is the first goal; around EUR 200/month
would be a welcome later result. Short-term profitability is not required.
Voluntary support and optional future purchases, advertising or streaming Pro
are documented in the [hobby financing concept](hobby-financing-concept.md).
Previous EUR 0.99 Canon pricing remains a possibility rather than a settled
overall monetization model. No payment or advertising implementation is requested
by this clarification.

Scope update, 2026-10-06: optional Google and email/password accounts (Apple later) with
server-side progress are now requested as the next extension. The
[account concept](accounts-and-progress-concept.md) describes the proposed use of
free Go authentication libraries and a database schema. The
[mobile plan](mobile-app-implementation-plan.md) specifies a future shared Android/iOS
app, offline use and one-time paid Canon access. The first web/backend account
implementation now uses a local mail viewer; see
[account development](accounts-development.md) for its status and limits.
The local-only statements below describe the original baseline MVP.

This document records the user's current scope and the proposed implementation defaults. It supersedes conflicting launch, authentication, and persistence requirements in `reel-canon-project-start.md`.

## Confirmed scope

- First Canon: **IMDb Top 250**. The 1001 Movies Canon is a future collection.
- Build small **crawling modules per Canon**, including source fetching and parsing, with a shared import pipeline. Source-format parsers may be reused. Collection membership must remain independent of the metadata provider.
- Support dynamic Canons and configurable refresh intervals. IMDb Top 250 should later be refreshed every **1–3 months**; the exact interval remains configurable.
- No live Canon retrieval is required now. Build the module contracts and update flow first; validate parsing/import with saved fixtures. No scheduled job is activated at this stage.
- Use **TMDb** for movie metadata and posters.
- No end-user accounts, Google/Apple login, or cloud sync in the MVP. Progress must persist locally in the browser.
- The **admin application runs separately from the public application**, with its own frontend, startup/build, and access protection. Both use the same central catalog and shared backend domain/import logic.
- The admin may use a simpler technical stack than the public application. Proposed default: a standalone Go server with `html/template`, HTML forms, small JavaScript enhancements, and simple CSS; no second React/Vite application is required.
- Keep the React/Vite, React Router, Tailwind, TanStack Query, Go REST API, and PostgreSQL direction for the shared catalog.
- Keep the editorial design and decade-based sticker album. Watched state applies to a movie across all Canons.

## Reference project inspected

Local reference: `D:/Programming/projects/seryous` (SERYOUS).

Relevant files:

- `backend/internal/api/handlers/tmdb.go`: Go proxy injects the TMDb API key on the server.
- `backend/cmd/api/main.go`: environment-based configuration and API server setup.
- `backend/internal/api/handlers/search.go`: separate TMDb metadata and OMDb IMDb-rating enrichment, including a rating cache.
- `frontend/src/api/client.ts`: typed API client and German metadata language default.
- `frontend/src/lib/db.ts`: Dexie/IndexedDB separates user state from cached metadata.

Use these patterns as a reference and adapt them to movies. The new TMDb client should use request contexts, explicit HTTP timeouts, status validation, and bounded retries/concurrency. Do not copy SERYOUS's TV-specific models or import its user data.

SERYOUS has local backend credentials. A read-only TMDb lookup for IMDb ID `tt0111161` succeeded and returned movie ID `278` with a poster path. Credentials were not printed or copied into this project. The application must keep credentials in backend-only environment configuration.

## Proposed Canon crawling and import contract

Each Canon has a registered crawling module and configuration. A module owns the source-specific fetch and parse behavior. Keep fetching, parsing, enrichment, and persistence separate so updates can be tested without network access:

```text
Manual update command or future scheduled run
  -> Canon crawler (source fetcher + parser)
  -> normalized Canon snapshot
  -> validation
  -> IMDb-ID-to-TMDb resolution and metadata enrichment
  -> transactional catalog import
```

A crawler exposes its Canon identity, fetch operation, and parser. Fetching returns source bytes and retrieval metadata; parsing accepts these inputs and returns a normalized snapshot plus diagnostics. Parsing performs no database writes or metadata-provider requests. Saved files and test fixtures can be fed directly into the parser.

Proposed module organization:

```text
backend/internal/crawling/
  registry.go           # Canon-to-module registration
  runner.go             # shared update orchestration
  imdbtop250/           # source fetcher and parser
```

Per-Canon configuration contains source URL, crawler identifier, refresh policy (manual, interval, or static), refresh interval in calendar months, and enabled state. Execution state tracks the last attempt, last successful check, last published change, and next due time. A future scheduler uses this same runner; a manual CLI command can update a Canon on demand. Do not fetch sources from normal page loads or user-progress changes.

Snapshot fields:

- Canon slug, name, source URL, source format, retrieval date, and content hash.
- Edition/snapshot identifier and label.
- Entries containing IMDb ID, rank/position, source title, release year, and optional IMDb rating.
- Diagnostics for missing fields, duplicate IDs/ranks, invalid values, and unresolved references.

Implement an `ImdbTop250Crawler` with an independently testable `ImdbTop250Parser` first; provide a common JSON/CSV interchange format for saved or curated lists. A future Canon can register a different module or reuse a source parser without changing the movie catalog or progress model.

Validate exactly 250 unique IMDb IDs and positions 1–250 for a complete Top 250 snapshot. Reject incomplete or invalid snapshots before publication; retain the last valid edition. Reimporting the same snapshot must be idempotent.

IMDb rankings change. Use dated editions and preserve previous membership/rank information. A refresh must never delete a movie's local watched state. Keep metadata fetch failures visible and retryable; do not silently omit films with missing TMDb matches or posters.

Compare the validated membership and ordering against the current edition. Publish a new edition atomically only if membership or rank changes; a successful unchanged check updates execution state without creating another edition. Ratings and metadata can be refreshed separately. Raw response hashes support provenance, but changing markup alone must not create a new collection edition. Record additions, removals, and rank changes. Removed films remain in the catalog and keep local watched/watchlist state; current-edition counts and completion percentages may change after an update.

The runner prevents overlapping updates for the same Canon, supports timeouts and bounded retries, records failures, and retains the last valid edition if fetching or validation fails. TMDb enrichment uses the shared catalog/cache and fetches only missing or stale metadata.

An earlier direct IMDb chart fetch through the web tool returned HTTP 403. This is a future source-adapter concern, **not a blocker for building the module architecture now**. No additional live retrieval is requested at this stage, and no complete Top 250 dataset is present in the repository yet.

## Separate admin application and manual fallback

The user confirmed that Canon administration must run independently of the public application. Include a small separate admin application in the MVP, with scope limited to Canon management.

### Application boundary and repository structure

Keep a single monorepo, but provide independently runnable applications. Proposed implementation layout:

```text
frontend/                    # public React/Vite application
backend/
  cmd/api/                   # public catalog API process
  cmd/admin/                 # separately started protected Go admin server
  internal/                  # shared catalog, crawling, validation, import logic
    adminui/                 # server-rendered admin handlers, templates, assets
database/
  migrations/                # common PostgreSQL schema
contracts/                   # public API and normalized import definitions
```

The admin has its own entry point, routes, configuration, templates/assets, and Go build output. Serve its pages and protected actions from one Go process, using standard-library `html/template` and ordinary forms by default. Add small JavaScript enhancements for filtering, row editing, and update-status feedback only where useful. A separate admin JSON API, npm package, SPA router, and React build are not needed. Its routes can be `/canons` and `/canons/:canonId` on its own origin; the public application has no admin routes.

As an implementation default, run the public Go API and Go admin server on separate ports. The public process exposes public catalog reads; admin pages, administration actions, and crawler controls are registered only in the admin process. Share Go domain packages and the catalog database rather than duplicating models or synchronizing two databases. Use a read-only database role for the public catalog API and a separate administration role for catalog writes.

The public frontend/API and the admin server can be started and built independently. Administration can initially be available locally, with a separate private origin/deployment later. The public application must work when the admin application is stopped. Public readers see only published editions; drafts and overrides remain administration data. Publishing updates the shared catalog atomically, and the public catalog cache must revalidate against the published revision. Admin presentation is functional and compact; it does not need the public application's full editorial design system.

Canon overview:

- Name, film count, current published edition, and source.
- Crawler status, last successful check, next due time, and last error.
- Configurable refresh interval and pause/resume control.
- Manual update action using the same shared crawling runner. Crawling remains inactive until explicitly enabled in a later implementation step.

Canon editor:

- Searchable film list showing rank, title, year, IMDb ID, TMDb ID, and origin (crawler/manual).
- Add a film by stable IMDb ID or a TMDb search/selection resolved to an IMDb ID; unresolved entries remain drafts until their identity is resolved.
- Remove a film from the Canon, change its position, and correct its TMDb mapping without deleting the shared movie record or local user progress.
- Import/export normalized JSON/CSV as a fallback.
- Show validation diagnostics and a preview of additions, removals, and rank changes before publication.
- Keep draft edits separate from the published edition. Saving a draft does not change the public collection. Publishing creates a dated edition through the shared validation/import pipeline.

Incomplete drafts may be saved. Published IMDb Top 250 editions must satisfy the 250-film and unique-position rules; other Canons use their own validation rules.

### Manual changes and crawler updates

Retain the raw crawler snapshot separately from manual overrides (include, exclude, position, or mapping correction). Store the base edition and the reason/origin for each manual edit. A later crawler run must not silently overwrite those edits.

For Canons without manual overrides, valid crawler updates can follow the configured automatic publication policy. For Canons with overrides, prepare an update candidate, carry forward applicable overrides, and show source changes and conflicts in the admin area for review. Leave the published edition unchanged until the candidate is published. An administrator can retain or explicitly clear an override to return to source data.

Use an edition/revision check when saving or publishing so a stale editor cannot overwrite a newer update. Record publication history and allow restoring an earlier snapshot as a new edition. Canon-specific overrides affect only that Canon; shared movie identity/mapping corrections require an explicit catalog action and must preserve IMDb-based local progress keys.

### Admin access

The public MVP remains usable without accounts. Admin pages and read/write actions require separate server-side authorization before the area is accessible outside local development. Protect form mutations against CSRF and keep them off GET routes. Keep admin credentials on the server; no admin secret belongs in browser-delivered code or templates. An end-user OAuth system is not required for this small administration scope. Admin sessions and allowed origins belong only to the admin server.

## Metadata and ratings

Resolve imported IMDb IDs using TMDb's external-ID lookup, then retrieve movie details and credits for title, overview, year, runtime, genres, countries, and directors. Keep IMDb ID and TMDb ID separately. Poster URLs combine TMDb image configuration, image size, and poster path; provide a missing-poster fallback.

TMDb reference documentation:

- [Finding data by external ID](https://developer.themoviedb.org/docs/finding-data)
- [Image URL construction](https://developer.themoviedb.org/docs/image-basics)

Keep IMDb ratings distinct from TMDb vote averages. Prefer the IMDb rating supplied by a validated source snapshot; leave it nullable if missing. SERYOUS's OMDb integration is an optional future enrichment path and is not a required MVP dependency. Record rating source and retrieval time. Missing ratings must not hide films.

## Proposed local progress storage

Use **Dexie/IndexedDB**, following the SERYOUS pattern, rather than storing the movie collection in cookies.

- Store watched state, watched timestamp, optional watchlist state, and update timestamp by stable movie identity. Use IMDb ID as the local MVP progress key, independent of edition, rank, and database row allocation.
- Compute Canon and decade progress from membership plus local movie state.
- Use a separate metadata cache; clearing/refetching metadata must preserve user progress.
- For the initial 250-film catalog, combine API-provided catalog data with local state before applying watched/watchlist filters and pagination. Do not filter only an already paginated subset.
- Show that progress is saved in this browser. It is not synchronized to other devices and clearing browser storage removes it.
- Provide versioned JSON export/import as a small backup feature, validating imports before applying them.
- Put persistence behind a repository interface so account-backed synchronization can be added later.

No cookie-based anonymous session or server-side user-progress endpoints are required for this scope. Cookies can be reconsidered if a later feature needs them.

## Implementation defaults

- Include a simple watchlist; defer personal ratings and notes.
- Defer payments, Pro subscriptions, streaming, and advanced achievements.
- Define the canonical catalog schema and first REST API contract before feature implementation.
- Build the small separate admin application for Canon lists and draft/manual corrections; avoid expanding it into general user or subscription administration.
- Make the first vertical slice: browse IMDb Top 250, open a movie, mark watched, return to the album, see the poster and progress update, reload and retain state.
- Verify crawler parsing with fixtures, importer validation/idempotency, change detection and retention of the last valid edition on failures, manual-override retention/conflict handling, draft/publish behavior, server-side admin authorization, absence of admin endpoints from the public API, public app operation with administration stopped, persistent watched state across reloads and Canon editions, metadata matching, and responsive album behavior.

## Start readiness

Historical note: the readiness description below records the initial state. For the current implemented and tested state, see [MVP audit](mvp-audit.md) and the repository README.

Product decisions are sufficient to begin the schema, API contract, scaffolding, and first vertical slice.

Git, Node.js, npm, and Go are available. The project contains no application scaffold or initialized Git repository. Docker and PostgreSQL tools are not available in PATH; the shared catalog's local PostgreSQL setup remains an implementation prerequisite. Crawling contracts, the module registry, update orchestration, and fixture-backed tests can be built without retrieving a live Canon. Real dataset loading and scheduled execution come later. TMDb access was successfully checked using the reference project's configuration.
