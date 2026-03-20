// Package export provides DOT graph format export for Dippin workflows.
//
// The primary function ExportDOT converts an ir.Workflow into a valid DOT
// language string suitable for rendering with Graphviz. The mapping is
// documented in §15 of the Dippin design spec.
package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389/dippin/ir"
)

// ExportOptions controls the DOT output format.
type ExportOptions struct {
	// IncludePrompts includes full prompt/command text as node attributes.
	// Default (zero value) is false; set to true for full-fidelity export.
	IncludePrompts bool

	// RankDir controls the graph layout direction: "LR" (left-to-right)
	// or "TB" (top-to-bottom). Defaults to "TB" when empty.
	RankDir string

	// HighlightGoalGates applies a distinct fill color to nodes with
	// GoalGate: true.
	HighlightGoalGates bool

	// ExecutionPath is an ordered list of node IDs visited during execution.
	// If set, nodes on the path are highlighted and numbered.
	ExecutionPath []string
}

// ExportDOT renders a workflow as a DOT language string.
// The output is a valid digraph that can be processed by Graphviz tools.
func ExportDOT(w *ir.Workflow, opts ExportOptions) string {
	var b strings.Builder

	rankDir := opts.RankDir
	if rankDir == "" {
		rankDir = "TB"
	}

	graphName := w.Name
	if graphName == "" {
		graphName = "workflow"
	}

	b.WriteString(fmt.Sprintf("digraph %s {\n", dotID(graphName)))
	b.WriteString(fmt.Sprintf("  rankdir=%s;\n", rankDir))
	b.WriteString("  node [fontname=\"Helvetica\"];\n")
	b.WriteString("  edge [fontname=\"Helvetica\"];\n")

	// Emit nodes.
	for _, n := range w.Nodes {
		writeNodeDOT(&b, n, w, opts)
	}

	b.WriteByte('\n')

	// Emit edges.
	for _, e := range w.Edges {
		writeEdgeDOT(&b, e)
	}

	b.WriteString("}\n")
	return b.String()
}

// nodeShape maps a NodeKind to the corresponding DOT shape attribute.
// Per §15: agent→box, human→hexagon, tool→parallelogram,
// parallel→component, fan_in→tripleoctagon, subgraph→tab.
// Start and exit nodes override to Mdiamond and Msquare respectively.
func nodeShape(kind ir.NodeKind) string {
	switch kind {
	case ir.NodeAgent:
		return "box"
	case ir.NodeHuman:
		return "hexagon"
	case ir.NodeTool:
		return "parallelogram"
	case ir.NodeParallel:
		return "component"
	case ir.NodeFanIn:
		return "tripleoctagon"
	case ir.NodeSubgraph:
		return "tab"
	default:
		return "box"
	}
}

// writeNodeDOT emits a single DOT node statement.
func writeNodeDOT(b *strings.Builder, n *ir.Node, w *ir.Workflow, opts ExportOptions) {
	attrs := make(map[string]string)

	// Build execution order map if path is provided.
	order := make(map[string][]int)
	for i, id := range opts.ExecutionPath {
		order[id] = append(order[id], i+1)
	}

	// Shape: start and exit override the kind-based shape.
	if n.ID == w.Start {
		attrs["shape"] = "Mdiamond"
	} else if n.ID == w.Exit {
		attrs["shape"] = "Msquare"
	} else {
		attrs["shape"] = nodeShape(n.Kind)
	}

	// Label: use the human-readable label if set, otherwise the node ID.
	label := n.Label
	if label == "" {
		label = n.ID
	}

	// Annotate label with execution order if part of the path.
	if ids, ok := order[n.ID]; ok {
		var orderStrs []string
		for _, idx := range ids {
			orderStrs = append(orderStrs, fmt.Sprintf("%d", idx))
		}
		label = fmt.Sprintf("[%s] %s", strings.Join(orderStrs, ","), label)
		attrs["style"] = "bold,filled"
		attrs["fillcolor"] = "#e0f0ff"
	}
	attrs["label"] = label

	// Goal gate highlighting.
	if opts.HighlightGoalGates {
		if ac, ok := n.Config.(ir.AgentConfig); ok && ac.GoalGate {
			attrs["style"] = "filled"
			attrs["fillcolor"] = "#ffcccc"
		}
	}

	// Include prompts/commands as attributes for full-fidelity export.
	if opts.IncludePrompts {
		switch cfg := n.Config.(type) {
		case ir.AgentConfig:
			if cfg.Prompt != "" {
				attrs["prompt"] = escapeNewlines(cfg.Prompt)
			}
			if cfg.Model != "" {
				attrs["model"] = cfg.Model
			}
			if cfg.Provider != "" {
				attrs["provider"] = cfg.Provider
			}
		case ir.ToolConfig:
			if cfg.Command != "" {
				attrs["tool_command"] = escapeNewlines(cfg.Command)
			}
			if cfg.Timeout != 0 {
				attrs["timeout"] = formatDuration(cfg.Timeout)
			}
		case ir.HumanConfig:
			if cfg.Mode != "" {
				attrs["mode"] = cfg.Mode
			}
			if cfg.Default != "" {
				attrs["default"] = cfg.Default
			}
		case ir.SubgraphConfig:
			if cfg.Ref != "" {
				attrs["ref"] = cfg.Ref
			}
		case ir.ParallelConfig:
			if len(cfg.Targets) > 0 {
				attrs["targets"] = strings.Join(cfg.Targets, ",")
			}
		case ir.FanInConfig:
			if len(cfg.Sources) > 0 {
				attrs["sources"] = strings.Join(cfg.Sources, ",")
			}
		}
	}

	b.WriteString(fmt.Sprintf("  %s %s;\n", dotID(n.ID), formatDOTAttrs(attrs)))
}

