package validator

import (
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

// lintConditionalReachability checks DIP101: nodes that are only reachable
// through conditional edges may be unreachable at runtime if conditions are
// not satisfied. A node is flagged if ALL of its incoming edges are conditional
// (have a non-nil Condition), meaning there is no guaranteed path to it.
func lintConditionalReachability(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	incoming := buildIncomingEdgeMap(w)

	for _, n := range w.Nodes {
		if n.ID == w.Start {
			continue
		}
		edges := incoming[n.ID]
		if len(edges) == 0 {
			continue
		}
		if allEdgesConditional(edges) {
			diags = append(diags, Diagnostic{
				Code:     DIP101,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q is only reachable through conditional edges and may be skipped at runtime", n.ID),
				Location: n.Source,
				Help:     "add an unconditional edge to this node, or verify all conditions are exhaustive",
			})
		}
	}
	return diags
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
func lintDefaultEdge(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	for _, n := range w.Nodes {
		outgoing := w.EdgesFrom(n.ID)
		if len(outgoing) == 0 {
			continue
		}
		if hasMissingDefault(outgoing) {
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
