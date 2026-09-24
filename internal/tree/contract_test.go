package tree

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const specPath = "../../api/openapi.yaml"

// The tree must validate against the Node schema FROZEN IN #61, compiled from
// the spec rather than restated here.
//
// Restating the enums in Go would make this test agree with itself: the
// builder and the test would share one author's understanding of the contract,
// and both could drift from the document the API actually serves. Compiling
// the spec means a change to it either passes or fails here, which is the
// point of freezing it.
func TestTreeValidatesAgainstTheFrozenNodeSchema(t *testing.T) {
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
	if _, ok := schemas["Node"]; !ok {
		t.Fatal("the spec declares no Node schema; the tree has nothing to conform to")
	}

	// Re-rooted so the Node schema's self-$ref for `children` resolves.
	doc := map[string]any{"components": map[string]any{"schemas": schemas}}
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
	schema, err := c.Compile("spec.json#/components/schemas/Node")
	if err != nil {
		t.Fatalf("compiling Node: %v", err)
	}

	res := build(t)
	body, err := json.Marshal(res.Root)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc2 any
	if err := json.Unmarshal(body, &doc2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if err := schema.Validate(doc2); err != nil {
		t.Errorf("the tree does not satisfy the frozen Node schema:\n%v", err)
	}
}
