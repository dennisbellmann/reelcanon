import { useTranslation } from "react-i18next";
import { useCallback, useEffect, useState } from "react";
import { liveQuery } from "dexie";
import {
  Link,
  Route,
  Routes,
  ScrollRestoration,
  useLocation,
} from "react-router-dom";
import type { MovieProgress } from "../data/progress";
import { localProgressRepository, progressRepository } from "../data/progress";
import { getAccount, refreshAccount, useAccount } from "../api/account";
import { AccountPage } from "./AccountPage";
import { clearCloudCache } from "../data/cloud-progress";
import { scrollRestorationKey } from "../data/navigation";
import { CanonIndexPage } from "./CanonIndexPage";
import { CollectionPage } from "./CollectionPage";
import { MoviePage } from "./MoviePage";
import { BackupPage } from "./BackupPage";
import { supportUrl } from "../data/support";

export default function App() {
  const { t } = useTranslation();
  const location = useLocation();
  const isCollectionPage = /^\/canons\/[^/]+\/?$/.test(location.pathname);
  const account = useAccount();
  const [progress, setProgress] = useState<MovieProgress[]>([]);

  const [storageError, setStorageError] = useState(false);
  const refreshProgress = useCallback(async () => {
    const id = getAccount().user?.id;
    try {
      const next = await progressRepository.list();
      if (getAccount().user?.id !== id) return;
      setProgress(next);
      setStorageError(false);
    } catch {
      setStorageError(true);
    }
  }, []);
  useEffect(() => {
    const subscription = liveQuery(() =>
      localProgressRepository.list(),
    ).subscribe({
      next: (value) => {
        if (!getAccount().ready || getAccount().user || getAccount().error)
          return;
        setProgress(value);
        setStorageError(false);
      },
      error: () => setStorageError(true),
    });
    return () => subscription.unsubscribe();
  }, []);
  useEffect(() => {
    void refreshAccount().catch(() => {});
    const refresh = () => {
      void refreshAccount().catch(() => {});
    };
    window.addEventListener("focus", refresh);
    const timer = window.setInterval(refresh, 60_000);
    return () => {
      window.removeEventListener("focus", refresh);
      window.clearInterval(timer);
    };
  }, []);
  useEffect(() => {
    setProgress([]);
    clearCloudCache();
    if (account.ready && !account.error) void refreshProgress();
  }, [account.user?.id, account.ready, account.error, refreshProgress]);
  useEffect(() => {
    if (!account.user || account.error) return;
    const refresh = () => {
      void refreshProgress();
    };
    window.addEventListener("focus", refresh);
    const timer = window.setInterval(refresh, 30_000);
    return () => {
      window.removeEventListener("focus", refresh);
      window.clearInterval(timer);
    };
  }, [account.user?.id, account.error, refreshProgress]);

  return (
    <div className="min-h-screen bg-paper text-ink">
      <ScrollRestoration
        storageKey="reelcanon-scroll-positions"
        getKey={scrollRestorationKey}
      />
      <header className="border-b border-line">
        <div className="mx-auto flex max-w-7xl flex-wrap gap-4 items-center justify-between px-6 py-5 lg:px-10">
          <Link to="/canons" className="font-display text-2xl tracking-tight">
            reel canon<span className="text-accent">.</span>
          </Link>
          <nav className="flex flex-wrap items-center justify-end gap-4 text-xs uppercase tracking-[0.12em] text-muted">
            <Link className="hover:text-ink" to="/canons">
              {t("nav.collections")}
            </Link>
            <Link className="hover:text-ink" to="/account">
              {account.user ? t("nav.account") : t("nav.login")}
            </Link>
            <Link className="hover:text-ink" to="/backup">
              {t("nav.settings")}
            </Link>
          </nav>
        </div>
      </header>
      <main
        className={`mx-auto max-w-7xl px-6 lg:px-10 ${
          isCollectionPage ? "py-6 lg:py-8" : "py-12 lg:py-16"
        }`}
      >
        {(storageError || account.error) && (
          <p role="alert" className="mb-8 text-accent">
            {t("common.progressError")}
          </p>
        )}
        <Routes>
          <Route
            path="/account"
            element={<AccountPage refreshProgress={refreshProgress} />}
          />
          <Route
            path="/backup"
            element={<BackupPage refreshProgress={refreshProgress} />}
          />
          <Route path="/" element={<CanonIndexPage progress={progress} />} />
          <Route
            path="/canons"
            element={<CanonIndexPage progress={progress} />}
          />
          <Route
            path="/canons/:canonSlug"
            element={
              <CollectionPage
                progress={progress}
                refreshProgress={refreshProgress}
              />
            }
          />
          <Route
            path="/canons/:canonSlug/movies/:imdbId"
            element={
              <MoviePage
                progress={progress}
                refreshProgress={refreshProgress}
              />
            }
          />
          <Route path="*" element={<p>{t("common.notFound")}</p>} />
        </Routes>
      </main>
      <footer className="border-t border-line px-6 py-6 text-center text-xs text-muted">
        <Link className="underline underline-offset-4" to="/backup">
          {t("settings.about")}
        </Link>
        {supportUrl && (
          <>
            <span className="mx-3" aria-hidden="true">
              ·
            </span>
            <a
              className="underline underline-offset-4"
              href={supportUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              {t("support.action")}
            </a>
          </>
        )}
      </footer>
    </div>
  );
}
