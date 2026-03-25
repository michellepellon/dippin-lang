package doctor_test

import (
	"os"
	"testing"

	"github.com/2389-research/dippin-lang/cost"
	"github.com/2389-research/dippin-lang/doctor"
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

func TestDiagnose_Healthy(t *testing.T) {
	w := loadFixture(t, "testdata/healthy.dip")
	report := doctor.Diagnose(w, cost.DefaultPricing())

	if report.Grade != "A" {
		t.Errorf("expected grade A, got %s (score=%d)", report.Grade, report.Score)
	}
	if report.Score < 90 {
		t.Errorf("expected score >= 90, got %d", report.Score)
	}
	if report.Lint.Errors != 0 {
		t.Errorf("expected 0 lint errors, got %d", report.Lint.Errors)
	}
}

func TestDiagnose_Unhealthy(t *testing.T) {
	w := loadFixture(t, "testdata/unhealthy.dip")
	report := doctor.Diagnose(w, cost.DefaultPricing())

	if report.Score >= 90 {
		t.Errorf("expected score < 90 for unhealthy workflow, got %d", report.Score)
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected suggestions for unhealthy workflow")
	}
}

func TestScoreFloor(t *testing.T) {
	w := loadFixture(t, "testdata/unhealthy.dip")
	report := doctor.Diagnose(w, cost.DefaultPricing())

	if report.Score < 0 {
		t.Errorf("score should not go below 0, got %d", report.Score)
	}
}

func TestReportHasAllFields(t *testing.T) {
	w := loadFixture(t, "testdata/healthy.dip")
	report := doctor.Diagnose(w, cost.DefaultPricing())

	if report.Grade == "" {
		t.Error("grade should not be empty")
	}
	if report.Coverage.TotalNodes == 0 {
		t.Error("total nodes should not be 0")
	}
}