// writeEdgeDOT emits a single DOT edge statement.
func writeEdgeDOT(b *strings.Builder, e *ir.Edge) {
	attrs := make(map[string]string)

	if e.Label != "" {
		attrs["label"] = e.Label
	}

	if e.Condition != nil && e.Condition.Parsed != nil {
		condStr := formatCondition(e.Condition.Parsed)
		// If there's no separate label, use the condition text as the edge label.
		if e.Label == "" {
			attrs["label"] = condStr
		}
		attrs["condition"] = condStr
	}

	if e.Weight != 0 {
		attrs["weight"] = fmt.Sprintf("%d", e.Weight)
	}

	if e.Restart {
		attrs["restart"] = "true"
		// Visual hint: restart edges are dashed.
		attrs["style"] = "dashed"
	}

	b.WriteString(fmt.Sprintf("  %s -> %s", dotID(e.From), dotID(e.To)))
	if len(attrs) > 0 {
		b.WriteString(" ")
		b.WriteString(formatDOTAttrs(attrs))
	}
	b.WriteString(";\n")
}

// formatDOTAttrs renders a map of DOT attributes as a bracketed list,
// with keys in sorted order for deterministic output.
func formatDOTAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sortStrings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, dotQuote(attrs[k])))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// dotID formats a string as a valid DOT identifier.
// If the string is a simple identifier (alphanumeric + underscore, not starting
// with a digit), it is returned as-is. Otherwise it is double-quoted.
func dotID(s string) string {
	if s == "" {
		return `""`
	}
	if isSimpleDOTID(s) {
		return s
	}
	return dotQuote(s)
}

// isSimpleDOTID returns true if s is a valid unquoted DOT identifier.
func isSimpleDOTID(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must not start with a digit.
	if s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// dotQuote wraps a string in double quotes, escaping internal quotes and
// backslashes. Preserves DOT escape sequences like \n, \l, \r.
func dotQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			// Check if this backslash is part of a DOT escape sequence.
			if i+1 < len(s) {
				next := s[i+1]
				if next == 'n' || next == 'l' || next == 'r' {
					// Preserve DOT escape sequences.
					b.WriteByte('\\')
					b.WriteByte(next)
					i++
					continue
				}
			}
			b.WriteString(`\\`)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// escapeNewlines replaces literal newlines with the DOT \n escape for
// multi-line attribute values.
func escapeNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", `\n`)
}

// --- Condition formatting ---
// Replicates the formatter's condition serialization for DOT attribute values.

const (
	precOr  = 1
	precAnd = 2
	precNot = 3
)

func formatCondition(expr ir.ConditionExpr) string {
	return formatConditionExpr(expr, 0)
}

func formatConditionExpr(expr ir.ConditionExpr, parentPrec int) string {
	switch e := expr.(type) {
	case ir.CondCompare:
		return fmt.Sprintf("%s %s %s", e.Variable, e.Op, e.Value)
	case ir.CondAnd:
		s := fmt.Sprintf("%s and %s",
			formatConditionExpr(e.Left, precAnd),
			formatConditionExpr(e.Right, precAnd))
		if parentPrec != 0 && parentPrec != precAnd {
			return "(" + s + ")"
		}
		return s
	case ir.CondOr:
		s := fmt.Sprintf("%s or %s",
			formatConditionExpr(e.Left, precOr),
			formatConditionExpr(e.Right, precOr))
		if parentPrec != 0 && parentPrec != precOr {
			return "(" + s + ")"
		}
		return s
	case ir.CondNot:
		inner := formatConditionExpr(e.Inner, precNot)
		return "not " + inner
	default:
		return ""
	}
}

// --- Duration formatting ---

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	var parts []string
	if h := int(d.Hours()); h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
		d -= time.Duration(h) * time.Hour
	}
	if m := int(d.Minutes()); m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
		d -= time.Duration(m) * time.Minute
	}
	if s := int(d.Seconds()); s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	if len(parts) == 0 {
		return d.String()
	}
	return strings.Join(parts, "")
}

// sortStrings sorts a string slice in place. Avoids importing sort for this
// single use.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
