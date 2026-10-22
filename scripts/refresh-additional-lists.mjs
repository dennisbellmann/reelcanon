// Capture the reviewed 2024 1001 Films edition and the BFI 2022 Critics' Poll.
// Identity lookup is intentionally deferred to the Cockpit's exact title/year resolver.
import fs from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const { JSDOM, VirtualConsole } = require('../frontend/node_modules/jsdom');
const root = fileURLToPath(new URL('../', import.meta.url));
const cutoff = process.argv.find(value => value.startsWith('--date='))?.slice(7);
if (!/^\d{4}-\d{2}-\d{2}$/.test(cutoff ?? '')) throw Error('Supply --date=YYYY-MM-DD explicitly.');

const cache = path.join(root, '.local', 'usecase-research', cutoff);
await fs.mkdir(cache, { recursive: true });
const digest = value => createHash('sha256').update(value).digest('hex');

async function request(url, name) {
  const file = path.join(cache, `${name}.html`);
  try {
    const cached = await fs.readFile(file, 'utf8');
    if (cached.trim()) return cached;
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  const response = await fetch(url, {
    signal: AbortSignal.timeout(30000),
    headers: { 'User-Agent': 'ReelCanon-SourceResearch/1.0' },
  });
  if (!response.ok) throw Error(`${name}: source returned HTTP ${response.status}`);
  const body = await response.text();
  if (!body.trim() || body.length > 2 * 1024 * 1024) throw Error(`${name}: empty source or source exceeds 2 MB`);
  await fs.writeFile(file, body);
  return body;
}

function text(element) {
  return element?.textContent.replace(/\s+/g, ' ').trim() ?? '';
}

const wikiURL = 'https://de.wikipedia.org/wiki/1001_Movies_You_Must_See_Before_You_Die';
const wikiBody = await request(wikiURL, '1001-movies-2024-de');
const wikiWindow = new JSDOM(wikiBody, { virtualConsole: new VirtualConsole() }).window;
const wikiSection = wikiWindow.document.querySelector('#Umfassende_Liste_von_November_2024')?.closest('section');
const wikiTable = wikiSection?.querySelector('table');
if (!wikiTable) throw Error('Wikipedia source changed: November 2024 film table not found.');
let latestYear = 0;
const oneThousandOne = [...wikiTable.querySelectorAll('tr')].flatMap(row => {
  const cells = [...row.querySelectorAll('th,td')];
  if (cells.length < 4 || cells[0].tagName === 'TH') return [];
  const yearText = text(cells[0]);
  if (/^\d{4}$/.test(yearText)) latestYear = Number(yearText);
  const titleLink = cells[1].querySelector('a');
  const title = text(titleLink) || text(cells[1]);
  if (!title || latestYear < 1888) throw Error('Wikipedia source contains a row without a title or release year.');
  const href = titleLink?.getAttribute('href');
  return [{
    position: 0,
    title,
    releaseYear: latestYear,
    sourceURL: href ? new URL(href, wikiURL).href : wikiURL,
  }];
}).map((entry, index) => ({ ...entry, position: index + 1 }));
wikiWindow.close();
if (oneThousandOne.length !== 1031) throw Error(`Wikipedia source changed: expected 1,031 film records, received ${oneThousandOne.length}.`);

const bfiURL = 'https://www.bfi.org.uk/sight-and-sound/greatest-films-all-time';
const bfiBody = await request(bfiURL, 'sight-and-sound-2022-critics');
const resultsMarker = '"componentState":{"results":';
const resultsStart = bfiBody.indexOf(resultsMarker);
if (resultsStart < 0) throw Error('BFI source changed: poll results were not found.');
const arrayStart = bfiBody.indexOf('[', resultsStart + resultsMarker.length);
let depth = 0;
let inString = false;
let escaped = false;
let arrayEnd = -1;
for (let index = arrayStart; index < bfiBody.length; index++) {
  const char = bfiBody[index];
  if (inString) {
    if (escaped) escaped = false;
    else if (char === '\\') escaped = true;
    else if (char === '"') inString = false;
    continue;
  }
  if (char === '"') inString = true;
  else if (char === '[') depth++;
  else if (char === ']' && --depth === 0) { arrayEnd = index + 1; break; }
}
if (arrayStart < 0 || arrayEnd < 0) throw Error('BFI source changed: results JSON is malformed.');
const bfiRows = JSON.parse(bfiBody.slice(arrayStart, arrayEnd));
if (bfiRows.some(row => !Number.isInteger(row.rank) || !row.film?.name || !/^\d{4}$/.test(String(row.film.year)))) {
  throw Error('BFI source changed: a poll result is missing its rank, film title, or release year.');
}
const sightAndSound = bfiRows
  .map((row, sourceIndex) => ({ ...row, sourceIndex }))
  .sort((a, b) => a.rank - b.rank || a.sourceIndex - b.sourceIndex)
  .map((row, index) => ({
    position: index + 1,
    pollRank: row.rank,
    tied: Boolean(row.tied),
    title: row.film.name.trim(),
    releaseYear: Number(row.film.year),
    sourceURL: new URL(row.film.url, 'https://www.bfi.org.uk').href,
  }));
const ranks = new Set(sightAndSound.map(row => row.pollRank));
if (sightAndSound.length !== 264 || ranks.size < 1 || Math.min(...ranks) !== 1 || Math.max(...ranks) > 250) {
  throw Error(`BFI source changed: expected 264 films within poll ranks 1–250, received ${sightAndSound.length} films spanning ${ranks.size} rank values.`);
}
if (!sightAndSound[0].title.startsWith('Jeanne Dielman') || sightAndSound[0].releaseYear !== 1975 || sightAndSound[0].pollRank !== 1) {
  throw Error('BFI source changed: expected the 2022 Critics’ Poll headed by Jeanne Dielman (1975).');
}

const sources = [
  {
    slug: '1001-movies-2024',
    editionLabel: '2024-deutsch',
    sourceURL: wikiURL,
    method: 'Chronological order from the November 2024 list for the 15th German-language edition. IMDb identities are resolved by exact title and release year in the Cockpit.',
    sourceContentHash: digest(wikiBody),
    entries: oneThousandOne,
  },
  {
    slug: 'sight-and-sound-greatest-films-2022',
    editionLabel: '2022-critics-poll',
    sourceURL: bfiURL,
    method: 'BFI/Sight and Sound 2022 Critics\' Poll; ranked through place 250, with tied placements retained and official display order used. IMDb identities are resolved by exact title and release year in the Cockpit.',
    sourceContentHash: digest(bfiBody),
    entries: sightAndSound,
  },
];
for (const source of sources) {
  const output = { format: 'canon-research/v1', cutoff, retrievedAt: new Date().toISOString(), ...source };
  await fs.writeFile(path.join(root, 'data', 'sources', `${source.slug}.lock.json`), JSON.stringify(output, null, 2) + '\n');
  console.log(`Captured ${source.slug}: ${source.entries.length} films; SHA-256 ${source.sourceContentHash}.`);
}
