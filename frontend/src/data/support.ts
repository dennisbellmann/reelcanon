export function getSupportUrl(value: string | undefined): string | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    if (
      url.protocol !== "https:" ||
      url.hostname !== "ko-fi.com" ||
      url.port ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      !/^\/[a-z0-9](?:[a-z0-9_-]*[a-z0-9])?\/?$/i.test(url.pathname)
    )
      return null;
    return `https://ko-fi.com${url.pathname.replace(/\/$/, "")}`;
  } catch {
    return null;
  }
}

export const supportUrl = getSupportUrl(import.meta.env.VITE_SUPPORT_URL);
