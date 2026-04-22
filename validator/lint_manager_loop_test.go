//go:build !wasm

package validator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/simulate"
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
		PollInterval: -5 * time.Second,
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

// TestLintConditions_StackChildNamespace is a pinning test: a manager_loop
// stop_condition that references stack.child.* must never fire DIP120
// (variable missing namespace prefix). The stack.* namespace is valid for
// supervisor-exposed runtime variables such as stack.child.cycles and
// stack.child.outcome. This test exercises the full Lint() path so it will
// catch regressions if DIP120 is later extended to walk node-level conditions.
func TestLintConditions_StackChildNamespace(t *testing.T) {
	w := &ir.Workflow{
		Name:  "W",
		Start: "M",
		Exit:  "M",
		Nodes: []*ir.Node{
			{ID: "M", Kind: ir.NodeManagerLoop, Config: ir.ManagerLoopConfig{
				SubgraphRef:   "inner.dip",
				MaxCycles:     5,
				StopCondition: &ir.Condition{Raw: "stack.child.cycles = 10"},
			}},
		},
		Edges: []*ir.Edge{{From: "M", To: "M"}},
	}
	res := Lint(w)
	for _, d := range res.Diagnostics {
		if d.Code == DIP120 {
			t.Errorf("stack.child.* should be a recognized namespace, got DIP120: %s", d.Message)
		}
	}
}

func TestLintManagerLoop_DIP136_SteerContextReservedChar(t *testing.T) {
	// A steer_context key containing '=' or value containing ',' should fire DIP136.
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef: "inner.dip",
		MaxCycles:   5,
		SteerContext: map[string]string{
			"bad=key": "value",
			"good":    "val,ue",
		},
	})
	diags := lintManagerLoop(w)
	if !diagHasCode(diags, DIP136) {
		t.Errorf("expected DIP136 for steer_context with reserved delimiters, got %v", diags)
	}
}

func TestLintManagerLoop_DIP137_NotFiredWhenStopConditionOnlyInParsed(t *testing.T) {
	// Mirrors the formatter/exporter Parsed-fallback: a stop_condition with
	// only Parsed populated (Raw empty) still counts as a bounding signal.
	parsed, err := simulate.ParseCondition("stack.child.outcome = success")
	if err != nil {
		t.Fatalf("ParseCondition: %v", err)
	}
	w := managerLoopWorkflow(ir.ManagerLoopConfig{
		SubgraphRef:   "inner.dip",
		StopCondition: &ir.Condition{Parsed: parsed}, // Raw intentionally empty
	})
	diags := lintManagerLoop(w)
	if diagHasCode(diags, DIP137) {
		t.Errorf("DIP137 should NOT fire when StopCondition.Parsed is set: %v", diags)
	}
}
