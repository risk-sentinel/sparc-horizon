package fixtures

// The evidence artifacts the back-matter resources hash.
//
// They are synthetic, but they are real files: the digest in every
// back-matter resource is the SHA-256 of bytes this run actually wrote, so the
// chain resource -> observation -> finding -> risk -> POA&M item can be walked
// and checked rather than taken on trust. A hash of a file that does not exist
// is the kind of fixture that passes every test and proves nothing.

// ScanArtifact stands in for a normalised scanner run.
type ScanArtifact struct {
	Schema    string   `json:"$schema-note"`
	Tool      string   `json:"tool"`
	Boundary  string   `json:"boundary"`
	Period    string   `json:"period"`
	Collected string   `json:"collected"`
	Controls  []string `json:"controls"`
	Synthetic bool     `json:"synthetic"`
}

// AttestationArtifact stands in for a signed manual attestation.
type AttestationArtifact struct {
	Statement    string   `json:"statement"`
	Boundary     string   `json:"boundary"`
	Period       string   `json:"period"`
	Controls     []string `json:"controls"`
	SignedByRole string   `json:"signed-by-role"`
	Synthetic    bool     `json:"synthetic"`
}

// evidenceArtifact is one emitted file and the back-matter resource that will
// name it.
type evidenceArtifact struct {
	path string
	// kind is the value of the `evidence-kind` prop, from the namespace
	// schema's enumeration.
	kind   string
	title  string
	doc    any
	digest string
}

// manualControls are attested by a person rather than measured by a scanner.
// Contingency plan testing is a tabletop; authenticator management is a policy
// statement. Everything else in the fixture baseline is machine-checkable.
var manualControls = map[string]bool{"cp-4": true, "ia-5": true}

func (g *Generator) evidenceArtifacts(b Boundary) []evidenceArtifact {
	automated := make([]string, 0, len(NISTControls))
	manual := make([]string, 0, 2)
	for _, c := range NISTControls {
		if manualControls[c.ID] {
			manual = append(manual, c.ID)
			continue
		}
		automated = append(automated, c.ID)
	}
	for _, c := range SecurityHubControls {
		automated = append(automated, c.ID)
	}

	return []evidenceArtifact{
		{
			path:  pathEvidence(b, "scan"),
			kind:  "hdf-results",
			title: b.Name + " normalised scan results " + Period,
			doc: ScanArtifact{
				Schema:    "Synthetic stand-in for an HDF v3 results file; shape is illustrative, not the HDF schema.",
				Tool:      "sparc-horizon-fixture-scanner",
				Boundary:  b.Slug,
				Period:    Period,
				Collected: days(-3).Format("2006-01-02"),
				Controls:  automated,
				Synthetic: true,
			},
		},
		{
			path:  pathEvidence(b, "attestation"),
			kind:  "manual-attestation",
			title: b.Name + " manual attestation " + Period,
			doc: AttestationArtifact{
				Statement:    "The controls listed were reviewed for " + b.Name + " and are implemented as described in the system security plan.",
				Boundary:     b.Slug,
				Period:       Period,
				Controls:     manual,
				SignedByRole: roleISO,
				Synthetic:    true,
			},
		},
	}
}
