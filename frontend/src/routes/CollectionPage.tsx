import {
  errorMessage,
  renderMessage,
  type LocalizedMessage,
} from "../i18n-message";
import { useTranslation } from "react-i18next";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  Link,
  useLocation,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { catalogApi, type Movie } from "../api/client";
import type { MovieProgress } from "../data/progress";
import { progressRepository } from "../data/progress";
import { scrollRestorationKey } from "../data/navigation";
import {
  defaultFilters,
  selectMovies,
  type Filters,
} from "../data/catalog-view";
import { Poster } from "../components/Poster";
import { BottomSheet } from "../components/BottomSheet";

interface Props {
  progress: MovieProgress[];
  refreshProgress: () => Promise<void>;
}

function storedHideWatched(slug: string) {
  try {
    return (
      window.localStorage.getItem(`reelcanon-hide-watched:${slug}`) === "true"
    );
  } catch {
    return false;
  }
}

export function CollectionPage({ progress, refreshProgress }: Props) {
  const { t } = useTranslation();
  const { canonSlug = "" } = useParams();
  const location = useLocation();
  const collectionOrigin = {
    pathname: location.pathname,
    search: location.search,
    key: scrollRestorationKey(location),
  };
  const [params, setParams] = useSearchParams();
  const [hideWatchedPreference, setHideWatchedPreference] = useState<{
    slug: string;
    value: boolean;
  } | null>(null);
  const hideWatched =
    hideWatchedPreference?.slug === canonSlug
      ? hideWatchedPreference.value
      : storedHideWatched(canonSlug);
  const filters = Object.fromEntries(
    Object.entries(defaultFilters).map(([key, value]) => [
      key,
      params.get(key) ?? value,
    ]),
  ) as unknown as Filters;
  const gallery = ["gallery", "album"].includes(params.get("view") ?? "");
  const [sheet, setSheet] = useState<"filters" | "details" | null>(null);
  const [error, setError] = useState<LocalizedMessage | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const touchStart = useRef<number | null>(null);
  const decadeNavigation = useRef<HTMLElement | null>(null);
  const query = useQuery({
    queryKey: ["canon", canonSlug],
    queryFn: () => catalogApi.getCanon(canonSlug),
    refetchInterval: 60000,
  });
  const stateById = useMemo(
    () => new Map(progress.map((item) => [item.imdbId, item])),
    [progress],
  );
  const movies = query.data?.movies ?? [];
  const decades = [
    ...new Set(
      movies.flatMap((movie) =>
        movie.releaseYear ? [Math.floor(movie.releaseYear / 10) * 10] : [],
      ),
    ),
  ].sort((a, b) => a - b);
  const decadeProgress = decades.map((decade) => {
    const films = movies.filter(
      (movie) =>
        movie.releaseYear !== null &&
        Math.floor(movie.releaseYear / 10) * 10 === decade,
    );
    const watched = films.filter(
      (movie) => stateById.get(movie.imdbId)?.watched,
    ).length;
    return { decade, watched, total: films.length };
  });
  const requestedGalleryDecade =
    params.get("galleryDecade") ??
    (filters.decade === "all" ? null : filters.decade);
  const activeGalleryDecade =
    requestedGalleryDecade !== null &&
    decades.includes(Number(requestedGalleryDecade))
      ? Number(requestedGalleryDecade)
      : (decades[0] ?? null);
  const galleryMovies = selectMovies(movies, progress, {
    ...filters,
    decade: "all",
    sort: "year",
  }).filter(
    (movie) =>
      activeGalleryDecade === null ||
      (movie.releaseYear !== null &&
        Math.floor(movie.releaseYear / 10) * 10 === activeGalleryDecade),
  );
  const matchingMovies = gallery
    ? galleryMovies
    : selectMovies(movies, progress, filters);
  const visibleMovies = hideWatched
    ? matchingMovies.filter((movie) => !stateById.get(movie.imdbId)?.watched)
    : matchingMovies;
  const watchedCount = movies.filter(
    (movie) => stateById.get(movie.imdbId)?.watched,
  ).length;
  const watchedPercent = movies.length
    ? Math.round((watchedCount / movies.length) * 100)
    : 0;
  const activeDecadeProgress = decadeProgress.find(
    ({ decade }) => decade === activeGalleryDecade,
  ) ?? { watched: 0, total: 0 };
  const selectedDecade =
    activeGalleryDecade === null ? -1 : decades.indexOf(activeGalleryDecade);
  useEffect(() => {
    decadeNavigation.current
      ?.querySelector<HTMLButtonElement>('[aria-pressed="true"]')
      ?.scrollIntoView?.({
        block: "nearest",
        inline: "nearest",
        behavior: "smooth",
      });
  }, [activeGalleryDecade]);
  function setFilter(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (
      value === defaultFilters[key as keyof Filters] ||
      (key === "view" && value === "library")
    )
      next.delete(key);
    else next.set(key, value);
    if (key === "decade") next.delete("galleryDecade");
    setParams(next, { replace: true, preventScrollReset: true });
  }
  function setHideWatched(value: boolean) {
    setHideWatchedPreference({ slug: canonSlug, value });
    try {
      window.localStorage.setItem(
        `reelcanon-hide-watched:${canonSlug}`,
        String(value),
      );
    } catch {
      // Keep the current choice for this page when browser storage is unavailable.
    }
  }
  function nextDecade(direction: number) {
    const value = decades[selectedDecade + direction];
    if (value !== undefined) {
      const next = new URLSearchParams(params);
      next.set("galleryDecade", String(value));
      setParams(next, { replace: true, preventScrollReset: true });
    }
  }
  function selectDecade(decade: number) {
    const next = new URLSearchParams(params);
    next.set("galleryDecade", String(decade));
    setParams(next, { replace: true, preventScrollReset: true });
  }
  async function toggleWatched(movie: Movie) {
    setBusy(movie.imdbId);
    setError(null);
    try {
      await progressRepository.setWatched(
        movie.imdbId,
        !stateById.get(movie.imdbId)?.watched,
      );
      await refreshProgress();
    } catch (cause) {
      await refreshProgress();
      setError(errorMessage(cause, "common.saveError"));
    } finally {
      setBusy(null);
    }
  }
  if (query.isPending)
    return (
      <p role="status" className="text-muted">
        {t("collection.loading")}
      </p>
    );
  if (query.isError)
    return (
      <section className="py-20">
        <h1 className="font-display text-4xl">{t("collection.error")}</h1>
        <p className="mt-4 text-muted">{t("collection.errorHint")}</p>
        <button
          className="mt-6 border px-4 py-2"
          onClick={() => void query.refetch()}
        >
          {t("common.retry")}
        </button>
        <Link className="ml-4 underline" to="/canons">
          {t("collection.back")}
        </Link>
      </section>
    );
  const canon = query.data.canon;
  const options: {
    key: keyof Filters;
    label: string;
    values: [string, string][];
  }[] = [
    {
      key: "state",
      label: t("filter.state"),
      values: [
        ["all", t("filter.allFilms")],
        ["watched", t("filter.watched")],
        ["unwatched", t("filter.unwatched")],
        ["watchlist", t("filter.watchlist")],
      ],
    },
    {
      key: "decade",
      label: t("filter.decade"),
      values: [
        ["all", t("filter.allDecades")],
        ...decades.map((year): [string, string] => [
          String(year),
          t("collection.decade", { year }),
        ]),
      ],
    },
    {
      key: "year",
      label: t("filter.year"),
      values: [
        ["all", t("filter.allYears")],
        ...[
          ...new Set(
            movies.flatMap((movie) =>
              movie.releaseYear ? [movie.releaseYear] : [],
            ),
          ),
        ]
          .sort((a, b) => a - b)
          .map((year): [string, string] => [String(year), String(year)]),
      ],
    },
    {
      key: "genre",
      label: t("filter.genre"),
      values: [
        ["all", t("filter.allGenres")],
        ...[...new Set(movies.flatMap((movie) => movie.genres))]
          .sort()
          .map((value): [string, string] => [value, value]),
      ],
    },
    {
      key: "director",
      label: t("filter.director"),
      values: [
        ["all", t("filter.allDirectors")],
        ...[...new Set(movies.flatMap((movie) => movie.directors))]
          .sort()
          .map((value): [string, string] => [value, value]),
      ],
    },
    {
      key: "rating",
      label: t("filter.rating"),
      values: [
        ["all", t("filter.allRatings")],
        ["7", "IMDb ≥ 7"],
        ["8", "IMDb ≥ 8"],
        ["9", "IMDb ≥ 9"],
      ],
    },
    {
      key: "sort",
      label: t("filter.sort"),
      values: [
        ["rank", t("filter.rank")],
        ["title", t("filter.title")],
        ["year", t("filter.oldest")],
        ["newest", t("filter.newest")],
        ["rating", t("filter.imdb")],
        ["recent", t("filter.recent")],
      ],
    },
  ];
  const activeFilters = options.filter(
    ({ key }) => filters[key] !== defaultFilters[key],
  ).length;
  return (
    <>
      <section className="border-b border-line pb-6">
        <div className="grid gap-5 lg:grid-cols-[1fr_auto] lg:items-end">
          <div>
            <h1 className="max-w-3xl font-display text-5xl leading-[1.02] tracking-tight sm:text-6xl">
              <button
                type="button"
                className="text-left transition hover:text-accent"
                aria-haspopup="dialog"
                aria-expanded={sheet === "details"}
                aria-controls={
                  sheet === "details" ? "collection-details" : undefined
                }
                onClick={() => setSheet("details")}
              >
                {canon.name}
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  className="ml-3 inline-block h-5 w-5 align-middle text-muted sm:h-6 sm:w-6"
                >
                  <circle cx="12" cy="12" r="9" />
                  <path d="M12 11v6M12 7v1" />
                </svg>
              </button>
            </h1>
          </div>
          <div className="min-w-48 border-l border-line pl-4">
            <div className="flex items-baseline justify-between gap-3">
              <p className="eyebrow whitespace-nowrap">
                {t("collection.progress")}
              </p>
              <p className="font-display text-2xl leading-none">
                {watchedCount}{" "}
                <span className="text-muted">/ {canon.filmCount}</span>
              </p>
            </div>
            <div
              role="progressbar"
              aria-label={t("collection.watched")}
              aria-valuenow={watchedPercent}
              aria-valuemin={0}
              aria-valuemax={100}
              className="mt-2 h-1 w-48 bg-line"
            >
              <div
                className="h-full bg-accent transition-all"
                style={{ width: `${watchedPercent}%` }}
              />
            </div>
            <p className="mt-1 text-xs text-muted">
              {t("collection.percent", { percent: watchedPercent })}
            </p>
          </div>
        </div>
      </section>
      <section className="py-5">
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <div
            role="group"
            aria-label={t("collection.view")}
            className="flex gap-2"
          >
            <button
              aria-pressed={!gallery}
              className="border border-line px-4 py-2 aria-pressed:border-ink aria-pressed:bg-ink aria-pressed:text-paper"
              onClick={() => setFilter("view", "library")}
            >
              {t("filter.allFilms")}
            </button>
            <button
              aria-pressed={gallery}
              className="border border-line px-4 py-2 aria-pressed:border-ink aria-pressed:bg-ink aria-pressed:text-paper"
              onClick={() => setFilter("view", "gallery")}
            >
              {t("collection.galleryToggle")}
            </button>
          </div>
        </div>
        <div className="sticky top-0 z-30 -mx-6 border-y border-line bg-paper/95 px-6 py-2 backdrop-blur-sm lg:-mx-10 lg:px-10">
          <div className="flex flex-wrap items-center gap-3">
            {!gallery && (
              <label
                className="min-w-0 flex-[1_1_14rem]"
                htmlFor="movie-search"
              >
                <span className="sr-only">{t("collection.search")}</span>
                <input
                  id="movie-search"
                  className="block h-11 w-full border border-line bg-transparent px-3 py-2 text-sm text-ink"
                  placeholder={t("collection.searchHint")}
                  value={filters.search}
                  onChange={(event) => setFilter("search", event.target.value)}
                />
              </label>
            )}
            {!gallery && (
              <button
                type="button"
                className="flex h-11 items-center gap-2 border border-line px-4 text-sm hover:border-ink"
                aria-haspopup="dialog"
                aria-expanded={sheet === "filters"}
                aria-controls={
                  sheet === "filters" ? "collection-filters" : undefined
                }
                onClick={() => setSheet("filters")}
              >
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  className="h-4 w-4"
                >
                  <path d="M4 7h16M4 17h16M9 4v6M15 14v6" />
                </svg>
                {t("collection.filters")}
                {activeFilters > 0 && (
                  <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-accent px-1 text-xs text-white">
                    {activeFilters}
                    <span className="sr-only"> {t("collection.active")}</span>
                  </span>
                )}
              </button>
            )}
            {gallery && decades.length > 0 && (
              <div className="min-w-0 flex-[1_1_18rem]">
                <div className="flex items-end justify-between gap-4">
                  <h3 className="font-display text-4xl">
                    {t("collection.decade", { year: activeGalleryDecade })}
                  </h3>
                  <p className="pb-1 text-right text-sm text-muted">
                    {t("collection.progressSummary", {
                      watched: activeDecadeProgress.watched,
                      total: activeDecadeProgress.total,
                    })}{" "}
                    ·{" "}
                    {activeDecadeProgress.total
                      ? Math.round(
                          (activeDecadeProgress.watched /
                            activeDecadeProgress.total) *
                            100,
                        )
                      : 0}
                    %
                    {activeDecadeProgress.total > 0 &&
                    activeDecadeProgress.watched ===
                      activeDecadeProgress.total
                      ? t("collection.allWatched")
                      : ""}
                  </p>
                </div>
                <nav
                  ref={decadeNavigation}
                  className="mt-3 overflow-x-auto border-t border-line pt-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
                  tabIndex={0}
                  aria-label={t("collection.browseDecades")}
                  onKeyDown={(event) => {
                    if (event.key === "ArrowLeft") {
                      event.preventDefault();
                      nextDecade(-1);
                    }
                    if (event.key === "ArrowRight") {
                      event.preventDefault();
                      nextDecade(1);
                    }
                  }}
                >
                  <div className="flex min-w-max items-center gap-2">
                    {decadeProgress.map(({ decade, watched, total }) => (
                      <button
                        key={decade}
                        type="button"
                        aria-pressed={activeGalleryDecade === decade}
                        onClick={() => selectDecade(decade)}
                        className="group flex min-w-14 flex-col items-center gap-2 rounded-sm px-1 py-2 text-xs text-muted transition-colors hover:text-ink aria-pressed:font-semibold aria-pressed:text-accent"
                      >
                        <span>{t("collection.decade", { year: decade })}</span>
                        <span
                          aria-hidden="true"
                          className="block h-0.5 w-full bg-line"
                        >
                          <span
                            className="block h-full bg-current transition-[width] duration-300"
                            style={{
                              width: `${total ? (watched / total) * 100 : 0}%`,
                            }}
                          />
                        </span>
                        <span className="sr-only">
                          {t("collection.progressSummary", { watched, total })}
                        </span>
                      </button>
                    ))}
                  </div>
                </nav>
              </div>
            )}
            <label className="ml-auto flex h-11 shrink-0 cursor-pointer select-none items-center gap-3 border border-line px-3 text-sm hover:border-ink">
              <input
                type="checkbox"
                role="switch"
                className="peer sr-only"
                checked={hideWatched}
                onChange={(event) => setHideWatched(event.target.checked)}
              />
              <span>{t("collection.hideWatched")}</span>
              <span
                className={`relative h-5 w-9 shrink-0 rounded-full border transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-accent peer-focus-visible:ring-offset-2 ${hideWatched ? "border-accent bg-accent" : "border-line"}`}
              >
                <span
                  className={`absolute left-0.5 top-0.5 size-3.5 rounded-full transition-all ${hideWatched ? "translate-x-4 bg-paper" : "bg-muted"}`}
                />
              </span>
            </label>
          </div>
        </div>
        {error && (
          <p role="alert" className="mt-6 text-accent">
            {renderMessage(error)}
          </p>
        )}
        {!gallery && (
          <p role="status" className="mt-4 text-xs text-muted">
            {t("collection.results", {
              count: visibleMovies.length,
              total: movies.length,
            })}
          </p>
        )}
        <div
          className={gallery ? "[touch-action:pan-y]" : undefined}
          onTouchStart={(event) => {
            if (gallery) touchStart.current = event.touches[0]?.clientX ?? null;
          }}
          onTouchEnd={(event) => {
            if (gallery && touchStart.current !== null) {
              const distance =
                (event.changedTouches[0]?.clientX ?? touchStart.current) -
                touchStart.current;
              if (Math.abs(distance) > 50) nextDecade(distance < 0 ? 1 : -1);
            }
            touchStart.current = null;
          }}
        >
          {!visibleMovies.length ? (
            <p className="py-16 text-center text-muted">
              {t("collection.empty")}
            </p>
          ) : (
            <div
              className={
                gallery
                  ? "grid grid-cols-3 gap-2 pt-7 sm:grid-cols-4 sm:gap-3 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8"
                  : "grid grid-cols-2 gap-x-4 gap-y-8 pt-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
              }
            >
              {visibleMovies.map((movie) => {
                const watched = stateById.get(movie.imdbId)?.watched ?? false;
                const destination = `/canons/${encodeURIComponent(canonSlug)}/movies/${encodeURIComponent(movie.imdbId)}`;
                return (
                  <article
                    key={movie.imdbId}
                    className="group relative min-w-0"
                  >
                    <Link
                      to={destination}
                      state={{ collection: collectionOrigin }}
                      aria-label={gallery ? movie.title : undefined}
                      className="poster-frame relative block aspect-[2/3] overflow-hidden bg-[#e5e0d6]"
                    >
                      <Poster
                        movie={movie}
                        watched={watched}
                        collectionSlug={canonSlug}
                      />
                      {!gallery && (
                        <span className="absolute left-2 top-2 bg-paper/90 px-2 py-1 text-[10px] tracking-widest">
                          {String(movie.position).padStart(3, "0")}
                        </span>
                      )}
                      {!gallery && watched && (
                        <span className="absolute bottom-2 right-2 bg-accent px-2 py-1 text-[9px] uppercase tracking-widest text-white">
                          {t("filter.watched")}
                        </span>
                      )}
                      {!gallery && stateById.get(movie.imdbId)?.watchlist && (
                        <span className="absolute bottom-2 left-2 bg-paper px-2 py-1 text-[9px]">
                          {t("filter.watchlist")}
                        </span>
                      )}
                    </Link>
                    {!gallery && (
                      <div className="flex items-start justify-between gap-2 pt-3">
                        <div className="min-w-0">
                          <Link
                            to={destination}
                            state={{ collection: collectionOrigin }}
                            className="line-clamp-2 font-display text-lg leading-tight hover:text-accent"
                          >
                            {movie.title}
                          </Link>
                          <p className="mt-1 text-xs text-muted">
                            {movie.releaseYear ?? "—"}
                            {movie.directors[0]
                              ? ` · ${movie.directors[0]}`
                              : ""}
                          </p>
                        </div>
                        <button
                          type="button"
                          disabled={busy !== null}
                          aria-busy={busy === movie.imdbId}
                          onClick={() => void toggleWatched(movie)}
                          aria-label={
                            watched
                              ? t("movie.unwatchedLabel", {
                                  title: movie.title,
                                })
                              : t("movie.watchedLabel", {
                                  title: movie.title,
                                })
                          }
                          aria-pressed={watched}
                          className={
                            watched
                              ? "watched-toggle h-9 w-9 shrink-0 border border-accent bg-accent text-white"
                              : "watched-toggle h-9 w-9 shrink-0 border border-line hover:border-ink"
                          }
                        >
                          {watched ? "✓" : "+"}
                        </button>
                      </div>
                    )}
                  </article>
                );
              })}
            </div>
          )}
        </div>
      </section>
      {sheet === "filters" && (
        <BottomSheet
          id="collection-filters"
          title={t("collection.filters")}
          onClose={() => setSheet(null)}
          footer={
            <div className="flex flex-wrap items-center justify-between gap-3">
              <button
                className="min-h-11 text-sm underline underline-offset-4 hover:text-accent"
                onClick={() =>
                  setParams(gallery ? { view: "gallery" } : {}, {
                    replace: true,
                    preventScrollReset: true,
                  })
                }
              >
                {t("collection.reset")}
              </button>
              <button
                className="min-h-11 bg-ink px-5 py-3 text-sm text-paper hover:bg-accent"
                onClick={() => setSheet(null)}
              >
                {t("collection.show", { count: visibleMovies.length })}
              </button>
            </div>
          }
        >
          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
            {options.map((option) => (
              <label key={option.key} className="min-w-0 text-xs text-muted">
                {option.label}
                <select
                  aria-label={option.label}
                  className="mt-2 block min-h-11 w-full min-w-0 border border-line bg-paper px-3 py-2 text-base text-ink"
                  value={filters[option.key]}
                  onChange={(event) =>
                    setFilter(option.key, event.target.value)
                  }
                >
                  {option.values.map(([value, label]) => (
                    <option key={value} value={value}>
                      {label}
                    </option>
                  ))}
                </select>
              </label>
            ))}
          </div>
        </BottomSheet>
      )}
      {sheet === "details" && (
        <BottomSheet
          id="collection-details"
          title={canon.name}
          onClose={() => setSheet(null)}
        >
          <p className="max-w-2xl leading-7 text-muted">{canon.description}</p>
          <dl className="mt-6 grid grid-cols-[auto_1fr] gap-x-6 gap-y-4 text-sm">
            <dt className="text-muted">{t("collection.scope")}</dt>
            <dd>{t("common.films", { count: canon.filmCount })}</dd>
            <dt className="text-muted">{t("collection.edition")}</dt>
            <dd className="break-words">{canon.currentEdition ?? "—"}</dd>
            <dt className="text-muted">{t("collection.source")}</dt>
            <dd>
              <a
                className="underline underline-offset-4 hover:text-accent"
                href={canon.source}
                target="_blank"
                rel="noreferrer"
              >
                {t("collection.openSource")}
              </a>
            </dd>
          </dl>
        </BottomSheet>
      )}
    </>
  );
}
