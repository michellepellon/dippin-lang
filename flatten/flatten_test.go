// ABOUTME: Unit tests for the flatten package.
// ABOUTME: Tests subgraph ref resolution and inlining into flat workflows.
package flatten

import (
	"fmt"
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

// MapResolver is a test resolver backed by an in-memory map.
type MapResolver struct {
	Workflows map[string]*ir.Workflow
}

func (r *MapResolver) Resolve(refPath string, _ string) (*ir.Workflow, error) {
	w, ok := r.Workflows[refPath]
	if !ok {
		return nil, fmt.Errorf("ref not found: %s", refPath)
	}
	return w, nil
}

func TestFlattenSingleSubgraph(t *testing.T) {
	child := &ir.Workflow{
		Name:  "review",
		Start: "Analyze",
		Exit:  "Summarize",
		Nodes: []*ir.Node{
			{ID: "Analyze", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "analyze."}},
			{ID: "Summarize", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "summarize."}},
		},
		Edges: []*ir.Edge{
			{From: "Analyze", To: "Summarize"},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"./review.dip": child,
	}}

	parent := &ir.Workflow{
		Name:  "main",
		Start: "Build",
		Exit:  "Done",
		Nodes: []*ir.Node{
			{ID: "Build", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "build."}},
			{ID: "Review", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "Build", To: "Review"},
			{From: "Review", To: "Done"},
		},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 nodes: Build, Review_Analyze, Review_Summarize, Done
	if len(got.Nodes) != 4 {
		t.Fatalf("len(Nodes) = %d, want 4; nodes: %v", len(got.Nodes), nodeIDs(got))
	}

	wantIDs := map[string]bool{"Build": true, "Review_Analyze": true, "Review_Summarize": true, "Done": true}
	for _, n := range got.Nodes {
		if !wantIDs[n.ID] {
			t.Errorf("unexpected node %q", n.ID)
		}
		delete(wantIDs, n.ID)
	}
	for id := range wantIDs {
		t.Errorf("missing node %q", id)
	}

	wantEdges := map[string]bool{
		"Build->Review_Analyze":            true,
		"Review_Analyze->Review_Summarize": true,
		"Review_Summarize->Done":           true,
	}
	for _, e := range got.Edges {
		key := e.From + "->" + e.To
		if !wantEdges[key] {
			t.Errorf("unexpected edge %s", key)
		}
		delete(wantEdges, key)
	}
	for key := range wantEdges {
		t.Errorf("missing edge %s", key)
	}
}

func nodeIDs(w *ir.Workflow) []string {
	var ids []string
	for _, n := range w.Nodes {
		ids = append(ids, n.ID)
	}
	return ids
}
