package main

import (
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/cost"
)

// CmdCost estimates the execution cost of a workflow.
func (c *CLI) CmdCost(args []string) ExitCode {
	path, code := parseSingleFileArg("cost", "usage: dippin cost <file>", args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	report := cost.Analyze(w, cost.DefaultPricing())
	return c.renderCostReport(report)
}

// renderCostReport outputs the cost report in the selected format.
func (c *CLI) renderCostReport(r *cost.Report) ExitCode {
	if c.Format == FormatJSON {
		return c.renderJSON(r)
	}
	renderCostText(c.Stdout, r)
	return ExitOK
}

// renderCostText writes a human-readable cost report.
func renderCostText(w io.Writer, r *cost.Report) {
	fmt.Fprintln(w, "═══ Cost Estimate ═════════════════════════════════════════")
	fmt.Fprintf(w, "  %-24s %8s %8s %8s\n", "", "Min", "Expected", "Max")
	fmt.Fprintf(w, "  %-24s %8s %8s %8s\n", "────────────────────────", "────────", "────────", "────────")
	fmt.Fprintf(w, "  %-24s %8s %8s %8s\n", "TOTAL",
		formatUSD(r.Total.Min), formatUSD(r.Total.Expected), formatUSD(r.Total.Max))
	fmt.Fprintln(w)
	renderCostByProvider(w, r)
	renderTopCosts(w, r)
	renderAssumptions(w, r)
}

func renderCostByProvider(w io.Writer, r *cost.Report) {
	if len(r.ByProvider) == 0 {
		return
	}
	fmt.Fprintln(w, "─── By Provider ───────────────────────────────────────────")
	for provider, cr := range r.ByProvider {
		fmt.Fprintf(w, "  %-24s %8s %8s %8s\n", provider,
			formatUSD(cr.Min), formatUSD(cr.Expected), formatUSD(cr.Max))
	}
	fmt.Fprintln(w)
}

func renderTopCosts(w io.Writer, r *cost.Report) {
	if len(r.TopCosts) == 0 {
		return
	}
	fmt.Fprintln(w, "─── Top Cost Drivers ──────────────────────────────────────")
	for _, nc := range r.TopCosts {
		fmt.Fprintf(w, "  %-24s %8s (max)  %s/%s\n", nc.NodeID,
			formatUSD(nc.Cost.Max), nc.Provider, nc.Model)
	}
	fmt.Fprintln(w)
}

func renderAssumptions(w io.Writer, r *cost.Report) {
	if len(r.Assumptions) == 0 {
		return
	}
	fmt.Fprintln(w, "─── Assumptions ───────────────────────────────────────────")
	for _, a := range r.Assumptions {
		fmt.Fprintf(w, "  • %s\n", a)
	}
	fmt.Fprintln(w)
}

func formatUSD(v float64) string {
	return fmt.Sprintf("$%.2f", v)
}
