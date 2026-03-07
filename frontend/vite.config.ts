import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Resolve the proxy target in priority order:
//   1. VITE_API_URL  — set when running inside docker-compose (→ http://mockapi:8090)
//                      or when running npm run dev locally against the mock   (→ http://localhost:8090)
//   2. VITE_USE_POLLING === "true" — running inside the minikube dev-watch pod (→ cluster webapi ClusterIP)
//   3. otherwise     — no proxy; mockapi allows all origins via CORS so the
//                      browser can call http://localhost:8090 directly.
function resolveProxy(): Record<string, object> | undefined {
  if (process.env.VITE_API_URL) {
    return {
      "/api": {
        target: process.env.VITE_API_URL,
        changeOrigin: true,
        ws: true,
      },
    };
  }
  if (process.env.VITE_USE_POLLING === "true") {
    return {
      "/api": {
        target: process.env.WEBAPI_URL ?? "http://nebari-landing-webapi.nebari-system.svc.cluster.local:8080",
        changeOrigin: true,
        ws: true,
      },
    };
  }
  return undefined;
}

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: resolveProxy(),
    // usePolling is required when the source directory is on a network-mounted
    // filesystem (9p via minikube mount, NFS, etc.) where inotify is not
    // supported.  The VITE_USE_POLLING env var lets us opt-in only inside the
    // dev-watch pod without affecting the normal local dev experience.
    watch: {
      usePolling: process.env.VITE_USE_POLLING === "true",
      interval: 500,
    },
    // When running behind oauth2-proxy the browser connects to the proxy port
    // (PORT_LANDING, forwarded to 4180), not to Vite's internal port (80).
    // Setting clientPort to 0 tells Vite to echo back whatever port the
    // browser used, so the HMR WebSocket upgrade goes to the right place.
    hmr: process.env.VITE_USE_POLLING === "true"
      ? { clientPort: Number(process.env.VITE_HMR_CLIENT_PORT ?? 0) || undefined }
      : undefined,
  },
  // vite preview — allows developers to test the production build locally
  // with `npm run preview`. This is NOT used in production (nginx handles
  // proxying in the cluster). Set VITE_WEBAPI_URL to point at a local webapi.
  preview: {
    proxy: {
      "/api": {
        target: process.env.VITE_WEBAPI_URL ?? "http://localhost:8090",
        changeOrigin: true,
        ws: true,
      },
    },
  },

  css: {
    preprocessorOptions: {
      scss: {
        loadPaths: ["node_modules/@uswds/uswds/packages"],
        // Suppress deprecation warnings emitted by @uswds/uswds internals.
        // These are upstream issues in the dependency, not in our code.
        quietDeps: true,
        // Silence the @import deprecation specifically — USWDS still uses
        // @import internally and this propagates through our @use of its theme.
        // Remove once USWDS migrates to @use/@forward (tracked upstream).
        silenceDeprecations: ["import"],
      }
    },
  },
});
