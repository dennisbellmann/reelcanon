import { expect, test } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));
const apiBase = process.env.E2E_BASE_URL!;
const adminBase = process.env.E2E_ADMIN_URL!;

test("Halloween JSON and Oscar module become reviewed, publishable collections", async ({
  browser,
  request,
}) => {
  const context = await browser.newContext({
    httpCredentials: {
      username: process.env.ADMIN_USER!,
      password: process.env.ADMIN_PASSWORD!,
    },
  });
  const cockpit = await context.newPage();
  const suffix = test.info().project.name;
  const halloweenSlug = `halloween-top-100-${suffix}`;
  const oscarSlug = `oscar-best-picture-${suffix}`;
  await cockpit.goto(`${adminBase}/canons`);
  await expect(
    cockpit.getByRole("heading", { name: "Sammlungen" }),
  ).toBeVisible();

  await cockpit
    .locator('input[name="file"]')
    .setInputFiles(path.join(root, "data/halloween-top-100.json"));
  await cockpit.locator('input[name="slug"]').first().fill(halloweenSlug);
  await cockpit.getByRole("button", { name: "JSON-Sammlung anlegen" }).click();
  await expect(
    cockpit.getByRole("heading", { name: "Top 100 Halloween Movies" }),
  ).toBeVisible();
  await expect(
    cockpit.getByText("100 Filme", { exact: false }).first(),
  ).toBeVisible();
  await publishCollection(cockpit, {
    slug: halloweenSlug,
    name: "Top 100 Halloween Movies",
    description:
      "100 Horrorfilme für Halloween. Rotten-Tomatoes-Rangliste, Stand 06.10.2026.",
    position: "2",
    poster: "/artwork/halloween-top-100.svg",
    reason: "Quelle und vollständige Rangfolge geprüft",
  });

  await cockpit.goto(`${adminBase}/canons`);
  await cockpit
    .locator('select[name="module"]')
    .selectOption("oscar-best-picture");
  await cockpit.locator('input[name="slug"]').nth(1).fill(oscarSlug);
  await cockpit
    .getByRole("button", { name: "Modul einlesen und Sammlung anlegen" })
    .click();
  await expect(
    cockpit.getByRole("heading", { name: "Oscar · Bester Film" }),
  ).toBeVisible();
  await expect(
    cockpit.getByText("98 Filme", { exact: false }).first(),
  ).toBeVisible();
  await publishCollection(cockpit, {
    slug: oscarSlug,
    name: "Oscar · Bester Film",
    description:
      "Alle 98 Gewinner des Oscars für den besten Film bis 2026, in Vergabereihenfolge.",
    position: "3",
    poster: "/artwork/oscar-best-picture.svg",
    reason: "Alle Gewinner von Wings bis One Battle after Another geprüft",
  });

  for (const [slug, baseSlug] of [
    [halloweenSlug, "halloween-top-100"],
    [oscarSlug, "oscar-best-picture"],
  ]) {
    await cockpit.goto(`${adminBase}/canons`);
    const row = cockpit
      .getByRole("row")
      .filter({ has: cockpit.locator(`a[href="/canons/${slug}"]`) });
    await row.getByRole("button", { name: "Live schalten" }).click();
    const response = await request.get(`${apiBase}/api/v1/canons/${slug}`);
    expect(response.status()).toBe(200);
    const canon = await response.json();
    const expected = baseSlug === "halloween-top-100" ? 100 : 98;
    expect(canon.canon.displayPosition).toBe(
      baseSlug === "halloween-top-100" ? 2 : 3,
    );
    expect(canon.canon.posterUrl).toBe(`/artwork/${baseSlug}.svg`);
    expect(canon.canon.filmCount).toBe(expected);
    expect(canon.movies).toHaveLength(expected);
  }
  await context.close();
});

async function publishCollection(
  page: import("@playwright/test").Page,
  collection: {
    slug: string;
    name: string;
    description: string;
    position: string;
    poster: string;
    reason: string;
  },
) {
  await page.getByLabel("Sammlungsname").fill(collection.name);
  await page.getByLabel("Beschreibung").fill(collection.description);
  await page.getByLabel("Position in der Übersicht").fill(collection.position);
  await page.getByLabel("Poster", { exact: true }).fill(collection.poster);
  await expect(
    page.getByLabel("Titel ist bereits im Poster enthalten"),
  ).toBeChecked();
  const form = page.locator('form[action$="/presentation"]');
  await form.getByLabel("Änderungsgrund").fill(collection.reason);
  await form.getByRole("button", { name: "Darstellung speichern" }).click();
  await expect(
    page.getByText("Darstellung im Entwurf gespeichert."),
  ).toBeVisible();
  await expect(page.getByText(/Validierung erfolgreich/)).toBeVisible();
  await page
    .getByRole("button", { name: "Geprüfte Edition veröffentlichen" })
    .click();
  await expect(
    page.getByText("Edition veröffentlicht.", { exact: true }),
  ).toBeVisible();
}
