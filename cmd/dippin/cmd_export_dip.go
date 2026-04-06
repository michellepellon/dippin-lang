// ABOUTME: CLI command for exporting a flattened workflow back to .dip format.
// ABOUTME: Resolves all subgraph refs and outputs a single flat .dip file.
package main

import (
	"flag"
	"fmt"

	"github.com/2389-research/dippin-lang/flatten"
	"github.com/2389-research/dippin-lang/formatter"
)

// CmdExportDIP exports a flattened workflow as canonical .dip text.
func (c *CLI) CmdExportDIP(args []string) ExitCode {
	fs := flag.NewFlagSet("export-dip", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin export-dip <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	w, err = flatten.Flatten(w, &flatten.DiskResolver{}, flatten.Options{})
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	fmt.Fprint(c.Stdout, formatter.Format(w))
	return ExitOK
}
