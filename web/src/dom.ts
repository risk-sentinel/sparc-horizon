// The only way the HUD makes elements. Text goes in as text, never as markup:
// every string here may have come from someone else's OSCAL document, so
// output is filtered by construction (SI-15) and eslint.config.js forbids the
// markup-parsing properties that would bypass it.

type Child = Node | string | number | false | null | undefined;
type Attrs = Record<string, string | number | boolean | undefined>;

export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Attrs = {},
  ...children: Child[]
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === false) continue;
    e.setAttribute(k, v === true ? "" : String(v));
  }
  append(e, children);
  return e;
}

export function append(parent: Node, children: Child[]): void {
  for (const c of children) {
    if (c === undefined || c === null || c === false) continue;
    parent.appendChild(typeof c === "string" || typeof c === "number" ? document.createTextNode(String(c)) : c);
  }
}

export function clear(e: Element, ...children: Child[]): void {
  e.replaceChildren();
  append(e, children);
}
