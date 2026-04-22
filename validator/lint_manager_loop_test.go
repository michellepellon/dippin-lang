//go:build !wasm

package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func managerLoopWorkflow(cfg ir.ManagerLoopConfig) *ir.Workflow {
	return &ir.Workflow{
		Name:  "W",
		Start: "M",
		Exit:  "M",
		Nodes: []*ir.Node{{ID: "M", Kind: ir.NodeManagerLoop, Config: cfg}},
		Edges: []*ir.Edge{{From: "M", To: "M"}},
	}
}

// diagHasCode reports whether diags contains a diagnostic with the given code.
func diagHasCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestLintManagerLoop_DIP135_MissingRef(t *testing.T) {
	w := managerLoopWorkflow(ir.ManagerLoopConfig{MaxCycles: 5})
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP135) {
		t.Errorf("expected DIP135 for missing subgraph_ref, got %v", diags)
	}
}

func TestLintManagerLoop_DIP135_RefFileDoesNotExist(t *testing.T) {
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef: "does_not_exist.dip",
		MaxCycles:   5,
	})
	w.Nodes[0].Source = ir.SourceLocation{File: "/tmp/fakeworkflow.dip"}
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP135) {
		t.Errorf("expected DIP135 for nonexistent ref, got %v", diags)
	}
}

func TestLintManagerLoop_DIP135_RefExists(t *testing.T) {
	// Create a temp file and a workflow that references it relatively.
	dir := t.TempDir()
	refFile := filepath.Join(dir, "child.dip")
	if err := os.WriteFile(refFile, []byte("workflow C\n"), 0644); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(dir, "parent.dip")
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef: "child.dip",
		MaxCycles:   5,
	})
	w.Nodes[0].Source = ir.SourceLocation{File: srcFile}
	diags := lintManagerLoop(w)
	if diagHasCode(diags, DIP135) {
		t.Errorf("unexpected DIP135 for existing ref: %v", diags)
	}
}

func TestLintManagerLoop_DIP136_NegativePollInterval(t *testing.T) {
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef:  "inner.dip",
		PollInterval: -5,
		MaxCycles:    5,
	})
	// Avoid DIP135 for nonexistent file by not setting Source.
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP136) {
		t.Errorf("expected DIP136 for negative poll_interval, got %v", diags)
	}
}

func TestLintManagerLoop_DIP136_NegativeMaxCycles(t *testing.T) {
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef: "inner.dip",
		MaxCycles:   -1,
		// Also needs some bound signal to avoid DIP137; but the test
		// is checking DIP136 specifically.
	})
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP136) {
		t.Errorf("expected DIP136 for negative max_cycles, got %v", diags)
	}
}

func TestLintManagerLoop_DIP137_Unbounded(t *testing.T) {
	w := managerLoopWorkflow(ir.ManagerLoopConfig{SubgraphRef: "inner.dip"})
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP137) {
		t.Errorf("expected DIP137 for unbounded manager_loop, got %v", diags)
	}
}

func TestLintManagerLoop_Clean(t *testing.T) {
	// Fully bounded manager_loop, no Source.File so DIP135 cannot resolve
	// and silently skips. Expect zero diagnostics.
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef:   "inner.dip",
		MaxCycles:     5,
		StopCondition: &ir.Condition{Raw: "stack.child.cycles = 10"},
	})
	diags := lintManagerLoop(w)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on clean manager_loop, got %d: %v", len(diags), diags)
	}
}

func TestLintManagerLoop_NegativeMaxCyclesDoesNotAlsoFireDIP137(t *testing.T) {
	// Verifies Fix 3: a negative max_cycles fires DIP136 but NOT DIP137,
	// because the user expressed bounding intent.
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef: "inner.dip",
		MaxCycles:   -1,
	})
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP136) {
		t.Errorf("expected DIP136, got %v", diags)
	}
	if diagHasCode(diags, DIP137) {
		t.Errorf("unexpected DIP137 — negative max_cycles should suppress unbounded warning: %v", diags)
	}
}

func TestLintManagerLoop_NonManagerNodeIgnored(t *testing.T) {
	w := &ir.Workflow{
		Name:  "W",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "hi"}}},
	}
	diags := lintManagerLoop(w)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on non-manager workflow, got %v", diags)
	}
}
