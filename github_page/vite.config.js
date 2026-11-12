import { defineConfig } from "vite";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

// The site is served from https://<user>.github.io/troopa/, so every asset
// path is relative to /troopa/. Override with TROOPA_SITE_BASE for a custom domain.
export default defineConfig({
  base: process.env.TROOPA_SITE_BASE ?? "/troopa/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: resolve(here, "index.html"),
        docs: resolve(here, "docs.html"),
        changelog: resolve(here, "changelog.html"),
      },
    },
  },
});
