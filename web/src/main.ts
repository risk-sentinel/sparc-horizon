// The Horizon HUD over API v0 (#122, P3). One focal point per lens: the next
// best action leads, the heatmap is management by exception, and every cell
// drills to the controls and then to the OSCAL evidence chain behind them.

import "./style.css";
import { Api, ApiError, personas as listPersonas } from "./api.ts";
import { append, clear, el } from "./dom.ts";
import {
  NODE_TYPES,
  STATES,
  addDays,
  byWorst,
  cellLabel,
  countdown,
  daysBetween,
  displayControl,
  drillable,
  familyName,
  findNode,
  formatRoute,
  names,
  parseRoute,
  pathTo,
  shortId,
  today,
  type Route,
} from "./model.ts";
import { initTheme, themeButton } from "./theme.ts";
import type { CellDetail, Chain, Export, Heat, NextAction, Persona, TreeNode } from "./types.ts";

const LOOK_AHEAD_DAYS = 365;

interface Loaded {
  route: Route;
  persona: Persona | undefined;
  tree: TreeNode;
  node: TreeNode;
  heat: Heat | undefined;
  next: NextAction | undefined;
  cell: CellDetail | undefined;
  chain: Chain | undefined;
  exp: Export | undefined;
  /** The server's default horizon: today, or its -horizon flag. */
  base: string;
}

const root = document.getElementById("app");
if (root === null) throw new Error("#app missing");
const app: HTMLElement = root;

let people: Persona[] = [];
let serverDefault: string | undefined;
let generation = 0;

function go(r: Route): void {
  const hash = formatRoute(r);
  if (location.hash === hash) void load();
  else location.hash = hash;
}

/** A 404 here may be absent or not yours; the UI never says which. */
async function optional<T>(p: Promise<T>): Promise<T | undefined> {
  try {
    return await p;
  } catch (e) {
    if (e instanceof ApiError && e.notVisible) return undefined;
    throw e;
  }
}

async function load(): Promise<void> {
  const mine = ++generation;
  const route = parseRoute(location.hash);
  app.setAttribute("aria-busy", "true");
  try {
    const persona = people.find((p) => p.id === route.persona) ?? people[0];
    if (persona !== undefined && route.persona !== persona.id) {
      go({ ...route, persona: persona.id, node: undefined, col: undefined, chain: undefined });
      return;
    }
    const api = new Api(persona?.id ?? "");
    const tree = await api.tree();
    const node = route.node === undefined ? tree : findNode(tree, route.node);
    if (node === undefined) {
      // Not in this caller's tree, perhaps after switching persona.
      go({ persona: route.persona, horizon: route.horizon });
      return;
    }
    const heatNode = drillable(node) ? node.id : undefined;
    const [heat, next, cell, chain, exp] = await Promise.all([
      heatNode === undefined ? undefined : optional(api.heat(heatNode, route.horizon)),
      optional(api.nextAction(node.id, route.horizon)),
      cellFor(api, tree, node, route),
      route.chain === undefined ? undefined : optional(api.chain(route.chain)),
      node.nodeType === "boundary" ? optional(api.export(node.id)) : undefined,
    ]);
    if (route.horizon === undefined) serverDefault = heat?.horizon ?? next?.horizon ?? serverDefault;
    if (mine !== generation) return;
    render({ route, persona, tree, node, heat, next, cell, chain, exp, base: serverDefault ?? today() });
  } catch (e) {
    if (mine !== generation) return;
    showError(e);
  } finally {
    if (mine === generation) app.removeAttribute("aria-busy");
  }
}

/**
 * A cell is a row's family. A row that is itself drillable answers /cell; a
 * system row does not (systems are leaves), so its cell is the parent's
 * family narrowed to that component.
 */
async function cellFor(api: Api, tree: TreeNode, node: TreeNode, route: Route): Promise<CellDetail | undefined> {
  if (route.col === undefined || !drillable(node)) return undefined;
  const row = route.row === undefined ? undefined : findNode(tree, route.row);
  if (row !== undefined && drillable(row)) return optional(api.cell(row.id, route.col, route.horizon));
  const c = await optional(api.cell(node.id, route.col, route.horizon));
  if (c === undefined || row === undefined) return c;
  return { ...c, controls: c.controls.filter((x) => x.component === row.id) };
}

