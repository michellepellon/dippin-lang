package validator

import (
	"regexp"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
	"mvdan.cc/sh/v3/syntax"
)

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

// extractBinary parses a shell command and returns the first non-builtin,
// non-preamble command name. Uses a proper shell AST parser to correctly
// handle variable assignments, pipes, subshells, command substitution, etc.
// Shell builtins (set, cd, echo, etc.) and preamble commands (mkdir, touch,
// etc.) are skipped to find the primary tool binary.
func extractBinary(command string) string {
	parser := syntax.NewParser(syntax.KeepComments(false))
	prog, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return ""
	}
	var bin string
	syntax.Walk(prog, walkForBinary(&bin))
	return bin
}

// walkForBinary returns a walk function that captures the first non-builtin
// command binary into bin.
func walkForBinary(bin *string) func(syntax.Node) bool {
	return func(node syntax.Node) bool {
		if *bin != "" {
			return false
		}
		name := callExprBinary(node)
		if name != "" && !isSkippableCommand(name) {
			*bin = name
			return false
		}
		return true
	}
}

// callExprBinary returns the literal binary name of a CallExpr node,
// or "" if the node is not a non-empty CallExpr with a literal first arg.
func callExprBinary(node syntax.Node) string {
	call, ok := node.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return ""
	}
	return extractWordLiteral(call.Args[0])
}

// extractWordLiteral returns the literal string of a simple Word,
// or "" if it contains expansions/substitutions.
func extractWordLiteral(w *syntax.Word) string {
	if len(w.Parts) != 1 {
		return ""
	}
	lit, ok := w.Parts[0].(*syntax.Lit)
	if !ok {
		return ""
	}
	return lit.Value
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
	"set": true, "cd": true, "export": true, "unset": true,
	"command": true,
}

// preambleCommands are external binaries commonly used for setup that
// should be skipped when finding the "real" tool binary in a command block.
var preambleCommands = map[string]bool{
	"mkdir": true, "touch": true, "chmod": true, "rm": true, "cp": true, "mv": true,
}

// isShellBuiltin returns true if the command is a shell builtin.
func isShellBuiltin(cmd string) bool {
	return shellBuiltins[cmd]
}

// isSkippableCommand returns true if the command should be skipped
// when looking for the primary tool binary (builtins + preamble).
func isSkippableCommand(cmd string) bool {
	return shellBuiltins[cmd] || preambleCommands[cmd]
}

// strQuote wraps a string in double quotes for diagnostic messages.
func strQuote(s string) string {
	return "\"" + s + "\""
}
