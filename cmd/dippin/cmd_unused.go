package main

import (
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/unused"
)

// CmdUnused detects dead-branch nodes and their wasted cost.
func (c *CLI) CmdUnused(args []string) ExitCode {
	path, code := parseSingleFileArg("unused", "usage: dippin unused <file>", args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	report := unused.Analyze(w)
	return c.renderUnusedReport(report)
}

// renderUnusedReport outputs the unused-node report in the selected format.
func (c *CLI) renderUnusedReport(r *unused.Report) ExitCode {
	if c.Format == FormatJSON {
		return c.renderJSON(r)
	}
	renderUnusedText(c.Stdout, r)
	return ExitOK
}

// renderUnusedText writes a human-readable unused-node report.
func renderUnusedText(w io.Writer, r *unused.Report) {
	fmt.Fprintln(w, "═══ Unused Nodes ══════════════════════════════════════════")
	renderUnusedNodes(w, r)
	renderWastedCost(w, r)
	renderUnusedSummary(w, r)
}

// renderUnusedNodes lists each unused node with its kind and label.
func renderUnusedNodes(w io.Writer, r *unused.Report) {
	for _, n := range r.UnusedNodes {
		label := ""
		if n.Label != "" {
			label = fmt.Sprintf("  (%s)", n.Label)
		}
		fmt.Fprintf(w, "  ✗ %-30s %-6s%s\n", n.NodeID, n.Kind, label)
	}
}

// renderWastedCost shows the estimated wasted cost range.
func renderWastedCost(w io.Writer, r *unused.Report) {
	fmt.Fprintln(w, "─── Wasted Cost ───────────────────────────────────────────")
	fmt.Fprintf(w, "  %s - %s estimated wasted per run\n",
		formatUSD(r.TotalWasted.Min), formatUSD(r.TotalWasted.Max))
}

// renderUnusedSummary shows the total count of unused nodes.
func renderUnusedSummary(w io.Writer, r *unused.Report) {
	fmt.Fprintln(w, "─── Summary ───────────────────────────────────────────────")
	fmt.Fprintf(w, "  %d unused node(s) found\n", len(r.UnusedNodes))
}
