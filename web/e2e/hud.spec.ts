// The HUD against a running server (S1-11, #122). Data-agnostic: it reads the
// personas the server offers and walks whatever their documents produce, so
// the same suite holds over the fixtures and over a tester's own OSCAL.

import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

interface Persona {
  id: string;
  name: string;
  "node-uuid": string;
}

interface Tree {
  id: string;
  name: string;
}

const WCAG = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

/** Records every CSP violation the page raises, from before its first script. */
async function trapCsp(page: Page): Promise<() => Promise<string[]>> {
  const console: string[] = [];
  page.on("console", (m) => {
    if (m.text().includes("Content Security Policy")) console.push(m.text());
  });
  await page.addInitScript(() => {
    const w = window as unknown as { __csp: string[] };
    w.__csp = [];
    document.addEventListener("securitypolicyviolation", (e) => {
      w.__csp.push(`${e.violatedDirective} ${e.blockedURI}`);
    });
  });
  return async () => [...console, ...(await page.evaluate(() => (window as unknown as { __csp: string[] }).__csp))];
}

async function axe(page: Page): Promise<void> {
  const r = await new AxeBuilder({ page }).withTags(WCAG).analyze();
  expect(r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`)).toEqual([]);
}

async function personas(page: Page): Promise<Persona[]> {
  const res = await page.request.get("/mock/personas");
  expect(res.ok()).toBe(true);
  const list = (await res.json()) as Persona[];
  expect(list.length).toBeGreaterThan(0);
  return list;
}

test("every persona sees its own tree, heat, cell and evidence chain, with no CSP violation", async ({ page }) => {
  const csp = await trapCsp(page);
  for (const p of await personas(page)) {
    const tree = (await (await page.request.get("/v1/tree", { headers: { "X-Horizon-Persona": p.id } })).json()) as Tree;
    await page.goto(`/#p=${encodeURIComponent(p.id)}`);
    await expect(page.locator("h1")).toHaveText(tree.name);
    await expect(page.locator("#persona")).toHaveValue(p.id);

    const cells = page.locator("button.cell");
    if ((await cells.count()) === 0) {
      await expect(page.locator(".all-quiet, .heat .quiet")).toBeVisible();
      continue;
    }
    await cells.first().click();
    await expect(page.locator("#cell-h")).toBeVisible();
    await expect(cells.first()).toHaveAttribute("aria-pressed", "true");

    const chain = page.locator(".controls button").first();
    if ((await chain.count()) > 0) {
      await chain.click();
      await expect(page.locator("#chain-h")).toBeVisible();
      await expect(page.locator(".chain li")).toHaveCount(5);
    }
  }
  expect(await csp()).toEqual([]);
});

test("both themes pass axe at WCAG 2.2 AA, with a cell and its chain open", async ({ page }) => {
  const [p] = await personas(page);
  if (p === undefined) throw new Error("no persona");
  for (const scheme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme: scheme, reducedMotion: "reduce" });
    await page.goto(`/#p=${encodeURIComponent(p.id)}`);
    await expect(page.locator("h1")).toBeVisible();
    await axe(page);
    const cell = page.locator("button.cell").first();
    if ((await cell.count()) > 0) {
      await cell.click();
      await expect(page.locator("#cell-h")).toBeVisible();
      const chain = page.locator(".controls button").first();
      if ((await chain.count()) > 0) {
        await chain.click();
        await expect(page.locator("#chain-h")).toBeVisible();
      }
      await axe(page);
    }
  }
});

test("the heatmap is reachable and operable from the keyboard", async ({ page }) => {
  const [p] = await personas(page);
  if (p === undefined) throw new Error("no persona");
  await page.goto(`/#p=${encodeURIComponent(p.id)}`);
  await expect(page.locator("h1")).toBeVisible();
  const cells = page.locator("button.cell");
  test.skip((await cells.count()) === 0, "nothing down: no cell to operate");
  for (let i = 0; i < 40; i++) {
    await page.keyboard.press("Tab");
    if (await page.evaluate(() => document.activeElement?.classList.contains("cell") ?? false)) break;
  }
  await expect(page.locator("button.cell:focus")).toHaveCount(1);
  await page.keyboard.press("Enter");
  await expect(page.locator("#cell-h")).toBeVisible();
});

