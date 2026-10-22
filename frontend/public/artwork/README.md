# Collection artwork candidates

## Canonical collection-poster style

The clean editorial SVG covers `halloween-top-100.svg` and
`oscar-best-picture.svg` establish the style for **all new and replacement
collection posters**. Continue this visual language unless the user explicitly
requests an exception:

- Portrait 2:3 composition on warm cream paper (`#eee6d5`), with a fine inset
  charcoal frame (`#292820`).
- One centered, original geometric cinema motif, using charcoal plus restrained
  rust (`#ba522c`) or brass (`#b9914e`) accents. Keep shapes simple and clear
  when the poster is small or shown in grayscale.
- A small letter-spaced Reel Canon collection line at the top; an integrated,
  high-contrast Georgia-style serif title below the illustration; one quiet
  sans-serif subtitle; generous breathing room and, at most, a fine divider.
- Prefer editable SVG artwork with a descriptive `aria-label`. Keep the title
  part of the poster and configure `posterTitleEmbedded` as true. Do not use
  movie stills, recognizable licensed characters, logos, or busy photo collages.

The user's approved visual reference is the pair of Halloween and Oscar
posters. The IMDb Top 250 poster added below follows this system. Earlier PNG
concepts and prompts remain archived here as historical candidates; they are
not the style baseline for future list covers.

### Current style references

- `halloween-top-100.svg`
- `oscar-best-picture.svg`
- `imdb-top-250.svg`
- `1001-movies-2024.svg`
- `sight-and-sound-greatest-films-2022.svg`
- `sci-fi-top-150.svg`

The three 2026 list posters continue the same editable editorial system while
using separate visual metaphors: 1001 Filme follows three story frames along a
single film strip; Sight & Sound turns an eye with a film-reel pupil into
outward sound waves; Sci-Fi opens an arch onto a ringed planet. Their
integrated titles, cream paper, charcoal frame and restrained rust/brass
accents tie them back to the collection family.

Generated with the built-in Imagegen tool on 2026-10-05 for Reel Canon.
All three originals are full-color 1024 × 1536 PNGs. The application reveals color from bottom to top in proportion to watched films.
The PosterBadge shows watched/total (for example 50/250).

1. `collection-01-filmrolle.png` — sculptural vintage film reel.
2. `collection-02-kinosaal.png` — classic cinema auditorium.
3. `collection-03-filmwelten.png` — connected imaginary film worlds.

Selected: Filmwelten. The collection title is incorporated directly into the
image. The final choice for the site's quieter editorial art direction is
`imdb-top-250-filmwelten-editorial.png`: larger serif lettering, a calm ivory
title area and no decorative gold rules. The three film worlds are retained.
The first title iteration, `imdb-top-250-filmwelten.png`, and all original
candidates are preserved. The Cockpit configures each collection's name, position, poster URL and whether
the title is embedded. Configuration is published with the edition; the frontend
no longer selects covers by slug. Migration 0006 assigns this poster to IMDb Top 250.

## Generation prompts

### 1. Filmrolle

Use case: stylized-concept. Asset type: portrait 2:3 collection cover artwork for Reel Canon, a clean editorial film-history web app. Primary request: create a premium original artwork celebrating a curated collection of 250 great films. Subject: one elegant sculptural vintage cinema film reel, a graceful strip of film looping around it. Style: sophisticated tactile screen-print illustration, subtle paper grain, restrained art-deco geometry, rich detail in the metal and film, strong iconic silhouette. Composition: centered confident vertical poster composition, generous breathing room, works at small card size, artwork fills the frame without a border. Lighting and color: warm ivory, cinematic deep red accents and brass gold, strong light-dark structure. This MUST be a full COLOR original; the app will display it in grayscale until the collection is completed, so it must also read beautifully in black and white. Constraints: no text, no letters, no numerals, no IMDb logo, no watermark, no interface, no actual existing movie posters, no collage of licensed characters.

### 2. Kinosaal

