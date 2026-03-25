package diff_test

import (
	"os"
	"testing"

	"github.com/2389-research/dippin-lang/cost"
	"github.com/2389-research/dippin-lang/diff"
	"github.com/2389-research/dippin-lang/ir"
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

func TestCompare_V1toV2(t *testing.T) {
	v1 := loadFixture(t, "testdata/v1.dip")
	v2 := loadFixture(t, "testdata/v2.dip")

	report := diff.Compare(v1, v2, cost.DefaultPricing())

	// Review was added in v2.
	if len(report.NodesAdded) != 1 || report.NodesAdded[0] != "Review" {
		t.Errorf("expected [Review] added, got %v", report.NodesAdded)
	}

	// No nodes removed.
	if len(report.NodesRemoved) != 0 {
		t.Errorf("expected no removals, got %v", report.NodesRemoved)
	}

	// Process node was modified (model and prompt changed).
	foundProcess := false
	for _, nd := range report.NodesModified {
		if nd.NodeID == "Process" {
			foundProcess = true
			if len(nd.Changes) == 0 {
				t.Error("expected field changes for Process node")
			}
		}
	}
	if !foundProcess {
		t.Error("expected Process node to appear in modifications")
	}
}

func TestCompare_EdgesChanged(t *testing.T) {
	v1 := loadFixture(t, "testdata/v1.dip")
	v2 := loadFixture(t, "testdata/v2.dip")

	report := diff.Compare(v1, v2, cost.DefaultPricing())

	// Process -> Done was removed, Process -> Review and Review -> Done added.
	if len(report.EdgesAdded) < 1 {
		t.Errorf("expected edges added, got %v", report.EdgesAdded)
	}
	if len(report.EdgesRemoved) < 1 {
		t.Errorf("expected edges removed, got %v", report.EdgesRemoved)
	}
}

func TestCompare_CostDelta(t *testing.T) {
	v1 := loadFixture(t, "testdata/v1.dip")
	v2 := loadFixture(t, "testdata/v2.dip")

	report := diff.Compare(v1, v2, cost.DefaultPricing())

	// v2 has an extra node and downgraded Process, cost should change.
	if report.CostDelta.OldCost.Expected == 0 && report.CostDelta.NewCost.Expected == 0 {
		t.Error("expected non-zero cost values")
	}
}

func TestCompare_Identical(t *testing.T) {
	v1 := loadFixture(t, "testdata/v1.dip")

	report := diff.Compare(v1, v1, cost.DefaultPricing())

	if len(report.NodesAdded) != 0 {
		t.Errorf("expected no nodes added, got %v", report.NodesAdded)
	}
	if len(report.NodesRemoved) != 0 {
		t.Errorf("expected no nodes removed, got %v", report.NodesRemoved)
	}
	if len(report.NodesModified) != 0 {
		t.Errorf("expected no nodes modified, got %v", report.NodesModified)
	}
	if len(report.EdgesAdded) != 0 {
		t.Errorf("expected no edges added, got %v", report.EdgesAdded)
	}
	if len(report.EdgesRemoved) != 0 {
		t.Errorf("expected no edges removed, got %v", report.EdgesRemoved)
	}
}

func TestCompare_FieldChanges(t *testing.T) {
	v1 := loadFixture(t, "testdata/v1.dip")
	v2 := loadFixture(t, "testdata/v2.dip")

	report := diff.Compare(v1, v2, cost.DefaultPricing())

	for _, nd := range report.NodesModified {
		for _, c := range nd.Changes {
			if c.Field == "" {
				t.Error("field name should not be empty")
			}
		}
	}
}
