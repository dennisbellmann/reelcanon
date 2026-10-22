import { apiBase } from "../data/public-path";

export interface Canon {
  slug: string;
  name: string;
  description: string;
  source: string;
  filmCount: number;
  currentEdition: string | null;
  imdbIds?: string[];
  displayPosition?: number;
  posterUrl?: string;
  posterTitleEmbedded?: boolean;
}

export interface Movie {
  imdbId: string;
  tmdbId: number | null;
  title: string;
  originalTitle: string | null;
  releaseYear: number | null;
  runtimeMinutes: number | null;
  overview: string;
  posterUrl: string | null;
  imdbRating: number | null;
  directors: string[];
  genres: string[];
  countries: string[];
  position: number;
}

export interface CanonCatalog {
  canon: Canon;
  movies: Movie[];
}

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    headers: { Accept: "application/json" },
    cache: "no-cache",
    signal: AbortSignal.timeout(15000),
  });
  if (!response.ok)
    throw new Error(`Catalog request failed (${response.status})`);
  return response.json() as Promise<T>;
}

export const catalogApi = {
  movieCanons: (imdbId: string) =>
    request<Canon[]>(`/movies/${encodeURIComponent(imdbId)}/canons`),
  listCanons: () => request<Canon[]>("/canons"),
  getCanon: (slug: string) =>
    request<CanonCatalog>(`/canons/${encodeURIComponent(slug)}`),
};
