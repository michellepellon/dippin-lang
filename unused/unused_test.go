package unused_test

import (
	"os"
	"testing"

	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/unused"
)

func TestAnalyze(t *testing.T) {
	src, err := os.ReadFile("testdata/has_dead_branch.dip")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	p := parser.NewParser(string(src), "testdata/has_dead_branch.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	report := unused.Analyze(w)

	assertDeadEndPresent(t, report)
	assertNoFalsePositives(t, report)
	assertWastedCost(t, report)
}

func assertDeadEndPresent(t *testing.T, report *unused.Report) {
	t.Helper()
	if len(report.UnusedNodes) != 1 {
		t.Fatalf("expected 1 unused node, got %d", len(report.UnusedNodes))
	}
	if report.UnusedNodes[0].NodeID != "DeadEnd" {
		t.Errorf("expected unused node DeadEnd, got %s", report.UnusedNodes[0].NodeID)
	}
}

func assertNoFalsePositives(t *testing.T, report *unused.Report) {
	t.Helper()
	for _, n := range report.UnusedNodes {
		switch n.NodeID {
		case "Start", "Gate", "Pass", "Exit":
			t.Errorf("node %s should not be unused", n.NodeID)
		}
	}
}

func assertWastedCost(t *testing.T, report *unused.Report) {
	t.Helper()
	if report.TotalWasted.Max <= 0 {
		t.Error("expected positive TotalWasted.Max")
	}
}
