// API v0's read shapes, as api/openapi.yaml defines them. Kept by hand: the
// contract is frozen, and a generator would be a dependency for nine types.

export type State = "nominal" | "watch" | "degraded" | "blocks";

export type NodeType = "federation" | "organization" | "boundary" | "system";

export interface Persona {
  id: string;
  role: string;
  "party-uuid": string;
  "node-uuid": string;
  "node-type": NodeType;
  name: string;
  about?: string;
}

export interface TreeNode {
  id: string;
  name: string;
  nodeType: NodeType;
  roles: string[];
  children?: TreeNode[];
}

export interface HeatCell {
  col: string;
  state: State;
  blockers: number;
  worsening: boolean;
  inherited: boolean;
}

export interface HeatRow {
  id: string;
  name: string;
  cells: HeatCell[];
}

export interface Heat {
  node: string;
  horizon: string;
  axis: string;
  columns: string[];
  collapsed: string[];
  rows: HeatRow[];
}

export interface CellControl {
  controlId: string;
  component: string;
  state: State;
  reason: string;
  expires?: string;
  blocksAto: boolean;
  chain: string;
}

export interface CellDetail {
  node: string;
  col: string;
  horizon: string;
  controls: CellControl[];
}

export interface NextAction {
  node: string;
  title: string;
  consequence: string;
  controlId?: string;
  chain?: string;
  horizon: string;
}

export interface Chain {
  requirement: string;
  controlId: string;
  component: string;
  resource?: string;
  observation?: string;
  expires?: string;
  finding?: string;
  risk?: string;
  blocksAto: boolean;
  poamItem?: string;
}

export interface ExportDocument {
  model: string;
  href: string;
  sha256: string;
}

export interface Export {
  boundary: string;
  documents: ExportDocument[];
}

export interface ApiErrorBody {
  error: string;
  detail?: string;
}