Use case: stylized-concept. Asset type: portrait 2:3 collection cover artwork for Reel Canon, a clean editorial film-history web app. Primary request: create a premium original artwork celebrating a curated collection of great cinema. Scene: a beautiful empty classic cinema auditorium, rows of plush deep-red velvet seats leading toward a luminous screen, a fine projector beam through the air. Style: atmospheric painted cinematic illustration with tangible textures and elegant film grain, not a photograph or UI mockup. Composition: symmetrical cinematic perspective, sculptural pools of light, clean bold shapes that read at small card size, full-bleed vertical poster, no frame. Lighting and color: red velvet, warm golden amber light, deep almost-black shadows and quiet ivory screen. This MUST be a full COLOR original; the app will display it in grayscale until all films are watched, so use beautiful tonal contrast independent of hue. Constraints: no people, no text, no letters, no numerals, no logos, no watermark, no interface, no recognizable scene from a real film.

### 3. Filmwelten

Use case: stylized-concept. Asset type: portrait 2:3 collection cover artwork for Reel Canon, a clean editorial film-history web app. Primary request: create a premium original artwork celebrating the variety and wonder of great cinema. Subject: an elegant surreal anthology of film worlds inside three interlocking cinematic frames: a monumental city silhouette, an open landscape and a luminous night sky; a flowing film strip gently connects the worlds. Style: exquisite textured cut-paper editorial illustration with sculptural depth, subtle print grain and clean composition, poetic art-deco poster sensibility. Composition: balanced geometric vertical artwork, coherent rather than busy, iconic at small collection-card size, full bleed with no border. Lighting and color: warm ivory, rich rust-red, luminous amber gold and deep midnight blue; clear silhouettes and distinct value planes. This MUST be a full COLOR original; the app will display it in grayscale until the collection is completed, so ensure strong black-and-white tonal readability. Constraints: no people or faces, no text, no letters, no numerals, no logos, no watermark, no UI, no recognizable licensed movie shots or characters.

## Title refinements

### 4. Integrated title — first iteration

Use case: text-localization. Edit target: the attached Filmwelten collection artwork, portrait 2:3. Primary request: integrate the collection title directly into the original illustration with the EXACT text "IMDb Top 250" (I uppercase, M uppercase, D lowercase, b lowercase; then Top 250). Keep the existing illustration, the three connected film worlds, the flowing filmstrip, warm rust-red/ivory/gold/midnight-blue palette, paper grain, and art-deco character. Typography should look deliberately designed as part of this premium cinema poster: elegant, large, extremely legible editorial/art-deco lettering in the lower portion over the quiet ivory/cloud foreground, balanced spacing, dark midnight-blue lettering with restrained gold detail if useful. The title may use two balanced lines but must read verbatim as "IMDb Top 250" and remain easy to read at small collection-card size and in grayscale. Make only the minimum local adjustment to cloud texture immediately behind the title for contrast; retain the rest of the composition, scene detail, colors and proportions. Generate the FULL COLOR original; the app applies grayscale until completion. No additional text, no duplicate title, no slogan, no watermark, no external border, no UI overlay, no yellow IMDb brand-logo box.

### 5. Filmwelten — final editorial refinement

Use case: style-transfer. Edit target: the attached Filmwelten poster with the title IMDb Top 250. Refine this same concept into a QUIET, SOPHISTICATED editorial film-collection artwork for a website with warm ivory paper, charcoal ink, restrained deep-red accents and Georgia serif typography. Art direction: the restraint and confidence of an independent arthouse film poster and a premium newspaper culture supplement, in the broad aesthetic direction of A24 and New York Times, without using either logo or copying an existing layout. Preserve the recognizable Filmwelten idea: three film worlds (city, night sky, open landscape) connected by a flowing film strip, portrait 2:3. Make the treatment cleaner and less ornate: larger calmer shapes, less intricate scenery and cloud decoration, more breathing room. Keep subtle tactile paper grain. Replace shiny golden filigree, multiple decorative rules and ornamental type with confident high-contrast editorial serif typography, excellent spacing, dark ink on a generous quiet ivory title area at the bottom. The EXACT and ONLY text is "IMDb Top 250". It may be on two balanced lines, with the exact mixed-case spelling IMDb, and must be immediately legible at small card size. No outlines, bevels, drop shadows, slogans or ornamental frame around the title. Color original: restrained warm cream, charcoal, soft burnt umber/rust and muted midnight blue, with modest atmospheric warmth; refined color rather than flashy saturation or metallic gold. Maintain strong tonal contrast so the same artwork also looks superb in grayscale. Preserve the three-world film-history subject and original tall proportion; do not turn it into an unrelated scene, movie still, logo, app screenshot or generic geometric placeholder. The result should feel curated, literary and timeless, not a fantasy-game cover.
