package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// The check that makes freezing API v0 mean something: every golden the mock
// serves validates against the schema its path declares, in the spec itself.
//
// A contract whose own examples do not satisfy its own schemas is a contract
// nobody has run anything against — and it would be reviewed, approved and
// built on by two phases before anyone noticed.

const specPath = "../../api/openapi.yaml"

// goldenSchema maps a golden's path prefix to the component schema the spec
// says that endpoint returns.
var goldenSchema = []struct {
	prefix string
	schema string
}{
	// Scaffolding the mock needs and the contract does not describe. Listed
	// rather than pattern-skipped, so a new unmapped file is a failure and
	// not a silent omission.
	{"api/personas.json", ""},
	{"api/README.md", ""},
	{"tree.json", "Node"},
	{"next-action.json", "NextAction"},
	{"/heat/", "Heat"},
	{"/cell/", "CellDetail"},
	{"api/chain/", "Chain"},
}

func schemaFor(path string) string {
	for _, m := range goldenSchema {
		if strings.Contains(path, m.prefix) {
			return m.schema
		}
	}
	return "unmapped"
}

// loadSpecSchemas compiles every component schema in the frozen spec. OpenAPI
// 3.1 schemas are JSON Schema, so they compile directly rather than needing a
// translation nobody would maintain.
func loadSpecSchemas(t *testing.T) (*jsonschema.Compiler, map[string]any) {
	t.Helper()

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading the spec: %v", err)
	}
	var spec map[string]any
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parsing the spec: %v", err)
	}

	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if len(schemas) == 0 {
		t.Fatal("the spec declares no component schemas")
	}

	// Re-rooted so $ref: "#/components/schemas/X" resolves.
	doc := map[string]any{"components": map[string]any{"schemas": schemas}}
	// yaml.v3 gives map[string]any already, but nested maps from YAML can be
	// map[any]any in some shapes; round-trip through JSON to normalise.
	normalised, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("normalising the spec: %v", err)
	}
	var clean any
	if err := json.Unmarshal(normalised, &clean); err != nil {
		t.Fatalf("normalising the spec: %v", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("spec.json", clean); err != nil {
		t.Fatalf("loading the spec: %v", err)
	}
	return c, schemas
}

func TestGoldensValidateAgainstTheFrozenContract(t *testing.T) {
	tree := generate(t)
	compiled := compileSpecSchemas(t)

	validated, skipped := 0, 0
	for _, path := range tree.Paths() {
		if !strings.HasPrefix(path, "api/") {
			continue
		}
		switch name := schemaFor(path); name {
		case "":
			skipped++
		case "unmapped":
			t.Errorf("%s: no schema mapped for this golden. Either the spec gained an endpoint without a golden, or a golden was added without saying what it is", path)
		default:
			if validateGolden(t, compiled[name], name, path, tree[path]) {
				validated++
			}
		}
	}

	// A loop over nothing validates nothing and exits clean.
	if validated < 20 {
		t.Errorf("validated %d goldens; the mock serves far more than that, so the matcher is not matching", validated)
	}
	t.Logf("validated %d goldens against the frozen contract (%d scaffolding files skipped)", validated, skipped)
}

// compileSpecSchemas compiles every component schema the frozen spec declares.
func compileSpecSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()

	compiler, schemas := loadSpecSchemas(t)
	out := map[string]*jsonschema.Schema{}
	for name := range schemas {
		s, err := compiler.Compile("spec.json#/components/schemas/" + name)
		if err != nil {
			t.Fatalf("compiling schema %s: %v", name, err)
		}
		out[name] = s
	}
	return out
}

func validateGolden(t *testing.T, schema *jsonschema.Schema, name, path string, body []byte) bool {
	t.Helper()

	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Errorf("%s: %v", path, err)
		return false
	}
	if err := schema.Validate(doc); err != nil {
		t.Errorf("%s does not satisfy %s:\n%v", path, name, err)
		return false
	}
	return true
}

// Both directions: the spec cannot declare an endpoint the mock cannot answer,
// and the mock cannot serve a shape the spec does not describe.
// The endpoints the mock answers from goldens, and the marker that identifies
// each one's files.
var servedFromGoldens = map[string]string{
	"/v1/tree":                   "tree.json",
	"/v1/nodes/{id}/heat":        "/heat/",
	"/v1/nodes/{id}/cell":        "/cell/",
	"/v1/controls/{uuid}/chain":  "api/chain/",
	"/v1/nodes/{id}/next-action": "next-action.json",
}

// Declared by the frozen contract and answered by the service rather than by a
// fixture. Listed so that "no golden" is a stated position and not an
// omission — the difference matters when the next person wonders whether the
// mock is incomplete or the endpoint is deliberate.
var notMocked = map[string]string{
	"/v1/attestations":            "write path; P4 implements it",
	"/v1/attestations/{id}/sign":  "write path; P4 implements it",
	"/v1/decisions":               "write path; P5 implements it",
	"/v1/whatif":                  "write path; P7 implements it",
	"/v1/nodes/{id}/blast-radius": "P6, out of default scope",
	"/v1/export/oscal/{boundary}": "serves the fixture OSCAL directly, not a golden",
}

// Both directions: the spec cannot declare an endpoint the mock cannot answer,
// and the mock cannot serve a shape the spec does not describe.
func TestEveryContractPathHasAGolden(t *testing.T) {
	tree := generate(t)
	paths := specPaths(t)

	assertEveryPathIsAccountedFor(t, paths)
	assertEveryGoldenHasAPath(t, paths)

	counts := goldensPerEndpoint(tree)
	for endpoint := range servedFromGoldens {
		if counts[endpoint] == 0 {
			t.Errorf("%s: the contract declares it and no golden answers it", endpoint)
		}
	}
	t.Logf("goldens per endpoint: %s", summarise(counts))
}

func specPaths(t *testing.T) map[string]map[string]any {
	t.Helper()

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading the spec: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parsing the spec: %v", err)
	}
	if len(spec.Paths) == 0 {
		t.Fatal("the spec declares no paths")
	}
	return spec.Paths
}

func assertEveryPathIsAccountedFor(t *testing.T, paths map[string]map[string]any) {
	t.Helper()

	for path := range paths {
		_, mocked := servedFromGoldens[path]
		_, stated := notMocked[path]
		if !mocked && !stated {
			t.Errorf("%s is in the frozen contract but neither has a golden nor is listed as not mocked", path)
		}
	}
}

func assertEveryGoldenHasAPath(t *testing.T, paths map[string]map[string]any) {
	t.Helper()

	for path := range servedFromGoldens {
		if _, ok := paths[path]; !ok {
			t.Errorf("%s has goldens but is not in the contract", path)
		}
	}
}

func goldensPerEndpoint(tree Tree) map[string]int {
	counts := map[string]int{}
	for _, p := range tree.Paths() {
		for endpoint, marker := range servedFromGoldens {
			if strings.Contains(p, marker) {
				counts[endpoint]++
			}
		}
	}
	return counts
}

func summarise(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}
