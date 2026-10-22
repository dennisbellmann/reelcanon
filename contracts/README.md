# Catalog contracts

`canon-snapshot/v1` is the interchange format used by canonical import and future source-specific crawler modules. It records IMDb identity, source ranking, and source metadata separately from TMDb catalog metadata.

The registered `imdb-top-250` and `1001-movies` rules require exactly 250 or 1,001 unique IMDb IDs respectively, with every position present exactly once. Each source adapter can reuse the normalized format and shared catalog importer; its slug determines which validation rule applies. Importing an unchanged ordered membership is idempotent and does not create another published edition.

The public read API is documented in [openapi.yaml](openapi.yaml). Admin routes are served by a separate Go process and are intentionally absent from the public API contract.

The optional account extension has its current authenticated HTTP contract in
[accounts-api.md](accounts-api.md). Catalog reads remain public; personal state
is served through a separate writable database connection.
