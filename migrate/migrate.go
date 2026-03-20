package migrate

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/2389-research/dippin-lang/formatter"
	"github.com/2389-research/dippin-lang/ir"
)

// Migrate parses a DOT digraph string and produces a Dippin IR workflow.
// It applies all post-migration cleanup automatically:
//   - Shape → node kind mapping
//   - Un-escaping of prompts and commands (handled by the DOT lexer)
//   - Namespace prefixing for condition variables (bare "outcome" → "ctx.outcome")
//   - Start/exit identification from Mdiamond/Msquare shapes
//   - Graph-level attribute extraction to WorkflowDefaults
func Migrate(dotSource string) (*ir.Workflow, error) {
	dg, err := parseDOT(dotSource)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return convertDOTGraph(dg)
}

// MigrateToSource parses DOT and returns canonical .dip source text.
// Convenience: equivalent to formatter.Format(Migrate(dotSource)).
func MigrateToSource(dotSource string) (string, error) {
	w, err := Migrate(dotSource)
	if err != nil {
		return "", err
	}
	return formatter.Format(w), nil
}

// --- Shape → Kind Mapping ---

// shapeToKind maps DOT shape attributes to IR node kinds.
// Mdiamond and Msquare are handled specially (start/exit markers).
// diamond is handled with disambiguation logic.
var shapeToKind = map[string]ir.NodeKind{
	"box":           ir.NodeAgent,
	"hexagon":       ir.NodeHuman,
	"parallelogram": ir.NodeTool,
	"component":     ir.NodeParallel,
	"tripleoctagon": ir.NodeFanIn,
	"tab":           ir.NodeSubgraph,
}

// convertDOTGraph transforms a parsed DOT graph into an IR workflow.
func convertDOTGraph(dg *dotGraph) (*ir.Workflow, error) {
	w := &ir.Workflow{
		Name:    dg.Name,
		Version: "1",
	}

	// Extract graph-level attributes.
	extractGraphDefaults(dg.GraphAttrs, w)

	// Build a set of edge-implicit nodes for quick lookup.
	nodeIndex := make(map[string]int) // ID → index in dg.Nodes

	for i, n := range dg.Nodes {
		nodeIndex[n.ID] = i
	}

	// First pass: identify start/exit nodes and build IR nodes.
	startID := ""
	exitID := ""
	for _, dn := range dg.Nodes {
		shape := dn.Attrs["shape"]
		if shape == "Mdiamond" {
			startID = dn.ID
		}
		if shape == "Msquare" {
			exitID = dn.ID
		}
	}

	// Build IR nodes in declaration order.
	for _, dn := range dg.Nodes {
		node, err := convertNode(dn, dg.Edges)
		if err != nil {
			return nil, fmt.Errorf("migrate: node %q: %w", dn.ID, err)
		}
		w.Nodes = append(w.Nodes, node)
	}

	// Set start/exit.
	w.Start = startID
	w.Exit = exitID

	// Build IR edges.
	for _, de := range dg.Edges {
		edge, err := convertEdge(de)
		if err != nil {
			return nil, fmt.Errorf("migrate: edge %s->%s: %w", de.From, de.To, err)
		}
		w.Edges = append(w.Edges, edge)
	}

	// Post-pass: infer parallel targets and fan_in sources from edges.
	inferParallelFanIn(w)

	return w, nil
}

// extractGraphDefaults populates workflow-level fields from DOT graph attributes.
func extractGraphDefaults(attrs map[string]string, w *ir.Workflow) {
	for k, v := range attrs {
		switch k {
		case "goal":
			w.Goal = v
		case "rankdir":
			// Presentation-only; ignored.
		case "default_max_retry", "max_retries":
			if n, err := strconv.Atoi(v); err == nil {
				w.Defaults.MaxRetries = n
			}
		case "max_restarts":
			if n, err := strconv.Atoi(v); err == nil {
				w.Defaults.MaxRestarts = n
			}
		case "default_fidelity", "fidelity":
			w.Defaults.Fidelity = v
		case "model":
			w.Defaults.Model = v
		case "provider":
			w.Defaults.Provider = v
		}
	}
}

