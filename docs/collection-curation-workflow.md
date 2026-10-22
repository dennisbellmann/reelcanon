# Collection creation: reproducible workflow

The initial Cockpit use cases are **Top 100 Halloween Movies** and **Oscar · Bester Film**. The additional lists are **1001 Filme · Ausgabe 2024**, **Sight & Sound · Greatest Films 2022**, and **Sci-Fi · 150 Essential Films**. All output files are ordinary `canon-snapshot/v1` snapshots with sequential positions, source references, capture dates, content hashes and poster presentation. Each list has reviewed IMDb identities in its source lock.

## Inputs and decisions

Halloween uses ranks 1–100 from Rotten Tomatoes’ current horror guide as captured on 6 October 2026. The source uses both critic and audience signals, so the collection is described as a Halloween watchlist, not as an IMDb score ranking. The ranked sequence stays intact. The corresponding Rotten Tomatoes title and year are matched to an IMDb movie identity; ambiguous matches require an explicit, documented entry in `data/sources/identity-aliases.json`. *The Cabinet of Dr. Caligari* is listed as 1919 by Rotten Tomatoes but its selected IMDb film identifies as 1920; *We Are Still Here* is disambiguated to the 2015 horror film. The 2026 annual update includes unreleased-at-the-cutoff selections; the source rank snapshot makes that visible and repeatable.

The Oscar collection reads the Academy’s ceremony archive year by year, takes only the winner of the best-picture category (including its historical names), excludes nominees and retains Academy order. As of 6 October 2026, the archive has 98 winners from the first ceremony through the 98th Academy Awards. The award year and film release year are separate data: a 2022 winner can be a 2021 release.

The **1001 Filme** collection follows the 15th German-language edition released in November 2024. The German Wikipedia article’s table is captured in chronological order and currently contains 1,031 individual film records. That source count is retained rather than pruning records to match the book’s marketed number.

The **Sci-Fi** collection takes all 150 positions from a September 26, 2020 Internet Archive capture of Rotten Tomatoes’ 150 Essential Sci-Fi Movies guide. It is an editorially selected essentials list, not a score-only ranking; the source introduction says each film had at least 20 reviews. Its source order is retained. The current page now contains an empty position, so the complete archived edition is used for this snapshot.

The **Sight & Sound** collection uses the BFI’s 2022 Critics’ Poll page, not the separate Directors’ Poll. It contains 264 films through official poll rank 250 because tied placements are included. Original BFI ranks are preserved in the source lock; the snapshot uses sequential positions in BFI order, with equal ranks left in the source’s display order.

## Refresh source research

Run from the repository root only when intentionally updating the source lock files. The refresh scripts read Rotten Tomatoes, the Academy archive, the German Wikipedia 2024 list, the BFI Critics’ Poll, and the archived Sci-Fi guide. Identity research checks IMDb title suggestions, follows linked German Wikipedia pages to Wikidata’s IMDb property, and cross-checks localized titles and years with TMDb. Any remaining mismatch must have an explicit, evidenced alias. Dates are explicit; downloaded responses are SHA-256 hashed and cached.

```sh
node scripts/refresh-usecase-sources.mjs --date=2026-10-06
node scripts/refresh-additional-lists.mjs --date=2026-10-09
node scripts/refresh-scifi-top-150.mjs --date=2026-10-09
node scripts/resolve-additional-list-identities.mjs --date=2026-10-09
node scripts/build-usecase-lists.mjs
node scripts/build-usecase-lists.mjs --check
```

The locks preserve each source order, title, release year and evidence URL; the original collections also preserve reviewed IMDb identities. The offline builder validates counts, sequential positions and source fields before writing snapshots. A repeat build reads only the checked-in locks and produces byte-identical JSON. Online refresh and offline build remain separate so an upstream list change cannot silently rewrite a collection.

The additional-list identity resolver needs a configured `TMDB_API_KEY` in the repository `.env` for its localized title/year fallback. IMDb suggestion responses, Wikipedia page identities and Wikidata property results are cached under `.local/usecase-research/` and hashed into the source locks.

## Create collections in the Cockpit

1. Open `/canons` and sign into the Cockpit. Select the Halloween JSON snapshot at `data/halloween-top-100.json`. The Cockpit verifies all 100 unique identities before it creates the draft.
2. Open `data/crawler-modules.json`’s **Oscar · alle Gewinner Bester Film (1929–2026)** module and choose it under **Aus Quellenmodul**. This first integration uses the checked-in `data/oscar-best-picture.json` snapshot as a file-backed module, then records a manual source run. The runner also accepts configured HTTPS JSON endpoints; redirects and user-supplied URLs are rejected. Both flows finish in a draft; neither makes a collection public.
3. Create drafts from `data/1001-movies-2024.json`, `data/sight-and-sound-greatest-films-2022.json`, and `data/sci-fi-top-150.json` using the JSON upload. Their snapshots already contain all individually reviewed IMDb IDs, so this import does not depend on a title lookup at import time.
4. In each editor, check the full count and order, and set the intended collection name, description, poster and display position. Each poster embeds its collection title, so leave **Titel ist bereits im Poster enthalten** enabled. The Halloween poster goes in position 2, Oscar in position 3, 1001 Filme in position 4, Sight & Sound in position 5 and Sci-Fi in position 6.
5. Save each presentation with a reason, confirm that validation succeeds and that the preview has the first and last positions, then publish. A successful publish automatically starts TMDb enrichment in the background when backend credentials are configured; unresolved matches remain visible in the editor for retry or correction. Public visibility is an independent switch on `/canons`; switch it on only after that review.
6. The public list card discovers each collection from the API and shows the watched count, such as `50/100`, `50/98`, `50/1031`, `50/264` or `50/150`, with progress color rising from the bottom of the cover.

The server config comes from `CANON_CRAWLER_MODULES_FILE` (default `../data/crawler-modules.json` for the admin launched from `backend/`). Module keys must be unique, URLs must use HTTPS, and redirects are rejected. Operators can add an adapter in this manifest without putting source URLs or credentials in the public app. A new source format needs an explicit parser in `backend/internal/crawling/sources`.

## Source records

- Halloween ranking: [Rotten Tomatoes, 200 Best Horror Movies of All Time](https://editorial.rottentomatoes.com/guide/best-horror-movies-of-all-time/).
- Award definition and current winner: [Academy Awards database](https://www.oscars.org/oscars/ceremonies/2026), including the [98th ceremony in 2026](https://www.oscars.org/oscars/ceremonies/2026).
- 1001 Filme: [German Wikipedia article, list as of November 2024](https://de.wikipedia.org/wiki/1001_Movies_You_Must_See_Before_You_Die).
- Sight & Sound: [BFI Greatest Films of All Time, 2022 Critics’ Poll](https://www.bfi.org.uk/sight-and-sound/greatest-films-all-time).
- Sci-Fi: [Rotten Tomatoes, 150 Essential Sci-Fi Movies (2020 archived edition)](https://web.archive.org/web/20200926195854/https://editorial.rottentomatoes.com/guide/best-sci-fi-movies-of-all-time/); [current Rotten Tomatoes guide](https://editorial.rottentomatoes.com/guide/best-sci-fi-movies-of-all-time/).
- IMDb title identity for examples that needed review: [Caligari](https://www.imdb.com/title/tt0010323/), [The Cat and the Canary](https://www.imdb.com/title/tt0017739/), [The Phantom of the Opera](https://www.imdb.com/title/tt0016220/), [Marty](https://www.imdb.com/title/tt0048356/), [CODA](https://www.imdb.com/title/tt10366460/).
