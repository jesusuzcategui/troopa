import { defineConfig } from "vite";

// The site is served from https://<user>.github.io/troopa/, so every asset
// path is relative to /troopa/. Override with TROOPA_SITE_BASE for a custom domain.
export default defineConfig({
  base: process.env.TROOPA_SITE_BASE ?? "/troopa/",
  build: { outDir: "dist", emptyOutDir: true },
});
