import { expect, test } from "@playwright/test";

const copy = {
  de: {
    settings: "Einstellungen",
    language: "Sprache",
    collections: "Sammlungen",
    filters: "Filter & Sortierung",
    decade: "Nach Jahrzehnt filtern",
    year: "1970er",
    mark: "Fixture Film A als gesehen markieren",
  },
  en: {
    settings: "Settings",
    language: "Language",
    collections: "Collections",
    filters: "Filter & sort",
    decade: "Filter by decade",
    year: "1970s",
    mark: "Mark Fixture Film A as watched",
  },
  es: {
    settings: "Ajustes",
    language: "Idioma",
    collections: "Colecciones",
    filters: "Filtrar y ordenar",
    decade: "Filtrar por década",
    year: "Años 1970",
    mark: "Marcar Fixture Film A como vista",
  },
  fr: {
    settings: "Réglages",
    language: "Langue",
    collections: "Collections",
    filters: "Filtres et tri",
    decade: "Filtrer par décennie",
    year: "Années 1970",
    mark: "Marquer Fixture Film A comme vu",
  },
} as const;

for (const [language, labels] of Object.entries(copy)) {
  test(`language ${language} persists across reload and works on a narrow screen`, async ({
    page,
  }, testInfo) => {
    await page.goto("/backup");
    await page
      .getByRole("combobox", { name: "Sprache" })
      .selectOption(language);
    await expect(
      page.getByRole("heading", { name: labels.settings, exact: true }),
    ).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("lang", language);
    await page.reload();
    await expect(
      page.getByRole("combobox", { name: labels.language }),
    ).toHaveValue(language);
    await page.setViewportSize({ width: 320, height: 740 });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(320);
    await page.goto("/canons/fixture-films");
    await expect(
      page.getByRole("button", { name: labels.mark, exact: true }),
    ).toBeVisible();
    await expect(page.locator("main")).not.toContainText(/Browser|Server|API/);
    await page
      .getByRole("button", { name: labels.filters, exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: labels.filters });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByRole("combobox", { name: labels.decade }),
    ).toContainText(labels.year);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBeLessThanOrEqual(320);
    await page.screenshot({
      path: testInfo.outputPath(`${language}-filters.png`),
    });
    await page.keyboard.press("Escape");
    await page
      .getByRole("link", { name: labels.collections, exact: true })
      .click();
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.screenshot({
      path: testInfo.outputPath(`${language}-collections.png`),
    });
  });
}
