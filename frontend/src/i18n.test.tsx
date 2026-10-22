import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useTranslation } from "react-i18next";
import i18n, {
  changeLanguage,
  detectLanguage,
  formatNumber,
  languageStorageKey,
  resolveLanguage,
} from "./i18n";
import { LanguageSelect } from "./components/LanguageSelect";
import {
  LocalizedError,
  apiErrorMessage,
  message,
  renderMessage,
} from "./i18n-message";

afterEach(() => {
  cleanup();
  localStorage.removeItem(languageStorageKey);
  vi.restoreAllMocks();
});

describe("language preferences", () => {
  it("matches regional language tags in preference order and falls back to English", () => {
    expect(resolveLanguage(["it-IT", "es-MX", "fr"])).toBe("es");
    expect(resolveLanguage(["FR-ca"])).toBe("fr");
    expect(resolveLanguage(["ja-JP"])).toBe("en");
  });
  it("prefers a saved choice and ignores unsupported stored values", () => {
    vi.spyOn(navigator, "languages", "get").mockReturnValue(["es-MX", "en"]);
    localStorage.setItem(languageStorageKey, "fr");
    expect(detectLanguage()).toBe("fr");
    localStorage.setItem(languageStorageKey, "unsupported");
    expect(detectLanguage()).toBe("es");
  });
  it("allows switching when browser storage is blocked", async () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    await changeLanguage("fr");
    expect(document.documentElement.lang).toBe("fr");
  });
});

describe("localized messages", () => {
  it("uses translated API codes and a local fallback for unknown errors", async () => {
    await changeLanguage("fr");
    expect(
      renderMessage(apiErrorMessage("password_too_short", "errors.request")),
    ).toContain("Mot de passe trop court");
    for (const code of ["not_known", "toString", null]) {
      expect(renderMessage(apiErrorMessage(code, "errors.request"))).toContain(
        "Échec de la requête",
      );
    }
  });
  it.each([
    ["de", "1 Film", "2 Filme", "1 Film anzeigen"],
    ["en", "1 film", "2 films", "Show 1 film"],
    ["es", "1 película", "2 películas", "Mostrar 1 película"],
    ["fr", "1 film", "2 films", "Afficher 1 film"],
  ])("handles counts in %s", async (language, one, other, action) => {
    await i18n.changeLanguage(language);
    expect(i18n.t("common.films", { count: 1 })).toBe(one);
    expect(i18n.t("common.films", { count: 2 })).toBe(other);
    expect(i18n.t("collection.show", { count: 1 })).toBe(action);
    expect(i18n.t("common.films", { count: 1_000_000 })).not.toContain(
      "common.films",
    );
  });
  it("switches live, saves the choice and relocalizes an existing error", async () => {
    const existing = new LocalizedError(message("backup.tooLarge"));
    function Settings() {
      const { t } = useTranslation();
      return (
        <>
          <LanguageSelect />
          <h1>{t("index.title")}</h1>
          <p role="alert">{renderMessage(existing.text)}</p>
        </>
      );
    }
    render(<Settings />);
    await userEvent
      .setup()
      .selectOptions(screen.getByRole("combobox", { name: "Sprache" }), "es");
    await screen.findByRole("heading", { name: "Una vida de cine." });
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Copia demasiado grande",
      ),
    );
    expect(localStorage.getItem(languageStorageKey)).toBe("es");
    expect(document.documentElement.lang).toBe("es");
    expect(document.title).toContain("Una vida de cine");
  });
  it("formats ratings for the active language and renders interpolation as text", async () => {
    await changeLanguage("de");
    expect(formatNumber(8.5, { minimumFractionDigits: 1 })).toBe("8,5");
    function Label() {
      const { t } = useTranslation();
      return (
        <p>{t("movie.poster", { title: '<img src=x onerror="alert(1)">' })}</p>
      );
    }
    const { container } = render(<Label />);
    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).toContain("<img");
  });
});
