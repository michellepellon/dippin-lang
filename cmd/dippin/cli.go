package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/2389-research/dippin-lang/export"
	"github.com/2389-research/dippin-lang/formatter"
	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/migrate"
	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/scaffold"
	"github.com/2389-research/dippin-lang/simulate"
	"github.com/2389-research/dippin-lang/validator"
)

// OutputFormat controls whether diagnostics are rendered as human-readable text
// or machine-readable JSON (spec §12).
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
	FormatDOT
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
	case "dot":
		c.Format = FormatDOT
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
	case "check":
		return c.CmdCheck(cmdArgs)
	case "fmt":
		return c.CmdFmt(cmdArgs)
	case "new":
		return c.CmdNew(cmdArgs)
	case "export-dot":
		return c.CmdExportDOT(cmdArgs)
	case "migrate":
		return c.CmdMigrate(cmdArgs)
	case "validate-migration":
		return c.CmdValidateMigration(cmdArgs)
	case "simulate":
		return c.CmdSimulate(cmdArgs)
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
	fmt.Fprintln(w, "  check [--format text|json] <file> Parse+validate+lint (JSON default, for LLM tooling)")
	fmt.Fprintln(w, "  fmt [--check] [--write] <file>    Format a .dip file")
	fmt.Fprintln(w, "  new [--name N] [--write F] <template>")
	fmt.Fprintln(w, "                                    Generate a starter .dip file")
	fmt.Fprintln(w, "                                    Templates: minimal, parallel, conditional,")
	fmt.Fprintln(w, "                                               review-loop, human-gate")
	fmt.Fprintln(w, "  export-dot [--rankdir] [--prompts] <file>")
	fmt.Fprintln(w, "                                    Export workflow to DOT format")
	fmt.Fprintln(w, "  migrate [--output <file>] <file.dot>")
	fmt.Fprintln(w, "                                    Convert DOT to .dip")
	fmt.Fprintln(w, "  validate-migration <old.dot> <new.dip>")
	fmt.Fprintln(w, "                                    Check parity between DOT and .dip")
	fmt.Fprintln(w, "  simulate [flags] <file>           Simulate workflow execution (JSONL events)")
	fmt.Fprintln(w, "                                    --scenario key=val  Inject context values")
	fmt.Fprintln(w, "                                    --interactive       Prompt at human nodes")
	fmt.Fprintln(w, "                                    --all-paths         Enumerate all paths")
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

// CmdCheck runs parse + validate + lint in one shot and outputs a compact
// JSON summary to stdout. Designed for LLM tool-calling loops.
// CmdCheck parses its own --format flag, defaulting to JSON (unlike other commands).
func (c *CLI) CmdCheck(args []string) ExitCode {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	formatStr := fs.String("format", "json", "output format (json|text)")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin check [--format json|text] <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)

	// Parse.
	data, err := os.ReadFile(path)
	if err != nil {
		if *formatStr == "json" {
			renderCheckJSON(c.Stdout, false, nil, err.Error())
		} else {
			fmt.Fprintf(c.Stderr, "error: %v\n", err)
		}
		return ExitError
	}

	p := parser.NewParser(string(data), path)
	w, err := p.Parse()
	if err != nil {
		if *formatStr == "json" {
			renderCheckJSON(c.Stdout, false, nil, err.Error())
		} else {
			fmt.Fprintf(c.Stderr, "error: %v\n", err)
		}
		return ExitError
	}

	// Validate + lint.
	valRes := validator.Validate(w)
	lintRes := validator.Lint(w)
	allDiags := append(valRes.Diagnostics, lintRes.Diagnostics...)

	if *formatStr == "json" {
		renderCheckJSON(c.Stdout, !valRes.HasErrors(), allDiags, "")
	} else {
		if len(allDiags) > 0 {
			renderDiagnosticsText(c.Stdout, allDiags)
		} else {
			fmt.Fprintln(c.Stdout, "check passed")
		}
	}

	if valRes.HasErrors() {
		return ExitError
	}
	return ExitOK
}

// renderCheckJSON writes the compact check output to w.
func renderCheckJSON(w io.Writer, valid bool, diags []validator.Diagnostic, parseErr string) {
	type checkDiag struct {
		Code     string `json:"code"`
		Severity string `json:"severity"`
		Message  string `json:"message"`
		Line     int    `json:"line,omitempty"`
		Fix      string `json:"fix,omitempty"`
	}
	type checkResult struct {
		Valid            bool        `json:"valid"`
		Errors           int         `json:"errors"`
		Warnings         int         `json:"warnings"`
		Diagnostics      []checkDiag `json:"diagnostics"`
		SuggestedActions []string    `json:"suggested_actions"`
	}

	res := checkResult{
		Valid:       valid,
		Diagnostics: make([]checkDiag, 0, len(diags)),
	}

	if parseErr != "" {
		res.Errors = 1
		res.Diagnostics = append(res.Diagnostics, checkDiag{
			Code:     "DIP000",
			Severity: "error",
			Message:  parseErr,
		})
	}

	seen := make(map[string]bool)
	for _, d := range diags {
		cd := checkDiag{
			Code:     d.Code,
			Severity: d.Severity.String(),
			Message:  d.Message,
			Line:     d.Location.Line,
			Fix:      d.Fix,
		}
		res.Diagnostics = append(res.Diagnostics, cd)

		if d.Severity == validator.SeverityError {
			res.Errors++
		} else if d.Severity == validator.SeverityWarning {
			res.Warnings++
		}

		if d.Fix != "" && !seen[d.Fix] {
			seen[d.Fix] = true
			res.SuggestedActions = append(res.SuggestedActions, d.Fix)
		}
	}

	if res.SuggestedActions == nil {
		res.SuggestedActions = []string{}
	}

	b, _ := json.Marshal(res)
	fmt.Fprintln(w, string(b))
}

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

	source := formatter.Format(w)

	if *writePath != "" {
		if err := os.WriteFile(*writePath, []byte(source), 0644); err != nil {
			fmt.Fprintf(c.Stderr, "error writing file: %v\n", err)
			return ExitError
		}
		return ExitOK
	}

	fmt.Fprint(c.Stdout, source)
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

