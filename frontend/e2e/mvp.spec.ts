import { expect, test } from "@playwright/test";

test("watched state follows a film across Canons and changed editions", async ({
  page,
  browser,
}, testInfo) => {
  await page.goto("/canons/fixture-films");
  await page
    .getByRole("button", {
      name: "Fixture Film A als gesehen markieren",
      exact: true,
    })
    .click();
  const context = await browser.newContext({
    httpCredentials: {
      username: process.env.ADMIN_USER!,
      password: process.env.ADMIN_PASSWORD!,
    },
  });
  const admin = await context.newPage();
  await admin.goto(`${process.env.E2E_ADMIN_URL}/canons`);
  const slug = `state-${testInfo.project.name}`;
  await admin.getByLabel("Name", { exact: true }).fill("State Test Canon");
  await admin.getByLabel("Slug", { exact: true }).fill(slug);
  await admin
    .getByLabel("Quellen-URL", { exact: true })
    .fill("https://example.com/states");
  await admin
    .getByRole("button", { name: "Sammlung anlegen", exact: true })
    .click();
  await admin.getByText("JSON-Entwurf bearbeiten", { exact: true }).click();
  const snapshot = JSON.parse(
    await admin.getByLabel("Normalisierter Snapshot").inputValue(),
  );
  snapshot.entries = [
    {
      imdbId: "tt0111161",
      position: 1,
      sourceTitle: "Fixture Film A",
      releaseYear: 1994,
    },
  ];
  await admin
    .getByLabel("Normalisierter Snapshot")
    .fill(JSON.stringify(snapshot));
  await admin
    .locator('form[action$="/draft"] input[name="reason"]')
    .fill("Shared watched identity");
  await admin.getByRole("button", { name: "Entwurf speichern" }).click();
  await admin
    .getByRole("button", { name: "Geprüfte Edition veröffentlichen" })
    .click();
  await admin.goto(`${process.env.E2E_ADMIN_URL}/canons`);
  await admin
    .getByRole("row")
    .filter({
      has: admin.getByRole("link", { name: "State Test Canon", exact: true }),
    })
    .filter({ has: admin.getByText(slug, { exact: true }) })
    .getByRole("button", { name: "Live schalten" })
    .click();
  await page.goto(`/canons/${slug}`);
  await expect(page.getByRole("progressbar")).toHaveAttribute(
    "aria-valuenow",
    "100",
  );
  await page.getByRole("link", { name: "Fixture Film A", exact: true }).click();
  await expect(
    page.getByRole("link", { name: "Fixture Films", exact: true }),
  ).toBeVisible();
  await admin.goto(`${process.env.E2E_ADMIN_URL}/canons/fixture-films`);
  await admin.getByText("JSON-Entwurf bearbeiten", { exact: true }).click();
  const revised = JSON.parse(
    await admin.getByLabel("Normalisierter Snapshot").inputValue(),
  );
  revised.entries.reverse();
  revised.entries.forEach((entry: { position: number }, index: number) => {
    entry.position = index + 1;
  });
  await admin
    .getByLabel("Normalisierter Snapshot")
    .fill(JSON.stringify(revised));
  await admin
    .locator('form[action$="/draft"] input[name="reason"]')
    .fill("New edition with reordered ranks");
  await admin.getByRole("button", { name: "Entwurf speichern" }).click();
  await admin
    .getByRole("button", { name: "Geprüfte Edition veröffentlichen" })
    .click();
  await page.goto("/canons/fixture-films");
  await expect(
    page.getByRole("button", {
      name: "Fixture Film A als ungesehen markieren",
      exact: true,
    }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("progressbar")).toHaveAttribute(
    "aria-valuenow",
    "33",
  );
  await page.goto("/canons");
  const sharedCard = page.locator(`a[href="/canons/${slug}"]`);
  await expect(sharedCard).toContainText("Alle Filme gesehen");
  await expect(sharedCard.getByRole("img")).toHaveCSS("filter", "none");
  await expect(
    page.locator('a[href="/canons/fixture-films"]').getByRole("img"),
  ).toHaveCSS("filter", "grayscale(1)");
  await context.close();
});

test("collection artwork reveals color with progress and becomes fully colored on completion", async ({
  page,
}) => {
  await page.goto("/canons");
  const card = page.locator('a[href="/canons/fixture-films"]');
  const artwork = card.getByRole("img", { name: "Artwork: Fixture Films" });
  await expect(artwork).toHaveCSS("filter", "grayscale(1)");
  await expect
    .poll(() =>
      artwork.evaluate(
        (image) =>
          image instanceof HTMLImageElement &&
          image.complete &&
          image.naturalWidth > 0,
      ),
    )
    .toBe(true);
  await expect(
    card.getByRole("heading", { name: "Fixture Films", exact: true }),
  ).toBeVisible();
  await expect(card).toContainText("0 / 3 gesehen");
  await expect(card.locator(".poster-badge")).toHaveText("0/3");
  await card.click();
  for (const title of ["Fixture Film A", "Fixture Film B", "Fixture Film C"]) {
    await page
      .getByRole("button", {
        name: `${title} als gesehen markieren`,
        exact: true,
      })
      .click();
    await expect(
      page.getByRole("button", {
        name: `${title} als ungesehen markieren`,
        exact: true,
      }),
    ).toHaveAttribute("aria-pressed", "true");
  }
  await page.getByRole("link", { name: "Sammlungen", exact: true }).click();
  await expect(card).toContainText("Alle Filme gesehen");
  await expect(artwork).toHaveCSS("filter", "none");
  await page.reload();
  await expect(artwork).toHaveCSS("filter", "none");
  await card.click();
  await page
    .getByRole("button", {
      name: "Fixture Film C als ungesehen markieren",
      exact: true,
    })
    .click();
  await expect(
    page.getByRole("button", {
      name: "Fixture Film C als gesehen markieren",
      exact: true,
    }),
  ).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("link", { name: "Sammlungen", exact: true }).click();
  await expect(artwork).toHaveCSS("filter", "grayscale(1)");
  await expect(card).toContainText("2 / 3 gesehen");
  await expect(card.locator(".poster-badge")).toHaveText("2/3");
  await expect(card.locator(".collection-poster-color")).toHaveCSS(
    "clip-path",
    "inset(33.3333% 0px 0px)",
  );
  await page.setViewportSize({ width: 320, height: 740 });
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(320);
});

test("watched/watchlist persist through detail navigation, reload and validated backup", async ({
  page,
}) => {
  await page.goto("/canons");
  await page.getByRole("link", { name: /Fixture Films/ }).click();
  await expect(
    page.getByRole("heading", { name: "Fixture Films", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Fixture Film A", exact: true }).click();
  await page
    .getByRole("button", { name: "Als gesehen markieren", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Zur Merkliste", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Aus Merkliste entfernen" }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Als ungesehen markieren" }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("button", { name: "Aus Merkliste entfernen" }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("link", { name: "← Zur Sammlung" }).click();
  await expect(page.getByRole("progressbar")).toHaveAttribute(
    "aria-valuenow",
    "33",
  );
  await page.getByRole("button", { name: /Filter & Sortierung/ }).click();
  await page
    .getByRole("combobox", { name: "Status", exact: true })
    .selectOption("watchlist");
  await page.getByRole("button", { name: "1 Film anzeigen" }).click();
  await expect(page.getByRole("status")).toHaveText("1 von 3 Filmen");
  await page.getByRole("link", { name: "Einstellungen", exact: true }).click();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Sicherung exportieren" }).click();
  const download = await downloadPromise;
  const backupPath = await download.path();
  expect(backupPath).not.toBeNull();
  const chooserPromise = page.waitForEvent("filechooser");
  await page
    .getByRole("button", { name: "Sicherung importieren", exact: true })
    .click();
  await (await chooserPromise).setFiles(backupPath!);
  await expect(page.getByRole("status")).toHaveText(
    "1 Film geprüft. Sicherung zusammengeführt.",
  );
  await page
    .getByLabel("Sicherung importieren (JSON, maximal 5 MB)")
    .setInputFiles({
      name: "invalid.json",
      mimeType: "application/json",
      buffer: Buffer.from('{"format":"bad"}'),
    });
  await expect(page.getByRole("alert")).toContainText("Sicherungsformat");
});

test("combined filters, search, decade navigation and narrow mobile layout", async ({
  page,
}) => {
  await page.goto("/canons/fixture-films");
  await page.getByRole("textbox", { name: "Filme suchen" }).fill("Film B");
  await expect(page.getByRole("status")).toHaveText("1 von 3 Filmen");
  await page.getByRole("button", { name: /Filter & Sortierung/ }).click();
  await page
    .getByRole("combobox", { name: "Jahr", exact: true })
    .selectOption("1994");
  await page.getByRole("button", { name: "0 Filme anzeigen" }).click();
  await expect(
    page.getByText("Keine Treffer. Passe die Filter an.", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: /Filter & Sortierung/ }).click();
  await page.getByRole("button", { name: "Zurücksetzen" }).click();
  await page.getByRole("button", { name: "3 Filme anzeigen" }).click();
  await page
    .getByRole("button", { name: "Postergalerie", exact: true })
    .click();
  await expect(page.getByRole("heading", { name: "1970er" })).toBeVisible();
  await page.getByRole("button", { name: /^1990er/ }).click();
  await expect(page.getByRole("heading", { name: "1990er" })).toBeVisible();
  await page.getByLabel("Jahrzehnte blättern").focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("heading", { name: "2000er" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("heading", { name: "2000er" })).toBeVisible();
  await page.setViewportSize({ width: 320, height: 740 });
  const width = await page.evaluate(() => ({
    scroll: document.documentElement.scrollWidth,
    viewport: window.innerWidth,
  }));
  expect(width.scroll).toBeLessThanOrEqual(width.viewport);
});

test("bottom sheets hide controls and provenance, trap focus and preserve filters", async ({
  page,
  request,
}) => {
  const { canon } = await (
    await request.get("/api/v1/canons/fixture-films")
  ).json();
  await page.goto("/canons/fixture-films");
  const filters = page.getByRole("button", { name: /Filter & Sortierung/ });
  await expect(filters).toBeVisible();
  await expect(page.getByRole("combobox")).toHaveCount(0);
  await expect(page.getByText("Edition", { exact: true })).toHaveCount(0);
  await expect(page.getByRole("link", { name: /Quelle/ })).toHaveCount(0);

  await filters.click();
  const sheet = page.getByRole("dialog", { name: "Filter & Sortierung" });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole("combobox")).toHaveCount(7);
  await expect(sheet.getByRole("button", { name: "Schließen" })).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(
    sheet.getByRole("button", { name: "3 Filme anzeigen" }),
  ).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(sheet.getByRole("button", { name: "Schließen" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(sheet).toHaveCount(0);
  await expect(filters).toBeFocused();

  await filters.click();
  await sheet
    .getByRole("combobox", { name: "Jahr", exact: true })
    .selectOption("1972");
  await sheet.getByRole("button", { name: "1 Film anzeigen" }).click();
  await expect(page.getByRole("status")).toHaveText("1 von 3 Filmen");
  await expect(filters).toContainText("1");
  await page.reload();
  await expect(page.getByRole("status")).toHaveText("1 von 3 Filmen");
  await filters.click();
  await expect(
    sheet.getByRole("combobox", { name: "Jahr", exact: true }),
  ).toHaveValue("1972");
  await sheet.getByRole("button", { name: "Zurücksetzen" }).click();
  await sheet.getByRole("button", { name: "3 Filme anzeigen" }).click();
  await expect(page.getByRole("textbox", { name: "Filme suchen" })).toHaveValue(
    "",
  );

  const title = page.getByRole("button", {
    name: "Fixture Films",
    exact: true,
  });
  await title.click();
  const details = page.getByRole("dialog", {
    name: "Fixture Films",
    exact: true,
  });
  await expect(details).toContainText(canon.currentEdition);
  await expect(
    details.getByRole("link", { name: "Quelle öffnen ↗" }),
  ).toHaveAttribute("href", canon.source);
  const bounds = (await details.boundingBox())!;
  const viewport = page.viewportSize()!;
  expect(Math.round(bounds.y + bounds.height)).toBe(viewport.height);
  expect(bounds.width).toBeLessThanOrEqual(viewport.width);
  await page.mouse.click(viewport.width / 2, Math.max(1, bounds.y / 2));
  await expect(details).toHaveCount(0);
  await expect(title).toBeFocused();
  await expect(page.locator("body")).not.toHaveCSS("overflow", "hidden");

  await page.setViewportSize({ width: 320, height: 568 });
  await filters.click();
  await expect(sheet).toBeVisible();
  const layout = await sheet.evaluate((element) => {
    const body = element.querySelector(".overflow-y-auto")!;
    return {
      bottom: element.getBoundingClientRect().bottom,
      height: window.innerHeight,
      width: document.documentElement.scrollWidth,
      viewport: window.innerWidth,
      scrollable: body.scrollHeight > body.clientHeight,
    };
  });
  expect(Math.round(layout.bottom)).toBe(layout.height);
  expect(layout.width).toBeLessThanOrEqual(layout.viewport);
  expect(layout.scrollable).toBe(true);
  await expect(
    sheet.getByRole("button", { name: "3 Filme anzeigen" }),
  ).toBeInViewport();
  await expect(page.locator("body")).toHaveCSS("overflow", "hidden");
  await sheet.getByRole("button", { name: "Schließen" }).click();
});

test("poster gallery shows covers and retains watched state and detail navigation", async ({
  page,
}) => {
  await page.goto("/canons/fixture-films");
  await expect(
    page
      .getByRole("article")
      .filter({ hasText: "Fixture Film B" })
      .locator("p"),
  ).toContainText(["1972"]);
  await page
    .getByRole("button", { name: "Postergalerie", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Postergalerie", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("article")).toHaveCount(1);
  await expect(page.getByRole("status")).toHaveCount(0);
  await expect(page.getByRole("article").locator("p")).toHaveCount(0);
  await expect(page.getByRole("article").getByRole("button")).toHaveCount(0);
  const hideWatched = page.getByRole("switch", {
    name: "Gesehene ausblenden",
  });
  await expect(hideWatched).toBeVisible();
  await page.getByRole("link", { name: "Fixture Film B", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Fixture Film B", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Als gesehen markieren" }).click();
  await expect(
    page.getByRole("button", { name: "Als ungesehen markieren" }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.goBack();
  await expect(
    page.getByRole("button", { name: "Postergalerie", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await expect(
    page.getByRole("progressbar", { name: "Gesehene Filme" }),
  ).toHaveAttribute("aria-valuenow", "33");
  await expect(page.getByText(/Alle gesehen/)).toBeVisible();
  await expect(page.getByText(/33 % gesehen/)).toBeVisible();
  await page.reload();
  await expect(page.getByRole("article")).toHaveCount(1);
  await page.getByRole("link", { name: "Fixture Film B", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Fixture Film B", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Als ungesehen markieren" }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.goBack();
  await expect(
    page.getByRole("button", { name: "Postergalerie", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page.getByText("Gesehene ausblenden", { exact: true }).click();
  await expect(hideWatched).toBeChecked();
  await expect(page.getByRole("article")).toHaveCount(0);
  await page.getByText("Gesehene ausblenden", { exact: true }).click();
  await expect(hideWatched).not.toBeChecked();
  await expect(page.getByRole("article")).toHaveCount(1);
  await page.getByRole("button", { name: "Alle Filme", exact: true }).click();
  await expect(page.getByRole("article")).toHaveCount(3);
  await expect(
    page
      .getByRole("article")
      .filter({ hasText: "Fixture Film B" })
      .locator("p"),
  ).toContainText(["1972"]);
});

test("detail return restores scroll, filters and gallery after browser back or reload", async ({
  page,
  request,
}) => {
  const catalog = await (
    await request.get("/api/v1/canons/fixture-films")
  ).json();
  const movies = Array.from({ length: 96 }, (_, index) => ({
    ...catalog.movies[0],
    imdbId: `tt${String(9000000 + index)}`,
    title: `Scroll Film ${String(index + 1).padStart(2, "0")}`,
    releaseYear: 1994,
    position: index + 1,
  }));
  await page.route("**/api/v1/canons/fixture-films", async (route) => {
    // A delayed catalog also exercises restoration after a cold detail reload.
    await new Promise((resolve) => setTimeout(resolve, 150));
    await route.fulfill({
      json: {
        ...catalog,
        canon: { ...catalog.canon, filmCount: movies.length },
        movies,
      },
    });
  });

  for (const view of ["library", "gallery"]) {
    const path = `/canons/fixture-films?view=${view}&decade=1990&genre=Drama&sort=title`;
    await page.goto(path);
    const titleLink = page.getByRole("link", {
      name: "Scroll Film 49",
      exact: true,
    });
    await titleLink.scrollIntoViewIfNeeded();
    const original = await page.evaluate(() => window.scrollY);
    expect(original).toBeGreaterThan(200);
    await titleLink.click();
    await expect(
      page.getByRole("heading", { name: "Scroll Film 49", exact: true }),
    ).toBeVisible();
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
    await page.goBack();
    await expect(page).toHaveURL(new URL(path, process.env.E2E_BASE_URL).href);
    await expect
      .poll(async () =>
        Math.abs((await page.evaluate(() => window.scrollY)) - original),
      )
      .toBeLessThanOrEqual(2);

    await titleLink.click();
    await expect(
      page.getByRole("heading", { name: "Scroll Film 49", exact: true }),
    ).toBeVisible();
    await page.reload();
    await page.getByRole("link", { name: "← Zur Sammlung" }).click();
    await expect(page).toHaveURL(new URL(path, process.env.E2E_BASE_URL).href);
    await expect
      .poll(async () =>
        Math.abs((await page.evaluate(() => window.scrollY)) - original),
      )
      .toBeLessThanOrEqual(2);
    await expect(titleLink).toBeInViewport();
    await expect(
      page.getByRole("button", { name: "Postergalerie", exact: true }),
    ).toHaveAttribute("aria-pressed", view === "gallery" ? "true" : "false");

    // Returning via the in-page link must remain correct on a second visit.
    await titleLink.click();
    await page.getByRole("link", { name: "← Zur Sammlung" }).click();
    await expect
      .poll(async () =>
        Math.abs((await page.evaluate(() => window.scrollY)) - original),
      )
      .toBeLessThanOrEqual(2);
  }
  await page.goto("/backup");
  await expect(
    page.getByRole("heading", { name: "Fortschritt sichern" }),
  ).toBeVisible();
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
});

test("public API rejects admin writes and unknown movies have a return path", async ({
  page,
  request,
}) => {
  for (const action of ["publish", "draft", "visibility"])
    expect(
      (await request.post(`/api/v1/canons/fixture-films/${action}`)).status(),
    ).toBe(404);
  await page.goto("/canons/fixture-films/movies/tt9999999");
  await expect(
    page.getByRole("heading", { name: "Film nicht gefunden" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "← Zur Sammlung" }).click();
  await expect(
    page.getByRole("heading", { name: "Fixture Films", exact: true }),
  ).toBeVisible();
});

test("admin protects drafts, rejects stale edits and publishes reviewed editions", async ({
  browser,
  request,
}, testInfo) => {
  const adminURL = process.env.E2E_ADMIN_URL!;
  expect((await request.get(`${adminURL}/canons`)).status()).toBe(401);
  const context = await browser.newContext({
    httpCredentials: {
      username: process.env.ADMIN_USER!,
      password: process.env.ADMIN_PASSWORD!,
    },
  });
  const page = await context.newPage();
  await page.goto(`${adminURL}/canons`);
  const slug = `e2e-${testInfo.project.name}`;
  await page.getByLabel("Name", { exact: true }).fill("E2E Admin Collection");
  await page.getByLabel("Slug", { exact: true }).fill(slug);
  await page
    .getByLabel("Quellen-URL", { exact: true })
    .fill("https://example.com/e2e");
  await page
    .getByRole("button", { name: "Sammlung anlegen", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "E2E Admin Collection" }),
  ).toBeVisible();
  await page.getByText("JSON-Entwurf bearbeiten", { exact: true }).click();
  const snapshot = JSON.parse(
    await page.getByLabel("Normalisierter Snapshot").inputValue(),
  );
  snapshot.entries = [
    { imdbId: "tt0111161", position: 1, sourceTitle: "A", releaseYear: 1994 },
    { imdbId: "tt0068646", position: 2, sourceTitle: "B", releaseYear: 1972 },
  ];
  await page
    .getByLabel("Normalisierter Snapshot")
    .fill(JSON.stringify(snapshot));
  await page
    .locator('form[action$="/draft"] input[name="reason"]')
    .fill("Initial test publication");
  await page.getByRole("button", { name: "Entwurf speichern" }).click();
  const stale = await context.newPage();
  await stale.goto(page.url());
  expect((await request.get(`/api/v1/canons/${slug}`)).status()).toBe(404);
  await page
    .getByRole("button", { name: "Geprüfte Edition veröffentlichen" })
    .click();
  await expect(page.getByRole("status")).toContainText(
    "Edition veröffentlicht.",
  );
  await stale.getByText("JSON-Entwurf bearbeiten", { exact: true }).click();
  await stale.getByRole("button", { name: "Entwurf speichern" }).click();
  await expect(stale.locator("body")).toContainText("reload before saving");
  await page.goto(`${adminURL}/canons`);
  const row = page
    .getByRole("row")
    .filter({ has: page.getByRole("link", { name: "E2E Admin Collection" }) });
  await row.getByRole("button", { name: "Live schalten" }).click();
  const response = await request.get(`/api/v1/canons/${slug}`);
  expect(response.status()).toBe(200);
  expect((await response.json()).movies).toHaveLength(2);
  await context.close();
});

test("cockpit publishes collection design and PosterBadge follows watched progress", async ({
  page,
  browser,
}, testInfo) => {
  const slug = `design-${testInfo.project.name}`;
  const displayName =
    testInfo.project.name === "mobile" ? "AAA A Cinema" : "AAA B Cinema";
  const context = await browser.newContext({
    httpCredentials: {
      username: process.env.ADMIN_USER!,
      password: process.env.ADMIN_PASSWORD!,
    },
  });
  const admin = await context.newPage();
  await admin.goto(`${process.env.E2E_ADMIN_URL}/canons`);
  await admin.getByLabel("Name", { exact: true }).fill("Design draft");
  await admin.getByLabel("Slug", { exact: true }).fill(slug);
  await admin
    .getByLabel("Quellen-URL", { exact: true })
    .fill("https://example.com/design");
  await admin
    .getByRole("button", { name: "Sammlung anlegen", exact: true })
    .click();
  await admin.getByText("JSON-Entwurf bearbeiten", { exact: true }).click();
  const snapshot = JSON.parse(
    await admin.getByLabel("Normalisierter Snapshot").inputValue(),
  );
  snapshot.entries = [
    { imdbId: "tt0111161", position: 1, sourceTitle: "Fixture Film A" },
    { imdbId: "tt0068646", position: 2, sourceTitle: "Fixture Film B" },
  ];
  await admin
    .getByLabel("Normalisierter Snapshot")
    .fill(JSON.stringify(snapshot));
  await admin
    .locator('form[action$="/draft"] input[name="reason"]')
    .fill("Design fixture");
  await admin.getByRole("button", { name: "Entwurf speichern" }).click();
  await admin
    .getByRole("button", { name: "Geprüfte Edition veröffentlichen" })
    .click();
  await admin.goto(`${process.env.E2E_ADMIN_URL}/canons`);
  await admin
    .getByRole("row")
    .filter({ has: admin.getByText(slug, { exact: true }) })
    .getByRole("button", { name: "Live schalten" })
    .click();
  await admin.goto(`${process.env.E2E_ADMIN_URL}/canons/${slug}`);
  await admin.getByLabel("Sammlungsname", { exact: true }).fill(displayName);
  await admin.getByLabel("Position in der Übersicht").fill("0");
  await admin
    .getByLabel("Poster", { exact: true })
    .fill("/artwork/collection-02-kinosaal.png");
  await admin
    .locator('form[action$="/presentation"] input[name="reason"]')
    .fill("New collection design");
  await admin.getByRole("button", { name: "Darstellung speichern" }).click();
  await page.goto("/canons");
  const card = page.locator(`a[href="/canons/${slug}"]`);
  await expect(card).toHaveAccessibleName(/Design draft/);
  await admin
    .getByRole("button", { name: "Geprüfte Edition veröffentlichen" })
    .click();
  await page.reload();
  await expect(card).toHaveAccessibleName(new RegExp(displayName));
  await expect(
    page
      .locator("a")
      .filter({ has: page.locator(".poster-badge") })
      .first(),
  ).toHaveAttribute("href", `/canons/${slug}`);
  await expect(card.getByRole("img")).toHaveAttribute(
    "src",
    "/artwork/collection-02-kinosaal.png",
  );
  await expect(card.locator(".poster-badge")).toHaveText("0/2");
  await card.click();
  await page
    .getByRole("button", {
      name: "Fixture Film A als gesehen markieren",
      exact: true,
    })
    .click();
  await page.goto("/canons");
  await expect(card.locator(".poster-badge")).toHaveText("1/2");
  await expect(card.locator(".collection-poster-color")).toHaveCSS(
    "clip-path",
    "inset(50% 0px 0px)",
  );
  await context.close();
});
