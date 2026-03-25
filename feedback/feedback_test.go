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
