// ABOUTME: Unit tests for the flatten package.
// ABOUTME: Tests subgraph ref resolution and inlining into flat workflows.
package flatten

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestFlattenMultipleSubgraphs(t *testing.T) {
	lintChild := &ir.Workflow{
		Name:  "lint",
		Start: "Check",
		Exit:  "Report",
		Nodes: []*ir.Node{
			{ID: "Check", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "check."}},
			{ID: "Report", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "report."}},
		},
		Edges: []*ir.Edge{
			{From: "Check", To: "Report"},
		},
	}
	testChild := &ir.Workflow{
		Name:  "test",
		Start: "Run",
		Exit:  "Summary",
		Nodes: []*ir.Node{
			{ID: "Run", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "run."}},
			{ID: "Summary", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "summary."}},
		},
		Edges: []*ir.Edge{
			{From: "Run", To: "Summary"},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"lint.dip": lintChild,
		"test.dip": testChild,
	}}

	parent := &ir.Workflow{
		Name:  "pipeline",
		Start: "Start",
		Exit:  "End",
		Nodes: []*ir.Node{
			{ID: "Start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "start."}},
			{ID: "Lint", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "lint.dip"}},
			{ID: "Test", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "test.dip"}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "end."}},
		},
		Edges: []*ir.Edge{
			{From: "Start", To: "Lint"},
			{From: "Lint", To: "Test"},
			{From: "Test", To: "End"},
		},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Start, Lint_Check, Lint_Report, Test_Run, Test_Summary, End
	if len(got.Nodes) != 6 {
		t.Fatalf("len(Nodes) = %d, want 6; nodes: %v", len(got.Nodes), nodeIDs(got))
	}

	wantIDs := map[string]bool{
		"Start": true, "Lint_Check": true, "Lint_Report": true,
		"Test_Run": true, "Test_Summary": true, "End": true,
	}
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
		"Start->Lint_Check":       true,
		"Lint_Check->Lint_Report": true,
		"Lint_Report->Test_Run":   true,
		"Test_Run->Test_Summary":  true,
		"Test_Summary->End":       true,
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

func TestFlattenPreservesEdgeConditions(t *testing.T) {
	checkChild := &ir.Workflow{
		Name:  "check",
		Start: "Run",
		Exit:  "Run",
		Nodes: []*ir.Node{
			{ID: "Run", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "run check."}},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"check.dip": checkChild,
	}}

	parent := &ir.Workflow{
		Name:  "gated",
		Start: "Gate",
		Exit:  "Done",
		Nodes: []*ir.Node{
			{ID: "Gate", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "gate."}},
			{ID: "Check", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "check.dip"}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{
				From:  "Gate",
				To:    "Check",
				Label: "approved",
				Condition: &ir.Condition{
					Raw: "ctx.outcome = success",
				},
			},
			{From: "Check", To: "Done"},
		},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the edge Gate->Check_Run and verify condition/label are preserved.
	var found bool
	for _, e := range got.Edges {
		if e.From == "Gate" && e.To == "Check_Run" {
			found = true
			if e.Label != "approved" {
				t.Errorf("Label = %q, want %q", e.Label, "approved")
			}
			if e.Condition == nil {
				t.Fatal("Condition is nil, want non-nil")
			}
			if e.Condition.Raw != "ctx.outcome = success" {
				t.Errorf("Condition.Raw = %q, want %q", e.Condition.Raw, "ctx.outcome = success")
			}
			break
		}
	}
	if !found {
		t.Errorf("edge Gate->Check_Run not found; edges: %v", edgeKeys(got))
	}
}

func edgeKeys(w *ir.Workflow) []string {
	var keys []string
	for _, e := range w.Edges {
		keys = append(keys, e.From+"->"+e.To)
	}
	return keys
}

func TestFlattenCircularRef(t *testing.T) {
	workflowA := &ir.Workflow{
		Name:  "a",
		Start: "X",
		Exit:  "X",
		Nodes: []*ir.Node{
			{ID: "X", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "b.dip"}},
		},
	}
	workflowB := &ir.Workflow{
		Name:  "b",
		Start: "Y",
		Exit:  "Y",
		Nodes: []*ir.Node{
			{ID: "Y", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "a.dip"}},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"a.dip": workflowA,
		"b.dip": workflowB,
	}}

	root := &ir.Workflow{
		Name:  "root",
		Start: "Entry",
		Exit:  "Entry",
		Nodes: []*ir.Node{
			{ID: "Entry", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "a.dip"}},
		},
	}

	_, err := Flatten(root, resolver, Options{})
	if err == nil {
		t.Fatal("expected error for circular ref, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "cycle")
	}
}

