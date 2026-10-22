import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { useState } from "react";
import type { Canon } from "../api/client";
import type { CollectionArtwork } from "../data/collection-artwork";
import { PosterBadge } from "./PosterBadge";

interface Props {
  canon: Canon;
  artwork: CollectionArtwork;
  watchedIds: Set<string>;
}

export function CollectionCard({ canon, artwork, watchedIds }: Props) {
  const { t } = useTranslation();
  const [failedURL, setFailedURL] = useState<string | null>(null);
  const hasArtwork = failedURL !== artwork.url;
  const ids = canon.imdbIds ?? [];
  const watched = ids.filter((id) => watchedIds.has(id)).length;
  const fraction =
    canon.filmCount > 0 ? Math.min(watched / canon.filmCount, 1) : 0;
  const complete =
    canon.filmCount > 0 &&
    ids.length === canon.filmCount &&
    watched === canon.filmCount;

  return (
    <Link
      to={`/canons/${encodeURIComponent(canon.slug)}`}
      className="group block min-w-0"
      aria-label={`${canon.name} · ${complete ? t("collection.complete") : t("collection.progressSummary", { watched, total: canon.filmCount })}`}
    >
      <div className="relative aspect-[2/3] overflow-hidden bg-[#e5e0d6]">
        {hasArtwork && (
          <div className="absolute inset-0 transition-transform duration-500 group-hover:scale-[1.02]">
            <img
              src={artwork.url}
              alt={t("collection.artwork", { title: canon.name })}
              width="1024"
              height="1536"
              loading="lazy"
              onError={() => setFailedURL(artwork.url)}
              className={`h-full w-full object-cover ${complete ? "" : "poster-unwatched"}`}
            />
            {!complete && (
              <img
                src={artwork.url}
                alt=""
                aria-hidden="true"
                width="1024"
                height="1536"
                loading="lazy"
                className="collection-poster-color absolute inset-0 h-full w-full object-cover"
                style={{ clipPath: `inset(${(1 - fraction) * 100}% 0 0 0)` }}
              />
            )}
          </div>
        )}
        <h2
          className={
            artwork.titleEmbedded && hasArtwork
              ? "sr-only"
              : "absolute inset-x-0 bottom-0 bg-gradient-to-t from-paper via-paper/90 to-transparent px-5 pb-16 pt-16 text-center font-display text-3xl leading-tight text-ink"
          }
        >
          {canon.name}
        </h2>
        <PosterBadge watched={watched} total={canon.filmCount} />
      </div>
      <p className="mt-3 text-sm text-muted">
        {complete
          ? t("collection.complete")
          : t("collection.progressSummary", {
              watched,
              total: canon.filmCount,
            })}
      </p>
    </Link>
  );
}