// convertNode converts a DOT node to an IR node.
func convertNode(dn dotNode, edges []dotEdge) (*ir.Node, error) {
	shape := dn.Attrs["shape"]
	kind := resolveKind(shape, dn.Attrs)

	node := &ir.Node{
		ID:   dn.ID,
		Kind: kind,
	}

	// Set label.
	if label, ok := dn.Attrs["label"]; ok {
		node.Label = label
	}

	// Build kind-specific config.
	switch kind {
	case ir.NodeAgent:
		cfg := buildAgentConfig(dn.Attrs)
		node.Config = cfg
		node.Retry = buildRetryConfig(dn.Attrs)
	case ir.NodeHuman:
		node.Config = buildHumanConfig(dn.Attrs)
	case ir.NodeTool:
		cfg, err := buildToolConfig(dn.Attrs)
		if err != nil {
			return nil, err
		}
		node.Config = cfg
	case ir.NodeParallel:
		node.Config = buildParallelConfig(dn.Attrs)
	case ir.NodeFanIn:
		node.Config = buildFanInConfig(dn.Attrs)
	case ir.NodeSubgraph:
		node.Config = buildSubgraphConfig(dn.Attrs)
	default:
		node.Config = ir.AgentConfig{}
	}

	return node, nil
}

// resolveKind determines the IR node kind from the DOT shape and attributes.
// Implements the diamond disambiguation logic from §5.
func resolveKind(shape string, attrs map[string]string) ir.NodeKind {
	// Start/exit markers become agent nodes.
	if shape == "Mdiamond" || shape == "Msquare" {
		return ir.NodeAgent
	}

	// Diamond disambiguation: per §5.
	if shape == "diamond" {
		if _, hasTool := attrs["tool_command"]; hasTool {
			return ir.NodeTool
		}
		// All other diamonds become agent nodes (routing or prompt-based).
		return ir.NodeAgent
	}

	// Direct mapping.
	if kind, ok := shapeToKind[shape]; ok {
		return kind
	}

	// Default: agent.
	return ir.NodeAgent
}

// --- Config builders ---

func buildAgentConfig(attrs map[string]string) ir.AgentConfig {
	cfg := ir.AgentConfig{}

	applyPromptAttrs(&cfg, attrs)
	applyModelAttrs(&cfg, attrs)
	applyBehaviorAttrs(&cfg, attrs)
	applyPerformanceAttrs(&cfg, attrs)

	return cfg
}

// applyPromptAttrs applies prompt-related attributes to agent config.
func applyPromptAttrs(cfg *ir.AgentConfig, attrs map[string]string) {
	if v, ok := attrs["prompt"]; ok {
		cfg.Prompt = v
	}
	if v, ok := attrs["system_prompt"]; ok {
		cfg.SystemPrompt = v
	}
}

// applyModelAttrs applies model and provider attributes to agent config.
func applyModelAttrs(cfg *ir.AgentConfig, attrs map[string]string) {
	// Legacy: llm_model → model.
	if v, ok := attrs["model"]; ok {
		cfg.Model = v
	}
	if v, ok := attrs["llm_model"]; ok {
		cfg.Model = v
	}
	// Legacy: llm_provider → provider.
	if v, ok := attrs["provider"]; ok {
		cfg.Provider = v
	}
	if v, ok := attrs["llm_provider"]; ok {
		cfg.Provider = v
	}
	if v, ok := attrs["reasoning_effort"]; ok {
		cfg.ReasoningEffort = v
	}
	if v, ok := attrs["fidelity"]; ok {
		cfg.Fidelity = v
	}
}

// applyBehaviorAttrs applies behavior-related attributes to agent config.
func applyBehaviorAttrs(cfg *ir.AgentConfig, attrs map[string]string) {
	if v, ok := attrs["goal_gate"]; ok && isTruthy(v) {
		cfg.GoalGate = true
	}
	if v, ok := attrs["auto_status"]; ok && isTruthy(v) {
		cfg.AutoStatus = true
	}
	if v, ok := attrs["max_turns"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxTurns = n
		}
	}
}

// applyPerformanceAttrs applies performance-related attributes to agent config.
func applyPerformanceAttrs(cfg *ir.AgentConfig, attrs map[string]string) {
	if v, ok := attrs["cmd_timeout"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.CmdTimeout = d
		}
	}
	if v, ok := attrs["cache_tools"]; ok && isTruthy(v) {
		cfg.CacheTools = true
	}
	if v, ok := attrs["compaction"]; ok {
		cfg.Compaction = v
	}
}

func buildHumanConfig(attrs map[string]string) ir.HumanConfig {
	cfg := ir.HumanConfig{}
	if v, ok := attrs["mode"]; ok {
		cfg.Mode = v
	}
	if v, ok := attrs["default"]; ok {
		cfg.Default = v
	}
	return cfg
}

func buildToolConfig(attrs map[string]string) (ir.ToolConfig, error) {
	cfg := ir.ToolConfig{}
	if v, ok := attrs["tool_command"]; ok {
		cfg.Command = v
	}
	if v, ok := attrs["timeout"]; ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid timeout %q: %w", v, err)
		}
		cfg.Timeout = d
	}
	return cfg, nil
}

