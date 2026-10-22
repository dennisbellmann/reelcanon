import { LocalizedError, message } from "../i18n-message";
import Dexie, { type Table } from "dexie";
import { getAccount, isAccountInitializing } from "../api/account";
import {
  clearCloudCache,
  importCloudBackup,
  listCloudProgress,
  updateCloudProgress,
} from "./cloud-progress";

export interface MovieProgress {
  imdbId: string;
  watched: boolean;
  watchlist: boolean;
  watchedAt: string | null;
  updatedAt: string;
}

class ReelCanonDatabase extends Dexie {
  progress!: Table<MovieProgress, string>;

  constructor() {
    super("reel-canon");
    this.version(1).stores({
      progress: "imdbId, watched, watchlist, updatedAt",
    });
  }
}

const database = new ReelCanonDatabase();

export interface ProgressRepository {
  list(): Promise<MovieProgress[]>;
  clear(): Promise<void>;
  setWatched(imdbId: string, watched: boolean): Promise<void>;
  setWatchlist(imdbId: string, watchlist: boolean): Promise<void>;
  exportBackup(): Promise<string>;
  importBackup(text: string): Promise<number>;
}

const identity = /^tt[0-9]{7,10}$/;
const isDate = (value: unknown): value is string =>
  typeof value === "string" &&
  /^\d{4}-\d{2}-\d{2}T/.test(value) &&
  Number.isFinite(Date.parse(value));

export function parseBackup(text: string): MovieProgress[] {
  if (text.length > 5_000_000)
    throw new LocalizedError(message("backup.tooLarge"));
  let backup: unknown;
  try {
    backup = JSON.parse(text);
  } catch {
    throw new LocalizedError(message("errors.invalidJson"));
  }
  if (
    !backup ||
    typeof backup !== "object" ||
    !("format" in backup) ||
    backup.format !== "reel-canon-progress/v1" ||
    !("progress" in backup) ||
    !Array.isArray(backup.progress) ||
    backup.progress.length > 20000
  ) {
    throw new LocalizedError(message("errors.backupFormat"));
  }
  const seen = new Set<string>();
  return backup.progress.map((row: unknown) => {
    if (
      !row ||
      typeof row !== "object" ||
      !("imdbId" in row) ||
      typeof row.imdbId !== "string" ||
      !identity.test(row.imdbId) ||
      seen.has(row.imdbId) ||
      !("watched" in row) ||
      typeof row.watched !== "boolean" ||
      !("watchlist" in row) ||
      typeof row.watchlist !== "boolean" ||
      !("updatedAt" in row) ||
      !isDate(row.updatedAt) ||
      !("watchedAt" in row) ||
      (row.watched ? !isDate(row.watchedAt) : row.watchedAt !== null)
    ) {
      throw new LocalizedError(message("errors.backupEntries"));
    }
    seen.add(row.imdbId);
    return {
      imdbId: row.imdbId,
      watched: row.watched,
      watchlist: row.watchlist,
      watchedAt: row.watchedAt as string | null,
      updatedAt: row.updatedAt,
    };
  });
}

async function update(
  imdbId: string,
  field: "watched" | "watchlist",
  value: boolean,
) {
  if (!identity.test(imdbId))
    throw new LocalizedError(message("errors.movieId"));
  await database.transaction("rw", database.progress, async () => {
    const current = await database.progress.get(imdbId);
    const now = new Date().toISOString();
    const next: MovieProgress = {
      imdbId,
      watched: current?.watched ?? false,
      watchlist: current?.watchlist ?? false,
      watchedAt: current?.watchedAt ?? null,
      updatedAt: now,
      [field]: value,
    };
    if (field === "watched")
      next.watchedAt = value ? (current?.watchedAt ?? now) : null;
    if (!next.watched && !next.watchlist)
      await database.progress.delete(imdbId);
    else await database.progress.put(next);
  });
}

export const localProgressRepository: ProgressRepository = {
  list: () => database.progress.toArray(),
  clear: () => database.progress.clear(),
  async setWatched(imdbId: string, watched: boolean) {
    await update(imdbId, "watched", watched);
  },
  async setWatchlist(imdbId: string, watchlist: boolean) {
    await update(imdbId, "watchlist", watchlist);
  },
  async exportBackup() {
    return JSON.stringify(
      {
        format: "reel-canon-progress/v1",
        exportedAt: new Date().toISOString(),
        progress: await database.progress.toArray(),
      },
      null,
      2,
    );
  },
  async importBackup(text) {
    const rows = parseBackup(text);
    await database.transaction("rw", database.progress, async () => {
      for (const row of rows) {
        const current = await database.progress.get(row.imdbId);
        if (
          !current ||
          Date.parse(row.updatedAt) > Date.parse(current.updatedAt)
        ) {
          if (row.watched || row.watchlist) await database.progress.put(row);
          else await database.progress.delete(row.imdbId);
        }
      }
    });
    return rows.length;
  },
};

function cloudMode() {
  if (isAccountInitializing())
    throw new LocalizedError(message("errors.waitSession"));
  const account = getAccount();
  if (account.error) throw new LocalizedError(account.error);
  return !!account.user;
}
export const progressRepository: ProgressRepository = {
  list: () =>
    cloudMode() ? listCloudProgress() : localProgressRepository.list(),
  clear: async () => {
    if (cloudMode()) throw new LocalizedError(message("errors.clearAccount"));
    clearCloudCache();
    await localProgressRepository.clear();
  },
  setWatched: (id, value) =>
    cloudMode()
      ? updateCloudProgress(id, "watched", value)
      : localProgressRepository.setWatched(id, value),
  setWatchlist: (id, value) =>
    cloudMode()
      ? updateCloudProgress(id, "watchlist", value)
      : localProgressRepository.setWatchlist(id, value),
  exportBackup: async () => {
    if (!cloudMode()) return localProgressRepository.exportBackup();
    const progress = (await listCloudProgress()).map(
      ({ imdbId, watched, watchlist, watchedAt, updatedAt }) => ({
        imdbId,
        watched,
        watchlist,
        watchedAt,
        updatedAt,
      }),
    );
    return JSON.stringify(
      {
        format: "reel-canon-progress/v1",
        exportedAt: new Date().toISOString(),
        progress,
      },
      null,
      2,
    );
  },
  importBackup: async (text) => {
    if (!cloudMode()) return localProgressRepository.importBackup(text);
    const parsed = parseBackup(text);
    const normalized = JSON.stringify({
      format: "reel-canon-progress/v1",
      progress: parsed,
    });
    const result = await importCloudBackup(normalized);
    if (result.unknown.length)
      throw new LocalizedError(
        message("backup.unknown", {
          ...result,
          ids: result.unknown.join(", "),
        }),
      );
    return result.added;
  },
};
