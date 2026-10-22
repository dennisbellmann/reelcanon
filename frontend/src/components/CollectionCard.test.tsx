import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";
import type { Canon } from "../api/client";
import { collectionArtwork } from "../data/collection-artwork";
import { CollectionCard } from "./CollectionCard";

const canon: Canon = {
  slug: "editorial-collection",
  name: "Unsere Sammlung",
  description: "",
  source: "",
  filmCount: 250,
  currentEdition: "edition-1",
  imdbIds: Array.from(
    { length: 250 },
    (_, i) => `tt${String(i).padStart(7, "0")}`,
  ),
  posterUrl: "/artwork/imdb-top-250-filmwelten-editorial.png",
  posterTitleEmbedded: true,
};
afterEach(cleanup);

describe("collection PosterBadge and color progress", () => {
  it.each([0, 50, 250])(
    "shows %i/250 using collection membership",
    (watched) => {
      const { container } = render(
        <MemoryRouter>
          <CollectionCard
            canon={canon}
            artwork={collectionArtwork(canon)}
            watchedIds={
              new Set([...canon.imdbIds!.slice(0, watched), "tt9999999"])
            }
          />
        </MemoryRouter>,
      );
      expect(screen.getByText(`${watched}/250`)).toBeVisible();
      expect(screen.getByRole("img")).toHaveAttribute("src", canon.posterUrl);
      const colorLayer = container.querySelector(".collection-poster-color");
      if (watched === 250) {
        expect(colorLayer).toBeNull();
        expect(screen.getByRole("img")).not.toHaveClass("poster-unwatched");
      } else {
        expect(colorLayer).toHaveStyle({
          clipPath: `inset(${100 - (watched / 250) * 100}% 0 0 0)`,
        });
      }
    },
  );

  it("keeps name and badge readable when the cover fails", () => {
    render(
      <MemoryRouter>
        <CollectionCard
          canon={canon}
          artwork={collectionArtwork(canon)}
          watchedIds={new Set()}
        />
      </MemoryRouter>,
    );
    fireEvent.error(screen.getByRole("img"));
    expect(screen.getByRole("heading", { name: canon.name })).not.toHaveClass(
      "sr-only",
    );
    expect(screen.getByText("0/250")).toBeVisible();
  });
});
