import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import de from "../../localization/de.json";
import en from "../../localization/en.json";
import es from "../../localization/es.json";
import fr from "../../localization/fr.json";

export const languages = {
  de: "Deutsch",
  en: "English",
  es: "Español",
  fr: "Français",
} as const;
export type Language = keyof typeof languages;
export const languageStorageKey = "reelcanon-language";
export function resolveLanguage(preferences: readonly string[]): Language {
  for (const preference of preferences) {
    const base = preference.toLowerCase().split(/[-_]/)[0];
    if (Object.hasOwn(languages, base)) return base as Language;
  }
  return "en";
}
export function detectLanguage(): Language {
  try {
    const saved = localStorage.getItem(languageStorageKey);
    if (saved && Object.hasOwn(languages, saved)) return saved as Language;
  } catch {
    /* A language choice still works when storage is unavailable. */
  }
  return resolveLanguage(navigator.languages ?? [navigator.language]);
}
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "translation";
    keySeparator: false;
    resources: { translation: typeof en };
    strictKeyChecks: true;
  }
}
void i18next.use(initReactI18next).init({
  resources: {
    de: { translation: de },
    en: { translation: en },
    es: { translation: es },
    fr: { translation: fr },
  },
  lng: detectLanguage(),
  fallbackLng: "en",
  supportedLngs: Object.keys(languages),
  load: "languageOnly",
  keySeparator: false,
  initAsync: false,
  interpolation: { escapeValue: false }, // React escapes text nodes.
});
export const t = i18next.t.bind(i18next);
export function currentLanguage(): Language {
  return resolveLanguage([i18next.resolvedLanguage ?? i18next.language]);
}
function updateDocument() {
  document.documentElement.lang = currentLanguage();
  document.title = t("meta.title");
  document
    .querySelector('meta[name="description"]')
    ?.setAttribute("content", t("meta.description"));
}
i18next.on("languageChanged", updateDocument);
updateDocument();
export async function changeLanguage(language: Language) {
  await i18next.changeLanguage(language);
  try {
    localStorage.setItem(languageStorageKey, language);
  } catch {
    /* Keep the in-memory choice. */
  }
}
export function formatNumber(
  value: number,
  options?: Intl.NumberFormatOptions,
) {
  return new Intl.NumberFormat(currentLanguage(), options).format(value);
}
export default i18next;
