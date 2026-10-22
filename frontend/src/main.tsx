import { t } from "./i18n";
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import { catalogApi } from "./api/client";
import App from "./routes/App";
import "./styles.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 60_000, retry: 1 } },
});
const basePath = import.meta.env.BASE_URL.replace(/\/$/, "");
if (
  basePath &&
  window.location.pathname !== basePath &&
  !window.location.pathname.startsWith(`${basePath}/`)
) {
  const nestedPath = window.location.pathname.replace(/^\/+/, "");
  const targetPath = nestedPath ? `${basePath}/${nestedPath}` : `${basePath}/`;
  window.location.replace(`${targetPath}${window.location.search}${window.location.hash}`);
}

const router = createBrowserRouter([
  {
    path: "*",
    element: <App />,
    hydrateFallbackElement: (
      <p role="status" className="px-6 py-12 text-muted">
        {t("common.loading")}
      </p>
    ),
    loader: async ({ request }) => {
      const { pathname: requestedPath } = new URL(request.url);
      const pathname =
        basePath && requestedPath.startsWith(`${basePath}/`)
          ? requestedPath.slice(basePath.length)
          : requestedPath === basePath
            ? "/"
            : requestedPath;
      const collection = pathname.match(
        /^\/canons\/([^/]+)(?:\/movies\/[^/]+)?\/?$/,
      );
      try {
        // Restore scrolling after the catalog is ready, including a cold return
        // from a reloaded detail page where React Query has no cached films.
        if (collection) {
          const slug = decodeURIComponent(collection[1]);
          await queryClient.ensureQueryData({
            queryKey: ["canon", slug],
            queryFn: () => catalogApi.getCanon(slug),
          });
        } else if (pathname === "/" || pathname === "/canons") {
          await queryClient.ensureQueryData({
            queryKey: ["canons"],
            queryFn: catalogApi.listCanons,
          });
        }
      } catch {
        // The page's query renders its existing error and retry controls.
      }
      return null;
    },
  },
], {
  basename: basePath || "/",
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </React.StrictMode>,
);
