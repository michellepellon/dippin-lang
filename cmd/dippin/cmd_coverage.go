package main

import (
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/coverage"
)

// CmdCoverage analyzes edge coverage and reachability.
func (c *CLI) CmdCoverage(args []string) ExitCode {
	path, code := parseSingleFileArg("coverage", "usage: dippin coverage <file>", args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	report := coverage.Analyze(w)
	return c.renderCoverageReport(report)
}

// renderCoverageReport outputs the coverage report in the selected format.
func (c *CLI) renderCoverageReport(r *coverage.Report) ExitCode {
	if c.Format == FormatJSON {
		return c.renderJSON(r)
	}
	renderCoverageText(c.Stdout, r)
	return ExitOK
}

// renderCoverageText writes a human-readable coverage report.
func renderCoverageText(w io.Writer, r *coverage.Report) {
	fmt.Fprintln(w, "═══ Coverage Analysis ═════════════════════════════════════")
	renderNodeCoverage(w, r)
	renderReachability(w, r)
	renderTermination(w, r)
}

func renderNodeCoverage(w io.Writer, r *coverage.Report) {
	if len(r.Nodes) == 0 {
		return
	}
	fmt.Fprintln(w, "─── Edge Coverage ─────────────────────────────────────────")
	for _, nc := range r.Nodes {
		icon := statusIcon(nc.Status)
		fmt.Fprintf(w, "  %s %-28s %s\n", icon, nc.NodeID, nc.Status)
		renderMissingEdges(w, nc)
	}
	fmt.Fprintln(w)
}

func renderMissingEdges(w io.Writer, nc coverage.NodeCoverage) {
	if nc.Status != "partial" {
		return
	}
	for _, m := range nc.MissingEdges {
		fmt.Fprintf(w, "      missing: %s\n", m)
	}
}

func renderReachability(w io.Writer, r *coverage.Report) {
	icon := "✓"
	if len(r.Reachability.UnreachableNodes) > 0 {
		icon = "✗"
	}
	fmt.Fprintf(w, "─── Reachability ──────────────────────────────────────────\n")
	fmt.Fprintf(w, "  %s %d/%d nodes reachable\n", icon,
		r.Reachability.ReachableNodes, r.Reachability.TotalNodes)
	for _, n := range r.Reachability.UnreachableNodes {
		fmt.Fprintf(w, "      unreachable: %s\n", n)
	}
	fmt.Fprintln(w)
}

func renderTermination(w io.Writer, r *coverage.Report) {
	icon := "✓"
	if !r.Termination.AllPathsTerminate {
		icon = "✗"
	}
	fmt.Fprintf(w, "─── Termination ───────────────────────────────────────────\n")
	fmt.Fprintf(w, "  %s all paths reach exit: %v\n", icon, r.Termination.AllPathsTerminate)
	for _, n := range r.Termination.SinkNodes {
		fmt.Fprintf(w, "      sink node: %s\n", n)
	}
}

func statusIcon(status string) string {
	switch status {
	case "covered", "no_conditions":
		return "✓"
	case "partial":
		return "✗"
	default:
		return "?"
	}
}
