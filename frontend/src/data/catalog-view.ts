import { currentLanguage } from "../i18n";
import type { Movie } from "../api/client";
import type { MovieProgress } from "./progress";

export interface Filters {
  search: string;
  decade: string;
  year: string;
  genre: string;
  director: string;
  rating: string;
  state: string;
  sort: string;
}
export const defaultFilters: Filters = {
  search: "",
  decade: "all",
  year: "all",
  genre: "all",
  director: "all",
  rating: "all",
  state: "all",
  sort: "rank",
};
export function selectMovies(
  movies: Movie[],
  progress: MovieProgress[],
  filters: Filters,
): Movie[] {
  const states = new Map(progress.map((row) => [row.imdbId, row]));
  const term = filters.search.trim().toLocaleLowerCase();
  return movies
    .filter((movie) => {
      const state = states.get(movie.imdbId);
      return (
        (!term ||
          `${movie.title} ${movie.originalTitle ?? ""} ${movie.directors.join(" ")}`
            .toLocaleLowerCase()
            .includes(term)) &&
        (filters.decade === "all" ||
          (movie.releaseYear !== null &&
            Math.floor(movie.releaseYear / 10) * 10 ===
              Number(filters.decade))) &&
        (filters.year === "all" ||
          movie.releaseYear === Number(filters.year)) &&
        (filters.genre === "all" || movie.genres.includes(filters.genre)) &&
        (filters.director === "all" ||
          movie.directors.includes(filters.director)) &&
        (filters.rating === "all" ||
          (movie.imdbRating !== null &&
            movie.imdbRating >= Number(filters.rating))) &&
        (filters.state === "all" ||
          (filters.state === "watched" && state?.watched) ||
          (filters.state === "unwatched" && !state?.watched) ||
          (filters.state === "watchlist" && state?.watchlist))
      );
    })
    .sort((a, b) => {
      let order = 0;
      switch (filters.sort) {
        case "title":
          order = a.title.localeCompare(b.title, currentLanguage());
          break;
        case "year":
          order = (a.releaseYear ?? Infinity) - (b.releaseYear ?? Infinity);
          break;
        case "newest":
          order = (b.releaseYear ?? -Infinity) - (a.releaseYear ?? -Infinity);
          break;
        case "rating":
          order = (b.imdbRating ?? -Infinity) - (a.imdbRating ?? -Infinity);
          break;
        case "recent":
          order =
            (Date.parse(states.get(b.imdbId)?.watchedAt ?? "") || 0) -
            (Date.parse(states.get(a.imdbId)?.watchedAt ?? "") || 0);
          break;
        default:
          order = a.position - b.position;
      }
      return (Number.isNaN(order) ? 0 : order) || a.position - b.position;
    });
}