test("blockers carry a glyph, not only a colour", async ({ page }) => {
  const [p] = await personas(page);
  if (p === undefined) throw new Error("no persona");
  await page.goto(`/#p=${encodeURIComponent(p.id)}`);
  await expect(page.locator("h1")).toBeVisible();
  for (const cell of await page.locator("button.cell").all()) {
    await expect(cell.locator(".glyph")).toHaveText(/[✖▼◆●]/);
    await expect(cell).toHaveAttribute("aria-label", /: (Blocks ATO|Degraded|Watch|Nominal)/);
  }
});

test("a node outside the caller's tree falls back to the caller's root", async ({ page }) => {
  const [p] = await personas(page);
  if (p === undefined) throw new Error("no persona");
  await page.goto(`/#p=${encodeURIComponent(p.id)}&n=00000000-0000-0000-0000-000000000000`);
  await expect(page).toHaveURL(new RegExp(`#p=${p.id}$`));
  await expect(page.locator("h1")).toBeVisible();
});

test("moving the horizon is in the URL and in the projection", async ({ page }) => {
  const [p] = await personas(page);
  if (p === undefined) throw new Error("no persona");
  await page.goto(`/#p=${encodeURIComponent(p.id)}`);
  await expect(page.locator("h1")).toBeVisible();
  const base = await page.locator("#horizon").inputValue();
  await page.locator("#horizon").fill("2030-01-01");
  await page.locator("#horizon").dispatchEvent("change");
  await expect(page).toHaveURL(/h=2030-01-01/);
  await expect(page.locator(".asof")).toContainText("Projected to 2030-01-01");
  await page.locator("#horizon-reset").click();
  await expect(page.locator("#horizon")).toHaveValue(base);
});

test("a boundary's OSCAL documents download and verify against the export's hash", async ({ page }) => {
  const list = await personas(page);
  let checked = 0;
  for (const p of list) {
    await page.goto(`/#p=${encodeURIComponent(p.id)}&n=${p["node-uuid"]}`);
    await expect(page.locator("h1")).toBeVisible();
    const buttons = page.locator(".export button");
    if ((await buttons.count()) === 0) continue;
    const download = page.waitForEvent("download");
    await buttons.first().click();
    await download;
    await expect(page.locator(".export .verify").first()).toContainText("sha256 matches");
    checked++;
  }
  expect(checked).toBeGreaterThan(0);
});

test("a drillable row opens its own heatmap, and the breadcrumb walks back", async ({ page }) => {
  for (const p of await personas(page)) {
    await page.goto(`/#p=${encodeURIComponent(p.id)}`);
    await expect(page.locator("h1")).toBeVisible();
    const row = page.locator(".grid tbody th a").first();
    if ((await row.count()) === 0) continue;
    const top = await page.locator("h1").textContent();
    const name = await row.textContent();
    await row.click();
    await expect(page.locator("h1")).toHaveText(name ?? "");
    await page.locator(".crumbs a").first().click();
    await expect(page.locator("h1")).toHaveText(top ?? "");
    return;
  }
  test.skip(true, "no persona's root has a drillable row in these documents");
});

test("no horizontal scroll at phone width", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });
  for (const p of await personas(page)) {
    await page.goto(`/#p=${encodeURIComponent(p.id)}&n=${p["node-uuid"]}`);
    await expect(page.locator("h1")).toBeVisible();
    const [scroll, width] = await page.evaluate(() => [document.documentElement.scrollWidth, window.innerWidth]);
    expect(scroll, `${p.id}: page is ${scroll}px wide in a ${width}px viewport`).toBeLessThanOrEqual(width);
  }
});
