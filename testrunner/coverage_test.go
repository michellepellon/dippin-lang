package testrunner

import (
	"testing"

	"github.com/2389-research/dippin-lang/event"
	"github.com/2389-research/dippin-lang/ir"
)

func TestComputeEdgeCoverage_AllCovered(t *testing.T) {
	w := &ir.Workflow{
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
		},
	}
	sr := &SuiteResult{
		Results: []CaseResult{
			{
				Events: []event.Event{
					event.EdgeTraverse{From: "A", To: "B"},
					event.EdgeTraverse{From: "B", To: "C"},
				},
			},
		},
	}
	cov := ComputeEdgeCoverage(w, sr)
	if cov.Covered != 2 {
		t.Errorf("covered = %d, want 2", cov.Covered)
	}
	if cov.Total != 2 {
		t.Errorf("total = %d, want 2", cov.Total)
	}
	if len(cov.Uncovered) != 0 {
		t.Errorf("uncovered = %v, want empty", cov.Uncovered)
	}
}

func TestComputeEdgeCoverage_Partial(t *testing.T) {
	w := &ir.Workflow{
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
			{From: "B", To: "D"},
			{From: "C", To: "D"},
		},
	}
	sr := &SuiteResult{
		Results: []CaseResult{
			{
				Events: []event.Event{
					event.EdgeTraverse{From: "A", To: "B"},
					event.EdgeTraverse{From: "B", To: "D"},
				},
			},
		},
	}
	cov := ComputeEdgeCoverage(w, sr)
	if cov.Covered != 2 {
		t.Errorf("covered = %d, want 2", cov.Covered)
	}
	if len(cov.Uncovered) != 2 {
		t.Errorf("uncovered count = %d, want 2", len(cov.Uncovered))
	}
	if cov.Percent != 50.0 {
		t.Errorf("percent = %.1f, want 50.0", cov.Percent)
	}
}

func TestComputeEdgeCoverage_AcrossCases(t *testing.T) {
	w := &ir.Workflow{
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
		},
	}
	sr := &SuiteResult{
		Results: []CaseResult{
			{Events: []event.Event{event.EdgeTraverse{From: "A", To: "B"}}},
			{Events: []event.Event{event.EdgeTraverse{From: "A", To: "C"}}},
		},
	}
	cov := ComputeEdgeCoverage(w, sr)
	if cov.Covered != 2 {
		t.Errorf("covered = %d, want 2 (aggregated across cases)", cov.Covered)
	}
}

func TestComputeEdgeCoverage_Empty(t *testing.T) {
	w := &ir.Workflow{}
	sr := &SuiteResult{}
	cov := ComputeEdgeCoverage(w, sr)
	if cov.Total != 0 || cov.Covered != 0 {
		t.Errorf("expected 0/0, got %d/%d", cov.Covered, cov.Total)
	}
}
