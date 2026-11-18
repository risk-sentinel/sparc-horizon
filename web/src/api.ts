// API v0 client. The caller is named by X-Horizon-Persona, as the mock reads it;
// the real server takes it from the token, and this header is then ignored.

import type {
  ApiErrorBody,
  CellDetail,
  Chain,
  Export,
  Heat,
  NextAction,
  Persona,
  TreeNode,
} from "./types.ts";

export const PERSONA_HEADER = "X-Horizon-Persona";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, detail: string) {
    super(detail);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }

  /** The refusal rule: absent and not-yours read the same. */
  get notVisible(): boolean {
    return this.status === 404;
  }
}

type Fetch = (input: string, init?: RequestInit) => Promise<Response>;

/** Builds a query string from the defined values only. */
export function query(params: Record<string, string | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") q.set(k, v);
  }
  const s = q.toString();
  return s === "" ? "" : `?${s}`;
}

export class Api {
  readonly persona: string;
  private readonly fetcher: Fetch;

  constructor(persona: string, fetcher: Fetch = (i, init) => fetch(i, init)) {
    this.persona = persona;
    this.fetcher = fetcher;
  }

  private headers(): Record<string, string> {
    const h: Record<string, string> = { Accept: "application/json" };
    if (this.persona !== "") h[PERSONA_HEADER] = this.persona;
    return h;
  }

  async raw(path: string): Promise<Response> {
    const res = await this.fetcher(path, { headers: this.headers(), credentials: "same-origin" });
    if (!res.ok) throw await toError(res);
    return res;
  }

  async get<T>(path: string): Promise<T> {
    const res = await this.raw(path);
    return (await res.json()) as T;
  }

  tree(): Promise<TreeNode> {
    return this.get("/v1/tree");
  }

  heat(node: string, horizon?: string): Promise<Heat> {
    return this.get(`/v1/nodes/${encodeURIComponent(node)}/heat${query({ horizon })}`);
  }

  cell(node: string, col: string, horizon?: string): Promise<CellDetail> {
    return this.get(`/v1/nodes/${encodeURIComponent(node)}/cell${query({ col, horizon })}`);
  }

  nextAction(node: string, horizon?: string): Promise<NextAction> {
    return this.get(`/v1/nodes/${encodeURIComponent(node)}/next-action${query({ horizon })}`);
  }

  chain(requirement: string): Promise<Chain> {
    return this.get(`/v1/controls/${encodeURIComponent(requirement)}/chain`);
  }

  export(boundary: string): Promise<Export> {
    return this.get(`/v1/export/oscal/${encodeURIComponent(boundary)}`);
  }
}

/** The mock's persona list. Absent on a real server, which knows its caller. */
export async function personas(fetcher: Fetch = (i, init) => fetch(i, init)): Promise<Persona[]> {
  const res = await fetcher("/mock/personas", { headers: { Accept: "application/json" } });
  if (res.status === 404) return [];
  if (!res.ok) throw await toError(res);
  return (await res.json()) as Persona[];
}

async function toError(res: Response): Promise<ApiError> {
  let body: Partial<ApiErrorBody> = {};
  try {
    body = (await res.json()) as Partial<ApiErrorBody>;
  } catch {
    // A non-JSON error body still yields a status.
  }
  return new ApiError(res.status, body.error ?? `http_${res.status}`, body.detail ?? res.statusText);
}
