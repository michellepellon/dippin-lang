package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/formatter"
	"github.com/2389-research/dippin-lang/scaffold"
)

// CmdNew generates a starter .dip file from a named template.
func (c *CLI) CmdNew(args []string) ExitCode {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	name := fs.String("name", "", "workflow name (default: template name)")
	writePath := fs.String("write", "", "write output to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintf(c.Stderr, "usage: dippin new [--name <name>] [--write <file>] <template>\n")
		fmt.Fprintf(c.Stderr, "templates: %s\n", strings.Join(scaffold.TemplateNames(), ", "))
		return ExitUsageError
	}

	template := fs.Arg(0)
	w, err := scaffold.Build(template, *name)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
		return ExitError
	}

	return writeOutput(c.Stdout, c.Stderr, *writePath, formatter.Format(w))
}