// CmdSimulate runs a dry-run simulation of a workflow, emitting JSONL events.
//
//   - Default: happy path (all success), output JSONL to stdout
//   - --scenario key=val: inject context values to explore different paths
//   - --interactive: prompt at human nodes via stdin
//   - --all-paths: enumerate all possible execution paths
func (c *CLI) CmdSimulate(args []string) ExitCode {
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)

	var scenarios scenarioFlags
	fs.Var(&scenarios, "scenario", "inject context value (key=val), repeatable")
	interactive := fs.Bool("interactive", false, "prompt at human nodes")
	allPaths := fs.Bool("all-paths", false, "enumerate all possible execution paths")

	// Reorder args so flags can appear before or after the filename.
	// This supports both:
	//   dippin simulate file.dip --scenario key=val
	//   dippin simulate --scenario key=val file.dip
	args = reorderSimulateArgs(args)

	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin simulate [--scenario key=val] [--interactive] [--all-paths] <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	// Validate workflow before simulating.
	valRes := validator.Validate(w)
	if valRes.HasErrors() {
		c.renderDiagnostics(valRes.Diagnostics)
		return ExitError
	}

	opts := simulate.Options{
		Scenario:    scenarios.values,
		Interactive: *interactive,
		AllPaths:    *allPaths,
	}

	if *interactive {
		opts.Stdin = os.Stdin
		opts.Stderr = c.Stderr
	}

	if *allPaths {
		results, err := simulate.RunAllPaths(w, opts)
		if err != nil {
			fmt.Fprintf(c.Stderr, "simulation error: %v\n", err)
			return ExitError
		}

		for i, res := range results {
			if i > 0 {
				// Separator between paths.
				fmt.Fprintln(c.Stdout)
			}
			if c.Format == FormatText {
				fmt.Fprintf(c.Stderr, "--- path %d: %s (%d nodes: %s) ---\n",
					i+1, res.Status, res.NodesVisited, strings.Join(res.Path, " → "))
			}
			if err := simulate.EmitJSONL(c.Stdout, res.Events); err != nil {
				fmt.Fprintf(c.Stderr, "output error: %v\n", err)
				return ExitError
			}
		}

		if c.Format == FormatText {
			fmt.Fprintf(c.Stderr, "\n%d path(s) enumerated\n", len(results))
		}
		return ExitOK
	}

	res, err := simulate.Run(w, opts)
	if err != nil {
		fmt.Fprintf(c.Stderr, "simulation error: %v\n", err)
		return ExitError
	}

	if c.Format == FormatDOT {
		dotOpts := export.ExportOptions{
			ExecutionPath: res.Path,
		}
		dot := export.ExportDOT(w, dotOpts)
		fmt.Fprint(c.Stdout, dot)
		return ExitOK
	}

	if err := simulate.EmitJSONL(c.Stdout, res.Events); err != nil {
		fmt.Fprintf(c.Stderr, "output error: %v\n", err)
		return ExitError
	}

	if c.Format == FormatText {
		fmt.Fprintf(c.Stderr, "simulation complete: %s (%d nodes visited)\n", res.Status, res.NodesVisited)
		fmt.Fprintf(c.Stderr, "path: %s\n", strings.Join(res.Path, " → "))
	}

	return ExitOK
}

// scenarioFlags implements flag.Value for repeatable --scenario key=val flags.
type scenarioFlags struct {
	values map[string]string
}

func (s *scenarioFlags) String() string {
	if s.values == nil {
		return ""
	}
	parts := make([]string, 0, len(s.values))
	for k, v := range s.values {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func (s *scenarioFlags) Set(val string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	parts := strings.SplitN(val, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("scenario must be key=value, got %q", val)
	}
	s.values[parts[0]] = parts[1]
	return nil
}

// reorderSimulateArgs moves any flag arguments that appear after a positional
// argument (the .dip file path) to before it. This allows natural CLI syntax:
//
//	dippin simulate file.dip --scenario key=val --all-paths
//
// Standard flag.FlagSet stops parsing at the first non-flag argument, so we
// partition args into flags and non-flags, then recombine as [flags... files...].
func reorderSimulateArgs(args []string) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// It's a flag. Check if it takes a value (--flag value or --flag=value).
			if strings.Contains(arg, "=") {
				// --flag=value form
				flags = append(flags, arg)
			} else {
				// Could be --flag value or a boolean flag.
				// Known value-taking flags for simulate: --scenario
				flagName := strings.TrimLeft(arg, "-")
				if flagName == "scenario" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					flags = append(flags, arg, args[i+1])
					i++
				} else {
					flags = append(flags, arg)
				}
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
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
