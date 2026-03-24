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

// commandDispatch maps subcommand names to their handler methods.
func (c *CLI) commandDispatch() map[string]func([]string) ExitCode {
	return map[string]func([]string) ExitCode{
		"parse":              c.CmdParse,
		"validate":           c.CmdValidate,
		"lint":               c.CmdLint,
		"check":              c.CmdCheck,
		"fmt":                c.CmdFmt,
		"new":                c.CmdNew,
		"export-dot":         c.CmdExportDOT,
		"migrate":            c.CmdMigrate,
		"validate-migration": c.CmdValidateMigration,
		"simulate":           c.CmdSimulate,
	}
}

// resolveFormat maps a format string to an OutputFormat value.
// Returns false if the format string is unknown.
func resolveFormat(s string) (OutputFormat, bool) {
	formats := map[string]OutputFormat{
		"text": FormatText,
		"json": FormatJSON,
		"dot":  FormatDOT,
	}
	f, ok := formats[s]
	return f, ok
}

// parseGlobalFlags parses global flags from args and returns the CLI, remaining
// args, and an exit code. If the exit code is not -1, the caller should return it.
func parseGlobalFlags(args []string, stdout, stderr io.Writer) (*CLI, []string, ExitCode) {
	c := &CLI{
		Stdout: stdout,
		Stderr: stderr,
		Format: FormatText,
	}

	globalFlags := flag.NewFlagSet("dippin", flag.ContinueOnError)
	globalFlags.SetOutput(stderr)
	formatStr := globalFlags.String("format", "text", "output format (text|json)")

	if err := globalFlags.Parse(args); err != nil {
		return nil, nil, ExitUsageError
	}

	f, ok := resolveFormat(*formatStr)
	if !ok {
		fmt.Fprintf(stderr, "unknown format: %s\n", *formatStr)
		return nil, nil, ExitUsageError
	}
	c.Format = f

	remaining := globalFlags.Args()
	if len(remaining) == 0 {
		printGlobalUsage(stderr)
		return nil, nil, ExitUsageError
	}

	return c, remaining, ExitCode(-1)
}

// Run is the testable entry point for the entire CLI. It accepts raw args
// (without the program name), captures output into the provided writers,
// and returns a deterministic exit code.
func Run(args []string, stdout, stderr io.Writer) ExitCode {
	if len(args) == 0 {
		printGlobalUsage(stderr)
		return ExitUsageError
	}

	c, remaining, code := parseGlobalFlags(args, stdout, stderr)
	if code != ExitCode(-1) {
		return code
	}

	return c.dispatch(remaining, stdout, stderr)
}

