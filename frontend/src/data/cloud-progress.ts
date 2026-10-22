import { LocalizedError, message } from "../i18n-message";
import { accountRequest, AccountError, getAccount } from "../api/account";
import type { MovieProgress } from "./progress";

export interface CloudProgress extends MovieProgress {
  revision: number;
}
export interface ImportResult {
  added: number;
  existing: number;
  unknown: string[];
}
let owner = "";
let rows = new Map<string, CloudProgress>();
function accountID() {
  const account = getAccount();
  if (account.error) throw new LocalizedError(account.error);
  if (!account.user) throw new LocalizedError(message("errors.relogin"));
  if (owner !== account.user.id) {
    owner = account.user.id;
    rows = new Map();
  }
  return owner;
}
export async function listCloudProgress(): Promise<MovieProgress[]> {
  const id = accountID();
  const result = await accountRequest<CloudProgress[]>("/me/progress");
  if (getAccount().user?.id !== id)
    throw new LocalizedError(message("errors.accountChanged"));
  rows = new Map(result.map((row) => [row.imdbId, row]));
  return result;
}
export async function updateCloudProgress(
  imdbId: string,
  field: "watched" | "watchlist",
  value: boolean,
) {
  const id = accountID();
  const revision = rows.get(imdbId)?.revision ?? 0;
  try {
    const row = await accountRequest<CloudProgress>(
      `/me/progress/${encodeURIComponent(imdbId)}/${field}`,
      {
        method: "PUT",
        headers: { "If-Match": `"${revision}"` },
        body: JSON.stringify({ value }),
      },
    );
    if (getAccount().user?.id !== id)
      throw new LocalizedError(message("errors.accountChanged"));
    rows.set(imdbId, row);
  } catch (cause) {
    if (cause instanceof AccountError && cause.status === 412) {
      await listCloudProgress();
      if (rows.get(imdbId)?.[field] === value) return;
      throw new LocalizedError(message("errors.conflict"));
    }
    throw cause;
  }
}
export async function importCloudBackup(text: string): Promise<ImportResult> {
  const id = accountID();
  const result = await accountRequest<ImportResult>("/me/progress/import", {
    method: "POST",
    body: text,
  });
  if (getAccount().user?.id !== id)
    throw new LocalizedError(message("errors.accountChanged"));
  await listCloudProgress();
  return result;
}
export function clearCloudCache() {
  owner = "";
  rows = new Map();
}
