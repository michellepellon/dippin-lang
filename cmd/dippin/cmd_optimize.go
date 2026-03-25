package main

import (
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/cost"
	"github.com/2389-research/dippin-lang/optimize"
)

// CmdOptimize analyzes a workflow for model cost optimization opportunities.
func (c *CLI) CmdOptimize(args []string) ExitCode {
	path, code := parseSingleFileArg("optimize", "usage: dippin optimize <file>", args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	report := optimize.Analyze(w, cost.DefaultPricing())
	return c.renderOptimizeReport(report)
}

// renderOptimizeReport outputs the optimization report in the selected format.
func (c *CLI) renderOptimizeReport(r *optimize.Report) ExitCode {
	if c.Format == FormatJSON {
		return c.renderJSON(r)
	}
	renderOptimizeText(c.Stdout, r)
	return ExitOK
}

// renderOptimizeText writes a human-readable optimization report.
func renderOptimizeText(w io.Writer, r *optimize.Report) {
	fmt.Fprintln(w, "═══ Optimization Report ═══════════════════════════════════")
	renderOptimizeCostSummary(w, r)
	renderOptimizeSuggestions(w, r)
}

func renderOptimizeCostSummary(w io.Writer, r *optimize.Report) {
	fmt.Fprintln(w, "─── Cost Summary ──────────────────────────────────────────")
	fmt.Fprintf(w, "  Current:   %s (expected)\n", formatUSD(r.CurrentCost.Expected))
	fmt.Fprintf(w, "  Optimized: %s (expected)\n", formatUSD(r.OptimizedCost.Expected))
	fmt.Fprintf(w, "  Savings:   %s (expected)\n", formatUSD(r.Savings.Expected))
	fmt.Fprintln(w)
}

func renderOptimizeSuggestions(w io.Writer, r *optimize.Report) {
	if len(r.Suggestions) == 0 {
		fmt.Fprintln(w, "─── No optimization suggestions — models are well-matched! ─")
		return
	}
	fmt.Fprintln(w, "─── Suggestions ───────────────────────────────────────────")
	for _, s := range r.Suggestions {
		fmt.Fprintf(w, "  [%s] %s\n", s.NodeID, s.Message)
		fmt.Fprintf(w, "    %s → %s", s.CurrentModel, s.SuggestModel)
		if s.Savings.Expected > 0 {
			fmt.Fprintf(w, "  (saves ~%s)", formatUSD(s.Savings.Expected))
		}
		fmt.Fprintln(w)
	}
}
