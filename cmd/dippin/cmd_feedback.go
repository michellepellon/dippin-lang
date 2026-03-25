package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/cost"
	"github.com/2389-research/dippin-lang/feedback"
)

// CmdFeedback compares predicted vs actual costs using telemetry data.
func (c *CLI) CmdFeedback(args []string) ExitCode {
	fs := flag.NewFlagSet("feedback", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 2 {
		fmt.Fprintln(c.Stderr, "usage: dippin feedback <workflow.dip> <telemetry.jsonl>")
		return ExitUsageError
	}

	w, err := loadWorkflow(fs.Arg(0))
	if err != nil {
		c.renderError(err, fs.Arg(0))
		return ExitError
	}

	predicted := cost.Analyze(w, cost.DefaultPricing())
	report, err := feedback.Analyze(predicted, fs.Arg(1))
	if err != nil {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
		return ExitError
	}

	return c.renderFeedbackReport(report)
}

// renderFeedbackReport outputs the feedback report in the selected format.
func (c *CLI) renderFeedbackReport(r *feedback.Report) ExitCode {
	if c.Format == FormatJSON {
		return c.renderJSON(r)
	}
	renderFeedbackText(c.Stdout, r)
	return ExitOK
}

// renderFeedbackText writes a human-readable feedback report.
func renderFeedbackText(w io.Writer, r *feedback.Report) {
	fmt.Fprintln(w, "═══ Cost Calibration Report ═══════════════════════════════")
	fmt.Fprintf(w, "  Overall Accuracy: %.1f%%\n\n", r.Accuracy)
	renderFeedbackNodes(w, r)
	renderFeedbackOutliers(w, r)
}

func renderFeedbackNodes(w io.Writer, r *feedback.Report) {
	if len(r.Nodes) == 0 {
		fmt.Fprintln(w, "  (no matching telemetry data)")
		return
	}
	fmt.Fprintln(w, "─── Per-Node Comparison ────────────────────────────────────")
	fmt.Fprintf(w, "  %-20s %10s %10s %8s\n", "Node", "Predicted", "Actual", "Ratio")
	fmt.Fprintf(w, "  %-20s %10s %10s %8s\n", "────────────────────", "──────────", "──────────", "────────")
	for _, nc := range r.Nodes {
		fmt.Fprintf(w, "  %-20s %10s %10s %7.1fx\n",
			nc.NodeID, formatUSD(nc.PredictedCost), formatUSD(nc.ActualCost), nc.Ratio)
	}
	fmt.Fprintln(w)
}

func renderFeedbackOutliers(w io.Writer, r *feedback.Report) {
	if len(r.Outliers) == 0 {
		fmt.Fprintln(w, "─── No outliers — predictions are well-calibrated! ────────")
		return
	}
	fmt.Fprintln(w, "─── Outliers ──────────────────────────────────────────────")
	for _, o := range r.Outliers {
		fmt.Fprintf(w, "  ⚠ %s (%.1fx): %s\n", o.NodeID, o.Ratio, o.Message)
	}
}