func (c *CLI) dispatch(remaining []string, stdout, stderr io.Writer) ExitCode {
	cmd := remaining[0]
	cmdArgs := remaining[1:]

	if cmd == "help" {
		printGlobalUsage(stdout)
		return ExitOK
	}

	if cmd == "version" {
		fmt.Fprintf(stdout, "dippin %s (commit: %s, built: %s)\n", version, commit, date)
		return ExitOK
	}

	handler, ok := c.commandDispatch()[cmd]
	if !ok {
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		return ExitUsageError
	}
	return handler(cmdArgs)
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
	fmt.Fprintln(w, "  version                           Show version info")
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
	path, code := parseSingleFileArg("validate", "usage: dippin validate <file>", args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	res := validator.Validate(w)
	c.renderDiagnostics(res.Diagnostics)

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
	path, code := parseSingleFileArg("lint", "usage: dippin lint <file>", args, c.Stderr)
	if code != ExitCode(-1) {
		return code
	}

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
	c.renderDiagnostics(allDiags)

	// Exit 1 only if there are errors; warnings alone pass.
	if valRes.HasErrors() {
		return ExitError
	}
	return ExitOK
}

// checkReportError reports an error in the appropriate format for the check command.
func checkReportError(stdout io.Writer, stderr io.Writer, formatStr string, errMsg string) {
	if formatStr == "json" {
		renderCheckJSON(stdout, false, nil, errMsg)
	} else {
		fmt.Fprintf(stderr, "error: %v\n", errMsg)
	}
}

// renderCheckTextResult renders check results in text format.
func renderCheckTextResult(w io.Writer, diags []validator.Diagnostic) {
	if len(diags) > 0 {
		renderDiagnosticsText(w, diags)
	} else {
		fmt.Fprintln(w, "check passed")
	}
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
	w, parseErr := parseFile(path)
	if parseErr != nil {
		checkReportError(c.Stdout, c.Stderr, *formatStr, parseErr.Error())
		return ExitError
	}

	return runCheckPipeline(c.Stdout, *formatStr, w)
}

// runCheckPipeline validates, lints, and renders the check result.
func runCheckPipeline(stdout io.Writer, formatStr string, w *ir.Workflow) ExitCode {
	valRes := validator.Validate(w)
	lintRes := validator.Lint(w)
	allDiags := append(valRes.Diagnostics, lintRes.Diagnostics...)

	if formatStr == "json" {
		renderCheckJSON(stdout, !valRes.HasErrors(), allDiags, "")
	} else {
		renderCheckTextResult(stdout, allDiags)
	}

	if valRes.HasErrors() {
		return ExitError
	}
	return ExitOK
}

// checkDiag is the JSON representation of a single diagnostic in check output.
type checkDiag struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

// checkResult is the JSON representation of the check command output.
type checkResult struct {
	Valid            bool        `json:"valid"`
	Errors           int         `json:"errors"`
	Warnings         int         `json:"warnings"`
	Diagnostics      []checkDiag `json:"diagnostics"`
	SuggestedActions []string    `json:"suggested_actions"`
}

// toCheckDiag converts a validator diagnostic to a checkDiag.
func toCheckDiag(d validator.Diagnostic) checkDiag {
	return checkDiag{
		Code:     d.Code,
		Severity: d.Severity.String(),
		Message:  d.Message,
		Line:     d.Location.Line,
		Fix:      d.Fix,
	}
}

// countSeverities counts errors and warnings in a slice of diagnostics.
func countSeverities(diags []validator.Diagnostic) (int, int) {
	var errors, warnings int
	for _, d := range diags {
		switch d.Severity {
		case validator.SeverityError:
			errors++
		case validator.SeverityWarning:
			warnings++
		}
	}
	return errors, warnings
}

// collectUniqueFixes returns unique fix suggestions from diagnostics.
func collectUniqueFixes(diags []validator.Diagnostic) []string {
	var actions []string
	seen := make(map[string]bool)
	for _, d := range diags {
		if d.Fix != "" && !seen[d.Fix] {
			seen[d.Fix] = true
			actions = append(actions, d.Fix)
		}
	}
	return actions
}

// aggregateCheckDiags converts validator diagnostics into check output fields,
// returning the converted diagnostics, error/warning counts, and unique suggested actions.
func aggregateCheckDiags(diags []validator.Diagnostic) ([]checkDiag, int, int, []string) {
	cds := make([]checkDiag, 0, len(diags))
	for _, d := range diags {
		cds = append(cds, toCheckDiag(d))
	}
	errors, warnings := countSeverities(diags)
	actions := collectUniqueFixes(diags)
	return cds, errors, warnings, actions
}

// renderCheckJSON writes the compact check output to w.
func renderCheckJSON(w io.Writer, valid bool, diags []validator.Diagnostic, parseErr string) {
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

	cds, errors, warnings, actions := aggregateCheckDiags(diags)
	res.Diagnostics = append(res.Diagnostics, cds...)
	res.Errors += errors
	res.Warnings += warnings
	res.SuggestedActions = actions

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

	return writeOutput(c.Stdout, c.Stderr, *writePath, formatter.Format(w))
}

// writeOutput writes content to a file path if given, otherwise to stdout.
func writeOutput(stdout, stderr io.Writer, path, content string) ExitCode {
	if path != "" {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(stderr, "error writing file: %v\n", err)
			return ExitError
		}
		return ExitOK
	}
	fmt.Fprint(stdout, content)
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
	data, formatted, code := c.parseAndFormat(path)
	if code != ExitCode(-1) {
		return code
	}

	if *check {
		return c.fmtCheck(path, string(data), formatted)
	}

	return writeOutput(c.Stdout, c.Stderr, boolToPath(*write, path), formatted)
}

// boolToPath returns path if cond is true, otherwise empty string.
func boolToPath(cond bool, path string) string {
	if cond {
		return path
	}
	return ""
}

// parseAndFormat reads and parses a file, returning the raw data, formatted
// output, and an exit code (ExitCode(-1) means success, continue processing).
func (c *CLI) parseAndFormat(path string) ([]byte, string, ExitCode) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(c.Stderr, "error: %v\n", err)
		return nil, "", ExitError
	}

	p := parser.NewParser(string(data), path)
	w, err := p.Parse()
	if err != nil {
		c.renderError(err, path)
		return nil, "", ExitError
	}

	return data, formatter.Format(w), ExitCode(-1)
}

// fmtCheck compares formatted output against original data and returns
// the appropriate exit code for --check mode.
func (c *CLI) fmtCheck(path, original, formatted string) ExitCode {
	if formatted != original {
		if c.Format == FormatText {
			fmt.Fprintf(c.Stderr, "%s: not canonically formatted\n", path)
		}
		return ExitError
	}
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

	args = reorderSimulateArgs(args)

	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin simulate [--scenario key=val] [--interactive] [--all-paths] <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, opts, code := c.prepareSimulation(path, scenarios.values, *interactive, *allPaths)
	if code != ExitCode(-1) {
		return code
	}

	if *allPaths {
		return c.simulateAllPaths(w, opts)
	}
	return c.simulateSingle(w, opts)
}

