package validator

import (
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

// lintConditionalReachability checks DIP101: nodes that are only reachable
// through conditional edges may be unreachable at runtime if conditions are
// not satisfied. A node is flagged if ALL of its incoming edges are conditional
// (have a non-nil Condition), meaning there is no guaranteed path to it.
//
// Edges whose sibling set forms an exhaustive condition (e.g., outcome=success
// + outcome=fail) are not flagged, since one branch is guaranteed to execute.
func lintConditionalReachability(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	incoming := buildIncomingEdgeMap(w)
	exhaustiveSources := findExhaustiveSources(w)

	for _, n := range w.Nodes {
		if d, ok := checkConditionalReachability(n, w.Start, incoming, exhaustiveSources); ok {
			diags = append(diags, d)
		}
	}
	return diags
}

// checkConditionalReachability checks a single node for DIP101.
func checkConditionalReachability(n *ir.Node, start string, incoming map[string][]*ir.Edge, exhaustive map[string]bool) (Diagnostic, bool) {
	if n.ID == start {
		return Diagnostic{}, false
	}
	edges := incoming[n.ID]
	if len(edges) == 0 {
		return Diagnostic{}, false
	}
	if allEdgesConditional(edges) && !allSourcesExhaustive(edges, exhaustive) {
		return Diagnostic{
			Code:     DIP101,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("node %q is only reachable through conditional edges and may be skipped at runtime", n.ID),
			Location: n.Source,
			Help:     "add an unconditional edge to this node, or verify all conditions are exhaustive",
		}, true
	}
	return Diagnostic{}, false
}

// allSourcesExhaustive returns true if every source node feeding these edges
// has exhaustive outgoing conditions.
func allSourcesExhaustive(edges []*ir.Edge, exhaustive map[string]bool) bool {
	for _, e := range edges {
		if !exhaustive[e.From] {
			return false
		}
	}
	return true
}

// buildIncomingEdgeMap builds a map of incoming edges per node.
func buildIncomingEdgeMap(w *ir.Workflow) map[string][]*ir.Edge {
	incoming := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		incoming[e.To] = append(incoming[e.To], e)
	}
	return incoming
}

// allEdgesConditional returns true if every edge has a non-nil Condition.
func allEdgesConditional(edges []*ir.Edge) bool {
	for _, e := range edges {
		if e.Condition == nil {
			return false
		}
	}
	return true
}

// lintDefaultEdge checks DIP102: nodes that have outgoing conditional edges
// but no unconditional (default/fallback) edge. Without a default edge,
// execution may get stuck at this node if no condition matches.
//
// Nodes whose outgoing conditions are exhaustive are not flagged.
func lintDefaultEdge(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	exhaustiveSources := findExhaustiveSources(w)

	for _, n := range w.Nodes {
		outgoing := w.EdgesFrom(n.ID)
		if len(outgoing) == 0 {
			continue
		}
		if hasMissingDefault(outgoing) && !exhaustiveSources[n.ID] {
			diags = append(diags, Diagnostic{
				Code:     DIP102,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has conditional outgoing edges but no unconditional default edge", n.ID),
				Location: n.Source,
				Help:     "add an unconditional edge as a fallback, or ensure conditions are exhaustive",
			})
		}
	}
	return diags
}

// hasMissingDefault returns true if edges contain conditional edges but no unconditional one.
func hasMissingDefault(edges []*ir.Edge) bool {
	hasConditional := false
	hasUnconditional := false
	for _, e := range edges {
		if e.Condition != nil {
			hasConditional = true
		} else {
			hasUnconditional = true
		}
	}
	return hasConditional && !hasUnconditional
}

// lintSuccessPath checks DIP105: there must be at least one path from the
// start node to the exit node using only non-restart edges. If no such path
// exists, the workflow can never complete normally.
func lintSuccessPath(w *ir.Workflow) []Diagnostic {
	if !hasValidStartAndExit(w) {
		return nil
	}

	adj := buildForwardAdjacency(w)
	visited := bfsReachable(w.Start, adj)

	if visited[w.Exit] {
		return nil
	}
	return []Diagnostic{{
		Code:     DIP105,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("no forward path from start node %q to exit node %q (excluding restart edges)", w.Start, w.Exit),
		Help:     "ensure there is at least one non-restart path from start to exit",
	}}
}

