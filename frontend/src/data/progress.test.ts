import { beforeEach, describe, expect, it } from "vitest";
import { parseBackup, progressRepository } from "./progress";

beforeEach(async () => {
  await progressRepository.clear();
});
describe("progress backup", () => {
  it("exports/imports watched timestamps and watchlist using stable IMDb keys", async () => {
    await progressRepository.setWatched("tt0111161", true);
    await progressRepository.setWatchlist("tt0068646", true);
    const expected = await progressRepository.list();
    const backup = await progressRepository.exportBackup();
    await progressRepository.clear();
    expect(await progressRepository.importBackup(backup)).toBe(2);
    expect(await progressRepository.list()).toEqual(expected);
  });
  it("rejects the entire file before writing when a later entry is invalid", async () => {
    await progressRepository.setWatched("tt0111161", true);
    const before = await progressRepository.list();
    const row = before[0];
    const input = JSON.stringify({
      format: "reel-canon-progress/v1",
      progress: [
        { ...row, imdbId: "tt0068646" },
        { ...row, watched: "yes" },
      ],
    });
    await expect(progressRepository.importBackup(input)).rejects.toThrow();
    expect(await progressRepository.list()).toEqual(before);
  });
  it("rejects duplicate identities, unsupported versions and invalid timestamps", async () => {
    await progressRepository.setWatched("tt0111161", true);
    const [row] = await progressRepository.list();
    for (const progress of [
      [row, row],
      [{ ...row, watchedAt: "not-a-date" }],
      [{ ...row, imdbId: "bad" }],
    ]) {
      expect(() =>
        parseBackup(
          JSON.stringify({ format: "reel-canon-progress/v1", progress }),
        ),
      ).toThrow();
    }
    expect(() => parseBackup('{"format":"future/v2","progress":[]}')).toThrow();
  });
  it("does not overwrite newer local changes during merge", async () => {
    await progressRepository.setWatched("tt0111161", true);
    const [row] = await progressRepository.list();
    await progressRepository.importBackup(
      JSON.stringify({
        format: "reel-canon-progress/v1",
        progress: [
          {
            ...row,
            watched: false,
            watchlist: true,
            watchedAt: null,
            updatedAt: "2020-01-01T00:00:00Z",
          },
        ],
      }),
    );
    expect(await progressRepository.list()).toEqual([row]);
  });
  it("preserves both flags under concurrent watched and watchlist writes", async () => {
    await Promise.all([
      progressRepository.setWatched("tt0111161", true),
      progressRepository.setWatchlist("tt0111161", true),
    ]);
    expect(await progressRepository.list()).toMatchObject([
      { watched: true, watchlist: true },
    ]);
  });
});
