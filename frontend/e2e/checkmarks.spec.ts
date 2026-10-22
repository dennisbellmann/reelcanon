import { expect, test } from "@playwright/test";

for (const view of ["library", "gallery"]) {
  test(`other checkmarks stay visually stable while toggling in ${view}`, async ({
    page,
  }) => {
    const ids = ["tt0111161", "tt0068646", "tt0468569"];
    const timestamp = "2026-10-01T00:00:00Z";
    const progress = ids.map((imdbId, index) => ({
      imdbId,
      watched: index === 1,
      watchlist: false,
      watchedAt: index === 1 ? timestamp : null,
      updatedAt: timestamp,
      revision: 1,
    }));
    await page.route("**/api/v1/me", (route) =>
      route.fulfill({
        json: {
          enabled: true,
          user: { id: "checkmark-test", email: "checkmarks@example.com" },
          csrfToken: "test-token",
          providers: { google: false },
        },
      }),
    );
    await page.route("**/api/v1/canons/checkmarks", (route) =>
      route.fulfill({
        json: {
          canon: {
            slug: "checkmarks",
            name: "Checkmarks",
            description: "Checkmark regression fixture",
            source: "https://example.com/checkmarks",
            filmCount: 3,
            currentEdition: "test",
          },
          movies: ids.map((imdbId, index) => ({
            imdbId,
            title: `Film ${index + 1}`,
            originalTitle: null,
            tmdbId: null,
            releaseYear: 1994,
            runtimeMinutes: null,
            overview: "",
            posterUrl: null,
            imdbRating: null,
            directors: [],
            genres: [],
            countries: [],
            position: index + 1,
          })),
        },
      }),
    );
    await page.route("**/api/v1/me/progress", (route) =>
      route.fulfill({ json: progress }),
    );
    let releaseSave = () => {};
    let notifySaveStarted = () => {};
    await page.route(
      `**/api/v1/me/progress/${ids[0]}/watched`,
      async (route) => {
        await new Promise<void>((resolve) => {
          releaseSave = resolve;
          notifySaveStarted();
        });
        const watched = route.request().postDataJSON().value as boolean;
        progress[0] = {
          ...progress[0],
          watched,
          watchedAt: watched ? timestamp : null,
          revision: progress[0].revision + 1,
        };
        await route.fulfill({ json: progress[0] });
      },
    );

    await page.goto(`/canons/checkmarks?view=${view}`);
    const buttons = page.locator("article button[aria-pressed]");
    await expect(buttons).toHaveCount(3);
    await expect(buttons.nth(1)).toHaveAttribute("aria-pressed", "true");
    const appearance = () =>
      buttons.evaluateAll((elements) =>
        elements.slice(1).map((element) => {
          const style = getComputedStyle(element);
          return {
            opacity: style.opacity,
            background: style.backgroundColor,
            color: style.color,
            border: style.borderColor,
            text: element.textContent,
            pressed: element.getAttribute("aria-pressed"),
          };
        }),
      );
    const initialAppearance = await appearance();
    for (const watched of [true, false]) {
      const saveStarted = new Promise<void>((resolve) => {
        notifySaveStarted = resolve;
      });
      await buttons.first().click();
      await saveStarted;
      await expect(buttons.first()).toHaveAttribute("aria-busy", "true");
      try {
        await expect(buttons.first()).toHaveCSS("opacity", "0.5");
        expect(await appearance()).toEqual(initialAppearance);
      } finally {
        releaseSave();
      }
      await expect(buttons.first()).toHaveAttribute(
        "aria-pressed",
        String(watched),
      );
      await expect(buttons.first()).toBeEnabled();
      expect(await appearance()).toEqual(initialAppearance);
    }
  });
}