function showError(e: unknown): void {
  const msg = e instanceof ApiError ? `${e.message} (${e.code}, HTTP ${e.status})` : String(e);
  clear(
    app,
    header(undefined),
    el("main", { id: "main" }, el("div", { class: "error", role: "alert" }, el("strong", {}, "The API refused or failed: "), msg)),
  );
}

// --- Layout -----------------------------------------------------------------

function render(v: Loaded): void {
  const focused = document.activeElement?.id;
  const horizon = v.route.horizon ?? v.base;
  clear(
    app,
    header(v),
    el(
      "main",
      { id: "main" },
      breadcrumb(v),
      el(
        "div",
        { class: "title" },
        el("h1", { id: "node-title", tabindex: -1 }, v.node.name),
        el("span", { class: "type" }, NODE_TYPES[v.node.nodeType]),
        el("span", { class: "asof" }, `Projected to ${horizon}`, horizon === today() ? "" : ` (${countdown(horizon, today())})`),
      ),
      nextActionCard(v),
      el("div", { class: "work" }, heatSection(v, horizon), detail(v, horizon)),
      exportSection(v),
    ),
  );
  if (focused !== undefined && focused !== "") document.getElementById(focused)?.focus();
}

function header(v: Loaded | undefined): HTMLElement {
  const bar = el(
    "header",
    { class: "bar" },
    el("a", { class: "skip", href: "#main" }, "Skip to content"),
    el("span", { class: "brand" }, "SPARC Horizon"),
  );
  if (v !== undefined && people.length > 0) bar.append(personaPicker(v));
  if (v !== undefined) bar.append(horizonControls(v));
  bar.append(themeButton());
  return bar;
}

function personaPicker(v: Loaded): HTMLElement {
  const select = el("select", { id: "persona" });
  for (const p of people) {
    select.append(el("option", { value: p.id, selected: p.id === v.persona?.id }, p.name));
  }
  select.addEventListener("change", () => {
    go({ persona: select.value, horizon: v.route.horizon });
  });
  return el(
    "div",
    { class: "field" },
    el("label", { for: "persona" }, "Viewing as"),
    select,
    v.persona?.about === undefined ? null : el("span", { class: "hint" }, v.persona.about),
  );
}

function horizonControls(v: Loaded): HTMLElement {
  const current = v.route.horizon ?? v.base;
  const offset = Math.max(0, Math.min(LOOK_AHEAD_DAYS, daysBetween(v.base, current)));
  const date = el("input", { id: "horizon", type: "date", value: current, required: true });
  const range = el("input", {
    id: "lookahead",
    type: "range",
    min: 0,
    max: LOOK_AHEAD_DAYS,
    step: 1,
    value: offset,
    "aria-valuetext": `${offset} days ahead, ${addDays(v.base, offset)}`,
  });
  const out = el("output", { for: "lookahead", id: "lookahead-out" }, `+${offset} d`);
  const commit = (h: string): void => {
    go({ ...v.route, horizon: h === v.base ? undefined : h });
  };
  date.addEventListener("change", () => {
    if (date.value !== "") commit(date.value);
  });
  range.addEventListener("input", () => {
    const d = Number(range.value);
    out.textContent = `+${d} d`;
    range.setAttribute("aria-valuetext", `${d} days ahead, ${addDays(v.base, d)}`);
    date.value = addDays(v.base, d);
  });
  range.addEventListener("change", () => {
    commit(addDays(v.base, Number(range.value)));
  });
  const reset = el("button", { type: "button", id: "horizon-reset", disabled: v.route.horizon === undefined }, "Reset");
  reset.addEventListener("click", () => {
    commit(v.base);
  });
  return el(
    "div",
    { class: "field horizon" },
    el("label", { for: "horizon" }, "Horizon"),
    date,
    el("label", { for: "lookahead", class: "visually-hidden" }, "Look ahead from ", v.base),
    range,
    out,
    reset,
  );
}

