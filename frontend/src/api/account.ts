import {
  LocalizedError,
  message,
  errorMessage,
  apiErrorMessage,
  type LocalizedMessage,
} from "../i18n-message";
import { useSyncExternalStore } from "react";

export interface AccountState {
  ready: boolean;
  enabled: boolean;
  user: { id: string; email: string } | null;
  csrfToken: string;
  providers: { google: boolean };
  developmentMailbox?: string;
  error?: LocalizedMessage;
}
let state: AccountState = {
  ready: false,
  enabled: false,
  user: null,
  csrfToken: "",
  providers: { google: false },
};
const listeners = new Set<() => void>();
let pending: Promise<void> | undefined;
const base = (import.meta.env.VITE_API_BASE_URL || "/api/v1").replace(
  /\/$/,
  "",
);
function publish(next: AccountState) {
  state = next;
  for (const listener of listeners) listener();
}
export const getAccount = () => state;
export const isAccountInitializing = () => !!pending && !state.ready;
export function useAccount() {
  return useSyncExternalStore((listener) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }, getAccount);
}
export function refreshAccount(): Promise<void> {
  if (pending) return pending;
  pending = (async () => {
    try {
      const response = await fetch(base + "/me", {
        credentials: "same-origin",
        cache: "no-store",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) throw new LocalizedError(message("errors.session"));
      const result = await response.json();
      if (typeof result.enabled !== "boolean" || !("user" in result))
        throw new LocalizedError(message("errors.accountResponse"));
      publish({ ...result, ready: true, error: undefined });
    } catch (cause) {
      publish({
        ...state,
        ready: true,
        error: errorMessage(cause, "errors.accountUnavailable"),
      });
      throw cause;
    } finally {
      pending = undefined;
    }
  })();
  return pending;
}
export class AccountError extends LocalizedError {
  constructor(
    text: LocalizedMessage,
    public status: number,
    public details?: unknown,
  ) {
    super(text);
  }
}
export async function accountRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  if (!state.ready) await refreshAccount();
  if (state.error) throw new LocalizedError(state.error);
  const headers = new Headers(options.headers);
  if (state.user) headers.set("X-Account-ID", state.user.id);
  headers.set("Accept", "application/json");
  if (options.body) headers.set("Content-Type", "application/json");
  if (options.method && !["GET", "HEAD"].includes(options.method))
    headers.set("X-CSRF-Token", state.csrfToken);
  const response = await fetch(base + path, {
    ...options,
    headers,
    credentials: "same-origin",
    cache: "no-store",
  });
  const result = await response.json();
  if (!response.ok) {
    if (response.status === 401) void refreshAccount().catch(() => {});
    throw new AccountError(
      apiErrorMessage(
        result.code,
        path === "/auth/login" ? "errors.login" : "errors.request",
      ),
      response.status,
      result,
    );
  }
  return result as T;
}
export async function emailAction(
  action: string,
  values: Record<string, string>,
) {
  const result = await accountRequest<{
    status?: string;
    error?: string;
    errors?: unknown;
    validation?: unknown;
  }>("/auth/" + action, { method: "POST", body: JSON.stringify(values) });
  if (
    result.status === "failure" ||
    result.status === "fail" ||
    result.error ||
    result.errors ||
    result.validation
  ) {
    throw new LocalizedError(
      message(action === "login" ? "errors.login" : "errors.accountAction"),
    );
  }
  await refreshAccountAfterWrite();
}
export const googleLoginURL = base + "/auth/google";
export async function refreshAccountAfterWrite() {
  // A focus/poll response started before a mutation must not become its final
  // session snapshot. Wait for it, then load the newly-issued cookie.
  if (pending) await pending.catch(() => {});
  await refreshAccount();
}
export async function logout(all = false) {
  await accountRequest("/auth/" + (all ? "logout-all" : "logout"), {
    method: "POST",
  });
  await refreshAccountAfterWrite();
}