// hasValidStartAndExit returns true if the workflow has valid start and exit nodes.
func hasValidStartAndExit(w *ir.Workflow) bool {
	return w.Start != "" && w.Exit != "" && w.Node(w.Start) != nil && w.Node(w.Exit) != nil
}

// knownExhaustiveSets lists value sets that are known to be mutually exhaustive
// for a given variable. If a node's outgoing edges collectively cover all values
// in any set for a variable, the conditions are exhaustive.
var knownExhaustiveSets = map[string][][]string{
	"ctx.outcome": {
		{"success", "fail"},
		{"success", "failure"},
	},
	"outcome": {
		{"success", "fail"},
		{"success", "failure"},
	},
}

// findExhaustiveSources returns a set of node IDs whose outgoing conditional
// edges form an exhaustive condition set (every branch is guaranteed to match).
func findExhaustiveSources(w *ir.Workflow) map[string]bool {
	result := make(map[string]bool)
	outgoing := buildOutgoingEdgeMap(w)

	for nodeID, edges := range outgoing {
		if edgesAreExhaustive(edges) {
			result[nodeID] = true
		}
	}
	return result
}

// buildOutgoingEdgeMap groups edges by their From node ID.
func buildOutgoingEdgeMap(w *ir.Workflow) map[string][]*ir.Edge {
	out := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		out[e.From] = append(out[e.From], e)
	}
	return out
}

// edgesAreExhaustive returns true if a set of sibling edges covers all values
// of a variable according to knownExhaustiveSets.
func edgesAreExhaustive(edges []*ir.Edge) bool {
	byVar := collectConditionValues(edges)
	return matchesExhaustiveSet(byVar)
}

// collectConditionValues groups equality condition values by variable name.
func collectConditionValues(edges []*ir.Edge) map[string]map[string]bool {
	byVar := make(map[string]map[string]bool)
	for _, e := range edges {
		cmp, ok := extractEqualityCondition(e)
		if !ok {
			continue
		}
		if byVar[cmp.Variable] == nil {
			byVar[cmp.Variable] = make(map[string]bool)
		}
		byVar[cmp.Variable][cmp.Value] = true
	}
	return byVar
}

// extractEqualityCondition returns the CondCompare if the edge has a simple
// equality condition (= or ==), and false otherwise.
func extractEqualityCondition(e *ir.Edge) (ir.CondCompare, bool) {
	if !hasCondition(e) {
		return ir.CondCompare{}, false
	}
	cmp, ok := e.Condition.Parsed.(ir.CondCompare)
	if !ok || !isEqualityOp(cmp.Op) {
		return ir.CondCompare{}, false
	}
	return cmp, true
}

// hasCondition returns true if the edge has a parsed condition.
func hasCondition(e *ir.Edge) bool {
	return e.Condition != nil && e.Condition.Parsed != nil
}

// isEqualityOp returns true for "=" and "==" operators.
func isEqualityOp(op string) bool {
	return op == "=" || op == "=="
}

// matchesExhaustiveSet returns true if any variable's values cover a known exhaustive set.
func matchesExhaustiveSet(byVar map[string]map[string]bool) bool {
	for variable, values := range byVar {
		if variableIsExhaustive(variable, values) {
			return true
		}
	}
	return false
}

// variableIsExhaustive returns true if the given values for a variable
// cover at least one known exhaustive set.
func variableIsExhaustive(variable string, values map[string]bool) bool {
	sets, known := knownExhaustiveSets[variable]
	if !known {
		return false
	}
	for _, set := range sets {
		if coversAll(values, set) {
			return true
		}
	}
	return false
}

// coversAll returns true if values contains every element in required.
func coversAll(values map[string]bool, required []string) bool {
	for _, r := range required {
		if !values[r] {
			return false
		}
	}
	return true
}
