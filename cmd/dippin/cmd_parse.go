package main

import (
	"encoding/json"
	"flag"
	"fmt"
)

// CmdParse parses a file and outputs the IR as indented JSON to stdout.
// Auto-detects .dip vs .dot input.
func (c *CLI) CmdParse(args []string) ExitCode {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin parse <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	b, _ := json.MarshalIndent(w, "", "  ")
	fmt.Fprintln(c.Stdout, string(b))
	return ExitOK
}