// prepareSimulation loads and validates the workflow, then builds simulation options.
func (c *CLI) prepareSimulation(path string, scenario map[string]string, interactive, allPaths bool) (*ir.Workflow, simulate.Options, ExitCode) {
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return nil, simulate.Options{}, ExitError
	}

	valRes := validator.Validate(w)
	if valRes.HasErrors() {
		c.renderDiagnostics(valRes.Diagnostics)
		return nil, simulate.Options{}, ExitError
	}

	opts := simulate.Options{
		Scenario:    scenario,
		Interactive: interactive,
		AllPaths:    allPaths,
	}
	if interactive {
		opts.Stdin = os.Stdin
		opts.Stderr = c.Stderr
	}

	return w, opts, ExitCode(-1)
}

// emitPathResult emits a single simulation path result (separator, header, JSONL).
func (c *CLI) emitPathResult(i int, res *simulate.Result) error {
	if i > 0 {
		fmt.Fprintln(c.Stdout)
	}
	if c.Format == FormatText {
		fmt.Fprintf(c.Stderr, "--- path %d: %s (%d nodes: %s) ---\n",
			i+1, res.Status, res.NodesVisited, strings.Join(res.Path, " → "))
	}
	return simulate.EmitJSONL(c.Stdout, res.Events)
}

// simulateAllPaths runs simulation in all-paths mode and renders results.
func (c *CLI) simulateAllPaths(w *ir.Workflow, opts simulate.Options) ExitCode {
	results, err := simulate.RunAllPaths(w, opts)
	if err != nil {
		fmt.Fprintf(c.Stderr, "simulation error: %v\n", err)
		return ExitError
	}

	for i, res := range results {
		if err := c.emitPathResult(i, res); err != nil {
			fmt.Fprintf(c.Stderr, "output error: %v\n", err)
			return ExitError
		}
	}

	if c.Format == FormatText {
		fmt.Fprintf(c.Stderr, "\n%d path(s) enumerated\n", len(results))
	}
	return ExitOK
}

// simulateSingle runs a single simulation path and renders the result.
func (c *CLI) simulateSingle(w *ir.Workflow, opts simulate.Options) ExitCode {
	res, err := simulate.Run(w, opts)
	if err != nil {
		fmt.Fprintf(c.Stderr, "simulation error: %v\n", err)
		return ExitError
	}

	if c.Format == FormatDOT {
		return c.renderSimulateDOT(w, res)
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

// renderSimulateDOT renders simulation results as a DOT graph.
func (c *CLI) renderSimulateDOT(w *ir.Workflow, res *simulate.Result) ExitCode {
	dotOpts := export.ExportOptions{
		ExecutionPath: res.Path,
	}
	dot := export.ExportDOT(w, dotOpts)
	fmt.Fprint(c.Stdout, dot)
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

// isValueFlag returns true if the flag name requires a separate value argument.
func isValueFlag(name string) bool {
	return name == "scenario"
}

// hasFollowingValue returns true if a value-taking flag at position i has a
// non-flag value following it.
func hasFollowingValue(args []string, i int) bool {
	return i+1 < len(args) && !strings.HasPrefix(args[i+1], "-")
}

// classifyArg appends a single arg (and possibly its value) to the flags or
// positional slices. It returns the number of args consumed (1 or 2).
func classifyArg(args []string, i int, flags, positional *[]string) int {
	arg := args[i]
	if !strings.HasPrefix(arg, "-") {
		*positional = append(*positional, arg)
		return 1
	}

	if strings.Contains(arg, "=") {
		*flags = append(*flags, arg)
		return 1
	}

	flagName := strings.TrimLeft(arg, "-")
	if isValueFlag(flagName) && hasFollowingValue(args, i) {
		*flags = append(*flags, arg, args[i+1])
		return 2
	}

	*flags = append(*flags, arg)
	return 1
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
	for i := 0; i < len(args); {
		i += classifyArg(args, i, &flags, &positional)
	}
	return append(flags, positional...)
}

// parseSingleFileArg is a helper for commands that take a single file argument
// with no extra flags. Returns the file path and ExitCode(-1) on success, or
// empty string and an error exit code on failure.
func parseSingleFileArg(name, usage string, args []string, stderr io.Writer) (string, ExitCode) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return "", ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, usage)
		return "", ExitUsageError
	}
	return fs.Arg(0), ExitCode(-1)
}

// parseFile reads and parses a .dip file, returning the workflow or an error.
func parseFile(path string) (*ir.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := parser.NewParser(string(data), path)
	return p.Parse()
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
