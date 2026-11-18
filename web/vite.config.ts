import { defineConfig } from "vite";

// `npm run dev` proxies the API to a mockserver on :8080, so the HUD is
// developed against the same process the container runs.
const api = "http://127.0.0.1:8080";

export default defineConfig({
  base: "./",
  build: {
    target: "es2023",
    outDir: "dist",
    emptyOutDir: true,
    // One script and one stylesheet, no inline code: the CSP allows 'self' only.
    assetsInlineLimit: 0,
    modulePreload: { polyfill: false },
  },
  server: {
    proxy: { "/v1": api, "/mock": api, "/oscal": api, "/healthz": api },
  },
});
