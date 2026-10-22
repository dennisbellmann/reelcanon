import en from "../../localization/en.json";
import { t } from "./i18n";

type BaseKey<K> = K extends `${infer Base}_${"one" | "other" | "many"}`
  ? Base
  : K;
export type MessageKey = BaseKey<keyof typeof en>;
export interface LocalizedMessage {
  key: MessageKey;
  values?: Record<string, unknown>;
}
export const message = (
  key: MessageKey,
  values?: Record<string, unknown>,
): LocalizedMessage => ({ key, values });
export const renderMessage = (value: LocalizedMessage) =>
  t(value.key, value.values);
export class LocalizedError extends Error {
  constructor(public readonly text: LocalizedMessage) {
    super(renderMessage(text));
  }
}
export const errorMessage = (
  cause: unknown,
  fallback: MessageKey,
): LocalizedMessage =>
  cause instanceof LocalizedError ? cause.text : message(fallback);

export function apiErrorMessage(
  code: unknown,
  fallback: MessageKey,
): LocalizedMessage {
  const key = `api.${code}`;
  return message(
    typeof code === "string" && Object.hasOwn(en, key)
      ? (key as MessageKey)
      : fallback,
  );
}
