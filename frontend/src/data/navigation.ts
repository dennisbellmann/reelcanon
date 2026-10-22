import type { Location } from "react-router-dom";

export function scrollRestorationKey(
  location: Pick<Location, "key" | "state" | "pathname" | "search">,
) {
  if (typeof location.state?.restoreScrollKey === "string")
    return location.state.restoreScrollKey;
  // Direct document navigations start with the same "default" history key.
  // Keep unrelated pages from inheriting each other's saved scroll position.
  return location.key === "default"
    ? `default:${location.pathname}${location.search}`
    : location.key;
}
