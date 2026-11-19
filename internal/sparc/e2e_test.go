package sparc

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/risk-sentinel/sparc-horizon/internal/keys"
	"github.com/risk-sentinel/sparc-horizon/internal/tree"
	"github.com/risk-sentinel/sparc-horizon/internal/view"
)

// nistAsSPARC serves NIST's example chain the way SPARC serves documents: the
// SSP carries SPARC's `node-type: system` and `parent-uuid`, every document is
// behind the list and export routes, over TLS, with a Bearer token.
func nistAsSPARC(t *testing.T, boundary string) *fakeSPARC {
	t.Helper()
	nist := os.DirFS("../oscal/testdata/nist")
	read := func(name string) string {
		b, err := fs.ReadFile(nist, name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	var ssp map[string]map[string]any
	if err := json.Unmarshal([]byte(read("ifa_ssp-example.json")), &ssp); err != nil {
		t.Fatal(err)
	}
	md := ssp["system-security-plan"]["metadata"].(map[string]any)
	props, _ := md["props"].([]any)
	md["props"] = append(props,
		map[string]any{"name": "node-type", "ns": keys.NamespaceURI, "value": "system"},
		map[string]any{"name": "parent-uuid", "ns": keys.NamespaceURI, "value": boundary})
	sspBody, err := json.Marshal(ssp)
	if err != nil {
		t.Fatal(err)
	}
	f := newFake()
	f.add(SSP, "ifa", string(sspBody))
	f.add(SAP, "ifa", read("ifa_assessment-plan.json"))
	f.add(SAR, "ifa", read("ifa_assessment-results.json"))
	f.add(POAM, "ifa", read("ifa_plan-of-action-and-milestones.json"))
	return f
}

// The acceptance measurement for #124: what Pull writes is what the mock
// serves, the boundary is keyed by SPARC's boundary uuid, and a second run
// with nothing changed transfers no bodies.
func TestPulledDocumentsAreWhatTheMockServes(t *testing.T) {
	const boundary = "6a1d2f8e-3c5b-4e7a-9f10-2b3c4d5e6f70"
	f := nistAsSPARC(t, boundary)
	cache, err := OpenCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := start(t, f, func(cfg *Config) { cfg.Cache = cache })
	out := t.TempDir() + "/oscal"

	var log bytes.Buffer
	if code := Pull(context.Background(), c, out, &log); code != 0 {
		t.Fatalf("first pull exited %d: %s", code, log.String())
	}
	if f.sent != 4 {
		t.Fatalf("first pull transferred %d bodies, want 4", f.sent)
	}

	set, err := view.LoadFS(os.DirFS(out))
	if err != nil {
		t.Fatal(err)
	}
	v, err := view.New(set, view.Options{RoleAliases: map[string]tree.Role{"owner": tree.SystemOwner}})
	if err != nil {
		t.Fatalf("the mock cannot build a view from the pulled folder: %v", err)
	}
	ps := v.Personas()
	if len(ps) != 1 || ps[0].NodeUUID != boundary {
		t.Fatalf("personas %+v, want the system owner on SPARC's boundary %s", ps, boundary)
	}
	if !strings.Contains(v.Summary(), "1 boundaries") {
		t.Errorf("summary: %s", v.Summary())
	}

	log.Reset()
	if code := Pull(context.Background(), c, out, &log); code != 0 {
		t.Fatalf("second pull exited %d: %s", code, log.String())
	}
	if f.sent != 4 || f.notMod != 4 {
		t.Fatalf("second pull: %d bodies sent in total, %d 304s; want 4 and 4", f.sent, f.notMod)
	}
	if strings.Count(log.String(), "0 transferred, 1 unchanged") != 4 {
		t.Errorf("second pull's log does not show four unchanged documents:\n%s", log.String())
	}
}
