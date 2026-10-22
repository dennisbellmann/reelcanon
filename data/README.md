# Bootstrap catalog

`imdb-top-250-2025-08-20.json` is a normalized **historical** IMDb Top 250 snapshot, not the live October 2026 ranking. It contains 250 unique IMDb IDs, ranks 1–250, titles and IMDb ratings.

Source: [saved archive](https://github.com/sharmadhiraj/free-json-datasets/blob/master/docs/sports-entertainment/imdb_top_movies.json), whose `updated_at` is August 20, 2025 09:42 AM UTC. Retrieved for Reel Canon on October 5, 2026. The raw-source SHA-256 is stored in the normalized snapshot. Descriptions and poster images from that source are not copied. TMDb supplies metadata and posters separately.

The direct IMDb chart request did not return usable data. A March 2026 archive inspected during verification contained only 25 films and was rejected as an incomplete source. No ranking is reconstructed from TMDb vote averages.

To recreate normalization from a separately downloaded source, run from `backend/`:

```sh
go run ./cmd/normalize-canon -file path/to/saved-archive.json -out ../data/imdb-top-250-2025-08-20.json
```

Import this snapshot, load TMDb metadata, then enable public visibility in the separate admin. A later complete, validated snapshot can replace the current edition through the same pipeline.

## Curated collection examples

halloween-top-100.json and oscar-best-picture.json are normalized, versioned use cases. Their source locks, identity evidence, refresh command and byte-for-byte offline rebuild are documented in [docs/collection-curation-workflow.md](../docs/collection-curation-workflow.md). Both list posters are original SVG artwork under [frontend/public/artwork/](../frontend/public/artwork/).

## Additional film-list sources

- `1001-movies-2024.json` captures the 15th German-language edition as listed in the November 2024 table on the German Wikipedia article. The source table has 1,031 individual film records. The source lock preserves chronological order, title, year and page link, plus individually reviewed IMDb identities.
- `sight-and-sound-greatest-films-2022.json` captures the BFI/Sight and Sound 2022 Critics’ Poll. The official list has 264 films through poll rank 250 because tied placements are included. Original BFI ranks are retained in the source lock; snapshot positions follow the BFI order from highest to lowest. IMDb identities are individually reviewed in the lock.
- `sci-fi-top-150.json` captures all 150 places in the complete 2020 archive of Rotten Tomatoes’ 150 Essential Sci-Fi Movies guide. Its introduction says each editorial selection had at least 20 reviews; the source order is retained.

Refresh these web snapshots only when intentionally updating the source files, then build the importable snapshots offline:

```sh
node scripts/refresh-additional-lists.mjs --date=2026-10-09
node scripts/refresh-scifi-top-150.mjs --date=2026-10-09
node scripts/resolve-additional-list-identities.mjs --date=2026-10-09
node scripts/build-usecase-lists.mjs
node scripts/build-usecase-lists.mjs --check
```
