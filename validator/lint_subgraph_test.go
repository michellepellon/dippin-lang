//go:build !wasm

package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestLintSubgraphRef_Missing(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{
				ID:   "Sub",
				Kind: ir.NodeSubgraph,
				Config: ir.SubgraphConfig{
					Ref: "nonexistent_pipeline.dip",
				},
				Source: ir.SourceLocation{File: "test.dip"},
			},
		},
	}
	diags := lintSubgraphRef(w)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Code != DIP126 {
		t.Errorf("expected DIP126, got %s", diags[0].Code)
	}
}

func TestLintSubgraphRef_Exists(t *testing.T) {
	// Create a temp file to use as the ref target.
	dir := t.TempDir()
	refFile := filepath.Join(dir, "sub.dip")
	if err := os.WriteFile(refFile, []byte("workflow Sub\n"), 0644); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(dir, "main.dip")

	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{
				ID:   "Sub",
				Kind: ir.NodeSubgraph,
				Config: ir.SubgraphConfig{
					Ref: "sub.dip",
				},
				Source: ir.SourceLocation{File: srcFile},
			},
		},
	}
	diags := lintSubgraphRef(w)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestLintSubgraphRef_EmptyRef(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{
				ID:     "Sub",
				Kind:   ir.NodeSubgraph,
				Config: ir.SubgraphConfig{Ref: ""},
			},
		},
	}
	diags := lintSubgraphRef(w)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for empty ref, got %d", len(diags))
	}
}

func TestLintSubgraphRef_NonSubgraphNode(t *testing.T) {
	w := &ir.Workflow{
		Nodes: []*ir.Node{
			{
				ID:     "Agent",
				Kind:   ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "hello"},
			},
		},
	}
	diags := lintSubgraphRef(w)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for non-subgraph node, got %d", len(diags))
	}
}
