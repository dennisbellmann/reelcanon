import { useTranslation } from "react-i18next";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { catalogApi } from "../api/client";
import type { MovieProgress } from "../data/progress";
import { collectionArtwork } from "../data/collection-artwork";
import { CollectionCard } from "../components/CollectionCard";

export function CanonIndexPage({ progress }: { progress: MovieProgress[] }) {
  const { t } = useTranslation();
  const watchedIds = useMemo(
    () =>
      new Set(
        progress.filter((movie) => movie.watched).map((movie) => movie.imdbId),
      ),
    [progress],
  );
  const query = useQuery({
    queryKey: ["canons"],
    queryFn: catalogApi.listCanons,
    refetchInterval: 60000,
  });

  if (query.isPending)
    return <p className="text-muted">{t("index.loading")}</p>;
  if (query.isError)
    return (
      <section className="mx-auto max-w-2xl py-20 text-center">
        <h1 className="mt-4 font-display text-4xl">{t("index.error")}</h1>
        <p className="mt-4 text-muted">{t("index.errorHint")}</p>
        <button
          className="mt-6 border border-line px-4 py-2"
          onClick={() => void query.refetch()}
        >
          {t("common.retry")}
        </button>
      </section>
    );

  return (
    <section>
      <div className="border-b border-line pb-10">
        <h1 className="mt-5 max-w-3xl font-display text-4xl leading-[1.02] tracking-tight sm:text-7xl">
          {t("index.title")}
        </h1>
        <p className="mt-5 max-w-2xl text-base leading-7 text-muted">
          {t("index.intro")}
        </p>
      </div>
      {query.data.length === 0 ? (
        <div className="mx-auto max-w-xl py-20 text-center">
          <p className="eyebrow">{t("index.empty")}</p>
          <p className="mt-4 leading-7 text-muted">{t("index.emptyHint")}</p>
        </div>
      ) : (
        <div className="grid gap-x-6 gap-y-10 pt-8 sm:grid-cols-2 lg:grid-cols-3">
          {query.data.map((canon) => (
            <CollectionCard
              key={canon.slug}
              canon={canon}
              artwork={collectionArtwork(canon)}
              watchedIds={watchedIds}
            />
          ))}
        </div>
      )}
    </section>
  );
}
