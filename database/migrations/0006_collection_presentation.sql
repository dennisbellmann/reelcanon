ALTER TABLE canons ADD COLUMN display_name TEXT;
ALTER TABLE canons ADD COLUMN display_description TEXT;
ALTER TABLE canons ADD COLUMN display_position INTEGER NOT NULL DEFAULT 0 CHECK (display_position >= 0);
ALTER TABLE canons ADD COLUMN poster_url TEXT NOT NULL DEFAULT '/artwork/collection-03-filmwelten.png';
ALTER TABLE canons ADD COLUMN poster_title_embedded BOOLEAN NOT NULL DEFAULT false;

UPDATE canons SET display_position=1,
    poster_url='/artwork/imdb-top-250-filmwelten-editorial.png',
    poster_title_embedded=true
WHERE slug='imdb-top-250';
