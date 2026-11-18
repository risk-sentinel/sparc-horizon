// Light, dark, or the system's choice. The choice is a per-viewer convenience,
// so storage failing (private mode, blocked site data) only loses the memory.

import { el } from "./dom.ts";

const KEY = "horizon-theme";
const ORDER = ["auto", "light", "dark"] as const;
type Theme = (typeof ORDER)[number];

function read(): Theme {
  try {
    const v = localStorage.getItem(KEY);
    return v === "light" || v === "dark" ? v : "auto";
  } catch {
    return "auto";
  }
}

function apply(t: Theme): void {
  if (t === "auto") document.documentElement.removeAttribute("data-theme");
  else document.documentElement.setAttribute("data-theme", t);
  try {
    if (t === "auto") localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, t);
  } catch {
    // Not remembered; still applied.
  }
}

export function initTheme(): void {
  apply(read());
}

export function themeButton(): HTMLButtonElement {
  const b = el("button", { type: "button", id: "theme", class: "theme" });
  const label = (): void => {
    b.textContent = `Theme: ${read()}`;
  };
  label();
  b.addEventListener("click", () => {
    const i = ORDER.indexOf(read());
    apply(ORDER[(i + 1) % ORDER.length] ?? "auto");
    label();
  });
  return b;
}
