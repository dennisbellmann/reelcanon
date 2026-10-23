const basePath = import.meta.env.BASE_URL;

export function publicPath(path: string): string {
  if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(path)) return path;
  return `${basePath}${path.replace(/^\/+/, "")}`;
}

export const apiBase =
  import.meta.env.VITE_API_BASE_URL || publicPath("api/v1");