func buildParallelConfig(attrs map[string]string) ir.ParallelConfig {
	cfg := ir.ParallelConfig{}
	if v, ok := attrs["targets"]; ok {
		cfg.Targets = splitComma(v)
	}
	return cfg
}

func buildFanInConfig(attrs map[string]string) ir.FanInConfig {
	cfg := ir.FanInConfig{}
	if v, ok := attrs["sources"]; ok {
		cfg.Sources = splitComma(v)
	}
	return cfg
}

func buildSubgraphConfig(attrs map[string]string) ir.SubgraphConfig {
	cfg := ir.SubgraphConfig{}
	if v, ok := attrs["ref"]; ok {
		cfg.Ref = v
	}
	return cfg
}

func buildRetryConfig(attrs map[string]string) ir.RetryConfig {
	rc := ir.RetryConfig{}
	if v, ok := attrs["retry_policy"]; ok {
		rc.Policy = v
	}
	if v, ok := attrs["max_retries"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			rc.MaxRetries = n
		}
	}
	if v, ok := attrs["retry_target"]; ok {
		rc.RetryTarget = v
	}
	if v, ok := attrs["fallback_target"]; ok {
		rc.FallbackTarget = v
	}
	return rc
}

// --- Edge conversion ---

func convertEdge(de dotEdge) (*ir.Edge, error) {
	e := &ir.Edge{
		From: de.From,
		To:   de.To,
	}

	if v, ok := de.Attrs["label"]; ok {
		e.Label = v
	}
	if v, ok := de.Attrs["condition"]; ok {
		cond, err := parseCondition(v)
		if err != nil {
			return nil, fmt.Errorf("condition %q: %w", v, err)
		}
		e.Condition = cond
	}
	if v, ok := de.Attrs["weight"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			e.Weight = n
		}
	}
	// Both restart and loop_restart (legacy) map to Edge.Restart.
	if v, ok := de.Attrs["restart"]; ok && isTruthy(v) {
		e.Restart = true
	}
	if v, ok := de.Attrs["loop_restart"]; ok && isTruthy(v) {
		e.Restart = true
	}

	return e, nil
}

// --- Condition parsing ---

// parseCondition parses a DOT condition string into an ir.Condition.
// It handles:
//   - Simple comparisons: outcome=success, tool_stdout contains pass
//   - AND/OR: outcome=success && tool_stdout contains done
//   - NOT: not outcome=fail, !outcome=fail
//   - Namespace prefixing: bare "outcome" → "ctx.outcome"
func parseCondition(raw string) (*ir.Condition, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	expr, err := parseCondExpr(raw)
	if err != nil {
		return nil, err
	}

	// Format the parsed condition back to a canonical raw string.
	canonRaw := formatCondExpr(expr)

	return &ir.Condition{
		Raw:    canonRaw,
		Parsed: expr,
	}, nil
}

// parseCondExpr parses a condition expression string into an AST.
func parseCondExpr(s string) (ir.ConditionExpr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty condition expression")
	}

	// Try to split on || (OR — lowest precedence).
	if parts, ok := splitLogicalOp(s, "||"); ok {
		left, err := parseCondExpr(parts[0])
		if err != nil {
			return nil, err
		}
		right, err := parseCondExpr(parts[1])
		if err != nil {
			return nil, err
		}
		return ir.CondOr{Left: left, Right: right}, nil
	}

	// Try to split on && (AND — higher precedence).
	if parts, ok := splitLogicalOp(s, "&&"); ok {
		left, err := parseCondExpr(parts[0])
		if err != nil {
			return nil, err
		}
		right, err := parseCondExpr(parts[1])
		if err != nil {
			return nil, err
		}
		return ir.CondAnd{Left: left, Right: right}, nil
	}

	// Handle NOT prefix.
	if strings.HasPrefix(s, "not ") {
		inner, err := parseCondExpr(s[4:])
		if err != nil {
			return nil, err
		}
		return ir.CondNot{Inner: inner}, nil
	}
	if strings.HasPrefix(s, "!") {
		inner, err := parseCondExpr(s[1:])
		if err != nil {
			return nil, err
		}
		return ir.CondNot{Inner: inner}, nil
	}

	// Parse a single comparison: var op value.
	return parseComparison(s)
}

// splitLogicalOp splits a condition string on the given logical operator (&&, ||).
// Returns the two parts if the operator is found at the top level (not inside parens).
func splitLogicalOp(s, op string) ([]string, bool) {
	depth := 0
	for i := 0; i <= len(s)-len(op); i++ {
		ch := s[i]
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		}
		if depth == 0 && s[i:i+len(op)] == op {
			left := strings.TrimSpace(s[:i])
			right := strings.TrimSpace(s[i+len(op):])
			if left != "" && right != "" {
				return []string{left, right}, true
			}
		}
	}
	return nil, false
}

