//go:build !wasm

package validator

import (
	"os/exec"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// lintToolSyntax runs bash -n on each tool command to catch syntax errors.
// DIP123: tool command has shell syntax errors.
func lintToolSyntax(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if d := checkToolSyntax(n); d != nil {
			diags = append(diags, *d)
		}
	}
	return diags
}

// checkToolSyntax runs bash -n on a single tool node's command.
func checkToolSyntax(n *ir.Node) *Diagnostic {
	cfg, ok := n.Config.(ir.ToolConfig)
	if !ok || cfg.Command == "" {
		return nil
	}
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(cfg.Command)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := extractBashError(string(out))
	return &Diagnostic{
		Code:     DIP123,
		Severity: SeverityWarning,
		Message:  "tool command has shell syntax error: " + msg,
		Location: n.Source,
	}
}

// extractBashError returns a concise error from bash -n output.
func extractBashError(output string) string {
	output = strings.TrimSpace(output)
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return "syntax error"
}

// lintToolBinary checks if the first real command in each tool node is
// on PATH. DIP125: tool command binary not found.
func lintToolBinary(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if d := checkToolBinary(n); d != nil {
			diags = append(diags, *d)
		}
	}
	return diags
}

// checkToolBinary extracts the first real binary from a tool command.
func checkToolBinary(n *ir.Node) *Diagnostic {
	cfg, ok := n.Config.(ir.ToolConfig)
	if !ok || cfg.Command == "" {
		return nil
	}
	bin := extractBinary(cfg.Command)
	if bin == "" || isShellBuiltin(bin) {
		return nil
	}
	return lookupBinary(bin, n.Source)
}

// lookupBinary checks if a binary exists on PATH.
func lookupBinary(bin string, loc ir.SourceLocation) *Diagnostic {
	if _, err := exec.LookPath(bin); err != nil {
		return &Diagnostic{
			Code:     DIP125,
			Severity: SeverityHint,
			Message:  "tool command binary " + strQuote(bin) + " not found on PATH",
			Location: loc,
		}
	}
	return nil
}
