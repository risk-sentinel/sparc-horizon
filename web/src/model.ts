// Pure functions behind the HUD: no DOM, no fetch, so node:test covers them.

import type { HeatCell, NodeType, State, TreeNode } from "./types.ts";

// --- States -----------------------------------------------------------------

/**
 * Each state carries a glyph as well as a colour (WCAG 1.4.1): a blocker is
 * readable in greyscale, by a screen reader, and in forced-colours mode.
 * Thresholds live server-side (pass ratio ≥ 0.95 / ≥ 0.85 / below).
 */
export const STATES: Record<State, { glyph: string; label: string; rank: number }> = {
  blocks: { glyph: "✖", label: "Blocks ATO", rank: 3 },
  degraded: { glyph: "▼", label: "Degraded", rank: 2 },
  watch: { glyph: "◆", label: "Watch", rank: 1 },
  nominal: { glyph: "●", label: "Nominal", rank: 0 },
};

export function isState(s: string): s is State {
  return Object.hasOwn(STATES, s);
}

/** Worst state first, so a list reads as triage. */
export function byWorst<T extends { state: State }>(a: T, b: T): number {
  return STATES[b.state].rank - STATES[a.state].rank;
}

// --- Control families -------------------------------------------------------

/** NIST SP 800-53 Rev. 5 families, for headers and labels. */
export const FAMILIES: Readonly<Record<string, string>> = {
  AC: "Access Control",
  AT: "Awareness and Training",
  AU: "Audit and Accountability",
  CA: "Assessment, Authorization, and Monitoring",
  CM: "Configuration Management",
  CP: "Contingency Planning",
  IA: "Identification and Authentication",
  IR: "Incident Response",
  MA: "Maintenance",
  MP: "Media Protection",
  PE: "Physical and Environmental Protection",
  PL: "Planning",
  PM: "Program Management",
  PS: "Personnel Security",
  PT: "PII Processing and Transparency",
  RA: "Risk Assessment",
  SA: "System and Services Acquisition",
  SC: "System and Communications Protection",
  SI: "System and Information Integrity",
  SR: "Supply Chain Risk Management",
};

export function familyName(col: string): string {
  return FAMILIES[col.toUpperCase()] ?? col;
}

/** Control ids as NIST prints them: `ac-2.1` → `AC-2(1)`. Others unchanged. */
export function displayControl(id: string): string {
  const m = /^([a-z]{2})-(\d+)(?:\.(\d+))?$/i.exec(id);
  if (m === null) return id;
  const [, fam = "", num = "", enh] = m;
  const suffix = enh === undefined ? "" : `(${enh})`;
  return `${fam.toUpperCase()}-${num}${suffix}`;
}

// --- Dates ------------------------------------------------------------------

const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;
const DAY_MS = 86_400_000;

/** A calendar date, YYYY-MM-DD, that exists. */
export function isIsoDate(s: string): boolean {
  const m = ISO_DATE.exec(s);
  if (m === null) return false;
  const d = new Date(`${s}T00:00:00Z`);
  return !Number.isNaN(d.getTime()) && d.toISOString().slice(0, 10) === s;
}

function utc(s: string): number {
  return Date.parse(`${s}T00:00:00Z`);
}

export function addDays(date: string, days: number): string {
  return new Date(utc(date) + days * DAY_MS).toISOString().slice(0, 10);
}

export function daysBetween(from: string, to: string): number {
  return Math.round((utc(to) - utc(from)) / DAY_MS);
}

/** Today in UTC, as the server computes it. */
export function today(now: Date = new Date()): string {
  return now.toISOString().slice(0, 10);
}

/**
 * Countdowns over timestamps (docs/01-scope.md): how far `date` is from `from`,
 * in words. Past dates read as overdue.
 */
export function countdown(date: string, from: string): string {
  const d = daysBetween(from, date);
  if (d === 0) return "today";
  if (d === 1) return "tomorrow";
  if (d === -1) return "1 day ago";
  return d > 0 ? `in ${d} days` : `${-d} days ago`;
}

