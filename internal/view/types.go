package view

// The response shapes of api/openapi.yaml, frozen in #61. Field order is the
// order the goldens were frozen in, so a response encodes byte for byte.

// Cell is one cell of a heat row.
type Cell struct {
	Col       string `json:"col"`
	State     string `json:"state"`
	Blockers  int    `json:"blockers"`
	Worsening bool   `json:"worsening"`
	Inherited bool   `json:"inherited"`
}

// Row is one child of the node being viewed.
type Row struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cells []Cell `json:"cells"`
}

// Heat is the heatmap response.
type Heat struct {
	Node      string   `json:"node"`
	Horizon   string   `json:"horizon"`
	Axis      string   `json:"axis"`
	Columns   []string `json:"columns"`
	Collapsed []string `json:"collapsed"`
	Rows      []Row    `json:"rows"`
}

// Control is one control in a cell drill-down, with why it is down.
type Control struct {
	ControlID string `json:"controlId"`
	Component string `json:"component"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
	Expires   string `json:"expires,omitempty"`
	BlocksATO bool   `json:"blocksAto"`
	Chain     string `json:"chain"`
}

// CellDetail is the cell drill-down response.
type CellDetail struct {
	Node     string    `json:"node"`
	Col      string    `json:"col"`
	Horizon  string    `json:"horizon"`
	Controls []Control `json:"controls"`
}

// Chain is the evidence chain for one implemented requirement, in the order
// docs/03-data-model.md fixes: resource, observation, finding, risk, POA&M
// item.
type Chain struct {
	Requirement string `json:"requirement"`
	ControlID   string `json:"controlId"`
	Component   string `json:"component"`
	Resource    string `json:"resource,omitempty"`
	Observation string `json:"observation,omitempty"`
	Expires     string `json:"expires,omitempty"`
	Finding     string `json:"finding,omitempty"`
	Risk        string `json:"risk,omitempty"`
	BlocksATO   bool   `json:"blocksAto"`
	PoamItem    string `json:"poamItem,omitempty"`
}

// NextAction is the single focal point a lens offers.
type NextAction struct {
	Node        string `json:"node"`
	Title       string `json:"title"`
	Consequence string `json:"consequence"`
	ControlID   string `json:"controlId"`
	Chain       string `json:"chain"`
	Horizon     string `json:"horizon"`
}

// ExportDocument is one document an export links.
type ExportDocument struct {
	Model  string `json:"model"`
	Href   string `json:"href"`
	SHA256 string `json:"sha256"`
}

// Export is the export response.
type Export struct {
	Boundary  string           `json:"boundary"`
	Documents []ExportDocument `json:"documents"`
}

// Persona is a caller the mock can serve. The persona header is not
// authentication; it picks whose roles to answer with.
type Persona struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	PartyUUID string `json:"party-uuid"`
	NodeUUID  string `json:"node-uuid"`
	NodeType  string `json:"node-type"`
	Name      string `json:"name"`
	About     string `json:"about,omitempty"`
}
