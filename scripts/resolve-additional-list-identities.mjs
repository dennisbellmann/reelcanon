// Resolve title/year records using exact IMDb suggestions, source-linked
// Wikidata IMDb identifiers, localized TMDb matches, and reviewed aliases.
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const root = fileURLToPath(new URL('../', import.meta.url));
const cutoff = process.argv.find(value => value.startsWith('--date='))?.slice(7);
if (!/^\d{4}-\d{2}-\d{2}$/.test(cutoff ?? '')) throw Error('Supply --date=YYYY-MM-DD explicitly.');
const cache = path.join(root, '.local', 'usecase-research', cutoff);
await fs.mkdir(cache, { recursive: true });
const hash = value => createHash('sha256').update(value).digest('hex');
const normalize = value => value.normalize('NFKD').replace(/\p{Diacritic}/gu, '').toLowerCase().replace(/[^a-z0-9]/g, '');
const aliases = JSON.parse(await fs.readFile(path.join(root, 'data', 'sources', 'identity-aliases.json'), 'utf8'));
const env = Object.fromEntries((await fs.readFile(path.join(root, '.env'), 'utf8')).split(/\r?\n/)
  .map(line => line.match(/^\s*([^#=\s]+)\s*=\s*(.*?)\s*$/)).filter(Boolean)
  .map(([, key, value]) => [key, value.replace(/^['"]|['"]$/g, '')]));
const tmdbKey = env.TMDB_API_KEY;

async function tmdbRequest(url) {
  const file = path.join(cache, `tmdb-${hash(url)}.txt`);
  try { const cached = await fs.readFile(file, 'utf8'); if (cached.trim()) return cached; }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
  for (let attempt = 0; attempt < 5; attempt++) {
    const response = await fetch(url, { signal: AbortSignal.timeout(30000), headers: { 'User-Agent': 'ReelCanon-SourceResearch/1.0' } });
    if (response.status === 429 || response.status >= 500) {
      const retryAfter = Number(response.headers.get('Retry-After'));
      const delay = Number.isFinite(retryAfter) && retryAfter > 0 ? Math.min(retryAfter, 30) * 1000 : Math.min(2000 * (2 ** attempt), 30000);
      if (attempt === 4) throw Error(`TMDb lookup returned HTTP ${response.status} after retries`);
      await new Promise(resolve => setTimeout(resolve, delay));
      continue;
    }
    if (!response.ok) throw Error(`TMDb lookup returned HTTP ${response.status}`);
    const body = await response.text();
    if (!body.trim() || body.length > 2 * 1024 * 1024) throw Error('TMDb response is empty or too large');
    await fs.writeFile(file, body);
    await new Promise(resolve => setTimeout(resolve, 150));
    return body;
  }
  throw Error('TMDb lookup failed');
}

async function wikiRequest(url, prefix) {
  const file = path.join(cache, `${prefix}-${hash(url)}.txt`);
  try { const cached = await fs.readFile(file, 'utf8'); if (cached.trim()) return cached; }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
  for (let attempt = 0; attempt < 5; attempt++) {
    let response;
    try {
      response = await fetch(url, { signal: AbortSignal.timeout(30000), headers: {
        'User-Agent': 'ReelCanon-SourceResearch/1.0 (film identity verification)', Accept: 'application/json',
      } });
    } catch (error) {
      if (attempt === 4) throw error;
      await new Promise(resolve => setTimeout(resolve, Math.min(1000 * (2 ** attempt), 16000)));
      continue;
    }
    if (response.status === 429 || response.status >= 500) {
      const retryAfter = Number(response.headers.get('Retry-After'));
      const delay = Number.isFinite(retryAfter) && retryAfter > 0 ? Math.min(retryAfter, 30) * 1000 : Math.min(2000 * (2 ** attempt), 30000);
      if (attempt === 4) throw Error(`${prefix} returned HTTP ${response.status} after retries`);
      await new Promise(resolve => setTimeout(resolve, delay));
      continue;
    }
    if (!response.ok) throw Error(`${prefix} returned HTTP ${response.status}`);
    const body = await response.text();
    if (!body.trim() || body.length > 5 * 1024 * 1024) throw Error(`${prefix} response is empty or too large`);
    await fs.writeFile(file, body);
    await new Promise(resolve => setTimeout(resolve, 200));
    return body;
  }
  throw Error(`${prefix} request failed`);
}

async function request(url) {
  const file = path.join(cache, `imdb-${hash(url)}.txt`);
  try { const cached = await fs.readFile(file, 'utf8'); if (cached.trim()) return cached; }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
  for (let attempt = 0; attempt < 5; attempt++) {
    const response = await fetch(url, { signal: AbortSignal.timeout(30000), headers: { 'User-Agent': 'ReelCanon-SourceResearch/1.0' } });
    if (response.status === 404) {
      await new Promise(resolve => setTimeout(resolve, 500));
      return JSON.stringify({ d: [] });
    }
    if (response.status === 429 || response.status >= 500) {
      const retryAfter = Number(response.headers.get('Retry-After'));
      const delay = Number.isFinite(retryAfter) && retryAfter > 0 ? Math.min(retryAfter, 30) * 1000 : Math.min(2000 * (2 ** attempt), 30000);
      if (attempt === 4) throw Error(`IMDb suggestion returned HTTP ${response.status} after retries`);
      await new Promise(resolve => setTimeout(resolve, delay));
      continue;
    }
    if (!response.ok) throw Error(`IMDb suggestion returned HTTP ${response.status}`);
    const body = await response.text();
    if (!body.trim() || body.length > 2 * 1024 * 1024) throw Error('IMDb suggestion response is empty or too large');
    await fs.writeFile(file, body);
    await new Promise(resolve => setTimeout(resolve, 500));
    return body;
  }
  throw Error('IMDb suggestion request failed');
}

async function resolve(row, slug) {
  const alias = aliases[`${row.title} (${row.releaseYear})`] ?? aliases[row.title] ?? {};
  const query = alias.query ?? `${row.title} ${alias.releaseYear ?? row.releaseYear}`;
  const url = `https://v3.sg.media-imdb.com/suggestion/${encodeURIComponent(query[0].toLowerCase())}/${encodeURIComponent(query)}.json`;
  const response = JSON.parse(await request(url));
  const titles = [row.title, ...(alias.titles ?? [])].map(normalize);
  const matches = (response.d ?? []).filter(item => /^tt\d{7,10}$/.test(item.id)
    && ['movie', ...(alias.types ?? [])].includes(item.qid)
    && titles.includes(normalize(item.l))
    && item.y === (alias.releaseYear ?? row.releaseYear));
  const selected = alias.imdbId ? matches.find(item => item.id === alias.imdbId) : matches.length === 1 ? matches[0] : undefined;
  if (selected) {
    return { sourceReleaseYear: row.releaseYear, releaseYear: selected.y, imdbId: selected.id, identityURL: url, identityTitle: selected.l,
      identityResponseHash: hash(JSON.stringify(response)) };
  }
  if (alias.imdbId) {
    return { sourceReleaseYear: row.releaseYear, releaseYear: alias.releaseYear ?? row.releaseYear, imdbId: alias.imdbId,
      identityURL: alias.evidence ?? `https://www.imdb.com/title/${alias.imdbId}/`, identityTitle: alias.titles?.[0] ?? row.title,
      identityResponseHash: hash(JSON.stringify({ sourceTitle: row.title, sourceYear: row.releaseYear, alias })),
      identityMethod: 'explicitly reviewed IMDb alias' };
  }

  if (slug === '1001-movies-2024') {
    const qid = itemByPosition.get(row.position);
    const imdbId = qid ? wikidataImdb.get(qid) : undefined;
    if (imdbId) {
      return { sourceReleaseYear: row.releaseYear, releaseYear: row.releaseYear, imdbId,
        identityURL: `https://www.imdb.com/title/${imdbId}/`, identityTitle: row.title,
        identityResponseHash: hash(JSON.stringify({ sourceURL: row.sourceURL, qid, imdbId, property: 'Wikidata P345' })),
        identityMethod: `source-linked Wikipedia article → ${qid} → Wikidata P345` };
    }
  }

  // IMDb suggestions favor the English title. Use TMDb's localized display
  // title plus the exact release year as a second cross-check when a source
  // title is in another language.
  if (tmdbKey) {
    const language = slug === '1001-movies-2024' ? 'de-DE' : 'en-US';
    const params = new URLSearchParams({ query: row.title, year: String(row.releaseYear), language, api_key: tmdbKey });
    const searchURL = `https://api.themoviedb.org/3/search/movie?${params}`;
    const search = JSON.parse(await tmdbRequest(searchURL));
    const title = normalize(row.title);
    const candidates = (search.results ?? []).filter(movie => {
      const year = Number(movie.release_date?.slice(0, 4));
      return year === row.releaseYear && [movie.title, movie.original_title].filter(Boolean).some(value => normalize(value) === title);
    });
    if (candidates.length === 1) {
      const movie = candidates[0];
      const detailsURL = `https://api.themoviedb.org/3/movie/${movie.id}?${new URLSearchParams({ language: 'en-US', api_key: tmdbKey })}`;
      const details = JSON.parse(await tmdbRequest(detailsURL));
      if (/^tt\d{7,10}$/.test(details.imdb_id ?? '')) {
        return { sourceReleaseYear: row.releaseYear, releaseYear: Number(details.release_date?.slice(0, 4)) || row.releaseYear,
          imdbId: details.imdb_id, identityURL: `https://www.imdb.com/title/${details.imdb_id}/`,
          identityTitle: details.original_title || details.title, identityResponseHash: hash(JSON.stringify({ search, details })),
          identityMethod: 'exact TMDb title/year cross-check and IMDb external ID' };
      }
    }
  }
  return { error: { position: row.position, title: row.title, year: row.releaseYear,
    candidates: (response.d ?? []).filter(item => /^tt\d{7,10}$/.test(item.id)).slice(0, 8)
      .map(item => ({ id: item.id, title: item.l, year: item.y, type: item.qid })) } };
}

const slugs = ['1001-movies-2024', 'sight-and-sound-greatest-films-2022'];
const locks = new Map();
const jobs = [];
for (const slug of slugs) {
  const lockPath = path.join(root, 'data', 'sources', `${slug}.lock.json`);
  const lock = JSON.parse(await fs.readFile(lockPath, 'utf8'));
  locks.set(slug, { lockPath, lock });
  for (const row of lock.entries) jobs.push({ slug, row });
}

// German Wikipedia links identify each 1001 film page directly. Wikidata's
// IMDb external-ID property provides a stronger cross-language mapping than
// matching the German page title against IMDb's English-title suggestions.
const wikiRows = jobs.filter(job => job.slug === '1001-movies-2024');
const itemByPosition = new Map();
for (let index = 0; index < wikiRows.length; index += 50) {
  const batch = wikiRows.slice(index, index + 50);
  const requested = batch.map(({ row }) => decodeURIComponent(new URL(row.sourceURL).pathname.split('/wiki/')[1]).replaceAll('_', ' '));
  const params = new URLSearchParams({ action: 'query', format: 'json', prop: 'pageprops', ppprop: 'wikibase_item', redirects: '1', titles: requested.join('|') });
  const url = `https://de.wikipedia.org/w/api.php?${params}`;
  const result = JSON.parse(await wikiRequest(url, 'wikipedia-pages'));
  const relations = [...(result.query?.normalized ?? []), ...(result.query?.redirects ?? [])];
  const canonicalTitle = title => {
    let value = title;
    for (let round = 0; round < 4; round++) {
      const relation = relations.find(item => normalize(item.from.replaceAll('_', ' ')) === normalize(value.replaceAll('_', ' ')));
      if (!relation) break;
      value = relation.to;
    }
    return value;
  };
  const pages = Object.values(result.query?.pages ?? {});
  for (let offset = 0; offset < batch.length; offset++) {
    const page = pages.find(item => normalize(item.title) === normalize(canonicalTitle(requested[offset])));
    if (page?.pageprops?.wikibase_item) itemByPosition.set(batch[offset].row.position, page.pageprops.wikibase_item);
  }
}
const qids = [...new Set(itemByPosition.values())];
const wikidataImdb = new Map();
const qidBatches = [];
for (let index = 0; index < qids.length; index += 20) qidBatches.push(qids.slice(index, index + 20));
for (let index = 0; index < qidBatches.length; index += 4) {
  const results = await Promise.all(qidBatches.slice(index, index + 4).map(async batch => {
    const query = `SELECT ?item ?imdb WHERE { VALUES ?item { ${batch.map(qid => `wd:${qid}`).join(' ')} } ?item wdt:P345 ?imdb. }`;
    const url = `https://query.wikidata.org/sparql?${new URLSearchParams({ query, format: 'json' })}`;
    return JSON.parse(await wikiRequest(url, 'wikidata-imdb'));
  }));
  for (const result of results) for (const binding of result.results?.bindings ?? []) {
    const qid = binding.item.value.split('/').pop();
    if (/^tt\d{7,10}$/.test(binding.imdb.value)) wikidataImdb.set(qid, binding.imdb.value);
  }
  console.log(`Wikidata identity batches: ${Math.min(index + 4, qidBatches.length)}/${qidBatches.length}.`);
}
console.log(`German Wikipedia pages: ${itemByPosition.size}; linked Wikidata films with IMDb IDs: ${wikidataImdb.size}.`);

const unresolved = [];
const progress = new Map(slugs.map(slug => [slug, 0]));
const batchSize = 4;
for (let index = 0; index < jobs.length; index += batchSize) {
  await Promise.all(jobs.slice(index, index + batchSize).map(async job => {
    const result = await resolve(job.row, job.slug);
    if (result.error) unresolved.push({ slug: job.slug, ...result.error });
    else Object.assign(job.row, result);
    progress.set(job.slug, progress.get(job.slug) + 1);
  }));
  if ((index + batchSize) % 16 === 0 || index + batchSize >= jobs.length) console.log(`Resolved ${slugs.map(slug => `${slug} ${progress.get(slug)}/${locks.get(slug).lock.entries.length}`).join(' · ')}.`);
}
if (unresolved.length) {
  const report = path.join(cache, 'additional-lists-unresolved.json');
  await fs.writeFile(report, JSON.stringify(unresolved, null, 2) + '\n');
  console.log(`Unresolved identity records: ${unresolved.length}.`);
  throw Error(`Explicit alias review required; candidates saved to ${report}`);
}
for (const [slug, { lockPath, lock }] of locks) {
  const ids = lock.entries.map(entry => entry.imdbId);
  if (new Set(ids).size !== ids.length) throw Error(`${slug} resolved duplicate IMDb identities; inspect the locked source before import.`);
  await fs.writeFile(lockPath, JSON.stringify(lock, null, 2) + '\n');
  console.log(`Saved ${slug}: ${ids.length} unique IMDb identities.`);
}
