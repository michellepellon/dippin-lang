package testrunner_test

import (
	"os"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/simulate"
	"github.com/2389-research/dippin-lang/testrunner"
)

func TestRunSuite(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	suite, err := testrunner.LoadTestFile("testdata/simple.test.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	result := testrunner.RunSuite(w, suite)
	reportFailures(t, result)
	if result.Total != 2 {
		t.Errorf("expected 2 total, got %d", result.Total)
	}
}

func reportFailures(t *testing.T, result *testrunner.SuiteResult) {
	t.Helper()
	for _, r := range result.Results {
		if !r.Passed {
			t.Errorf("test %q failed: %v", r.Name, r.Errors)
		}
	}
}

func TestFindTestFile(t *testing.T) {
	got := testrunner.FindTestFile("workflow.dip")
	if got != "workflow.test.json" {
		t.Errorf("got %q, want %q", got, "workflow.test.json")
	}
}

func TestLoadTestFile_NotFound(t *testing.T) {
	_, err := testrunner.LoadTestFile("nonexistent.test.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRunCase_WrongStatus(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "wrong status",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			Status: "fail",
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail")
	}
	if len(cr.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(cr.Errors))
	}
	want := `expected status "fail", got "success"`
	if cr.Errors[0] != want {
		t.Errorf("got %q, want %q", cr.Errors[0], want)
	}
}

func TestLoadTestFile_WrongSchema(t *testing.T) {
	_, err := testrunner.LoadTestFile("testdata/wrong_schema.test.json")
	if err == nil {
		t.Error("expected error for wrong schema")
	}
}

func TestLoadTestFile_EmptyTests(t *testing.T) {
	_, err := testrunner.LoadTestFile("testdata/empty_tests.test.json")
	if err == nil {
		t.Error("expected error for empty tests")
	}
}

func parseFixture(t *testing.T, path string) *ir.Workflow {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	p := parser.NewParser(string(src), path)
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return w
}
