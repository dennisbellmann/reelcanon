import { describe, expect, it } from "vitest";
import type { Movie } from "../api/client";
import { defaultFilters, selectMovies } from "./catalog-view";

const movie: Movie = {
  imdbId: "tt0111161",
  tmdbId: 278,
  title: "Die Verurteilten",
  originalTitle: "The Shawshank Redemption",
  releaseYear: 1994,
  runtimeMinutes: 142,
  overview: "",
  posterUrl: null,
  imdbRating: 9.3,
  directors: ["Frank Darabont"],
  genres: ["Drama"],
  countries: ["US"],
  position: 1,
};
const movies = [
  movie,
  {
    ...movie,
    imdbId: "tt0068646",
    title: "Der Pate",
    originalTitle: "The Godfather",
    releaseYear: 1972,
    imdbRating: 9.2,
    directors: ["Francis Ford Coppola"],
    position: 2,
  },
  {
    ...movie,
    imdbId: "tt0000001",
    title: "Missing metadata",
    originalTitle: null,
    releaseYear: null,
    imdbRating: null,
    directors: [],
    genres: [],
    position: 3,
  },
];
const progress = [
  {
    imdbId: "tt0111161",
    watched: true,
    watchlist: false,
    watchedAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
  },
  {
    imdbId: "tt0068646",
    watched: false,
    watchlist: true,
    watchedAt: null,
    updatedAt: "2026-10-01T00:00:00Z",
  },
];
describe("catalog filtering and sorting", () => {
  it("searches original titles and directors over the entire catalog", () => {
    expect(
      selectMovies(movies, progress, {
        ...defaultFilters,
        search: "shawshank",
      }).map((row) => row.imdbId),
    ).toEqual(["tt0111161"]);
    expect(
      selectMovies(movies, progress, {
        ...defaultFilters,
        search: "COPPOLA",
      }).map((row) => row.imdbId),
    ).toEqual(["tt0068646"]);
  });
  it("combines local states and metadata filters", () => {
    expect(
      selectMovies(movies, progress, {
        ...defaultFilters,
        state: "unwatched",
        decade: "1970",
        genre: "Drama",
        director: "Francis Ford Coppola",
        rating: "9",
        year: "1972",
      }).map((row) => row.imdbId),
    ).toEqual(["tt0068646"]);
    expect(
      selectMovies(movies, progress, {
        ...defaultFilters,
        state: "watchlist",
      }).map((row) => row.imdbId),
    ).toEqual(["tt0068646"]);
    expect(
      selectMovies(movies, progress, {
        ...defaultFilters,
        state: "watched",
      }).map((row) => row.imdbId),
    ).toEqual(["tt0111161"]);
  });
  it("keeps missing metadata visible and sorts null values last", () => {
    expect(selectMovies(movies, progress, defaultFilters)).toHaveLength(3);
    expect(
      selectMovies(movies, progress, { ...defaultFilters, sort: "year" }).map(
        (row) => row.imdbId,
      ),
    ).toEqual(["tt0068646", "tt0111161", "tt0000001"]);
    expect(
      selectMovies(movies, progress, { ...defaultFilters, sort: "rating" }).at(
        -1,
      )?.imdbId,
    ).toBe("tt0000001");
  });
});
