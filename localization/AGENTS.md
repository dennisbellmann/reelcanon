# Reel Canon translations

`de.json`, `en.json`, `es.json`, and `fr.json` are the source of truth for public
Web and Android/iOS UI copy. Edit these files, never generated native resources.

- Use stable semantic keys (`movie.markWatched`), not German text as keys.
- Keep all four catalogs complete. Keep the same named `{{placeholders}}` in every
  translation; their order may change. Use whole messages, not sentence fragments.
- Use i18next plural suffixes `_one`, `_other`, `_many` and pass `count` when
  displaying quantities. Keep the key set identical across the four catalogs.
- Match the voice: an inviting, editorial personal film album. German uses `du`,
  Spanish `tú`, French `vous`; English is direct and friendly. Preserve brand names.
  Prefer short headings, clear actions and one useful sentence over explanations.
- Everyday screens focus on films and the album. Storage, APIs, sync and implementation
  details belong in Settings. Keep destructive action consequences and recovery steps.
- Translate UI and accessibility labels. Keep route slugs, IDs, filter values,
  backup formats and persisted state independent of translated labels.
- Use `useTranslation()` in React; use `LocalizedMessage`/`LocalizedError` for stored
  notices so they change language when rendered. Do not display raw server errors.
- In Compose, use `Message.*` and `LocalStrings.current` inside the app, `useStrings()`
  at a root composition. Native resources follow the device/app language.
- Film titles, synopses, collection descriptions and provider genres are catalog
  content. Do not invent translations in these UI catalogs; see `docs/localization.md`.

After changes, run from the repository root:

```sh
node scripts/localization.mjs --write
npm --prefix frontend run format
npm --prefix frontend run i18n:check
npm --prefix frontend test
npm --prefix frontend run build
```

For native UI changes, also run `node scripts/run-mobile.mjs check` with a working
JDK/Android SDK. See `mobile/README.md`. Check narrow screens and long French/Spanish
labels. To add another language, update the Web language registry, checker locale
list, native language declarations and tests together.
