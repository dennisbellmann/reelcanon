// Explicit online research step. Run the offline builder for exact reproduction.
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const { JSDOM, VirtualConsole } = require('../frontend/node_modules/jsdom');
const root = fileURLToPath(new URL('../', import.meta.url));
const cutoff = process.argv.find(x => x.startsWith('--date='))?.slice(7);
if (!/^\d{4}-\d{2}-\d{2}$/.test(cutoff ?? '')) throw Error('Supply --date=YYYY-MM-DD explicitly.');
const cache = path.join(root, '.local', 'usecase-research', cutoff);
await fs.mkdir(cache, { recursive: true });
const digest = value => createHash('sha256').update(value).digest('hex');
const normalize = value => value.normalize('NFKD').replace(/\p{Diacritic}/gu, '').toLowerCase().replace(/[^a-z0-9]/g, '');
async function request(url) {
  const file = path.join(cache, `${digest(url)}.txt`);
  try { const cached = await fs.readFile(file, 'utf8'); if (cached.length) return cached; } catch (e) { if (e.code !== 'ENOENT') throw e; }
  for (let attempt = 0; attempt < 3; attempt++) {
    const response = await fetch(url, { signal: AbortSignal.timeout(30000), headers: { 'User-Agent': 'ReelCanon-SourceResearch/1.0' } });
    if (response.ok) { const body = await response.text(); if (!body.trim()) { await new Promise(resolve => setTimeout(resolve, 1000)); continue; } if (body.length > 2*1024*1024) throw Error('Source exceeds 2 MB'); await fs.writeFile(file, body); return body; }
    if (response.status !== 429 && response.status < 500) throw Error(`Source HTTP ${response.status}: ${new URL(url).hostname}`);
    await new Promise(resolve => setTimeout(resolve, (attempt+1)*1000));
  }
  throw Error(`Source failed: ${new URL(url).hostname}`);
}
async function batches(items, action) {
  const result = [];
  for (let i=0; i<items.length; i+=4) { result.push(...await Promise.all(items.slice(i,i+4).map(action))); }
  return result;
}
const horrorURL = 'https://editorial.rottentomatoes.com/guide/best-horror-movies-of-all-time/';
const horrorBody = await request(horrorURL);
let window = new JSDOM(horrorBody, { virtualConsole: new VirtualConsole() }).window;
const horror = [...window.document.querySelectorAll('.block-countdown')].map(row => ({
  position: Number(row.id.replace('countdown-index-', '')),
  title: row.querySelector('.meta-title')?.textContent.trim(),
  releaseYear: Number(row.querySelector('.meta-year')?.textContent.match(/\d{4}/)?.[0]),
  sourceURL: row.querySelector('.meta-title')?.href,
})).filter(row => row.position <= 100).sort((a,b) => a.position-b.position);
window.close();
if (horror.length !== 100 || horror.some((r,i) => r.position !== i+1 || !r.title || !r.releaseYear)) throw Error('Horror source changed: expected ranks 1–100');
console.log('Extracted 100 ranked horror films.');
const academy = await batches(Array.from({ length: Number(cutoff.slice(0,4))-1928 }, (_,i) => 1929+i), async year => {
  const sourceURL = `https://www.oscars.org/oscars/ceremonies/${year}`;
  const body = await request(sourceURL);
  const window = new JSDOM(body, { virtualConsole: new VirtualConsole() }).window;
  const doc = window.document;
  const number = Number(doc.querySelector('meta[name="description"]')?.content.match(/The (\d+)/)?.[1]);
  const categories = [...doc.querySelectorAll('.paragraph--type--award-category')];
  const cat = categories.find(n => /^(Best Picture|Outstanding Picture|Outstanding Production|Outstanding Motion Picture|Best Motion Picture|Motion Picture)$/.test(n.querySelector('.field--name-field-award-category-oscars')?.textContent.trim() ?? ''));
  const winners = [...(cat?.querySelectorAll('.paragraph--type--award-honoree') ?? [])].filter(n => n.querySelector('.winner'));
  if (winners.length !== 1 || !number) { const names=categories.map(n=>n.querySelector('.field--name-field-award-category-oscars')?.textContent.trim()); window.close(); throw Error(`Academy ${year}: expected one best picture winner; categories ${names.join(', ')}`); }
  const title = winners[0].querySelector('.field--name-field-award-film')?.textContent.trim();
  window.close();
  if (!title) throw Error(`Academy ${year}: film title missing`);
  return { position:number, archiveYear:year, title, sourceURL, sourceContentHash:digest(body) };
});
if (academy.some((r,i) => r.position !== i+1)) throw Error('Academy ceremonies contain a gap or duplicate');
console.log(`Extracted ${academy.length} Academy Best Picture winners.`);
// Explicit aliases are reviewable, never fuzzy title-only identity guesses.
const aliasesPath=path.join(root,'data','sources','identity-aliases.json');
const aliases=JSON.parse(await fs.readFile(aliasesPath,'utf8'));
const unresolved=[];
async function identify(row) {
  const alias=aliases[row.title] ?? {};
  const query=alias.query ?? row.title;
  const identityURL=`https://v3.sg.media-imdb.com/suggestion/${encodeURIComponent(query[0].toLowerCase())}/${encodeURIComponent(query)}.json`;
  const response=JSON.parse(await request(identityURL));
  const expectedTitles=[row.title,...(alias.titles ?? [])].map(normalize);
  const matches=(response.d ?? []).filter(item=> /^tt\d{7,10}$/.test(item.id) && ['movie',...(alias.types ?? [])].includes(item.qid) && expectedTitles.includes(normalize(item.l)) && (alias.releaseYear || row.releaseYear ? item.y===(alias.releaseYear ?? row.releaseYear) : item.y>=row.archiveYear-3 && item.y<=row.archiveYear));
  const match=alias.imdbId ? matches.find(item=>item.id===alias.imdbId) : matches.length===1 ? matches[0] : undefined;
  if (!match) { unresolved.push({title:row.title,year:row.releaseYear ?? row.archiveYear,candidates:(response.d ?? []).map(x=>({id:x.id,title:x.l,year:x.y,type:x.qid}))}); return row; }
  return {...row,imdbId:match.id,releaseYear:match.y,identityURL,identityTitle:match.l,identityResponseHash:digest(JSON.stringify(response))};
}
const identifiedHorror=await batches(horror,identify);
const identifiedAcademy=await batches(academy,identify);
if (unresolved.length) { await fs.writeFile(path.join(cache,'unresolved.json'),JSON.stringify(unresolved,null,2)+'\n'); console.log(JSON.stringify(unresolved,null,2)); throw Error(`${unresolved.length} ambiguous identities: review and add explicit aliases before building.`); }
await fs.mkdir(path.join(root,'data','sources'),{recursive:true});
for(const [slug,rows,url,method,hash] of [
  ['halloween-top-100',identifiedHorror,horrorURL,'Rotten Tomatoes horror guide ranks 1–100; rank order retained.',digest(horrorBody)],
  ['oscar-best-picture',identifiedAcademy,'https://www.oscars.org/oscars/ceremonies','Academy Best Picture and historical category names; ceremony order retained; nominees excluded.',digest(JSON.stringify(academy))]
]) {
  const output={format:'canon-research/v1',cutoff,retrievedAt:new Date().toISOString(),sourceURL:url,method,sourceContentHash:hash,entries:rows};
  await fs.writeFile(path.join(root,'data','sources',`${slug}.lock.json`),JSON.stringify(output,null,2)+'\n');
}
console.log('Saved reviewed source locks. Run node scripts/build-usecase-lists.mjs.');
