/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig({
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
    // When running in the dev-watch pod (VITE_USE_POLLING=true), Vite takes
    // nginx's place. Proxy /api/ to the webapi ClusterIP so the browser's
    // relative-URL API calls are forwarded server-side, with the Authorization
    // header that oauth2-proxy injected preserved end-to-end.
    proxy: process.env.VITE_USE_POLLING === "true" ? {
      "/api": {
        target: process.env.WEBAPI_URL ?? "http://nebari-landing-webapi.nebari-system.svc.cluster.local:8080",
        changeOrigin: true,
        // Forward WebSocket connections for the notifications hub.
        ws: true,
      },
    } : undefined,

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
});
