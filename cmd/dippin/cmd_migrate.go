package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/migrate"
	"github.com/2389-research/dippin-lang/parser"
)

// CmdMigrate converts a DOT file to .dip source.
//   - Default: print to stdout
//   - --output <file>: write to specified file
func (c *CLI) CmdMigrate(args []string) ExitCode {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	output := fs.String("output", "", "output file path")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin migrate [--output <file>] <file.dot>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	source, err := migrateFile(path)
	if err != nil {
		fmt.Fprintf(c.Stderr, "%v\n", err)
		return ExitError
	}

	return writeOutput(c.Stdout, c.Stderr, *output, source)
}

// migrateFile reads a DOT file and converts it to .dip source.
func migrateFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error: %w", err)
	}

	source, err := migrate.MigrateToSource(string(data))
	if err != nil {
		return "", fmt.Errorf("migration failed: %w", err)
	}
	return source, nil
}

// loadDOTWorkflow reads and parses a DOT file into an IR workflow.
func loadDOTWorkflow(path string, stderr io.Writer) (*ir.Workflow, ExitCode) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error reading %s: %v\n", path, err)
		return nil, ExitError
	}
	w, err := migrate.Migrate(string(data))
	if err != nil {
		fmt.Fprintf(stderr, "error parsing %s: %v\n", path, err)
		return nil, ExitError
	}
	return w, ExitCode(-1)
}

// loadDIPWorkflow reads and parses a .dip file into an IR workflow.
func (c *CLI) loadDIPWorkflow(path string) (*ir.Workflow, ExitCode) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error reading %s: %v\n", path, err)
		return nil, ExitError
	}
	p := parser.NewParser(string(data), path)
	w, err := p.Parse()
	if err != nil {
		c.renderError(err, path)
		return nil, ExitError
	}
	return w, ExitCode(-1)
}

// renderParityDiffs renders migration parity check differences.
func (c *CLI) renderParityDiffs(diffs []migrate.Difference) {
	if c.Format == FormatJSON {
		b, _ := json.MarshalIndent(diffs, "", "  ")
		fmt.Fprintln(c.Stderr, string(b))
		return
	}
	fmt.Fprintf(c.Stderr, "parity check failed: %d difference(s) found\n", len(diffs))
	for _, d := range diffs {
		fmt.Fprintf(c.Stderr, "  [%s] %s\n", d.Kind, d.Message)
	}
}

// CmdValidateMigration checks structural parity between a DOT file and a .dip file.
func (c *CLI) CmdValidateMigration(args []string) ExitCode {
	fs := flag.NewFlagSet("validate-migration", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 2 {
		fmt.Fprintln(c.Stderr, "usage: dippin validate-migration <old.dot> <new.dip>")
		return ExitUsageError
	}

	wOld, code := loadDOTWorkflow(fs.Arg(0), c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	wNew, code := c.loadDIPWorkflow(fs.Arg(1))
	if code != ExitCode(-1) {
		return code
	}

	return c.checkMigrationParity(wOld, wNew)
}

// checkMigrationParity compares two workflows and reports the result.
func (c *CLI) checkMigrationParity(wOld, wNew *ir.Workflow) ExitCode {
	diffs := migrate.CheckParity(wOld, wNew)
	if len(diffs) > 0 {
		c.renderParityDiffs(diffs)
		return ExitError
	}

	if c.Format == FormatText {
		fmt.Fprintln(c.Stdout, "migration parity check passed")
	}
	return ExitOK
}
