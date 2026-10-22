# Reel Canon

An editorial film collection tracker. The first Canon is IMDb Top 250. Film membership and movie metadata live in a shared PostgreSQL catalog. Guests keep progress in their browser; optional accounts save watched/watchlist state on the server.

Reel Canon is an ambitious hobby project. Financing should first cover running
costs; around EUR 200/month would be a welcome later result, with no short-term
profitability requirement. Voluntary support, optional ads/one-time ad removal,
and a future streaming Pro tier remain options, not enabled features or settled
pricing. See the [hobby financing concept](docs/hobby-financing-concept.md).
The [support implementation plan](docs/support-implementation-plan.md) recommends
Ko-fi Free for the website first and optional native store tips later, with
separate treatment of external links in Google Play and App Store builds.

## Monorepo layout

```text
frontend/                 Public React + Vite application
mobile/                   Kotlin/Compose Multiplatform app; Android tested on Windows
backend/cmd/api/          Catalog API and optional account endpoints
backend/cmd/mailbox/      Loopback-only development mail viewer
backend/cmd/admin/        Independently started protected admin process
backend/cmd/import-canon/ Validated snapshot import command
backend/internal/         Shared domain, catalog, crawling and import logic
database/migrations/      PostgreSQL schema migrations
contracts/                Public OpenAPI and normalized Canon snapshot format
scripts/                  Cross-platform repository tooling
```

The public API and admin app share domain code and a database, but have separate entry points, listeners, and routes. Public catalog reads expose published Canons. Account endpoints use a separate writable connection with restricted permissions. Guest progress is keyed by IMDb ID in IndexedDB; account progress references movie identity, so editions can change without losing watched state.

PostgreSQL is the canonical store for the published catalog. Imports arrive as `canon-snapshot/v1` JSON, but each film is upserted into the shared `movies` table with a unique IMDb ID; editions connect to those records through `canon_movies`, which stores list membership and rank. A film shared by multiple lists therefore has one catalog record. The `canon_source_snapshots` and draft payloads retain immutable JSON for source provenance, review and reproducible imports; they are not separate live movie records. The API serializes the current database state to JSON on request (`GET /api/v1/canons` and `GET /api/v1/canons/{slug}`); the server does not need a generated catalog JSON file. JSON snapshots remain useful as import/export artifacts.

## Local setup

Requirements: Go 1.26+, Node.js 24+, npm, GNU Make, and PostgreSQL 15+.

1. Create a PostgreSQL database and migration role. For deployment, configure `API_DATABASE_URL` with a read-only role and `ADMIN_DATABASE_URL` with a separate admin role; `DATABASE_URL` is the local development, migration, and import connection.
2. Copy `.env.example` to `.env`; set `DATABASE_URL` and replace the example admin password. Keep `.env` out of version control.
3. Run `make install` and `make migrate`.
4. Start the API, public frontend, and admin as separate processes:

   ```sh
   make api
   make frontend-dev
   make admin
   ```

   The public app defaults to `http://localhost:5173`, the API to `http://localhost:8080`, and the admin to `http://localhost:8081/canons`. The admin uses HTTP Basic Authentication and should only be exposed over HTTPS or on a trusted local/private network.

The Canon Cockpit creates collections from JSON files or registered source modules. Source entries may contain only a film title and optional year: with TMDb configured, the Cockpit resolves those to IMDb/TMDb identities before saving the import as a hidden draft. Automatic lookup accepts only one exact title/year match; ambiguous titles show candidate TMDb IDs, which can be entered in the editor or CSV for a manual choice. Per-film TMDb IDs can also correct a mistaken selection. Each import starts as a hidden draft. You can edit its name, description, display order and poster before publishing a restorable edition and switching it live. Registered modules read only their operator-configured file or HTTPS endpoint after an explicit Cockpit action; source updates also stop at a review draft. Scheduled fetching and automatic publication remain disabled. See the reproducible [Halloween and Oscar curation workflow](docs/collection-curation-workflow.md).

Web and mobile read published collection presentation from the API; a **PosterBadge** shows watched/total and poster color grows from bottom to top. JSON/CSV import/export, change preview, publication history/restore, manual override review, metadata retry/mapping correction and refresh configuration remain available.

To register an additional source module, add its `key`, `name`, `parser` (`snapshot`, `imdb-top-250` or `1001-movies`) and exactly one trusted relative JSON `file` or fixed HTTPS `url` to [`data/crawler-modules.json`](data/crawler-modules.json). The module sends its normalized snapshot to the same draft-and-review path. Follow the [snapshot contract](contracts/canon-snapshot.schema.json). For direct import:

