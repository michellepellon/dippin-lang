// Package formatter implements canonical Dippin source formatting.
// Given an ir.Workflow, it produces deterministic .dip source text.
// The output is idempotent: Format(w) always produces the same string
// for the same IR state.
package formatter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/2389-research/dippin-lang/ir"
)

// Format renders a workflow to canonical Dippin source text.
// The output always ends with exactly one trailing newline.
func Format(w *ir.Workflow) string {
	wr := &writer{}

	writeWorkflowHeader(wr, w)

	if !isDefaultsZero(w.Defaults) {
		wr.blank()
		writeDefaults(wr, w.Defaults)
	}

	for _, n := range w.Nodes {
		wr.blank()
		writeNode(wr, n)
	}

	if len(w.Edges) > 0 {
		wr.blank()
		writeEdges(wr, w.Edges)
	}

	return wr.String()
}

// writer wraps a strings.Builder with indentation tracking.
type writer struct {
	buf    strings.Builder
	indent int // current indent level (each level = 2 spaces)
}

// line writes a single indented line followed by a newline.
func (wr *writer) line(format string, args ...any) {
	content := fmt.Sprintf(format, args...)
	content = strings.TrimRight(content, " \t")
	prefix := strings.Repeat("  ", wr.indent)
	wr.buf.WriteString(prefix)
	wr.buf.WriteString(content)
	wr.buf.WriteByte('\n')
}

// blank writes an empty line.
func (wr *writer) blank() {
	wr.buf.WriteByte('\n')
}

// push increases the indentation level by one.
func (wr *writer) push() {
	wr.indent++
}

// pop decreases the indentation level by one.
func (wr *writer) pop() {
	wr.indent--
}

// multilineBlock emits a multiline field in the canonical form:
//
//	key:
//	  <line1>
//	  <line2>
func (wr *writer) multilineBlock(key, content string) {
	wr.line("%s:", key)
	content = strings.TrimRight(content, " \t\n\r")
	if content == "" {
		return
	}
	wr.push()
	for _, l := range strings.Split(content, "\n") {
		l = strings.TrimRight(l, " \t\r")
		if l == "" {
			wr.blank()
		} else {
			wr.line("%s", l)
		}
	}
	wr.pop()
}

// String returns the final output with exactly one trailing newline.
func (wr *writer) String() string {
	s := wr.buf.String()
	s = strings.TrimRight(s, "\n\r \t")
	return s + "\n"
}

// --- Section emitters ---

func writeWorkflowHeader(wr *writer, w *ir.Workflow) {
	wr.line("workflow %s", w.Name)
	wr.push()
	if w.Goal != "" {
		wr.line("goal: %s", quoteValue(w.Goal))
	}
	wr.line("start: %s", w.Start)
	wr.line("exit: %s", w.Exit)
	// We keep the indent at 1 for the rest of the top-level sections
}

func writeDefaults(wr *writer, d ir.WorkflowDefaults) {
	wr.line("defaults")
	wr.push()
	if d.Model != "" {
		wr.line("model: %s", quoteValue(d.Model))
	}
	if d.Provider != "" {
		wr.line("provider: %s", quoteValue(d.Provider))
	}
	if d.RetryPolicy != "" {
		wr.line("retry_policy: %s", quoteValue(d.RetryPolicy))
	}
	if d.MaxRetries != 0 {
		wr.line("max_retries: %d", d.MaxRetries)
	}
	if d.Fidelity != "" {
		wr.line("fidelity: %s", quoteValue(d.Fidelity))
	}
	if d.MaxRestarts != 0 {
		wr.line("max_restarts: %d", d.MaxRestarts)
	}
	if d.RestartTarget != "" {
		wr.line("restart_target: %s", d.RestartTarget)
	}
	if d.CacheTools {
		wr.line("cache_tools: true")
	}
	if d.Compaction != "" {
		wr.line("compaction: %s", quoteValue(d.Compaction))
	}
	wr.pop()
}

func writeNode(wr *writer, n *ir.Node) {
	switch cfg := n.Config.(type) {
	case ir.ParallelConfig:
		wr.line("parallel %s -> %s", n.ID, strings.Join(cfg.Targets, ", "))
	case ir.FanInConfig:
		wr.line("fan_in %s <- %s", n.ID, strings.Join(cfg.Sources, ", "))
	default:
		wr.line("%s %s", n.Kind, n.ID)
		wr.push()
		switch cfg := n.Config.(type) {
		case ir.AgentConfig:
			writeAgentFields(wr, n, cfg)
		case ir.HumanConfig:
			writeHumanFields(wr, n, cfg)
		case ir.ToolConfig:
			writeToolFields(wr, n, cfg)
		case ir.SubgraphConfig:
			writeSubgraphFields(wr, n, cfg)
		}
		wr.pop()
	}
}

