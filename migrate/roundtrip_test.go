package migrate

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/export"
	"github.com/2389-research/dippin-lang/ir"
)

// TestExportDOT_ManagerLoop_SteerContextReservedCharsRoundTrip verifies that
// steer_context values with reserved delimiters (',', '=', '%') round-trip
// losslessly through ExportDOT → parseFlattenedSteerContext.
//
// Note: ExportDOT and migrate.Migrate() use different DOT formats at the
// graph-attribute level (ExportDOT emits bare "key=val;" statements while
// the migrate DOT parser expects "graph [key=val];" blocks), so the test
// extracts the steer_context attr value directly from the DOT string and
// passes it through parseFlattenedSteerContext.
func TestExportDOT_ManagerLoop_SteerContextReservedCharsRoundTrip(t *testing.T) {
	// Values with reserved delimiters (',', '=', '%') must round-trip losslessly
	// through flattenSteerContext → parseFlattenedSteerContext.
	original := map[string]string{
		"hint":     "be concise, technical",
		"priority": "high=medium",
		"meta":     "100%",
	}
	w := &ir.Workflow{
		Name:  "W",
		Start: "S",
		Exit:  "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "M", Kind: ir.NodeManagerLoop, Config: ir.ManagerLoopConfig{
				SubgraphRef:  "inner",
				MaxCycles:    5,
				SteerContext: original,
			}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
		},
		Edges: []*ir.Edge{{From: "S", To: "M"}, {From: "M", To: "E"}},
	}
	dot := export.ExportDOT(w, export.ExportOptions{})

	// Extract the steer_context attribute value from the DOT string.
	// It appears as: steer_context="<value>"
	attrVal := extractDOTAttr(dot, "steer_context")
	if attrVal == "" {
		t.Fatalf("steer_context attr not found in DOT output:\n%s", dot)
	}

	// Decode via parseFlattenedSteerContext (the migrate path).
	got := parseFlattenedSteerContext(attrVal)

	for k, want := range original {
		if got[k] != want {
			t.Errorf("round-trip lost %q: got %q want %q", k, got[k], want)
		}
	}
}

// TestExportDOT_ManagerLoop_SteerContextNoReservedCharsPassThrough verifies
// that plain keys/values (no reserved chars) are not mangled by the encoding.
func TestExportDOT_ManagerLoop_SteerContextNoReservedCharsPassThrough(t *testing.T) {
	original := map[string]string{
		"hint":     "speed_up",
		"priority": "high",
	}
	w := &ir.Workflow{
		Name:  "W",
		Start: "S",
		Exit:  "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "M", Kind: ir.NodeManagerLoop, Config: ir.ManagerLoopConfig{
				SubgraphRef:  "inner",
				MaxCycles:    3,
				SteerContext: original,
			}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
		},
		Edges: []*ir.Edge{{From: "S", To: "M"}, {From: "M", To: "E"}},
	}
	dot := export.ExportDOT(w, export.ExportOptions{})

	attrVal := extractDOTAttr(dot, "steer_context")
	if attrVal == "" {
		t.Fatalf("steer_context attr not found in DOT output:\n%s", dot)
	}

	// Plain values must not be mangled — the encoded form should equal "k=v,k=v".
	keys := []string{"hint", "priority"}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+original[k])
	}
	wantFlat := strings.Join(parts, ",")
	if attrVal != wantFlat {
		t.Errorf("plain steer_context encoded as %q, want %q", attrVal, wantFlat)
	}

	got := parseFlattenedSteerContext(attrVal)
	for k, want := range original {
		if got[k] != want {
			t.Errorf("round-trip changed %q: got %q want %q", k, got[k], want)
		}
	}
}

// extractDOTAttr extracts the value of a DOT attribute from a line like:
//
//	key="value"
//
// It handles the quoted format produced by ExportDOT. Returns "" if not found.
func extractDOTAttr(dot, key string) string {
	needle := fmt.Sprintf(`%s="`, key)
	idx := strings.Index(dot, needle)
	if idx == -1 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(dot[start:], `"`)
	if end == -1 {
		return ""
	}
	return dot[start : start+end]
}
