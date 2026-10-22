import {
  errorMessage,
  renderMessage,
  type LocalizedMessage,
} from "../i18n-message";
import { useTranslation } from "react-i18next";
import { formatNumber } from "../i18n";
import { useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { catalogApi } from "../api/client";
import type { MovieProgress } from "../data/progress";
import { progressRepository } from "../data/progress";
import { Poster } from "../components/Poster";

interface Props {
  progress: MovieProgress[];
  refreshProgress: () => Promise<void>;
}

export function MoviePage({ progress, refreshProgress }: Props) {
  const { t } = useTranslation();
  const [error, setError] = useState<LocalizedMessage | null>(null);
  const [busy, setBusy] = useState(false);
  const { canonSlug = "", imdbId = "" } = useParams();
  const location = useLocation();
  const collectionPath = `/canons/${encodeURIComponent(canonSlug)}`;
  const origin = location.state?.collection;
  const fromCollection =
    origin?.pathname === collectionPath &&
    typeof origin?.search === "string" &&
    (!origin.search || origin.search.startsWith("?")) &&
    typeof origin?.key === "string";
  const returnTo = fromCollection
    ? `${collectionPath}${origin.search}`
    : collectionPath;
  const returnState = fromCollection
    ? { restoreScrollKey: origin.key }
    : undefined;
  const canonsQuery = useQuery({
    queryKey: ["movie-canons", imdbId],
    queryFn: () => catalogApi.movieCanons(imdbId),
    enabled: imdbId !== "",
  });
  const query = useQuery({
    queryKey: ["canon", canonSlug],
    queryFn: () => catalogApi.getCanon(canonSlug),
  });
  if (query.isPending)
    return <p className="text-muted">{t("movie.loading")}</p>;
  if (query.isError) return <p className="text-muted">{t("movie.error")}</p>;
  const movie = query.data?.movies.find((item) => item.imdbId === imdbId);
  if (!movie)
    return (
      <div>
        <Link
          className="eyebrow hover:text-accent"
          to={returnTo}
          state={returnState}
        >
          {t("movie.back")}
        </Link>
        <h1 className="mt-8 font-display text-4xl">{t("movie.notFound")}</h1>
      </div>
    );
  const watched =
    progress.find((item) => item.imdbId === movie.imdbId)?.watched ?? false;
  const watchlist =
    progress.find((item) => item.imdbId === movie.imdbId)?.watchlist ?? false;
  async function save(field: "watched" | "watchlist") {
    if (!movie) return;
    setBusy(true);
    setError(null);
    try {
      if (field === "watched")
        await progressRepository.setWatched(movie.imdbId, !watched);
      else await progressRepository.setWatchlist(movie.imdbId, !watchlist);
      await refreshProgress();
    } catch (cause) {
      await refreshProgress();
      setError(errorMessage(cause, "common.saveError"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="mx-auto max-w-5xl">
      <Link
        className="eyebrow hover:text-accent"
        to={returnTo}
        state={returnState}
      >
        {t("movie.back")}
      </Link>
      <div className="mt-8 grid gap-10 md:grid-cols-[minmax(220px,340px)_1fr] md:gap-16">
        <div className="poster-frame aspect-[2/3] bg-[#e5e0d6]">
          <Poster movie={movie} watched={watched} collectionSlug={canonSlug} />
        </div>
        <div className="py-2">
          <p className="eyebrow">
            {t("movie.position", {
              name: query.data.canon.name,
              position: movie.position,
            })}
          </p>
          <h1 className="mt-4 font-display text-5xl leading-none sm:text-7xl">
            {movie.title}
          </h1>
          {movie.originalTitle && movie.originalTitle !== movie.title && (
            <p className="mt-3 font-display text-2xl text-muted">
              {movie.originalTitle}
            </p>
          )}
          <p className="mt-6 text-sm text-muted">
            {[
              movie.releaseYear,
              movie.runtimeMinutes
                ? t("movie.runtime", { minutes: movie.runtimeMinutes })
                : null,
              movie.directors.join(", "),
              movie.countries.join(", "),
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
          {movie.imdbRating !== null && (
            <p className="mt-4 text-sm">
              IMDb{" "}
              <strong>
                {formatNumber(movie.imdbRating, {
                  minimumFractionDigits: 1,
                  maximumFractionDigits: 1,
                })}
              </strong>
            </p>
          )}
          {movie.genres.length > 0 && (
            <p className="mt-4 text-sm text-muted">
              {movie.genres.join(" · ")}
            </p>
          )}
          <a
            className="mt-4 inline-block text-sm underline"
            href={`https://www.imdb.com/title/${movie.imdbId}/`}
            target="_blank"
            rel="noreferrer"
          >
            {t("movie.imdb")}
          </a>
          <p className="mt-8 max-w-xl text-lg leading-8">
            {movie.overview || t("movie.noOverview")}
          </p>
          <button
            type="button"
            aria-pressed={watched}
            disabled={busy}
            onClick={() => void save("watched")}
            className="mt-10 border border-ink px-5 py-3 text-xs uppercase tracking-[0.16em] transition hover:bg-ink hover:text-paper"
          >
            {watched ? t("movie.markUnwatched") : t("movie.markWatched")}
          </button>
          <button
            type="button"
            disabled={busy}
            aria-pressed={watchlist}
            onClick={() => void save("watchlist")}
            className="mt-3 border border-line px-5 py-3 text-xs uppercase tracking-[0.16em] transition hover:border-ink sm:ml-3"
          >
            {progress.find((item) => item.imdbId === movie.imdbId)?.watchlist
              ? t("movie.removeWatchlist")
              : t("movie.addWatchlist")}
          </button>
          {error && (
            <p role="alert" className="mt-4 text-accent">
              {renderMessage(error)}
            </p>
          )}
          {canonsQuery.data && canonsQuery.data.length > 0 && (
            <div className="mt-8">
              <p className="eyebrow">{t("movie.collections")}</p>
              <ul className="mt-3 flex flex-wrap gap-3">
                {canonsQuery.data.map((canon) => (
                  <li key={canon.slug}>
                    <Link
                      className="text-sm underline"
                      to={`/canons/${encodeURIComponent(canon.slug)}/movies/${encodeURIComponent(movie.imdbId)}`}
                    >
                      {canon.name}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </article>
  );
}
