import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { Movie } from "../api/client";
import { MoviePosterArtwork } from "./MoviePosterArtwork";
import { publicPath } from "../data/public-path";

export function Poster({
  movie,
  watched,
  collectionSlug,
}: {
  movie: Movie;
  watched: boolean;
  collectionSlug: string;
}) {
  const { t } = useTranslation();
  const [failedURL, setFailedURL] = useState<string | null>(null);
  return movie.posterUrl && failedURL !== movie.posterUrl ? (
    <img
      src={publicPath(movie.posterUrl)}
      alt={t("movie.poster", { title: movie.title })}
      loading="lazy"
      width="500"
      height="750"
      onError={() => setFailedURL(movie.posterUrl)}
      className={`h-full w-full object-cover transition duration-500 group-hover:scale-[1.03] ${watched ? "" : "poster-unwatched"}`}
      style={{ filter: watched ? "none" : "grayscale(100%)" }}
    />
  ) : (
    <MoviePosterArtwork
      movie={movie}
      collectionSlug={collectionSlug}
      watched={watched}
    />
  );
}
