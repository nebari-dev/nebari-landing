import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
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