function breadcrumb(v: Loaded): HTMLElement {
  const path = pathTo(v.tree, v.node.id) ?? [v.tree];
  const ol = el("ol");
  path.forEach((n, i) => {
    const last = i === path.length - 1;
    ol.append(
      el(
        "li",
        {},
        last
          ? el("span", { "aria-current": "page" }, n.name)
          : el("a", { href: formatRoute({ persona: v.route.persona, node: n.id, horizon: v.route.horizon }) }, n.name),
      ),
    );
  });
  return el("nav", { "aria-label": "Breadcrumb", class: "crumbs" }, ol);
}

// --- Next best action -------------------------------------------------------

function nextActionCard(v: Loaded): HTMLElement | null {
  const n = v.next;
  if (n === undefined) return null;
  const card = el(
    "section",
    { class: "next", "aria-labelledby": "next-h" },
    el("h2", { id: "next-h" }, "Next best action"),
    el("p", { class: "next-title" }, n.title),
    el("p", { class: "consequence" }, n.consequence),
  );
  if (n.chain !== undefined) {
    const chain = n.chain;
    const b = el("button", { type: "button", id: "next-chain" }, "Show the evidence chain");
    b.addEventListener("click", () => {
      go({ ...v.route, chain });
    });
    card.append(b);
  }
  return card;
}

// --- Heatmap ----------------------------------------------------------------

function heatSection(v: Loaded, horizon: string): HTMLElement {
  const s = el("section", { class: "heat", "aria-labelledby": "heat-h" });
  if (!drillable(v.node)) {
    append(s, [el("h2", { id: "heat-h" }, "Controls"), el("p", { class: "quiet" }, "A system is a row in its boundary's heatmap.")]);
    return s;
  }
  const h = v.heat;
  append(s, [el("h2", { id: "heat-h" }, "Control families by ", childNoun(v.node))]);
  if (h === undefined) {
    s.append(el("p", { class: "quiet" }, "No heatmap for this node."));
    return s;
  }
  if (h.columns.length === 0) {
    s.append(
      el("p", { class: "all-quiet" }, el("span", { "aria-hidden": "true" }, STATES.nominal.glyph, " "), `Every family is nominal at ${horizon}.`),
    );
  } else {
    s.append(heatTable(v, h));
  }
  if (h.collapsed.length > 0) {
    s.append(
      el(
        "p",
        { class: "collapsed" },
        el("span", { class: "label" }, "Nominal, collapsed: "),
        ...h.collapsed.map((c) => el("abbr", { title: familyName(c) }, c)),
      ),
    );
  }
  s.append(legend());
  return s;
}

function childNoun(n: TreeNode): string {
  const kinds = new Set((n.children ?? []).map((c) => c.nodeType));
  if (kinds.size !== 1) return "child";
  const [k] = [...kinds];
  return k === undefined ? "child" : NODE_TYPES[k].toLowerCase();
}

function heatTable(v: Loaded, h: Heat): HTMLElement {
  const head = el("tr", {}, el("th", { scope: "col" }, el("span", { class: "visually-hidden" }, "Row")));
  for (const c of h.columns) {
    head.append(el("th", { scope: "col" }, el("abbr", { title: familyName(c) }, c)));
  }
  const body = el("tbody");
  for (const row of h.rows) {
    const child = findNode(v.tree, row.id);
    const rowHead = el("th", { scope: "row" });
    if (child !== undefined && drillable(child)) {
      rowHead.append(el("a", { href: formatRoute({ persona: v.route.persona, node: row.id, horizon: v.route.horizon }) }, row.name));
    } else {
      rowHead.append(row.name);
    }
    const tr = el("tr", {}, rowHead);
    for (const col of h.columns) {
      const cell = row.cells.find((c) => c.col === col);
      if (cell === undefined) {
        tr.append(el("td", { class: "none" }, el("span", { class: "visually-hidden" }, "No controls")));
        continue;
      }
      const selected = v.route.col === col && v.route.row === row.id;
      const b = el(
        "button",
        {
          type: "button",
          id: `cell-${row.id}-${col}`,
          class: `cell s-${cell.state}${cell.worsening ? " worse" : ""}${cell.inherited ? " inh" : ""}`,
          "aria-label": cellLabel(row.name, cell),
          "aria-pressed": selected ? "true" : "false",
        },
        el("span", { class: "glyph", "aria-hidden": "true" }, STATES[cell.state].glyph),
        cell.blockers > 0 ? el("span", { class: "n", "aria-hidden": "true" }, cell.blockers) : null,
        cell.worsening ? el("span", { class: "trend", "aria-hidden": "true" }, "↘") : null,
      );
      b.addEventListener("click", () => {
        go({ persona: v.route.persona, node: v.node.id, horizon: v.route.horizon, row: row.id, col });
      });
      tr.append(el("td", {}, b));
    }
    body.append(tr);
  }
  return el(
    "div",
    { class: "scroll" },
    el("table", { class: "grid" }, el("caption", { class: "visually-hidden" }, "Control family state by row"), el("thead", {}, head), body),
  );
}

