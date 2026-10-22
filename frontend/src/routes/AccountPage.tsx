import {
  message as localized,
  errorMessage,
  renderMessage,
  type LocalizedMessage,
} from "../i18n-message";
import { useTranslation } from "react-i18next";
import { useEffect, useState, type FormEvent } from "react";
import {
  accountRequest,
  emailAction,
  googleLoginURL,
  logout,
  refreshAccount,
  refreshAccountAfterWrite,
  useAccount,
} from "../api/account";
import { localProgressRepository } from "../data/progress";
import { importCloudBackup } from "../data/cloud-progress";

const buttonClass =
  "min-h-11 border border-ink px-5 py-3 text-sm hover:bg-ink hover:text-paper disabled:opacity-50";
const inputClass = "mt-2 min-h-11 w-full border border-line bg-paper px-3 py-2";

function readLink() {
  const params = new URLSearchParams(window.location.hash.slice(1));
  return {
    confirm: params.get("confirm"),
    reset: params.get("reset"),
    error: params.has("login-error"),
  };
}
export function AccountPage({
  refreshProgress,
}: {
  refreshProgress: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const account = useAccount();
  const [link] = useState(readLink);
  // Clear after React commits, so development StrictMode retains the link.
  useEffect(() => {
    if (window.location.hash)
      window.history.replaceState(
        null,
        "",
        window.location.pathname + window.location.search,
      );
  }, []);
  const [mode, setMode] = useState<
    "login" | "register" | "recover" | "resend" | "reset"
  >(link.reset ? "reset" : "login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [message, setMessage] = useState<LocalizedMessage | null>(null);
  const [error, setError] = useState<LocalizedMessage | null>(
    link.error ? localized("account.googleError") : null,
  );
  const [deleteChoice, setDeleteChoice] = useState(false);
  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await action();
      await refreshProgress();
    } catch (cause) {
      setError(errorMessage(cause, "common.actionError"));
    } finally {
      setBusy(false);
    }
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    await run(async () => {
      if (mode === "reset") {
        await emailAction("recover/end", {
          token: link.reset || "",
          password,
          confirm_password: confirmPassword,
        });
        setMode("login");
        setMessage(localized("account.passwordChanged"));
      } else {
        await emailAction(
          mode === "resend" ? "confirm/resend" : mode,
          mode === "recover" || mode === "resend"
            ? { email }
            : { email, password, confirm_password: confirmPassword },
        );
        if (mode === "register") {
          setMode("login");
          setMessage(localized("account.created"));
        }
        if (mode === "recover") setMessage(localized("account.recoverSent"));
        if (mode === "resend") setMessage(localized("account.confirmSent"));
      }
      setPassword("");
      setConfirmPassword("");
    });
  }
  return (
    <section className="max-w-4xl">
      <h1 className="font-display text-5xl tracking-tight">
        {t("account.title")}
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
      {!account.ready && (
        <p className="mt-6" role="status">
          {t("account.loading")}
        </p>
      )}
      {account.error && (
        <div className="mt-6">
          <p role="alert">{renderMessage(account.error)}</p>
          <button
            className={buttonClass + " mt-4"}
            onClick={() => void refreshAccount().catch(() => {})}
          >
            {t("account.reconnect")}
          </button>
        </div>
      )}
      {account.ready && !account.error && !account.enabled && (
        <p className="mt-6">{t("account.disabled")}</p>
      )}
      {account.enabled &&
        !account.error &&
        (account.user ? (
          <div className="mt-8 max-w-3xl space-y-8">
            <section
              aria-labelledby="account-status-heading"
              className="flex gap-4 border border-line border-l-4 border-l-accent bg-[#ebe5d9] p-5 sm:p-6"
              role="status"
            >
              <span
                aria-hidden="true"
                className="flex size-9 shrink-0 items-center justify-center border border-accent font-semibold text-accent"
              >
                ✓
              </span>
              <div className="min-w-0">
                <h2
                  id="account-status-heading"
                  className="font-display text-2xl"
                >
                  {t("account.statusSignedIn")}
                </h2>
                <p className="mt-1 break-all text-sm text-muted">
                  {t("account.signedIn", { email: account.user.email })}
                </p>
              </div>
            </section>

            <div className="grid gap-5 md:grid-cols-2">
              <section className="flex flex-col items-start border border-line p-5 sm:p-6">
                <h2 className="font-display text-2xl">
                  {t("account.manageSession")}
                </h2>
                <div className="mt-5 flex flex-wrap gap-3">
                  <button
                    disabled={busy}
                    className={buttonClass}
                    onClick={() => void run(() => logout())}
                  >
                    {t("account.logout")}
                  </button>
                  <button
                    disabled={busy}
                    className="min-h-11 border border-line px-4 py-3 text-sm hover:border-ink disabled:opacity-50"
                    onClick={() => void run(() => logout(true))}
                  >
                    {t("account.logoutAll")}
                  </button>
                </div>
              </section>

              <section className="flex flex-col items-start border border-line p-5 sm:p-6">
                <h2 className="font-display text-2xl">
                  {t("account.importGuest")}
                </h2>
                <p className="mt-3 mb-5 text-sm leading-6 text-muted">
                  {t("account.importHint")}
                </p>
                <button
                  disabled={busy}
                  className={buttonClass + " mt-auto"}
                  onClick={() =>
                    void run(async () => {
                      const result = await importCloudBackup(
                        await localProgressRepository.exportBackup(),
                      );
                      setMessage(
                        localized(
                          result.unknown.length
                            ? "backup.unknown"
                            : "account.importResult",
                          { ...result, ids: result.unknown.join(", ") },
                        ),
                      );
                    })
                  }
                >
                  {t("account.importAction")}
                </button>
              </section>
            </div>

            <section className="border border-accent/40 bg-[#f1e7df] p-5 sm:p-6">
              <h2 className="font-display text-2xl">{t("account.delete")}</h2>
              <p className="my-4 text-sm leading-6 text-muted">
                {t("account.deleteHint")}
              </p>
              <label className="mb-4 flex items-start gap-3 text-sm">
                <input
                  type="checkbox"
                  checked={deleteChoice}
                  onChange={(event) => setDeleteChoice(event.target.checked)}
                />
                {t("account.deleteConfirm")}
              </label>
              <button
                disabled={busy || !deleteChoice}
                className="min-h-11 border border-accent px-5 py-3 text-sm text-accent hover:bg-accent hover:text-white disabled:opacity-50"
                onClick={() =>
                  void run(async () => {
                    await accountRequest("/me", { method: "DELETE" });
                    await refreshAccountAfterWrite();
                    setDeleteChoice(false);
                    setMessage(localized("account.deleted"));
                  })
                }
              >
                {t("account.deleteAction")}
              </button>
            </section>
          </div>
        ) : (
          <div className="mt-8">
            {link.confirm && !confirmed && (
              <div className="mb-8 border border-line p-5">
                <h2 className="font-display text-2xl">
                  {t("account.confirmEmail")}
                </h2>
                <button
                  className={buttonClass + " mt-4"}
                  disabled={busy}
                  onClick={() =>
                    void run(async () => {
                      await emailAction("confirm", { cnf: link.confirm || "" });
                      setConfirmed(true);
                      setMessage(localized("account.confirmed"));
                    })
                  }
                >
                  {t("account.confirmAction")}
                </button>
              </div>
            )}
            {mode !== "reset" && (
              <div className="mb-6 flex flex-wrap gap-4">
                {(["login", "register", "recover", "resend"] as const).map(
                  (value) => (
                    <button
                      key={value}
                      disabled={busy}
                      onClick={() => {
                        setMode(value);
                        setError(null);
                        setMessage(null);
                        setPassword("");
                        setConfirmPassword("");
                      }}
                      className={
                        mode === value
                          ? "border-b border-ink pb-2"
                          : "pb-2 text-muted"
                      }
                    >
                      {value === "login"
                        ? t("nav.login")
                        : value === "register"
                          ? t("account.register")
                          : value === "recover"
                            ? t("account.forgot")
                            : t("account.resend")}
                    </button>
                  ),
                )}
              </div>
            )}
            <div
              className={
                account.providers.google && mode === "login"
                  ? "grid items-stretch gap-5 md:grid-cols-2"
                  : ""
              }
            >
              {account.providers.google && mode === "login" && (
                <div className="flex items-center justify-center border border-line p-5 sm:p-6">
                  <a
                    href={googleLoginURL}
                    className="flex min-h-12 w-full items-center justify-center gap-3 rounded-[4px] border border-[#747775] bg-white px-3 text-sm font-medium text-[#1f1f1f] shadow-sm transition-colors hover:bg-[#f8f9fa] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#4285f4] focus-visible:ring-offset-2"
                  >
                    <img
                      src="/google-g.png"
                      alt=""
                      className="size-5 shrink-0"
                    />
                    <span>{t("account.google")}</span>
                  </a>
                </div>
              )}
              <form
                onSubmit={(event) => void submit(event)}
                className="space-y-5 border border-line p-5 sm:p-6"
              >
                {mode === "reset" && (
                  <h2 className="font-display text-2xl">
                    {t("account.newPassword")}
                  </h2>
                )}
                {mode !== "reset" && (
                  <label className="block text-sm">
                    {t("account.email")}
                    <input
                      className={inputClass}
                      type="email"
                      autoComplete="email"
                      required
                      maxLength={320}
                      value={email}
                      onChange={(event) => setEmail(event.target.value)}
                    />
                  </label>
                )}
                {mode !== "recover" && mode !== "resend" && (
                  <label className="block text-sm">
                    {t("account.password")}
                    <input
                      className={inputClass}
                      type="password"
                      autoComplete={
                        mode === "login" ? "current-password" : "new-password"
                      }
                      required
                      minLength={mode === "login" ? 1 : 12}
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                    />
                  </label>
                )}
                {(mode === "register" || mode === "reset") && (
                  <>
                    <p className="text-sm text-muted">
                      {t("account.passwordHint")}
                    </p>
                    <label className="block text-sm">
                      {t("account.repeatPassword")}
                      <input
                        className={inputClass}
                        type="password"
                        autoComplete="new-password"
                        required
                        value={confirmPassword}
                        onChange={(event) =>
                          setConfirmPassword(event.target.value)
                        }
                      />
                    </label>
                  </>
                )}
                <button className={buttonClass} disabled={busy}>
                  {busy
                    ? t("account.busy")
                    : mode === "login"
                      ? t("nav.login")
                      : mode === "register"
                        ? t("account.register")
                        : mode === "reset"
                          ? t("account.savePassword")
                          : mode === "resend"
                            ? t("account.requestConfirm")
                            : t("account.requestReset")}
                </button>
              </form>
            </div>
          </div>
        ))}
      {account.developmentMailbox && (
        <p className="mt-8 border-t border-line pt-6 text-sm text-muted">
          {t("account.devMail")}{" "}
          <a
            href={account.developmentMailbox}
            target="_blank"
            rel="noreferrer"
            className="underline"
          >
            {t("account.mailbox")}
          </a>
          .
        </p>
      )}
    </section>
  );
}
