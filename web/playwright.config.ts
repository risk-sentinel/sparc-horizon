import { defineConfig, devices } from "@playwright/test";

// The smoke runs against a server that is already up — in CI, the built image
// (S1-11) — never one Playwright starts, so it tests what ships.
//   HUD_FIXTURES_URL  the image over its fixture federation
//   HUD_NIST_URL      the image over NIST's example set, mounted at /data
const fixtures = process.env["HUD_FIXTURES_URL"] ?? "http://127.0.0.1:8080";
const nist = process.env["HUD_NIST_URL"] ?? "http://127.0.0.1:8081";

export default defineConfig({
  testDir: "e2e",
  forbidOnly: true,
  retries: 0,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: { ...devices["Desktop Chrome"], channel: "chromium", trace: "retain-on-failure" },
  projects: [
    { name: "fixtures", use: { baseURL: fixtures } },
    { name: "nist", use: { baseURL: nist } },
  ],
});