function legend(): HTMLElement {
  return el(
    "ul",
    { class: "legend", "aria-label": "Legend" },
    ...(["blocks", "degraded", "watch", "nominal"] as const).map((s) =>
      el("li", {}, el("span", { class: `swatch s-${s}`, "aria-hidden": "true" }, STATES[s].glyph), STATES[s].label),
    ),
    el("li", {}, el("span", { class: "swatch", "aria-hidden": "true" }, "↘"), "Worsening"),
    el("li", {}, el("span", { class: "swatch inh-key", "aria-hidden": "true" }), "Inherited"),
  );
}

// --- Drill: cell, then chain -----------------------------------------------

function detail(v: Loaded, horizon: string): HTMLElement | null {
  if (v.chain === undefined && v.cell === undefined) {
    if (v.route.col !== undefined || v.route.chain !== undefined) {
      return el("aside", { class: "detail" }, el("p", { class: "quiet" }, "Nothing to show for that selection."));
    }
    return null;
  }
  const aside = el("aside", { class: "detail", "aria-label": "Detail" });
  if (v.cell !== undefined) aside.append(cellPanel(v, v.cell, horizon));
  if (v.chain !== undefined) aside.append(chainPanel(v, v.chain));
  return aside;
}

function cellPanel(v: Loaded, c: CellDetail, horizon: string): HTMLElement {
  const label = names(v.tree);
  const close = el("button", { type: "button", class: "close", id: "cell-close", "aria-label": "Close family detail" }, "×");
  close.addEventListener("click", () => {
    go({ ...v.route, row: undefined, col: undefined, chain: undefined });
  });
  const list = el("ol", { class: "controls" });
  for (const ctl of [...c.controls].sort(byWorst)) {
    const b = el(
      "button",
      { type: "button", id: `chain-${ctl.chain}`, "aria-pressed": v.route.chain === ctl.chain ? "true" : "false" },
      "Evidence chain",
    );
    b.addEventListener("click", () => {
      go({ ...v.route, chain: ctl.chain });
    });
    list.append(
      el(
        "li",
        { class: `ctl s-${ctl.state}` },
        el("span", { class: "badge", "aria-hidden": "true" }, STATES[ctl.state].glyph),
        el(
          "div",
          {},
          el("strong", {}, displayControl(ctl.controlId)),
          " on ",
          label.get(ctl.component) ?? el("code", { title: ctl.component }, shortId(ctl.component)),
          el("span", { class: "visually-hidden" }, `: ${STATES[ctl.state].label}`),
          ctl.blocksAto ? el("span", { class: "ato" }, "Blocks ATO") : null,
          el("p", {}, ctl.reason),
          ctl.expires === undefined
            ? null
            : el("p", { class: "expires" }, "Evidence expires ", el("time", { datetime: ctl.expires }, ctl.expires), ` — ${countdown(ctl.expires, horizon)} at the horizon`),
          b,
        ),
      ),
    );
  }
  return el(
    "section",
    { class: "panel", "aria-labelledby": "cell-h" },
    el(
      "div",
      { class: "panel-head" },
      el("h2", { id: "cell-h", tabindex: -1 }, `${c.col} · ${familyName(c.col)}`, v.route.row === undefined ? "" : ` · ${label.get(v.route.row) ?? ""}`),
      close,
    ),
    el("p", { class: "quiet" }, `${c.controls.length} control${c.controls.length === 1 ? "" : "s"} at ${c.horizon}, worst first.`),
    list,
  );
}

