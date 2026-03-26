package validator

import (
	"os/exec"
	"regexp"
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

// ctxVarPattern matches ${ctx.*} references in tool commands.
var ctxVarPattern = regexp.MustCompile(`\$\{ctx\.[^}]+\}`)

// lintToolCtxVars flags ${ctx.*} references in tool commands that won't
// resolve at parse time. DIP124: tool command references runtime variable.
func lintToolCtxVars(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		diags = append(diags, checkToolCtxVars(n)...)
	}
	return diags
}

// checkToolCtxVars checks a single tool node for ${ctx.*} references.
func checkToolCtxVars(n *ir.Node) []Diagnostic {
	cfg, ok := n.Config.(ir.ToolConfig)
	if !ok || cfg.Command == "" {
		return nil
	}
	matches := ctxVarPattern.FindAllString(cfg.Command, -1)
	if len(matches) == 0 {
		return nil
	}
	var diags []Diagnostic
	for _, m := range matches {
		diags = append(diags, Diagnostic{
			Code:     DIP124,
			Severity: SeverityWarning,
			Message:  "tool command references " + m + " which expands to empty at runtime",
			Location: n.Source,
		})
	}
	return diags
}

// shellPreamble patterns that should be skipped when finding the binary.
var shellPreamble = regexp.MustCompile(
	`^(set\s+-\w+|cd\s+\S+|export\s+\S+|mkdir\s+-p\s+\S+|#.*)$`,
)

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

// extractBinary finds the first non-preamble command's binary name.
func extractBinary(command string) string {
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || shellPreamble.MatchString(line) {
			continue
		}
		return firstToken(line)
	}
	return ""
}

// firstToken returns the first whitespace-delimited token of a line.
func firstToken(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// shellBuiltins are commands handled by the shell, not found on PATH.
var shellBuiltins = map[string]bool{
	"echo": true, "printf": true, "test": true, "[": true,
	"if": true, "then": true, "else": true, "fi": true,
	"for": true, "while": true, "do": true, "done": true,
	"case": true, "esac": true, "read": true, "eval": true,
	"exec": true, "exit": true, "return": true, "shift": true,
	"trap": true, "wait": true, "true": true, "false": true,
	"source": true, ".": true, "local": true, "declare": true,
}

// isShellBuiltin returns true if the command is a shell builtin.
func isShellBuiltin(cmd string) bool {
	return shellBuiltins[cmd]
}

// strQuote wraps a string in double quotes for diagnostic messages.
func strQuote(s string) string {
	return "\"" + s + "\""
}
