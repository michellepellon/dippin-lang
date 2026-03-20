package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/2389/dippin/export"
	"github.com/2389/dippin/formatter"
	"github.com/2389/dippin/ir"
	"github.com/2389/dippin/migrate"
	"github.com/2389/dippin/parser"
	"github.com/2389/dippin/validator"
)

// OutputFormat controls whether diagnostics are rendered as human-readable text
// or machine-readable JSON (spec §12).
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
)

// ExitCode encodes the result of a CLI invocation.
//   - 0: success / no issues
//   - 1: errors found (validation failures, parse errors, check-mode drift)
//   - 2: usage error (bad flags, missing arguments)
type ExitCode int

const (
	ExitOK         ExitCode = 0
	ExitError      ExitCode = 1
	ExitUsageError ExitCode = 2
)

// CLI holds parsed global state and dispatches to per-command implementations.
type CLI struct {
	Stdout io.Writer
	Stderr io.Writer
	Format OutputFormat
}

// Run is the testable entry point for the entire CLI. It accepts raw args
// (without the program name), captures output into the provided writers,
// and returns a deterministic exit code.
func Run(args []string, stdout, stderr io.Writer) ExitCode {
	if len(args) == 0 {
		printGlobalUsage(stderr)
		return ExitUsageError
	}

	c := &CLI{
		Stdout: stdout,
		Stderr: stderr,
		Format: FormatText,
	}

	// Parse global flags that appear before the subcommand.
	globalFlags := flag.NewFlagSet("dippin", flag.ContinueOnError)
	globalFlags.SetOutput(stderr)
	formatStr := globalFlags.String("format", "text", "output format (text|json)")

	if err := globalFlags.Parse(args); err != nil {
		return ExitUsageError
	}

	switch *formatStr {
	case "text":
		c.Format = FormatText
	case "json":
		c.Format = FormatJSON
	default:
		fmt.Fprintf(stderr, "unknown format: %s\n", *formatStr)
		return ExitUsageError
	}

	remaining := globalFlags.Args()
	if len(remaining) == 0 {
		printGlobalUsage(stderr)
		return ExitUsageError
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	switch cmd {
	case "parse":
		return c.CmdParse(cmdArgs)
	case "validate":
		return c.CmdValidate(cmdArgs)
	case "lint":
		return c.CmdLint(cmdArgs)
	case "fmt":
		return c.CmdFmt(cmdArgs)
	case "export-dot":
		return c.CmdExportDOT(cmdArgs)
	case "migrate":
		return c.CmdMigrate(cmdArgs)
	case "validate-migration":
		return c.CmdValidateMigration(cmdArgs)
	case "help":
		printGlobalUsage(stdout)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		return ExitUsageError
	}
}

func printGlobalUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: dippin [--format text|json] <command> [args]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  parse <file>                      Parse and output IR as JSON")
	fmt.Fprintln(w, "  validate <file>                   Run structural validation (DIP001-DIP009)")
	fmt.Fprintln(w, "  lint <file>                       Run validation and semantic linting")
	fmt.Fprintln(w, "  fmt [--check] [--write] <file>    Format a .dip file")
	fmt.Fprintln(w, "  export-dot [--rankdir] [--prompts] <file>")
	fmt.Fprintln(w, "                                    Export workflow to DOT format")
	fmt.Fprintln(w, "  migrate [--output <file>] <file.dot>")
	fmt.Fprintln(w, "                                    Convert DOT to .dip")
	fmt.Fprintln(w, "  validate-migration <old.dot> <new.dip>")
	fmt.Fprintln(w, "                                    Check parity between DOT and .dip")
	fmt.Fprintln(w, "  help                              Show this help")
}

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

// CmdValidate runs structural validation (DIP001-DIP009) on a workflow.
func (c *CLI) CmdValidate(args []string) ExitCode {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin validate <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	res := validator.Validate(w)
	if len(res.Diagnostics) > 0 {
		c.renderDiagnostics(res.Diagnostics)
	}

	if res.HasErrors() {
		return ExitError
	}
	if c.Format == FormatText {
		fmt.Fprintln(c.Stdout, "validation passed")
	}
	return ExitOK
}

// CmdLint runs both structural validation and semantic linting.
// Errors cause exit 1; warnings alone exit 0.
func (c *CLI) CmdLint(args []string) ExitCode {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin lint <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	// Run both validation and linting per spec — lint includes all checks.
	valRes := validator.Validate(w)
	lintRes := validator.Lint(w)

	// Merge all diagnostics.
	allDiags := append(valRes.Diagnostics, lintRes.Diagnostics...)
	if len(allDiags) > 0 {
		c.renderDiagnostics(allDiags)
	}

	// Exit 1 only if there are errors; warnings alone pass.
	if valRes.HasErrors() {
		return ExitError
	}
	return ExitOK
}

// CmdFmt formats a .dip file to canonical form.
//   - Default: print formatted output to stdout
//   - --check: exit 1 if input is not already canonical (for CI)
//   - --write: write formatted output back to the file in-place
func (c *CLI) CmdFmt(args []string) ExitCode {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	check := fs.Bool("check", false, "exit 1 if not canonically formatted")
	write := fs.Bool("write", false, "write formatted output back to source file")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin fmt [--check] [--write] <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
		return ExitError
	}

	p := parser.NewParser(string(data), path)
	w, err := p.Parse()
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	formatted := formatter.Format(w)

	if *check {
		if formatted != string(data) {
			if c.Format == FormatText {
				fmt.Fprintf(c.Stderr, "%s: not canonically formatted\n", path)
			}
			return ExitError
		}
		return ExitOK
	}

	if *write {
		if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
			fmt.Fprintf(c.Stderr, "error writing file: %v\n", err)
			return ExitError
		}
		return ExitOK
	}

	fmt.Fprint(c.Stdout, formatted)
	return ExitOK
}