// --- Cells ------------------------------------------------------------------

/** The accessible name of a heat cell: everything the colour and glyph say. */
export function cellLabel(row: string, cell: HeatCell): string {
  const parts = [`${row}, ${cell.col} ${familyName(cell.col)}: ${STATES[cell.state].label}`];
  if (cell.blockers > 0) parts.push(`${cell.blockers} blocking the authorization`);
  if (cell.worsening) parts.push("worsening");
  if (cell.inherited) parts.push("inherited");
  return parts.join(", ");
}

// --- Tree -------------------------------------------------------------------

/** The path from the root to `id`, inclusive, or undefined if not visible. */
export function pathTo(root: TreeNode, id: string): TreeNode[] | undefined {
  if (root.id === id) return [root];
  for (const child of root.children ?? []) {
    const p = pathTo(child, id);
    if (p !== undefined) return [root, ...p];
  }
  return undefined;
}

export function findNode(root: TreeNode, id: string): TreeNode | undefined {
  return pathTo(root, id)?.at(-1);
}

/** Systems are leaves: rows, never a heat of their own. */
export function drillable(node: Pick<TreeNode, "nodeType" | "children">): boolean {
  return node.nodeType !== "system" && (node.children?.length ?? 0) > 0;
}

/** Every node's name by UUID, to label components that are systems. */
export function names(root: TreeNode): Map<string, string> {
  const out = new Map<string, string>();
  const walk = (n: TreeNode): void => {
    out.set(n.id, n.name);
    (n.children ?? []).forEach(walk);
  };
  walk(root);
  return out;
}

export const NODE_TYPES: Readonly<Record<NodeType, string>> = {
  federation: "Federation",
  organization: "Organization",
  boundary: "Authorization boundary",
  system: "System",
};

export function shortId(uuid: string): string {
  return uuid.length > 8 ? `${uuid.slice(0, 8)}…` : uuid;
}

// --- Route ------------------------------------------------------------------

/**
 * What the screen shows, kept in the URL hash so a view can be linked,
 * reloaded, and walked back with the browser's own history.
 */
export interface Route {
  persona?: string | undefined;
  node?: string | undefined;
  horizon?: string | undefined;
  /** The heat row whose cell is open; the family is `col`. */
  row?: string | undefined;
  col?: string | undefined;
  chain?: string | undefined;
}

const KEYS = { persona: "p", node: "n", horizon: "h", row: "r", col: "c", chain: "x" } as const;
const UUID = /^[0-9a-f-]{36}$/i;
const COL = /^[A-Za-z0-9.-]{1,16}$/;
const PERSONA = /^[\w.-]{1,128}$/;

/** Parses a hash, dropping anything malformed rather than sending it on. */
export function parseRoute(hash: string): Route {
  const q = new URLSearchParams(hash.replace(/^#/, ""));
  const r: Route = {};
  const p = q.get(KEYS.persona);
  if (p !== null && PERSONA.test(p)) r.persona = p;
  const n = q.get(KEYS.node);
  if (n !== null && UUID.test(n)) r.node = n;
  const h = q.get(KEYS.horizon);
  if (h !== null && isIsoDate(h)) r.horizon = h;
  const row = q.get(KEYS.row);
  if (row !== null && UUID.test(row)) r.row = row;
  const c = q.get(KEYS.col);
  if (c !== null && COL.test(c)) r.col = c.toUpperCase();
  const x = q.get(KEYS.chain);
  if (x !== null && UUID.test(x)) r.chain = x;
  return r;
}

export function formatRoute(r: Route): string {
  const q = new URLSearchParams();
  for (const k of Object.keys(KEYS) as (keyof Route)[]) {
    const v = r[k];
    if (v !== undefined && v !== "") q.set(KEYS[k], v);
  }
  return `#${q.toString()}`;
}
