package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/graph"
	"github.com/2389-research/dippin-lang/ir"
)

// CmdGraph renders an ASCII DAG of the workflow.
func (c *CLI) CmdGraph(args []string) ExitCode {
	compact, path, code := parseGraphFlags(args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	return c.outputGraph(w, compact)
}

// parseGraphFlags parses graph command flags and returns compact flag, path, and exit code.
func parseGraphFlags(args []string, stderr io.Writer) (bool, string, ExitCode) {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)
	compact := fs.Bool("compact", false, "single-line compact output")

	if err := fs.Parse(args); err != nil {
		return false, "", ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: dippin graph [--compact] <file>")
		return false, "", ExitUsageError
	}
	return *compact, fs.Arg(0), ExitCode(-1)
}

// outputGraph renders the graph in the selected format.
func (c *CLI) outputGraph(w *ir.Workflow, compact bool) ExitCode {
	if c.Format == FormatJSON {
		return c.renderJSON(graph.Layers(w))
	}
	opts := graph.Options{Compact: compact}
	fmt.Fprint(c.Stdout, graph.Render(w, opts))
	return ExitOK
}
