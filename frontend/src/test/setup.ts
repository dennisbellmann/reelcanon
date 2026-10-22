import "fake-indexeddb/auto";
import "@testing-library/jest-dom/vitest";
import i18n from "../i18n";
import { beforeEach } from "vitest";
beforeEach(async () => {
  await i18n.changeLanguage("de");
});
