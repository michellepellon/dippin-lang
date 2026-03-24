package main

import (
	"fmt"

	"github.com/2389-research/dippin-lang/validator"
)

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
