import { useTranslation } from "react-i18next";
import {
  changeLanguage,
  currentLanguage,
  languages,
  type Language,
} from "../i18n";

export function LanguageSelect() {
  const { t } = useTranslation();
  return (
    <label className="flex flex-wrap items-center justify-between gap-4 text-sm">
      <span>{t("language.label")}</span>
      <select
        className="min-h-11 w-full max-w-full border border-line bg-paper px-3 py-2 text-ink sm:w-auto sm:min-w-48"
        value={currentLanguage()}
        onChange={(event) =>
          void changeLanguage(event.target.value as Language)
        }
      >
        {Object.entries(languages).map(([code, name]) => (
          <option key={code} value={code} lang={code}>
            {name}
          </option>
        ))}
      </select>
    </label>
  );
}
