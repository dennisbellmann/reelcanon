import { useCallback, useEffect, useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { CanonCatalog } from "../api/client";
import type { MovieProgress } from "../data/progress";
import { progressRepository } from "../data/progress";
import { CollectionPage } from "./CollectionPage";

const catalog: CanonCatalog = {
  canon: {
    slug: "fixture-films",
    name: "Fixture Films",
    description: "A test collection",
    source: "https://example.com/fixture-films",
    filmCount: 1,
    currentEdition: "test-edition",
  },
  movies: [
    {
      imdbId: "tt0111161",
      tmdbId: 278,
      title: "Fixture Film",
      originalTitle: null,
      releaseYear: 1994,
      runtimeMinutes: 142,
      overview: "Test synopsis",
      posterUrl: null,
      imdbRating: 9.3,
      directors: ["Test Director"],
      genres: ["Drama"],
      countries: ["United States"],
      position: 1,
    },
  ],
};

function Harness() {
  const [progress, setProgress] = useState<MovieProgress[]>([]);
  const refreshProgress = useCallback(async () => {
    setProgress(await progressRepository.list());
  }, []);
  useEffect(() => {
    void refreshProgress();
  }, [refreshProgress]);

  return (
    <Routes>
      <Route
        path="/canons/:canonSlug"
        element={
          <CollectionPage
            progress={progress}
            refreshProgress={refreshProgress}
          />
        }
      />
    </Routes>
  );
}

function renderCollection() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/canons/fixture-films"]}>
        <Harness />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(async () => {
  await progressRepository.clear();
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => catalog,
    }),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("local watched progress", () => {
  it("checks a film and keeps that state after the page is reloaded", async () => {
    const user = userEvent.setup();
    const firstPage = renderCollection();
    const checkButton = await screen.findByRole("button", {
      name: "Fixture Film als gesehen markieren",
    });

    await user.click(checkButton);

    await waitFor(() =>
      expect(checkButton).toHaveAttribute("aria-pressed", "true"),
    );
    expect(
      screen.getByText(
        (_content, element) =>
          element?.tagName === "P" && element.textContent?.trim() === "1 / 1",
      ),
    ).toBeInTheDocument();
    expect(await progressRepository.list()).toMatchObject([
      { imdbId: "tt0111161", watched: true },
    ]);

    firstPage.unmount();
    renderCollection();

    const persistedButton = await screen.findByRole("button", {
      name: "Fixture Film als ungesehen markieren",
    });
    expect(persistedButton).toHaveAttribute("aria-pressed", "true");
    expect(
      screen.getByText(
        (_content, element) =>
          element?.tagName === "P" && element.textContent?.trim() === "1 / 1",
      ),
    ).toBeInTheDocument();

    await user.click(persistedButton);
    await waitFor(() =>
      expect(persistedButton).toHaveAttribute("aria-pressed", "false"),
    );
    expect(await progressRepository.list()).toEqual([]);
  });

  it("stores watchlist state separately from watched state", async () => {
    await progressRepository.setWatched("tt0111161", true);
    await progressRepository.setWatchlist("tt0111161", true);
    await progressRepository.setWatched("tt0111161", false);

    expect(await progressRepository.list()).toMatchObject([
      { imdbId: "tt0111161", watched: false, watchlist: true },
    ]);
  });
});
