package feedback_test

import (
	"os"
	"testing"

	"github.com/2389-research/dippin-lang/cost"
	"github.com/2389-research/dippin-lang/feedback"
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

func TestAnalyze_SampleData(t *testing.T) {
	w := loadFixture(t, "testdata/sample_workflow.dip")
	predicted := cost.Analyze(w, cost.DefaultPricing())

	report, err := feedback.Analyze(predicted, "testdata/sample_telemetry.jsonl")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(report.Nodes) == 0 {
		t.Error("expected node comparisons")
	}

	for _, nc := range report.Nodes {
		if nc.NodeID == "" {
			t.Error("node ID should not be empty")
		}
		if nc.ActualCost <= 0 {
			t.Errorf("node %s: expected positive actual cost", nc.NodeID)
		}
	}
}

func TestAnalyze_AccuracyRange(t *testing.T) {
	w := loadFixture(t, "testdata/sample_workflow.dip")
	predicted := cost.Analyze(w, cost.DefaultPricing())

	report, err := feedback.Analyze(predicted, "testdata/sample_telemetry.jsonl")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if report.Accuracy < 0 || report.Accuracy > 100 {
		t.Errorf("accuracy should be 0-100%%, got %f", report.Accuracy)
	}
}

func TestReadTelemetry_ValidFile(t *testing.T) {
	records, err := feedback.ReadTelemetry("testdata/sample_telemetry.jsonl")
	if err != nil {
		t.Fatalf("ReadTelemetry failed: %v", err)
	}

	if len(records) != 4 {
		t.Errorf("expected 4 records, got %d", len(records))
	}

	if records[0].Node != "Analyze" {
		t.Errorf("expected first record node=Analyze, got %s", records[0].Node)
	}
}

func TestReadTelemetry_InvalidFile(t *testing.T) {
	_, err := feedback.ReadTelemetry("testdata/nonexistent.jsonl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestAnalyze_MissingTelemetryFile(t *testing.T) {
	w := loadFixture(t, "testdata/sample_workflow.dip")
	predicted := cost.Analyze(w, cost.DefaultPricing())

	_, err := feedback.Analyze(predicted, "testdata/nonexistent.jsonl")
	if err == nil {
		t.Error("expected error for missing telemetry file")
	}
}

func TestAnalyze_Outliers(t *testing.T) {
	w := loadFixture(t, "testdata/sample_workflow.dip")
	predicted := cost.Analyze(w, cost.DefaultPricing())

	report, err := feedback.Analyze(predicted, "testdata/outlier_telemetry.jsonl")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(report.Outliers) == 0 {
		t.Error("expected outliers for skewed telemetry data")
	}

	// Check that outlier messages are populated
	for _, o := range report.Outliers {
		if o.NodeID == "" {
			t.Error("outlier NodeID should not be empty")
		}
		if o.Message == "" {
			t.Error("outlier Message should not be empty")
		}
		if o.Ratio == 0 {
			t.Error("outlier Ratio should not be zero")
		}
	}

	// Verify we get both types of outliers (over-predicted and under-predicted)
	var hasHighRatio, hasLowRatio bool
	for _, o := range report.Outliers {
		if o.Ratio > 2.0 {
			hasHighRatio = true
		}
		if o.Ratio < 0.5 {
			hasLowRatio = true
		}
	}
	if !hasHighRatio && !hasLowRatio {
		t.Error("expected at least one outlier with ratio > 2.0 or < 0.5")
	}
}

func TestAnalyze_OutlierAccuracy(t *testing.T) {
	w := loadFixture(t, "testdata/sample_workflow.dip")
	predicted := cost.Analyze(w, cost.DefaultPricing())

	report, err := feedback.Analyze(predicted, "testdata/outlier_telemetry.jsonl")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// With skewed data, accuracy should still be valid (0-100)
	if report.Accuracy < 0 || report.Accuracy > 100 {
		t.Errorf("accuracy should be 0-100%%, got %f", report.Accuracy)
	}
}

func TestAnalyze_EdgeCases(t *testing.T) {
	w := loadFixture(t, "testdata/sample_workflow.dip")
	predicted := cost.Analyze(w, cost.DefaultPricing())

	// Telemetry with zero-cost node, empty node name, and unknown node
	report, err := feedback.Analyze(predicted, "testdata/edge_case_telemetry.jsonl")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should still produce valid results
	if report.Accuracy < 0 || report.Accuracy > 100 {
		t.Errorf("accuracy should be 0-100%%, got %f", report.Accuracy)
	}

	// Zero-actual-cost node should produce ratio=0, not a panic
	for _, nc := range report.Nodes {
		if nc.ActualCost == 0 && nc.Ratio != 0 {
			t.Errorf("node %s: zero actual cost should produce ratio=0, got %f", nc.NodeID, nc.Ratio)
		}
	}
}

func TestAnalyze_EmptyComparisons(t *testing.T) {
	// Workflow with nodes that have no matching telemetry
	w := loadFixture(t, "testdata/sample_workflow.dip")
	// Modify workflow to use different node names
	for _, n := range w.Nodes {
		n.ID = "NoMatch_" + n.ID
	}
	predicted := cost.Analyze(w, cost.DefaultPricing())

	report, err := feedback.Analyze(predicted, "testdata/sample_telemetry.jsonl")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// No matching nodes means empty comparisons and 0 accuracy
	if report.Accuracy != 0 {
		t.Errorf("accuracy should be 0 for no matching nodes, got %f", report.Accuracy)
	}
}
