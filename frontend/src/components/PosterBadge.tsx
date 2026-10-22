import { useTranslation } from "react-i18next";

export function PosterBadge({
  watched,
  total,
}: {
  watched: number;
  total: number;
}) {
  const { t } = useTranslation();
  return (
    <span
      className="poster-badge absolute bottom-3 right-3 flex items-center gap-2 bg-accent px-3 py-2 text-xs tabular-nums text-white"
      aria-label={t("collection.progressSummary", { watched, total })}
    >
      <svg
        aria-hidden="true"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        className="h-3 w-3"
      >
        <path d="m3 8 3 3 7-7" />
      </svg>
      {watched}/{total}
    </span>
  );
}