func TestFlattenMaxDepth(t *testing.T) {
	// Chain: level0 -> level1 -> level2 -> level3 -> level4
	workflows := make(map[string]*ir.Workflow)
	for i := 4; i >= 0; i-- {
		w := &ir.Workflow{
			Name:  fmt.Sprintf("level%d", i),
			Start: "N",
			Exit:  "N",
		}
		if i < 4 {
			ref := fmt.Sprintf("level%d.dip", i+1)
			w.Nodes = []*ir.Node{
				{ID: "N", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: ref}},
			}
		} else {
			w.Nodes = []*ir.Node{
				{ID: "N", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "leaf."}},
			}
		}
		workflows[fmt.Sprintf("level%d.dip", i)] = w
	}
	resolver := &MapResolver{Workflows: workflows}

	root := &ir.Workflow{
		Name:  "root",
		Start: "Top",
		Exit:  "Top",
		Nodes: []*ir.Node{
			{ID: "Top", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "level0.dip"}},
		},
	}

	_, err := Flatten(root, resolver, Options{MaxDepth: 3})
	if err == nil {
		t.Fatal("expected max depth error, got nil")
	}
	if !strings.Contains(err.Error(), "max depth") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "max depth")
	}
}

func TestFlattenMissingRef(t *testing.T) {
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{}}

	root := &ir.Workflow{
		Name:  "root",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "missing.dip"}},
		},
	}

	_, err := Flatten(root, resolver, Options{})
	if err == nil {
		t.Fatal("expected error for missing ref, got nil")
	}
	if !strings.Contains(err.Error(), "missing.dip") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "missing.dip")
	}
}

func TestFlattenMissingStartExit(t *testing.T) {
	noStart := &ir.Workflow{
		Name: "bad",
		Nodes: []*ir.Node{
			{ID: "X", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "x."}},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"bad.dip": noStart,
	}}

	root := &ir.Workflow{
		Name:  "root",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "bad.dip"}},
		},
	}

	_, err := Flatten(root, resolver, Options{})
	if err == nil {
		t.Fatal("expected error for missing start node, got nil")
	}
	if !strings.Contains(err.Error(), "no start node") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "no start node")
	}
}

func TestFlattenNested(t *testing.T) {
	inner := &ir.Workflow{
		Name:  "inner",
		Start: "P",
		Exit:  "Q",
		Nodes: []*ir.Node{
			{ID: "P", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "p."}},
			{ID: "Q", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "q."}},
		},
		Edges: []*ir.Edge{
			{From: "P", To: "Q"},
		},
	}
	middle := &ir.Workflow{
		Name:  "middle",
		Start: "M",
		Exit:  "Sub",
		Nodes: []*ir.Node{
			{ID: "M", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "m."}},
			{ID: "Sub", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "inner.dip"}},
		},
		Edges: []*ir.Edge{
			{From: "M", To: "Sub"},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"middle.dip": middle,
		"inner.dip":  inner,
	}}

	root := &ir.Workflow{
		Name:  "root",
		Start: "A",
		Exit:  "Outer",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a."}},
			{ID: "Outer", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "middle.dip"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "Outer"},
		},
	}

	got, err := Flatten(root, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 4 nodes: A, Outer_M, Outer_Sub_P, Outer_Sub_Q
	if len(got.Nodes) != 4 {
		t.Fatalf("len(Nodes) = %d, want 4; nodes: %v", len(got.Nodes), nodeIDs(got))
	}

	wantIDs := map[string]bool{
		"A": true, "Outer_M": true, "Outer_Sub_P": true, "Outer_Sub_Q": true,
	}
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
		"A->Outer_M":               true,
		"Outer_M->Outer_Sub_P":     true,
		"Outer_Sub_P->Outer_Sub_Q": true,
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

func TestDiskResolverResolve(t *testing.T) {
	dir := t.TempDir()
	dipContent := `workflow child
  start: A
  exit: B
  agent A
    label: A
  agent B
    label: B
  edges
    A -> B
`
	path := filepath.Join(dir, "child.dip")
	if err := os.WriteFile(path, []byte(dipContent), 0644); err != nil {
		t.Fatal(err)
	}

	resolver := &DiskResolver{}
	w, err := resolver.Resolve("child.dip", filepath.Join(dir, "parent.dip"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Name != "child" {
		t.Errorf("Name = %q, want %q", w.Name, "child")
	}
	if w.Start != "A" {
		t.Errorf("Start = %q, want %q", w.Start, "A")
	}
}

func TestFlattenPreservesParams(t *testing.T) {
	child := &ir.Workflow{
		Name: "child", Start: "X", Exit: "Y",
		Nodes: []*ir.Node{
			{ID: "X", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "x."}},
			{ID: "Y", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "y."}},
		},
		Edges: []*ir.Edge{{From: "X", To: "Y"}},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{"child.dip": child}}

	parent := &ir.Workflow{
		Name: "main", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{
				Ref:    "child.dip",
				Params: map[string]string{"model": "gpt-5.4", "strict": "true"},
			}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "e."}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, n := range got.Nodes {
		if n.Kind == ir.NodeSubgraph {
			t.Errorf("subgraph node %q should have been removed", n.ID)
		}
	}

	if len(got.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d, want 3; nodes: %v", len(got.Nodes), nodeIDs(got))
	}
}
