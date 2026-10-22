import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
const root=new URL('../',import.meta.url);
const check=process.argv.includes('--check');
const specs=[
  {slug:'halloween-top-100',name:'Top 100 Halloween Movies',position:2,count:100,description:'100 Horrorfilme für Halloween: die Plätze 1–100 der Rotten-Tomatoes-Horror-Rangliste. Reihenfolge der Quelle, Stand 06.10.2026.',poster:'/artwork/halloween-top-100.svg'},
  {slug:'oscar-best-picture',name:'Oscar · Bester Film',position:3,count:98,description:'Alle 98 Gewinner des Oscars für den besten Film bis zur 98. Verleihung 2026, vom ersten bis zum neuesten Gewinner. Historische Kategorienamen eingeschlossen; keine Nominierten.',poster:'/artwork/oscar-best-picture.svg'},
  {slug:'1001-movies-2024',name:'1001 Filme · Ausgabe 2024',position:4,count:1031,description:'15. deutschsprachige Ausgabe (November 2024), chronologische Reihenfolge. Die dokumentierte Quelle führt 1.031 einzelne Filmaufzeichnungen.',poster:'/artwork/1001-movies-2024.svg',edition:'2024-deutsch'},
  {slug:'sight-and-sound-greatest-films-2022',name:'Sight & Sound · Greatest Films 2022',position:5,count:264,description:'BFI-Kritiker-Poll 2022: Rangliste bis Platz 250; 264 Filme mit Gleichständen. Gleichstände folgen der BFI-Anzeigereihenfolge.',poster:'/artwork/sight-and-sound-greatest-films-2022.svg',edition:'2022-critics-poll'},
  {slug:'sci-fi-top-150',name:'Sci-Fi · 150 Essential Films',position:6,count:150,description:'Komplette Archivfassung der Rotten-Tomatoes-Liste „150 Essential Sci-Fi Movies“ vom 26.09.2020. 150 Filme in der Reihenfolge der Quelle; damals mindestens 20 Kritiken je Film laut Quelle.',poster:'/artwork/sci-fi-top-150.svg',edition:'rotten-tomatoes-essential-sci-fi-2020-archive'},
];
for(const spec of specs) {
  const lockURL=new URL(`data/sources/${spec.slug}.lock.json`,root);
  const lockText=await fs.readFile(lockURL,'utf8');
  const lock=JSON.parse(lockText);
  if(lock.format!=='canon-research/v1' || lock.entries.length!==spec.count || !lock.retrievedAt || !lock.cutoff || (spec.edition && lock.editionLabel!==spec.edition)) throw Error(`${spec.slug}: invalid source lock`);
  const ids=new Set();
  const entries=lock.entries.map((film,index)=>{
    const hasIdentity=Boolean(film.imdbId);
    if(film.position!==index+1 || (hasIdentity && (!/^tt\d{7,10}$/.test(film.imdbId) || ids.has(film.imdbId) || !film.identityURL)) || !film.sourceURL || !film.title || !Number.isInteger(film.releaseYear) || film.releaseYear<1888 || film.releaseYear>2100) throw Error(`${spec.slug}: invalid source film at ${index+1}`);
    if(hasIdentity) ids.add(film.imdbId);
    if(spec.slug==='sight-and-sound-greatest-films-2022' && (!Number.isInteger(film.pollRank) || film.pollRank<1 || film.pollRank>250)) throw Error(`${spec.slug}: invalid BFI rank at ${index+1}`);
    return {imdbId:film.imdbId??'',position:film.position,sourceTitle:film.title,releaseYear:film.releaseYear};
  });
  const snapshot={format:'canon-snapshot/v1',canon:{slug:spec.slug,name:spec.name,description:spec.description,sourceUrl:lock.sourceURL,presentation:{position:spec.position,posterUrl:spec.poster,titleEmbedded:true}},editionLabel:spec.edition??`reviewed-${lock.cutoff}`,retrievedAt:lock.retrievedAt,sourceContentHash:createHash('sha256').update(lockText).digest('hex'),entries};
  const output=JSON.stringify(snapshot,null,2)+'\n';
  const url=new URL(`data/${spec.slug}.json`,root);
  if(check) { if(await fs.readFile(url,'utf8')!==output) throw Error(`${spec.slug}: snapshot differs from reviewed source lock`); }
  else await fs.writeFile(url,output);
  const identityCount=entries.filter(entry=>entry.imdbId).length;
  const summary=identityCount===entries.length?`${identityCount} unique IMDb IDs`:`${entries.length} entries; ${identityCount} IMDb IDs preassigned, remaining titles resolve in the Cockpit`;
  console.log(`${check?'Verified':'Built'} ${spec.slug}: ${summary}, positions 1–${entries.length}.`);
}
