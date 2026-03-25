package optimize_test

import (
	"os"
	"testing"

	"github.com/2389-research/dippin-lang/cost"
	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/optimize"
	"github.com/2389-research/dippin-lang/parser"
)

func loadFixture(t *testing.T, path string) *ir.Workflow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	p := parser.NewParser(string(data), path)
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	return w
}

func TestAnalyze_Overprovisioned(t *testing.T) {
	w := loadFixture(t, "testdata/overprovisioned.dip")
	report := optimize.Analyze(w, cost.DefaultPricing())

	if len(report.Suggestions) == 0 {
		t.Fatal("expected optimization suggestions for overprovisioned workflow")
	}

	// Should suggest downgrading bookkeeping nodes.
	foundBookkeeping := false
	for _, s := range report.Suggestions {
		if s.Rule == "bookkeeping-node" {
			foundBookkeeping = true
		}
	}
	if !foundBookkeeping {
		t.Error("expected bookkeeping-node suggestion")
	}
}

func TestAnalyze_Optimal(t *testing.T) {
	w := loadFixture(t, "testdata/optimal.dip")
	report := optimize.Analyze(w, cost.DefaultPricing())

	// Optimal workflow should have no or few suggestions.
	for _, s := range report.Suggestions {
		if s.Rule == "simple-prompt-expensive-model" {
			t.Errorf("unexpected suggestion for optimal workflow: %s", s.Message)
		}
	}
}

func TestAnalyze_SavingsNonNegative(t *testing.T) {
	w := loadFixture(t, "testdata/overprovisioned.dip")
	report := optimize.Analyze(w, cost.DefaultPricing())

	if report.Savings.Expected < 0 {
		t.Errorf("savings should not be negative, got %f", report.Savings.Expected)
	}
	if report.OptimizedCost.Expected < 0 {
		t.Errorf("optimized cost should not be negative, got %f", report.OptimizedCost.Expected)
	}
}

func TestAnalyze_OptimizedCostLowerOrEqual(t *testing.T) {
	w := loadFixture(t, "testdata/overprovisioned.dip")
	report := optimize.Analyze(w, cost.DefaultPricing())

	if report.OptimizedCost.Expected > report.CurrentCost.Expected {
		t.Errorf("optimized cost (%f) should be <= current cost (%f)",
			report.OptimizedCost.Expected, report.CurrentCost.Expected)
	}
}

func TestAnalyze_SuggestionFields(t *testing.T) {
	w := loadFixture(t, "testdata/overprovisioned.dip")
	report := optimize.Analyze(w, cost.DefaultPricing())

	for _, s := range report.Suggestions {
		if s.NodeID == "" {
			t.Error("suggestion NodeID should not be empty")
		}
		if s.Rule == "" {
			t.Error("suggestion Rule should not be empty")
		}
		if s.Message == "" {
			t.Error("suggestion Message should not be empty")
		}
	}
}