// CmdExportDOT exports a workflow to DOT graph format.
//   - --rankdir=LR|TB (default TB)
//   - --prompts includes prompt text in DOT node attributes
func (c *CLI) CmdExportDOT(args []string) ExitCode {
	fs := flag.NewFlagSet("export-dot", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	rankdir := fs.String("rankdir", "TB", "graph layout direction (LR|TB)")
	prompts := fs.Bool("prompts", false, "include prompt text in DOT attributes")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin export-dot [--rankdir=LR|TB] [--prompts] <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	opts := export.ExportOptions{
		IncludePrompts: *prompts,
		RankDir:        *rankdir,
	}

	dot := export.ExportDOT(w, opts)
	fmt.Fprint(c.Stdout, dot)
	return ExitOK
}

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
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
		return ExitError
	}

	source, err := migrate.MigrateToSource(string(data))
	if err != nil {
		fmt.Fprintf(c.Stderr, "migration failed: %v\n", err)
		return ExitError
	}

	if *output != "" {
		if err := os.WriteFile(*output, []byte(source), 0644); err != nil {
			fmt.Fprintf(c.Stderr, "error writing output: %v\n", err)
			return ExitError
		}
		return ExitOK
	}

	fmt.Fprint(c.Stdout, source)
	return ExitOK
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

	dotPath := fs.Arg(0)
	dipPath := fs.Arg(1)

	dotData, err := os.ReadFile(dotPath)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error reading %s: %v\n", dotPath, err)
		return ExitError
	}
	wOld, err := migrate.Migrate(string(dotData))
	if err != nil {
		fmt.Fprintf(c.Stderr, "error parsing %s: %v\n", dotPath, err)
		return ExitError
	}

	dipData, err := os.ReadFile(dipPath)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error reading %s: %v\n", dipPath, err)
		return ExitError
	}
	p := parser.NewParser(string(dipData), dipPath)
	wNew, err := p.Parse()
	if err != nil {
		c.renderError(err, dipPath)
		return ExitError
	}

	diffs := migrate.CheckParity(wOld, wNew)
	if len(diffs) > 0 {
		if c.Format == FormatJSON {
			b, _ := json.MarshalIndent(diffs, "", "  ")
			fmt.Fprintln(c.Stderr, string(b))
		} else {
			fmt.Fprintf(c.Stderr, "parity check failed: %d difference(s) found\n", len(diffs))
			for _, d := range diffs {
				fmt.Fprintf(c.Stderr, "  [%s] %s\n", d.Kind, d.Message)
			}
		}
		return ExitError
	}

	if c.Format == FormatText {
		fmt.Fprintln(c.Stdout, "migration parity check passed")
	}
	return ExitOK
}

// loadWorkflow reads a file and parses it to IR. It auto-detects .dot vs .dip
// based on the file extension.
func loadWorkflow(path string) (*ir.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path, ".dot") {
		return migrate.Migrate(string(data))
	}
	p := parser.NewParser(string(data), path)
	return p.Parse()
}

// renderError formats a Go error as a diagnostic and writes it to stderr.
// In JSON mode it wraps the error in the spec §12 JSON diagnostic format.
func (c *CLI) renderError(err error, path string) {
	if c.Format == FormatJSON {
		diag := validator.Diagnostic{
			Code:     "DIP000",
			Severity: validator.SeverityError,
			Message:  err.Error(),
			Location: ir.SourceLocation{File: path},
		}
		c.renderDiagnostics([]validator.Diagnostic{diag})
	} else {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
	}
}

// renderDiagnostics outputs diagnostics to stderr in either text or JSON format.
// The JSON format matches spec §12 exactly.
func (c *CLI) renderDiagnostics(diags []validator.Diagnostic) {
	if len(diags) == 0 {
		return
	}

	if c.Format == FormatJSON {
		renderDiagnosticsJSON(c.Stderr, diags)
	} else {
		renderDiagnosticsText(c.Stderr, diags)
	}
}

// renderDiagnosticsText writes diagnostics in the spec §12 text format:
//
//	error[DIP003]: unknown node reference "InterpretX" in edge
//	  --> pipeline.dip:45:5
//	  = help: did you mean "Interpret"?
func renderDiagnosticsText(w io.Writer, diags []validator.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(w, d.String())
	}
}

// renderDiagnosticsJSON writes diagnostics as a JSON array matching the spec §12
// machine-readable format.
func renderDiagnosticsJSON(w io.Writer, diags []validator.Diagnostic) {
	type jsonLoc struct {
		File      string `json:"file"`
		Line      int    `json:"line"`
		Column    int    `json:"column"`
		EndLine   int    `json:"end_line"`
		EndColumn int    `json:"end_column"`
	}
	type jsonDiag struct {
		Code     string  `json:"code"`
		Severity string  `json:"severity"`
		Message  string  `json:"message"`
		Location jsonLoc `json:"location"`
		Help     string  `json:"help,omitempty"`
		Fix      string  `json:"fix,omitempty"`
	}

	out := make([]jsonDiag, 0, len(diags))
	for _, d := range diags {
		out = append(out, jsonDiag{
			Code:     d.Code,
			Severity: d.Severity.String(),
			Message:  d.Message,
			Location: jsonLoc{
				File:      d.Location.File,
				Line:      d.Location.Line,
				Column:    d.Location.Column,
				EndLine:   d.Location.EndLine,
				EndColumn: d.Location.EndColumn,
			},
			Help: d.Help,
			Fix:  d.Fix,
		})
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(b))
}
