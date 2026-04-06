// ABOUTME: Unit tests for the flatten package.
// ABOUTME: Tests subgraph ref resolution and inlining into flat workflows.
package flatten

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestFlattenNoSubgraphs(t *testing.T) {
	w := &ir.Workflow{
		Name:  "simple",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
		},
	}

	got, err := Flatten(w, nil, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "simple" {
		t.Errorf("Name = %q, want %q", got.Name, "simple")
	}
	if got.Start != "A" {
		t.Errorf("Start = %q, want %q", got.Start, "A")
	}
	if got.Exit != "B" {
		t.Errorf("Exit = %q, want %q", got.Exit, "B")
	}
	if len(got.Nodes) != 2 {
		t.Fatalf("len(Nodes) = %d, want 2", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Fatalf("len(Edges) = %d, want 1", len(got.Edges))
	}
	if got.Edges[0].From != "A" || got.Edges[0].To != "B" {
		t.Errorf("Edge = %s->%s, want A->B", got.Edges[0].From, got.Edges[0].To)
	}
}
