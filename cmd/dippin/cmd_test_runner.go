package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/testrunner"
	"github.com/2389-research/dippin-lang/validator"
)

// CmdTest runs scenario tests defined in .test.json files against a workflow.
func (c *CLI) CmdTest(args []string) ExitCode {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	verbose := fs.Bool("verbose", false, "show execution path for each test")
	coverage := fs.Bool("coverage", false, "show edge coverage summary")

	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin test [--verbose] [--coverage] <file.dip>")
		return ExitUsageError
	}

	return c.runTestFile(fs.Arg(0), *verbose, *coverage)
}

// runTestFile loads the workflow and test suite, runs it, and renders results.
func (c *CLI) runTestFile(path string, verbose, coverage bool) ExitCode {
	w, suite, code := c.loadTestInputs(path)
	if code != ExitCode(-1) {
		return code
	}

	result := testrunner.RunSuite(w, suite)
	if c.Format == FormatJSON {
		return c.renderJSON(result)
	}
	code = c.renderTestText(result, verbose)
	if coverage {
		c.renderEdgeCoverage(w, result)
	}
	return code
}

// loadTestInputs loads and validates the workflow and test suite.
func (c *CLI) loadTestInputs(path string) (*ir.Workflow, *testrunner.TestSuite, ExitCode) {
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return nil, nil, ExitError
	}
	if valRes := validator.Validate(w); valRes.HasErrors() {
		c.renderDiagnostics(valRes.Diagnostics)
		return nil, nil, ExitError
	}
	suite, err := testrunner.LoadTestFile(testrunner.FindTestFile(path))
	if err != nil {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
		return nil, nil, ExitError
	}
	return w, suite, ExitCode(-1)
}

// renderTestText outputs test results in human-readable text format.
func (c *CLI) renderTestText(result *testrunner.SuiteResult, verbose bool) ExitCode {
	fmt.Fprintln(c.Stdout, "\u2550\u2550\u2550 Test Results \u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
	for _, cr := range result.Results {
		c.renderCaseResult(cr, verbose)
	}
	c.renderTestSummary(result)
	return c.testExitCode(result)
}

// renderCaseResult outputs a single test case result line.
func (c *CLI) renderCaseResult(cr testrunner.CaseResult, verbose bool) {
	if cr.Passed {
		fmt.Fprintf(c.Stdout, "  PASS  %s\n", cr.Name)
	} else {
		fmt.Fprintf(c.Stdout, "  FAIL  %s \u2014 %s\n", cr.Name, strings.Join(cr.Errors, "; "))
	}
	if verbose && len(cr.Path) > 0 {
		fmt.Fprintf(c.Stdout, "        path: %s\n", strings.Join(cr.Path, " \u2192 "))
	}
}

// renderTestSummary outputs the summary footer.
func (c *CLI) renderTestSummary(result *testrunner.SuiteResult) {
	fmt.Fprintln(c.Stdout, "\u2500\u2500\u2500 Summary \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500")
	fmt.Fprintf(c.Stdout, "  %d tests: %d passed, %d failed\n",
		result.Total, result.Passed, result.Failed)
}

// testExitCode returns ExitOK if all tests passed, ExitError otherwise.
func (c *CLI) testExitCode(result *testrunner.SuiteResult) ExitCode {
	if result.Failed > 0 {
		return ExitError
	}
	return ExitOK
}

// renderEdgeCoverage outputs the edge coverage summary.
func (c *CLI) renderEdgeCoverage(w *ir.Workflow, sr *testrunner.SuiteResult) {
	cov := testrunner.ComputeEdgeCoverage(w, sr)
	fmt.Fprintf(c.Stdout,
		"\n\u2500\u2500\u2500 Edge Coverage \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\n")
	fmt.Fprintf(c.Stdout, "  %d/%d edges covered (%.1f%%)\n", cov.Covered, cov.Total, cov.Percent)
	for _, u := range cov.Uncovered {
		fmt.Fprintf(c.Stdout, "  \u2717 %s\n", u)
	}
}