```sh
make import-canon FILE=path/to/imdb-top-250.json
```

The format is documented in [`contracts/canon-snapshot.schema.json`](contracts/canon-snapshot.schema.json). The Cockpit resolves optional source identities; publication and direct catalog import still require unique IMDb IDs. IMDb Top 250 requires exactly 250 unique IMDb IDs and ranks 1–250; `1001-movies` requires 1,001. Unchanged source membership/rank refreshes supplied source metadata without another edition. Manual publication always creates a restorable edition, including presentation-only changes. Changed membership creates an immutable edition transactionally. With active manual overrides, source imports retain the raw source and prepare a review candidate while preserving the public edition. Existing drafts are retained. Review and publish in the admin; reset individual overrides to the source through a new draft. Stale draft/base revisions are rejected. Failed checks retain the last valid edition and record an error. The 1001 module shares the interchange pipeline; its live source adapter remains deferred.

## First complete catalog and TMDb

The repository includes [`data/imdb-top-250-2025-08-20.json`](data/imdb-top-250-2025-08-20.json), a **historical archive** with 250 films. Its provenance and source hash are described in [`data/README.md`](data/README.md). It is not the live October 2026 ranking, and the public description identifies the archive date. Import it with `make import-canon FILE=data/imdb-top-250-2025-08-20.json`.

Set backend-only `TMDB_API_KEY` or `TMDB_READ_ACCESS_TOKEN`. Publishing a collection now starts TMDb enrichment in the background; missing matches remain visible and can be retried in the Cockpit. To rerun it manually, use `make enrich CANON=sci-fi-top-150` (the default Canon is `imdb-top-250`). TMDb lookup verifies IMDb identity, loads German details/credits and image configuration, uses four workers, explicit timeouts and bounded retries, and caches successful metadata for 30 days. Films with missing matches or posters remain present, with retryable errors in the admin. IMDb ratings come from the source snapshot and are kept separate from TMDb vote averages. Mapping corrections are explicit shared-catalog actions with an audit reason. No API credential is delivered to the public browser.

After import/enrichment, enable the Canon at `http://localhost:8081/canons`. The public app provides title/original-title/director search; watched, unwatched, watchlist, decade, year, genre, director and minimum-rating filters; sorting; film details and shared Canon memberships; and a decade sticker album with progress, buttons, keyboard and touch navigation. Missing posters have a fallback. Watched/watchlist state is stored transactionally in IndexedDB by IMDb ID and observed across tabs. `/backup` exports/imports validated versioned JSON, merging newer entries without replacing unrelated progress.

## Production and local Windows startup

`make build` writes independent executables into `backend/bin/` and the frontend into `frontend/dist/`. Set `FRONTEND_DIST=../frontend/dist` when starting the API from `backend/` to serve the SPA and API together at port 8080, including direct film links and `/backup`. The separate admin is independently runnable on 8081. API catalog responses revalidate; mounted catalog pages refresh every minute. Stopping the admin does not stop the public app.

For this Windows workspace, PostgreSQL 17.11 is available in `%LOCALAPPDATA%/ReelCanon/postgres`, bound to `127.0.0.1:54329`. `.env` contains the local database connection, a dedicated read-only API role, and generated admin credentials and is ignored by Git. Use:

```sh
make            # same as make start: build and start all local services
make stop       # stop website/API, cockpit and development mailbox
make restart    # stop, rebuild and start again
```

If all services are already running, `make` / `make start` checks their readiness and prints the links again without rebuilding or restarting. Use `make restart` to apply code changes.

These Make targets use Windows PowerShell. Start builds the current backend and frontend, starts the existing portable PostgreSQL instance, applies migrations, and runs the public app, admin and development mailbox in hidden processes. Once the API, website, protected cockpit and mailbox respond, it prints the website and cockpit links (normally `http://127.0.0.1:8080/canons` and `http://127.0.0.1:8081/canons`) and the mailbox link. Cockpit credentials remain in `.env`. Logs and tracked process IDs are in `.local/`; a failed startup stops the application processes it started. `make stop` leaves PostgreSQL running and preserves its data. Start with `make install` once if dependencies are missing. The underlying scripts can also be called directly: `./scripts/start-local.ps1 -Rebuild` and `./scripts/stop-local.ps1`.

