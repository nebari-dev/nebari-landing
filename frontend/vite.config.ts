import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
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