/** The evidence chain in its fixed order (docs/03-data-model.md). */
function chainPanel(v: Loaded, ch: Chain): HTMLElement {
  const label = names(v.tree);
  const close = el("button", { type: "button", class: "close", id: "chain-close", "aria-label": "Close evidence chain" }, "×");
  close.addEventListener("click", () => {
    go({ ...v.route, chain: undefined });
  });
  const steps: [string, string | undefined, string][] = [
    ["Back-matter resource", ch.resource, "The hashed evidence artifact."],
    ["Observation", ch.observation, ch.expires === undefined ? "No expiry recorded." : `Expires ${ch.expires}, ${countdown(ch.expires, today())} from today.`],
    ["Finding", ch.finding, "The assessor's determination."],
    ["Risk", ch.risk, ch.blocksAto ? "Blocks the authorization." : "Does not block the authorization."],
    ["POA&M item", ch.poamItem, "The remediation commitment."],
  ];
  const ol = el("ol", { class: "chain" });
  for (const [name, id, note] of steps) {
    ol.append(
      el(
        "li",
        { class: id === undefined ? "missing" : "" },
        el("strong", {}, name),
        id === undefined ? el("span", { class: "quiet" }, " — none in the documents") : el("code", {}, id),
        id === undefined ? null : el("span", { class: "note" }, note),
      ),
    );
  }
  return el(
    "section",
    { class: "panel", "aria-labelledby": "chain-h" },
    el("div", { class: "panel-head" }, el("h2", { id: "chain-h", tabindex: -1 }, `Evidence chain · ${displayControl(ch.controlId)}`), close),
    el(
      "p",
      { class: "quiet" },
      "Component ",
      label.get(ch.component) ?? ch.component,
      ". Requirement ",
      el("code", {}, ch.requirement),
      ".",
    ),
    ol,
  );
}

// --- Export ------------------------------------------------------------------

function exportSection(v: Loaded): HTMLElement | null {
  const e = v.exp;
  if (e === undefined) return null;
  const api = new Api(v.persona?.id ?? "");
  const list = el("ul", { class: "docs" });
  for (const d of e.documents) {
    const status = el("span", { class: "verify" }, el("span", { class: "quiet" }, "Not downloaded"));
    const name = d.href.split("/").pop() ?? d.href;
    const b = el("button", { type: "button", "aria-label": `Download and verify ${name}` }, "Download and verify");
    b.addEventListener("click", () => {
      void download(api, d.href, name, d.sha256, status);
    });
    list.append(
      el(
        "li",
        {},
        el("strong", {}, d.model),
        el("code", { class: "file" }, name),
        el("span", { class: "sha" }, "sha256 ", el("code", { title: d.sha256 }, `${d.sha256.slice(0, 16)}…`)),
        el("span", { class: "act" }, b, status),
      ),
    );
  }
  return el(
    "section",
    { class: "export", "aria-labelledby": "export-h" },
    el("h2", { id: "export-h" }, "OSCAL documents"),
    el("p", { class: "quiet" }, "Everything on this page recomputes from these documents. Each download is checked against the hash the export lists."),
    list,
  );
}

async function download(api: Api, href: string, name: string, want: string, status: HTMLElement): Promise<void> {
  clear(status, "Downloading…");
  try {
    const buf = await (await api.raw(href)).arrayBuffer();
    const got = [...new Uint8Array(await crypto.subtle.digest("SHA-256", buf))]
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    if (got !== want) {
      clear(status, el("span", { class: "bad" }, "✖ sha256 does not match the export; not saved"));
      return;
    }
    const url = URL.createObjectURL(new Blob([buf], { type: "application/json" }));
    const a = el("a", { href: url, download: name });
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => {
      URL.revokeObjectURL(url);
    }, 10_000);
    clear(status, el("span", { class: "good" }, "✔ sha256 matches"));
  } catch (e) {
    clear(status, el("span", { class: "bad" }, `✖ ${e instanceof Error ? e.message : String(e)}`));
  }
}

// --- Start ------------------------------------------------------------------

async function start(): Promise<void> {
  initTheme();
  try {
    people = await listPersonas();
  } catch (e) {
    showError(e);
    return;
  }
  window.addEventListener("hashchange", () => {
    void load();
  });
  await load();
}

void start();
