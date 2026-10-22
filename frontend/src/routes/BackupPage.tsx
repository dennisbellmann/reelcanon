import {
  message as localized,
  errorMessage,
  renderMessage,
  LocalizedError,
  type LocalizedMessage,
} from "../i18n-message";
import { useTranslation } from "react-i18next";
import { LanguageSelect } from "../components/LanguageSelect";
import { useRef, useState } from "react";
import { progressRepository } from "../data/progress";
import { useAccount } from "../api/account";
import { supportUrl } from "../data/support";

export function BackupPage({
  refreshProgress,
}: {
  refreshProgress: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const account = useAccount();
  const [message, setMessage] = useState<LocalizedMessage | null>(null);
  const [error, setError] = useState<LocalizedMessage | null>(null);
  const [busy, setBusy] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  async function exportBackup() {
    setError(null);
    setBusy(true);
    try {
      const url = URL.createObjectURL(
        new Blob([await progressRepository.exportBackup()], {
          type: "application/json",
        }),
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = `reel-canon-${new Date().toISOString().slice(0, 10)}.json`;
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      setMessage(localized("backup.exported"));
    } catch {
      setError(localized("backup.exportError"));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="max-w-4xl">
      <h1 className="font-display text-5xl tracking-tight">
        {t("nav.settings")}
      </h1>
      {error && (
        <p
          role="alert"
          className="mt-6 border-l-4 border-accent bg-[#f1e7df] px-4 py-3 text-accent"
        >
          {renderMessage(error)}
        </p>
      )}
      {message && (
        <p
          role="status"
          className="mt-6 border-l-4 border-ink bg-[#ebe5d9] px-4 py-3"
        >
          {renderMessage(message)}
        </p>
      )}
      <div className="my-8 max-w-3xl border border-line p-5 sm:p-6">
        <LanguageSelect />
      </div>
      <h2 className="max-w-3xl font-display text-3xl">{t("backup.title")}</h2>
      <p className="mt-6 max-w-3xl leading-7 text-muted">
        {account.user ? t("backup.accountIntro") : t("backup.guestIntro")}
      </p>
      <div className="mt-8 grid max-w-3xl gap-5 sm:grid-cols-2">
        <section className="flex flex-col items-start border border-line p-5 sm:p-6">
          <h3 className="font-display text-2xl">{t("backup.create")}</h3>
          <p className="mt-3 mb-5 text-sm leading-6 text-muted">
            {t("backup.downloadHint")}
          </p>
          <button
            disabled={busy}
            onClick={() => void exportBackup()}
            className="mt-auto min-h-11 border border-ink px-5 py-3 text-sm transition-colors hover:bg-ink hover:text-paper disabled:opacity-50"
          >
            {t("backup.export")}
          </button>
        </section>
        <section className="flex flex-col items-start border border-line p-5 sm:p-6">
          <h3 className="font-display text-2xl">{t("backup.read")}</h3>
          <p className="mt-3 mb-5 text-sm leading-6 text-muted">
            {t("backup.uploadHint")}
          </p>
          <button
            disabled={busy}
            onClick={() => fileInput.current?.click()}
            className="mt-auto min-h-11 border border-ink px-5 py-3 text-sm transition-colors hover:bg-ink hover:text-paper disabled:opacity-50"
          >
            {t("backup.import")}
          </button>
        </section>
      </div>
      <label className="sr-only" htmlFor="backup-file">
        {t("backup.fileLabel")}
      </label>
      <input
        id="backup-file"
        ref={fileInput}
        hidden
        type="file"
        accept="application/json,.json"
        disabled={busy}
        onChange={async (event) => {
          const input = event.currentTarget;
          const file = input.files?.[0];
          if (!file) return;
          setError(null);
          setMessage(null);
          setBusy(true);
          try {
            if (file.size > 5_000_000)
              throw new LocalizedError(localized("backup.tooLarge"));
            const count = await progressRepository.importBackup(
              await file.text(),
            );
            await refreshProgress();
            setMessage(localized("backup.merged", { count }));
          } catch (cause) {
            await refreshProgress();
            setError(errorMessage(cause, "backup.importError"));
          } finally {
            setBusy(false);
            input.value = "";
          }
        }}
      />
      <div className="mt-6 max-w-3xl space-y-3 border-l-2 border-line pl-4 text-sm leading-6 text-muted">
        <p>
          {account.user ? t("backup.accountMerge") : t("backup.guestMerge")}
        </p>
        <p>
          {account.user ? t("backup.accountStorage") : t("backup.guestStorage")}
        </p>
      </div>
      <details className="mt-8 max-w-3xl border-t border-line pt-6 text-sm text-muted">
        <summary className="cursor-pointer">{t("settings.about")}</summary>
        <p className="mt-4 leading-6">
          {t("credits.data")}{" "}
          <a
            className="underline"
            href="https://www.themoviedb.org"
            rel="noreferrer"
          >
            TMDb
          </a>
          . {t("credits.tmdb")}
        </p>
      </details>
      {supportUrl && (
        <aside className="mt-8 max-w-3xl border-t border-line pt-6">
          <h2 className="font-display text-3xl">{t("support.title")}</h2>
          <p className="mt-3 max-w-xl text-sm leading-6 text-muted">
            {t("support.body")}
          </p>
          <a
            className="mt-5 inline-flex min-h-11 items-center border border-ink px-5 py-3 text-sm transition-colors hover:bg-ink hover:text-paper focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
            href={supportUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            {t("support.action")}
            <span className="ml-2" aria-hidden="true">
              ↗
            </span>
            <span className="sr-only"> {t("support.external")}</span>
          </a>
        </aside>
      )}
    </section>
  );
}
