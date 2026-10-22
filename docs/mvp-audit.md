# MVP verification — 2026-10-05

Scope: `docs/mvp-decisions.md` takes precedence over the broader original vision. Accounts, cloud sync, payments, streaming, the 1001 Movies collection, and scheduled/live crawling remain deferred.

Initial baseline: `make ci` passed, but only three snapshot-validation tests and two local-progress tests existed. No local PostgreSQL instance or complete catalog was configured.

## Verified MVP

- [x] Real PostgreSQL setup and isolated integration tests.
- [x] Separate protected admin: drafts, editing, JSON/CSV import/export, diagnostics, change preview, publication, revision checks, history/restore, retained/resettable manual overrides, refresh configuration/status.
- [x] TMDb matching, details/credits/posters, bounded retries, cached enrichment and visible retryable failures.
- [x] Complete-catalog search/filter/sort in an accessible bottom sheet; a separate cover-only poster gallery with decade progress, keyboard and touch navigation.
- [x] Collection provenance opens from the collection heading in a bottom sheet. Watched progress uses “gesehen” throughout.
- [x] Collection cards use portrait artwork with the collection name integrated into the image. Cards stay grayscale until every film in the current published edition is watched, then turn colorful; completion persists after reload.
- [x] Detail return restores the previous scroll position, query filters and view, including browser Back, detail reload and repeated in-page returns.
- [x] Local watched/watchlist persistence keyed only by the stable IMDb ID, shared across all collections (including collections added later); transactional writes, cross-tab observation, visible backup import/export, validated versioned backups, and storage error feedback.
- [x] Production serving and direct route loading, catalog revalidation, separate API/admin processes.
- [x] Desktop/mobile browser verification and repeatable E2E harness.
- [x] Setup/start/stop instructions and dataset provenance.

## Evidence

- `make ci`: Go formatting/vet, TypeScript, 22 top-level Go tests (including actual PostgreSQL transactions), 10 frontend tests, separate backend executable builds and production frontend build passed.
- `make test-e2e`: 18 Chromium E2E tests passed, nine desktop and nine mobile. They verify discovery/detail navigation, watched/watchlist persistence after reload, the visible backup import button and export/import/rejection, combined filters/search, decade navigation, 320px layout, bottom-sheet focus/closing/scrolling, provenance, the cover-only poster gallery, scroll restoration across detail navigation/reload, cross-Canon/cross-edition progress, collection artwork completion/color changes after reload, authentication, draft privacy, stale edits, publication and public/admin route separation.
- Test isolation: a dedicated database whose name ends in `_test`, disposable per-test schemas and a separate E2E schema; test servers use 8090/8091. The normal catalog is not used by automated E2E mutations. Test schemas and processes are cleaned afterward.
- Local real catalog: 250 films, 250 verified TMDb matches, 250 posters, 250 runtimes, 250 director lists, no enrichment errors.
- Manual independence check: stopped the admin process; the public API still returned 250 films, and a direct film URL returned HTTP 200. Restarted both applications using the documented scripts.
- Public process uses a dedicated PostgreSQL login with SELECT grants and `default_transaction_read_only=on`. Admin credentials are generated locally and stay in ignored `.env`.
- Browser console check found no warnings/errors. Mobile Lighthouse initially identified insufficient muted-text contrast and an HTML fallback for `robots.txt`; both were corrected.
- Final mobile Lighthouse: accessibility **100**, best practices **100**, SEO **100**. The remaining agentic-browsing finding concerns an optional `llms.txt` file and is outside the product MVP. Reports are retained locally in `.local/lighthouse/`.

## Data and deliberate limits

The first complete dataset is a **historical IMDb Top 250 archive from 20 August 2025**. Its source URL, timestamp and SHA-256 are saved and its date is visible publicly. It is not advertised as the current October 2026 chart. A direct IMDb request yielded no usable list; an inspected March 2026 archive held only 25 films and was rejected. The accepted full saved archive and normalization are documented in `data/README.md`. TMDb supplied current movie metadata independently of collection membership.

No scheduled crawler or live source adapter is activated. The shared registry, fixture-tested parser/runner, import/change detection, process-safe update lock and recorded run status are ready for a later adapter. Metadata enrichment requires backend TMDb credentials; the one-time local enrichment used the existing reference configuration without copying its secret into the repository. For future refreshes, configure your own backend environment variables.

The agreed MVP excludes end-user accounts/cloud sync, payments, streaming, advanced achievements and the 1001 Movies collection. Progress is browser-local and can be lost if browser storage is cleared; the UI explains this and provides backups. Deployment to a public host was not requested; the tested applications run locally.
