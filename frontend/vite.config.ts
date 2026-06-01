/// <reference types="vitest/config" />

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import path from "path";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const usePolling = env.VITE_USE_POLLING === "true";
  const useMocks = env.VITE_USE_MOCKS === "1";
  const webapiTarget =
    env.WEBAPI_URL ??
    (usePolling
      ? "http://nebari-landing-webapi.nebari-system.svc.cluster.local:8080"
      : "http://localhost:8080");

  return {
    // Enable both React and Tailwind's Vite plugin.
    // The Tailwind plugin is required for Tailwind v4 utilities and shadcn
    // component styles to be compiled during development and build.
    plugins: [react(), tailwindcss()],

    resolve: {
      alias: {
        // shadcn uses the @ alias for generated imports such as "@/components"
        // and "@/lib/utils", so we map @ to the src directory here.
        "@": path.resolve(__dirname, "./src"),
      },
    },

    server: {
      // When mocks are disabled, Vite takes nginx's place for local development
      // and proxies relative /api/* calls to a webapi. Normal local dev defaults
      // to localhost:8080; dev-watch polling mode defaults to the in-cluster
      // ClusterIP unless WEBAPI_URL overrides it.
      proxy: !useMocks
        ? {
            "/api": {
              target: webapiTarget,
              changeOrigin: true,
              // Forward WebSocket connections for the notifications hub.
              ws: true,
            },
          }
        : undefined,

      // usePolling is required when the source directory is on a network-mounted
      // filesystem (9p via minikube mount, NFS, etc.) where inotify is not
      // supported.  The VITE_USE_POLLING env var lets us opt-in only inside the
      // dev-watch pod without affecting the normal local dev experience.
      watch: {
        usePolling,
        interval: 500,
        ignored: [
          "**/.playwright/**",
          "**/test-results/**",
          "**/playwright-report/**",
          "**/.playwright-artifacts-*/**",
        ],
      },

      // When running behind oauth2-proxy the browser connects to the proxy port
      // (PORT_LANDING, forwarded to 4180), not to Vite's internal port (80).
      // Setting clientPort to 0 tells Vite to echo back whatever port the
      // browser used, so the HMR WebSocket upgrade goes to the right place.
      hmr: usePolling
        ? { clientPort: Number(env.VITE_HMR_CLIENT_PORT ?? 0) || undefined }
        : undefined,
    },

    test: {
      environment: "jsdom",
      globals: true,
      setupFiles: "./src/test/setup.ts",
      css: true,
      include: ["tests/unit/**/*.{test,spec}.ts", "tests/unit/**/*.{test,spec}.tsx"],
      exclude: ["tests/e2e/**", "node_modules/**", "dist/**"],
    },

    // No SCSS preprocessor configuration is needed for shadcn/Tailwind.
    // USWDS-specific Sass load paths were removed as part of the migration away
    // from @uswds/uswds. Styling now comes from Tailwind utilities, theme tokens
    // in src/index.css, and shadcn component classes.
  };
});
