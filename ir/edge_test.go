package ir_test

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestConditionExprInterface(t *testing.T) {
	// Verify all 4 condition types satisfy the sealed interface.
	var _ ir.ConditionExpr = ir.CondCompare{}
	var _ ir.ConditionExpr = ir.CondAnd{}
	var _ ir.ConditionExpr = ir.CondOr{}
	var _ ir.ConditionExpr = ir.CondNot{}
}

func TestEdgesFromParallel(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "split", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"a", "b"}}},
			{ID: "a", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
			{ID: "b", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "B"}},
		},
	}
	edges := w.EdgesFrom("split")
	if len(edges) != 2 {
		t.Fatalf("EdgesFrom(split) = %d, want 2", len(edges))
	}
}

func TestEdgesToFanIn(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "a", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
			{ID: "b", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "B"}},
			{ID: "join", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"a", "b"}}},
		},
	}
	edges := w.EdgesTo("join")
	if len(edges) != 2 {
		t.Fatalf("EdgesTo(join) = %d, want 2", len(edges))
	}
}

func TestEdgesDedup(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "split", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"a"}}},
			{ID: "a", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
		},
		Edges: []*ir.Edge{
			{From: "split", To: "a"},
		},
	}
	edges := w.EdgesFrom("split")
	// Explicit edge + implicit parallel edge should dedup to 1
	if len(edges) != 1 {
		t.Errorf("EdgesFrom(split) = %d, want 1 (dedup)", len(edges))
	}
}

func TestNodeByIDNotFound(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "a", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
		},
	}
	if w.Node("nonexistent") != nil {
		t.Error("expected nil for nonexistent node")
	}
}

func TestEdgesFromNonexistent(t *testing.T) {
	w := &ir.Workflow{}
	edges := w.EdgesFrom("ghost")
	if len(edges) != 0 {
		t.Errorf("EdgesFrom(ghost) = %d, want 0", len(edges))
	}
}

func TestEdgesToNonexistent(t *testing.T) {
	w := &ir.Workflow{}
	edges := w.EdgesTo("ghost")
	if len(edges) != 0 {
		t.Errorf("EdgesTo(ghost) = %d, want 0", len(edges))
	}
}

func TestNodeIDsEmpty(t *testing.T) {
	w := &ir.Workflow{}
	ids := w.NodeIDs()
	if len(ids) != 0 {
		t.Errorf("NodeIDs() = %v, want empty", ids)
	}
}

func TestAllEdges_ExplicitOnly(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "B"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "C"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
		},
	}
	all := w.AllEdges()
	if len(all) != 2 {
		t.Fatalf("AllEdges() = %d, want 2", len(all))
	}
}

func TestAllEdges_WithParallelAndFanIn(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "S"}},
			{ID: "split", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"a", "b"}}},
			{ID: "a", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
			{ID: "b", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "B"}},
			{ID: "join", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"a", "b"}}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "E"}},
		},
		Edges: []*ir.Edge{
			{From: "start", To: "split"},
			{From: "join", To: "end"},
		},
	}
	all := w.AllEdges()
	// start->split, split->a, split->b, a->join, b->join, join->end = 6
	if len(all) != 6 {
		t.Fatalf("AllEdges() = %d, want 6", len(all))
	}

	keys := make(map[string]bool)
	for _, e := range all {
		keys[e.From+"->"+e.To] = true
	}
	expected := []string{"start->split", "split->a", "split->b", "a->join", "b->join", "join->end"}
	for _, k := range expected {
		if !keys[k] {
			t.Errorf("missing edge %s in AllEdges()", k)
		}
	}
}

func TestAllEdges_Dedup(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{ID: "split", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"a"}}},
			{ID: "a", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "A"}},
		},
		Edges: []*ir.Edge{
			{From: "split", To: "a"},
		},
	}
	all := w.AllEdges()
	if len(all) != 1 {
		t.Errorf("AllEdges() = %d, want 1 (dedup)", len(all))
	}
}

func TestAllEdges_Empty(t *testing.T) {
	w := &ir.Workflow{}
	all := w.AllEdges()
	if len(all) != 0 {
		t.Errorf("AllEdges() = %d, want 0", len(all))
	}
}