// parseComparison parses a single condition comparison like "outcome=success"
// or "tool_stdout contains pass".
func parseComparison(s string) (ir.ConditionExpr, error) {
	s = strings.TrimSpace(s)

	// Try != first (before =) to avoid matching the = in !=.
	if idx := strings.Index(s, "!="); idx > 0 {
		variable := strings.TrimSpace(s[:idx])
		value := strings.TrimSpace(s[idx+2:])
		return ir.CondCompare{
			Variable: addNamespacePrefix(variable),
			Op:       "!=",
			Value:    value,
		}, nil
	}

	// Try = (equality).
	if idx := strings.Index(s, "="); idx > 0 {
		variable := strings.TrimSpace(s[:idx])
		value := strings.TrimSpace(s[idx+1:])
		return ir.CondCompare{
			Variable: addNamespacePrefix(variable),
			Op:       "=",
			Value:    value,
		}, nil
	}

	// Try word-based operators: contains, startswith, endswith, in.
	for _, op := range []string{" contains ", " startswith ", " endswith ", " in "} {
		if idx := strings.Index(s, op); idx > 0 {
			variable := strings.TrimSpace(s[:idx])
			value := strings.TrimSpace(s[idx+len(op):])
			return ir.CondCompare{
				Variable: addNamespacePrefix(variable),
				Op:       strings.TrimSpace(op),
				Value:    value,
			}, nil
		}
	}

	return nil, fmt.Errorf("cannot parse condition comparison: %q", s)
}

// addNamespacePrefix adds the ctx. namespace to bare condition variable names.
// Variables that already contain a dot are left as-is (graph.*, ctx.*).
// The legacy "context." prefix is normalized to "ctx.".
func addNamespacePrefix(variable string) string {
	variable = strings.TrimSpace(variable)

	// Already namespaced with "context." → normalize to "ctx."
	if strings.HasPrefix(variable, "context.") {
		return "ctx." + variable[len("context."):]
	}

	// Already namespaced (contains a dot).
	if strings.Contains(variable, ".") {
		return variable
	}

	// Bare variable name → add ctx. prefix.
	return "ctx." + variable
}

// formatCondExpr renders a condition AST back to a canonical string.
func formatCondExpr(expr ir.ConditionExpr) string {
	return formatCondExprPrec(expr, 0)
}

const (
	condPrecOr  = 1
	condPrecAnd = 2
	condPrecNot = 3
)

func formatCondExprPrec(expr ir.ConditionExpr, parentPrec int) string {
	switch e := expr.(type) {
	case ir.CondCompare:
		return fmt.Sprintf("%s %s %s", e.Variable, e.Op, e.Value)
	case ir.CondAnd:
		s := fmt.Sprintf("%s and %s",
			formatCondExprPrec(e.Left, condPrecAnd),
			formatCondExprPrec(e.Right, condPrecAnd))
		if parentPrec != 0 && parentPrec != condPrecAnd {
			return "(" + s + ")"
		}
		return s
	case ir.CondOr:
		s := fmt.Sprintf("%s or %s",
			formatCondExprPrec(e.Left, condPrecOr),
			formatCondExprPrec(e.Right, condPrecOr))
		if parentPrec != 0 && parentPrec != condPrecOr {
			return "(" + s + ")"
		}
		return s
	case ir.CondNot:
		inner := formatCondExprPrec(e.Inner, condPrecNot)
		return "not " + inner
	default:
		return ""
	}
}

// --- Parallel/Fan-in inference ---

// inferParallelFanIn fills in Targets and Sources from edges when not
// explicitly set in DOT attributes.
func inferParallelFanIn(w *ir.Workflow) {
	for _, n := range w.Nodes {
		switch cfg := n.Config.(type) {
		case ir.ParallelConfig:
			if len(cfg.Targets) == 0 {
				edges := w.EdgesFrom(n.ID)
				targets := make([]string, 0, len(edges))
				for _, e := range edges {
					targets = append(targets, e.To)
				}
				n.Config = ir.ParallelConfig{Targets: targets}
			}
		case ir.FanInConfig:
			if len(cfg.Sources) == 0 {
				edges := w.EdgesTo(n.ID)
				sources := make([]string, 0, len(edges))
				for _, e := range edges {
					sources = append(sources, e.From)
				}
				n.Config = ir.FanInConfig{Sources: sources}
			}
		}
	}
}

// --- Helpers ---

func isTruthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
