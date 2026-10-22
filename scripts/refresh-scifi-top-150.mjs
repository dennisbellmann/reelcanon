// Capture all ranks from the complete September 2020 Rotten Tomatoes guide
// snapshot and resolve
// each entry to one exact IMDb movie identity. Run only for deliberate updates.
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const { JSDOM, VirtualConsole } = require('../frontend/node_modules/jsdom');
const root = fileURLToPath(new URL('../', import.meta.url));
const cutoff = process.argv.find(value => value.startsWith('--date='))?.slice(7);
if (!/^\d{4}-\d{2}-\d{2}$/.test(cutoff ?? '')) throw Error('Supply --date=YYYY-MM-DD explicitly.');

const sourceURL = 'https://editorial.rottentomatoes.com/guide/best-sci-fi-movies-of-all-time/';
const archiveURL = 'https://web.archive.org/web/20200926195854id_/https://editorial.rottentomatoes.com/guide/best-sci-fi-movies-of-all-time/';
const cache = path.join(root, '.local', 'usecase-research', cutoff);
await fs.mkdir(cache, { recursive: true });
const digest = value => createHash('sha256').update(value).digest('hex');
const normalize = value => value.normalize('NFKD').replace(/\p{Diacritic}/gu, '').toLowerCase().replace(/[^a-z0-9]/g, '');

async function request(url, key) {
  const file = path.join(cache, `${key}.txt`);
  try { const cached = await fs.readFile(file, 'utf8'); if (cached.trim()) return cached; }
  catch (error) { if (error.code !== 'ENOENT') throw error; }
  const response = await fetch(url, {
    signal: AbortSignal.timeout(30000),
    headers: { 'User-Agent': 'ReelCanon-SourceResearch/1.0' },
  });
  if (!response.ok) throw Error(`${key}: source returned HTTP ${response.status}`);
  const body = await response.text();
  if (!body.trim() || body.length > 2 * 1024 * 1024) throw Error(`${key}: empty source or source exceeds 2 MB`);
  await fs.writeFile(file, body);
  return body;
}

const sourceBody = await request(archiveURL, 'rotten-tomatoes-essential-sci-fi-2020-archive');
const window = new JSDOM(sourceBody, { virtualConsole: new VirtualConsole() }).window;
const films = [...window.document.querySelectorAll('.countdown-item')]
  .map(row => ({
    position: Number(row.querySelector('.countdown-index')?.textContent.match(/\d+/)?.[0]),
    title: row.querySelector('.article_movie_title h2 a')?.textContent.replace(/\s+/g, ' ').trim(),
    releaseYear: Number(row.querySelector('.start-year')?.textContent.match(/\d{4}/)?.[0]),
    sourceURL: row.querySelector('.article_movie_title h2 a')?.href,
  }))
  .filter(film => film.position >= 1 && film.position <= 150)
  .sort((a, b) => a.position - b.position);
window.close();
const invalidFilms = films.filter((film, index) => film.position !== index + 1 || !film.title || !film.releaseYear || !film.sourceURL);
if (films.length !== 150 || invalidFilms.length) {
  throw Error(`Rotten Tomatoes source changed: expected ranks 1–150, received ${films.length} rows; invalid: ${JSON.stringify(invalidFilms.slice(0, 5))}`);
}
console.log(`Extracted ${films.length} ranked Sci-Fi films.`);

const aliases = JSON.parse(await fs.readFile(path.join(root, 'data', 'sources', 'identity-aliases.json'), 'utf8'));
const unresolved = [];
async function identify(film) {
  const alias = aliases[`${film.title} (${film.releaseYear})`] ?? aliases[film.title] ?? {};
  const query = alias.query ?? film.title;
  const url = `https://v3.sg.media-imdb.com/suggestion/${encodeURIComponent(query[0].toLowerCase())}/${encodeURIComponent(query)}.json`;
  const response = JSON.parse(await request(url, `imdb-${digest(url)}`));
  const acceptedTitles = [film.title, ...(alias.titles ?? [])].map(normalize);
  const matches = (response.d ?? []).filter(item => /^tt\d{7,10}$/.test(item.id)
    && ['movie', ...(alias.types ?? [])].includes(item.qid)
    && acceptedTitles.includes(normalize(item.l))
    && item.y === (alias.releaseYear ?? film.releaseYear));
  const match = alias.imdbId ? matches.find(item => item.id === alias.imdbId) : matches.length === 1 ? matches[0] : undefined;
  if (!match) {
    unresolved.push({ position: film.position, title: film.title, releaseYear: film.releaseYear,
      candidates: (response.d ?? []).filter(item => /^tt\d{7,10}$/.test(item.id)).slice(0, 8)
        .map(item => ({ id: item.id, title: item.l, year: item.y, type: item.qid })) });
    return film;
  }
  return { ...film, sourceReleaseYear: film.releaseYear, releaseYear: match.y, imdbId: match.id, identityURL: url, identityTitle: match.l,
    identityResponseHash: digest(JSON.stringify(response)) };
}

const identified = [];
for (let index = 0; index < films.length; index += 4) {
  identified.push(...await Promise.all(films.slice(index, index + 4).map(identify)));
  console.log(`Resolved ${Math.min(index + 4, films.length)}/${films.length}.`);
}
if (unresolved.length) {
  const report = path.join(cache, 'sci-fi-unresolved.json');
  await fs.writeFile(report, JSON.stringify(unresolved, null, 2) + '\n');
  throw Error(`${unresolved.length} titles need explicit alias review; candidates saved to ${report}`);
}

const output = {
  format: 'canon-research/v1', cutoff, retrievedAt: new Date().toISOString(),
  slug: 'sci-fi-top-150', editionLabel: 'rotten-tomatoes-essential-sci-fi-2020-archive',
  sourceURL, archiveURL,
  method: 'Complete September 26, 2020 Internet Archive capture of Rotten Tomatoes\' 150 Essential Sci-Fi Movies guide; all source ranks 1–150 retained in displayed order. Each selected film had at least 20 reviews according to the source introduction.',
  sourceContentHash: digest(sourceBody), entries: identified,
};
  await fs.writeFile(path.join(root, 'data', 'sources', 'sci-fi-top-150.lock.json'), JSON.stringify(output, null, 2) + '\n');
console.log(`Saved sci-fi-top-150: ${identified.length} unique IMDb IDs; source SHA-256 ${output.sourceContentHash}.`);