For a temporary home-network preview, set `API_ADDR=0.0.0.0:8080` in the ignored `.env` and reserve this PC's LAN address in the router. `make restart` then prints a link such as `http://192.168.178.82:8080/canons`; the API is on the same origin at `/api/v1`. Allow inbound TCP port 8080 only for the Windows Private profile and local subnet. The development account service still requires a loopback origin, so this HTTP address is for viewing the site; Google login and account database progress require HTTPS on a Google-compatible domain.

The startup commands do not download software or load movie data automatically. For another machine, provision PostgreSQL yourself or use `docker compose up -d`; `compose.yaml` exposes it only on loopback and matches `.env.example`. Docker is optional. On other platforms, use the individual `make api`, `make frontend-dev` and `make admin` commands described above.

For deployment, use separate read-only public and writable admin database roles, configure the chosen database URLs, and place the admin behind HTTPS on its own private origin. No scheduled crawler is enabled by these startup commands.

## Accounts without an external mail server

The first account implementation is described in
[accounts-development.md](docs/accounts-development.md).
`make accounts-local` applies migrations, provisions a restricted local account
database role and enables the development configuration in the ignored `.env`.
`make dev-mail` starts the local mail viewer on port 8025. Registration,
confirmation, login, password reset and server progress are available at
`/account`; no actual email is sent. Google is prepared but stays hidden until
OAuth credentials are configured. The file mail transport requires a loopback
origin; production SMTP is a subsequent step.

## Verification

Public Web and mobile UI support German, English, Spanish and French. Copy is
managed centrally in [`localization/`](localization/AGENTS.md), with a shared
voice, checked message keys and generated native resources. See the
[localization guide](docs/localization.md) for scope and maintenance commands.

`make ci` checks formatting, Go vet, TypeScript, Go/frontend tests, and builds all commands and the production SPA. PostgreSQL integration tests require `TEST_DATABASE_URL`; its database name must end with `_test`. Tests use isolated schemas and clean them afterward. Without this variable, integration tests explicitly skip; they do not claim database verification.

```powershell
$env:TEST_DATABASE_URL='postgres://reelcanon@127.0.0.1:54329/reelcanon_test?sslmode=disable'
make ci
# Once per machine:
cd frontend
npx playwright install chromium
cd ..
make test-e2e
```

The E2E harness builds the production app, creates an isolated schema in the test database, seeds clearly named fixture films, and starts separate API/admin processes on ports 8090/8091 with temporary admin credentials. It runs desktop and mobile Chromium flows and then stops the processes and removes only its own test schema. It never writes to the normal catalog. GitHub Actions provisions PostgreSQL and runs both `make ci` and `make test-e2e`.

## Make targets

Run `make help` for the main commands. `make ci` is the single CI entry point: it checks Go and frontend formatting, runs `go vet`, TypeScript checks, Go and frontend tests, then builds the backend commands and frontend production bundle. GitHub Actions calls this same target.

## MVP status

### Account extension (2026-10-06)

Email/password login and server-side watched/watchlist state are implemented for
local development using free Go libraries and a local mail viewer. Google login
is prepared and requires OAuth credentials; Apple follows later. See
[development setup](docs/accounts-development.md) and the
[account concept](docs/accounts-and-progress-concept.md). The active schema is
in migration 0005; the [SQL draft](database/proposals/accounts-and-progress.sql)
also describes future linking and Apple lifecycle work. The [mobile implementation plan](docs/mobile-app-implementation-plan.md)
covers shared Android/iOS code with Kotlin/Compose Multiplatform, offline data and
image downloads, and one-time list purchases for a target EUR price of 0.99.
The first mobile implementation now lives in [`mobile/`](mobile/README.md): native
Android, shared Compose screens, local Room progress, offline metadata/posters,
and web-compatible JSON backups. Android build/emulator instructions are documented
there; an iOS entry point is prepared but remains untested on Windows. Mobile
account integration, cloud sync and real store billing are still planned.

The [Canon Cockpit concept](docs/canon-cockpit-concept.md) proposes extending the
existing separate admin with guided list creation, protected internal previews,
revision-bound publication and central price profiles with per-store availability.
It is planning only; no preview access or pricing has been activated.

The original local MVP is implemented and tested: full historical Top 250, TMDb metadata/posters, browsing/search/filter/sort, film details, local watched/watchlist state, backup, decade album, protected separate admin and safe publication/import/update flow. See [`docs/mvp-audit.md`](docs/mvp-audit.md) for its verification evidence. The new account extension has been compiled/built; its complete auth flows have not yet been tested. Production SMTP, streaming, payments, 1001 Movies and live/scheduled crawling remain future work.
