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

func TestLoadTestFile_InvalidJSON(t *testing.T) {
	_, err := testrunner.LoadTestFile("testdata/invalid_json.test.json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRunCase_SimulationError(t *testing.T) {
	// A workflow with no start node triggers a simulation error.
	w := &ir.Workflow{
		Start: "missing",
		Exit:  "also_missing",
	}
	tc := testrunner.TestCase{
		Name:     "sim error",
		Scenario: map[string]string{},
		Expect:   testrunner.Expectation{Status: "success"},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail on simulation error")
	}
	if len(cr.Errors) == 0 {
		t.Error("expected at least one error")
	}
}

func TestRunSuite_WithFailure(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	suite := &testrunner.TestSuite{
		Tests: []testrunner.TestCase{
			{
				Name:     "passes",
				Scenario: map[string]string{"outcome": "success"},
				Expect:   testrunner.Expectation{Status: "success"},
			},
			{
				Name:     "fails",
				Scenario: map[string]string{"outcome": "success"},
				Expect:   testrunner.Expectation{Status: "fail"},
			},
		},
	}

	result := testrunner.RunSuite(w, suite)
	if result.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
}

func TestCheckVisited_NodeNotFound(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "visited missing",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			Visited: []string{"NonexistentNode"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail when visited node not in path")
	}
}

func TestCheckNotVisited_NodePresent(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "not visited but present",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			NotVisited: []string{"Gate"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail when not_visited node is in path")
	}
}

func TestCheckPathContains_NotFound(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "path contains missing",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			PathContains: []string{"Gate", "NonexistentNode"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail when path_contains node not found")
	}
}

func TestCheckImmediatelyAfter_Pass(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "immediately after pass",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			ImmediatelyAfter: map[string]string{"Gate": "Pass"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if !cr.Passed {
		t.Errorf("expected test to pass, got errors: %v", cr.Errors)
	}
}

func TestCheckImmediatelyAfter_WrongNext(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "immediately after wrong next",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			ImmediatelyAfter: map[string]string{"Gate": "Fix"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail when next node doesn't match")
	}
}

func TestCheckImmediatelyAfter_NotFound(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "immediately after not found",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			ImmediatelyAfter: map[string]string{"Nonexistent": "Pass"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail when from node not in path")
	}
}

func TestCheckImmediatelyAfter_LastElement(t *testing.T) {
	simulate.ResetRunCounter()
	w := parseFixture(t, "testdata/simple.dip")
	tc := testrunner.TestCase{
		Name:     "immediately after last element",
		Scenario: map[string]string{"outcome": "success"},
		Expect: testrunner.Expectation{
			ImmediatelyAfter: map[string]string{"Exit": "Pass"},
		},
	}

	cr := testrunner.RunCase(w, tc)
	if cr.Passed {
		t.Error("expected test to fail when from node is last in path")
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
