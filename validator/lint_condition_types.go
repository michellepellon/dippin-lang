package validator

import (
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// reservedVarPrefixes lists variable prefixes that are always available
// at runtime and should not trigger DIP121 warnings.
var reservedVarPrefixes = []string{
	"ctx.outcome",
	"ctx.status",
	"ctx.tool_stdout",
	"ctx.tool_stderr",
	"ctx.tool_marker",
	"ctx.internal.",
	"graph.",
	"params.",
}

// isReservedVariable returns true if the variable matches a reserved runtime key.
func isReservedVariable(v string) bool {
	for _, prefix := range reservedVarPrefixes {
		if v == prefix || strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

// stripNamespace removes the namespace prefix from a variable name.
// "ctx.varname" → "varname", "varname" → "varname".
func stripNamespace(v string) string {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return v
}

// lintConditionUndefinedOutput checks DIP121: edge condition references a
// variable not produced by the source node's IO.Writes.
func lintConditionUndefinedOutput(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, e := range w.Edges {
		if e.Condition == nil || e.Condition.Parsed == nil {
			continue
		}
		diags = append(diags, checkEdgeVarWrites(w, e)...)
	}
	return diags
}

// checkEdgeVarWrites checks one edge's condition variables against source node writes.
func checkEdgeVarWrites(w *ir.Workflow, e *ir.Edge) []Diagnostic {
	src := w.Node(e.From)
	if src == nil || len(src.IO.Writes) == 0 {
		return nil
	}

	writeSet := make(map[string]bool, len(src.IO.Writes))
	for _, key := range src.IO.Writes {
		writeSet[key] = true
	}

	comparisons := extractComparisons(e.Condition.Parsed)
	var diags []Diagnostic
	for _, cmp := range comparisons {
		diags = append(diags, checkOneVarWrite(e, cmp, writeSet)...)
	}
	return diags
}

// checkOneVarWrite checks a single comparison variable against the write set.
func checkOneVarWrite(e *ir.Edge, cmp ir.CondCompare, writeSet map[string]bool) []Diagnostic {
	if isReservedVariable(cmp.Variable) {
		return nil
	}
	bareKey := stripNamespace(cmp.Variable)
	if writeSet[bareKey] {
		return nil
	}
	return []Diagnostic{{
		Code:     DIP121,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("edge %s → %s: condition references %q but node %q does not declare it in writes", e.From, e.To, cmp.Variable, e.From),
		Location: e.Source,
		Help:     fmt.Sprintf("add writes: %s to node %q, or use a reserved variable", bareKey, e.From),
	}}
}

// lintConditionUndeclaredValue checks DIP122: edge condition tests a value
// not in the source tool's ToolConfig.Outputs.
func lintConditionUndeclaredValue(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, e := range w.Edges {
		if e.Condition == nil || e.Condition.Parsed == nil {
			continue
		}
		diags = append(diags, checkEdgeValueOutputs(w, e)...)
	}
	return diags
}

// checkEdgeValueOutputs checks one edge's condition values against source tool outputs.
func checkEdgeValueOutputs(w *ir.Workflow, e *ir.Edge) []Diagnostic {
	outputSet := toolOutputSet(w.Node(e.From))
	if outputSet == nil {
		return nil
	}

	comparisons := extractComparisons(e.Condition.Parsed)
	var diags []Diagnostic
	for _, cmp := range comparisons {
		diags = append(diags, checkOneValueOutput(e, cmp, outputSet)...)
	}
	return diags
}

// toolOutputSet returns the output set for a tool node, or nil if not applicable.
func toolOutputSet(src *ir.Node) map[string]bool {
	if src == nil {
		return nil
	}
	cfg, ok := src.Config.(ir.ToolConfig)
	if !ok || len(cfg.Outputs) == 0 {
		return nil
	}
	outputSet := make(map[string]bool, len(cfg.Outputs))
	for _, o := range cfg.Outputs {
		outputSet[o] = true
	}
	return outputSet
}

// checkOneValueOutput checks a single comparison value against the output set.
func checkOneValueOutput(e *ir.Edge, cmp ir.CondCompare, outputSet map[string]bool) []Diagnostic {
	if outputSet[cmp.Value] {
		return nil
	}
	return []Diagnostic{{
		Code:     DIP122,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("edge %s → %s: condition tests value %q but tool %q does not declare it in outputs", e.From, e.To, cmp.Value, e.From),
		Location: e.Source,
		Help:     fmt.Sprintf("add %q to tool %q outputs, or check for typos", cmp.Value, e.From),
	}}
}