func writeAgentFields(wr *writer, n *ir.Node, cfg ir.AgentConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if len(n.Classes) > 0 {
		wr.line("class: %s", strings.Join(n.Classes, ", "))
	}
	if cfg.Model != "" {
		wr.line("model: %s", quoteValue(cfg.Model))
	}
	if cfg.Provider != "" {
		wr.line("provider: %s", quoteValue(cfg.Provider))
	}
	if cfg.ReasoningEffort != "" {
		wr.line("reasoning_effort: %s", quoteValue(cfg.ReasoningEffort))
	}
	if cfg.Fidelity != "" {
		wr.line("fidelity: %s", quoteValue(cfg.Fidelity))
	}
	if cfg.GoalGate {
		wr.line("goal_gate: true")
	}
	if cfg.AutoStatus {
		wr.line("auto_status: true")
	}
	if cfg.MaxTurns != 0 {
		wr.line("max_turns: %d", cfg.MaxTurns)
	}
	if n.Retry.Policy != "" {
		wr.line("retry_policy: %s", quoteValue(n.Retry.Policy))
	}
	if n.Retry.MaxRetries != 0 {
		wr.line("max_retries: %d", n.Retry.MaxRetries)
	}
	if n.Retry.RetryTarget != "" {
		wr.line("retry_target: %s", n.Retry.RetryTarget)
	}
	if n.Retry.FallbackTarget != "" {
		wr.line("fallback_target: %s", n.Retry.FallbackTarget)
	}
	if len(n.IO.Reads) > 0 {
		wr.line("reads: %s", strings.Join(n.IO.Reads, ", "))
	}
	if len(n.IO.Writes) > 0 {
		wr.line("writes: %s", strings.Join(n.IO.Writes, ", "))
	}
	if cfg.Prompt != "" {
		wr.multilineBlock("prompt", cfg.Prompt)
	}
}

func writeHumanFields(wr *writer, n *ir.Node, cfg ir.HumanConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if cfg.Mode != "" {
		wr.line("mode: %s", quoteValue(cfg.Mode))
	}
	if cfg.Default != "" {
		wr.line("default: %s", quoteValue(cfg.Default))
	}
	if len(n.IO.Reads) > 0 {
		wr.line("reads: %s", strings.Join(n.IO.Reads, ", "))
	}
	if len(n.IO.Writes) > 0 {
		wr.line("writes: %s", strings.Join(n.IO.Writes, ", "))
	}
}

func writeToolFields(wr *writer, n *ir.Node, cfg ir.ToolConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if cfg.Timeout != 0 {
		wr.line("timeout: %s", formatDuration(cfg.Timeout))
	}
	if len(n.IO.Reads) > 0 {
		wr.line("reads: %s", strings.Join(n.IO.Reads, ", "))
	}
	if len(n.IO.Writes) > 0 {
		wr.line("writes: %s", strings.Join(n.IO.Writes, ", "))
	}
	if cfg.Command != "" {
		wr.multilineBlock("command", cfg.Command)
	}
}

func writeSubgraphFields(wr *writer, n *ir.Node, cfg ir.SubgraphConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if cfg.Ref != "" {
		wr.line("ref: %s", quoteValue(cfg.Ref))
	}
	if len(cfg.Params) > 0 {
		wr.line("params:")
		wr.push()
		keys := make([]string, 0, len(cfg.Params))
		for k := range cfg.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			wr.line("%s: %s", k, quoteValue(cfg.Params[k]))
		}
		wr.pop()
	}
}

func writeEdges(wr *writer, edges []*ir.Edge) {
	wr.line("edges")
	wr.push()
	for _, e := range edges {
		writeEdge(wr, e)
	}
	wr.pop()
}

func writeEdge(wr *writer, e *ir.Edge) {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s -> %s", e.From, e.To))

	if e.Condition != nil {
		if e.Condition.Parsed != nil {
			parts = append(parts, fmt.Sprintf("when %s", formatCondition(e.Condition.Parsed)))
		} else if e.Condition.Raw != "" {
			parts = append(parts, fmt.Sprintf("when %s", e.Condition.Raw))
		}
	}
	if e.Label != "" {
		parts = append(parts, fmt.Sprintf("label: %s", quoteValue(e.Label)))
	}
	if e.Weight != 0 {
		parts = append(parts, fmt.Sprintf("weight: %d", e.Weight))
	}
	if e.Restart {
		parts = append(parts, "restart: true")
	}

	wr.line("%s", strings.Join(parts, "  "))
}

// --- Condition formatting ---

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
		// Parenthesize if parent is a different compound operator (OR wrapping AND,
		// NOT wrapping AND). parentPrec==0 means top-level, no parens needed.
		if parentPrec != 0 && parentPrec != precAnd {
			return "(" + s + ")"
		}
		return s
	case ir.CondOr:
		s := fmt.Sprintf("%s or %s",
			formatConditionExpr(e.Left, precOr),
			formatConditionExpr(e.Right, precOr))
		// Parenthesize if parent is a different compound operator (AND wrapping OR,
		// NOT wrapping OR).
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

func quoteValue(s string) string {
	if s == "" {
		return `""`
	}
	if needsQuoting(s) {
		return `"` + s + `"`
	}
	return s
}

// needsQuoting returns true if the value needs to be enclosed in double quotes.
// Simple identifiers (alphanumeric, underscore, dash, dot, slash, colon) are unquoted.
func needsQuoting(s string) bool {
	for _, ch := range s {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_', ch == '-', ch == '.', ch == '/', ch == ':':
		default:
			return true
		}
	}
	return false
}

// formatDuration renders a time.Duration as a compact human-readable string
// suitable for Dippin source: "30s", "5m", "1h30m".
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
		// Sub-second durations.
		return d.String()
	}
	return strings.Join(parts, "")
}

func isDefaultsZero(d ir.WorkflowDefaults) bool {
	return d == ir.WorkflowDefaults{}
}
