package validator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// Validate runs all graph-structure checks (DIP001–DIP009) on the workflow
// and returns all diagnostics found. It always runs all checks — never
// short-circuits — so that a single pass reports everything.
func Validate(w *ir.Workflow) Result {
	var diags []Diagnostic

	// Run all checks unconditionally. DIP008 first because duplicate detection
	// is logically prior to other graph checks.
	diags = append(diags, checkNoDuplicateNodes(w)...)
	diags = append(diags, checkStartExists(w)...)
	diags = append(diags, checkExitExists(w)...)
	diags = append(diags, checkEdgeEndpoints(w)...)
	diags = append(diags, checkExitNoOutgoing(w)...)
	diags = append(diags, checkNoDuplicateEdges(w)...)
	diags = append(diags, checkReachability(w)...)
	diags = append(diags, checkNoCycles(w)...)
	diags = append(diags, checkParallelFanIn(w)...)

	return Result{Diagnostics: diags}
}

// checkStartExists verifies DIP001: the start node ID is set and references
// an existing node.
func checkStartExists(w *ir.Workflow) []Diagnostic {
	if w.Start == "" {
		return []Diagnostic{{
			Code:     DIP001,
			Severity: SeverityError,
			Message:  "workflow has no start node declared",
			Help:     "add a start: field to the workflow",
		}}
	}
	if w.Node(w.Start) == nil {
		return []Diagnostic{{
			Code:     DIP001,
			Severity: SeverityError,
			Message:  fmt.Sprintf("start node %q is declared but does not exist in the node list", w.Start),
			Help:     fmt.Sprintf("add a node with ID %q to the workflow", w.Start),
		}}
	}
	return nil
}

// checkExitExists verifies DIP002: the exit node ID is set and references
// an existing node.
func checkExitExists(w *ir.Workflow) []Diagnostic {
	if w.Exit == "" {
		return []Diagnostic{{
			Code:     DIP002,
			Severity: SeverityError,
			Message:  "workflow has no exit node declared",
			Help:     "add an exit: field to the workflow",
		}}
	}
	if w.Node(w.Exit) == nil {
		return []Diagnostic{{
			Code:     DIP002,
			Severity: SeverityError,
			Message:  fmt.Sprintf("exit node %q is declared but does not exist in the node list", w.Exit),
			Help:     fmt.Sprintf("add a node with ID %q to the workflow", w.Exit),
		}}
	}
	return nil
}

// checkEdgeEndpoints verifies DIP003: every edge endpoint references an existing node.
// If a dangling reference is close to an existing node ID (Levenshtein ≤ 2),
// a "did you mean?" suggestion is included.
func checkEdgeEndpoints(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	nodeSet := buildNodeSet(w)

	for _, e := range w.Edges {
		if _, ok := nodeSet[e.From]; !ok && e.From != "" {
			d := Diagnostic{
				Code:     DIP003,
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge references unknown source node %q", e.From),
				Location: e.Source,
			}
			if suggestion := closestNodeID(w, e.From); suggestion != "" {
				d.Help = fmt.Sprintf("did you mean %q?", suggestion)
			} else {
				d.Help = fmt.Sprintf("declare a node with ID %q or fix the edge source", e.From)
			}
			diags = append(diags, d)
		}
		if _, ok := nodeSet[e.To]; !ok && e.To != "" {
			d := Diagnostic{
				Code:     DIP003,
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge references unknown target node %q", e.To),
				Location: e.Source,
			}
			if suggestion := closestNodeID(w, e.To); suggestion != "" {
				d.Help = fmt.Sprintf("did you mean %q?", suggestion)
			} else {
				d.Help = fmt.Sprintf("declare a node with ID %q or fix the edge target", e.To)
			}
			diags = append(diags, d)
		}
	}
	return diags
}

// checkReachability verifies DIP004: all nodes are reachable from the start node.
// Uses BFS traversal including restart edges (restart edges are valid paths for
// reachability purposes).
func checkReachability(w *ir.Workflow) []Diagnostic {
	// If start doesn't exist, we can't do reachability.
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	// Build adjacency list from edges for BFS.
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	// Include implicit edges from parallel → targets and sources → fan_in. (REACH)
	for _, n := range w.Nodes {
		switch cfg := n.Config.(type) {
		case ir.ParallelConfig:
			adj[n.ID] = append(adj[n.ID], cfg.Targets...)
		case ir.FanInConfig:
			for _, src := range cfg.Sources {
				adj[src] = append(adj[src], n.ID)
			}
		}
	}

	visited := make(map[string]bool)
	queue := []string{w.Start}
	visited[w.Start] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range adj[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	var diags []Diagnostic
	for _, n := range w.Nodes {
		if !visited[n.ID] {
			diags = append(diags, Diagnostic{
				Code:     DIP004,
				Severity: SeverityError,
				Message:  fmt.Sprintf("node %q is unreachable from start node %q", n.ID, w.Start),
				Location: n.Source,
				Help:     fmt.Sprintf("add an edge leading to %q or remove it", n.ID),
			})
		}
	}
	return diags
}

// checkNoCycles verifies DIP005: no unconditional cycles exist after excluding
// edges marked restart: true. Per ADR 1, restart edges are back-edges that
// trigger downstream clear + re-execution and are not considered cycles.
func checkNoCycles(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	// Build adjacency list excluding restart edges.
	// Do NOT include implicit parallel/fan_in edges here — fork-join
	// patterns (parallel → targets → fan_in) are structural, not cycles.
	// Implicit edges are only needed for reachability (DIP004).
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if !e.Restart {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}

	// DFS with white/gray/black coloring for cycle detection.
	const (
		white = 0 // Not visited
		gray  = 1 // In current DFS path (visiting)
		black = 2 // Fully processed (done)
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	var diags []Diagnostic

	reported := make(map[string]bool) // avoid duplicate cycle reports
	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		for _, next := range adj[node] {
			if color[next] == gray {
				// Found a cycle — reconstruct path from next → ... → node → next.
				// Only report if we haven't already reported a cycle through this back-edge target.
				if !reported[next] {
					reported[next] = true
					cycle := reconstructCycle(parent, node, next)
					diags = append(diags, Diagnostic{
						Code:     DIP005,
						Severity: SeverityError,
						Message:  fmt.Sprintf("unconditional cycle detected: %s", strings.Join(cycle, " → ")),
						Help:     "break the cycle by removing an edge or marking it restart: true",
					})
				}
			}
			if color[next] == white {
				parent[next] = node
				dfs(next)
			}
		}
		color[node] = black
	}

	// Start DFS from all nodes to catch cycles in disconnected components too.
	for _, n := range w.Nodes {
		if color[n.ID] == white {
			dfs(n.ID)
		}
	}

	return diags
}

// reconstructCycle builds the cycle path from the DFS parent map.
// Given that we found an edge from → to where to is already gray (in the
// current path), we walk parent pointers from "from" back to "to" to
// reconstruct: to → ... → from → to.
//
// The loop is bounded by the total number of nodes to prevent infinite
// iteration when the parent chain is incomplete (e.g. due to implicit
// edges added for parallel/fan-in nodes that were never DFS-visited).
func reconstructCycle(parent map[string]string, from, to string) []string {
	path := []string{to}
	curr := from
	seen := make(map[string]bool)
	seen[to] = true
	// Walk at most len(parent)+1 steps to guarantee termination.
	maxSteps := len(parent) + 2
	for curr != to && curr != "" && !seen[curr] && maxSteps > 0 {
		seen[curr] = true
		path = append(path, curr)
		curr = parent[curr]
		maxSteps--
	}
	path = append(path, to)
	// Reverse so it reads: to → ... → from → to
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// checkExitNoOutgoing verifies DIP006: the exit node has no outgoing edges.
func checkExitNoOutgoing(w *ir.Workflow) []Diagnostic {
	if w.Exit == "" || w.Node(w.Exit) == nil {
		return nil
	}

	outgoing := w.EdgesFrom(w.Exit)
	if len(outgoing) == 0 {
		return nil
	}

	var diags []Diagnostic
	for _, e := range outgoing {
		diags = append(diags, Diagnostic{
			Code:     DIP006,
			Severity: SeverityError,
			Message:  fmt.Sprintf("exit node %q has outgoing edge to %q", w.Exit, e.To),
			Location: e.Source,
			Help:     "remove the outgoing edge or change the exit node",
		})
	}
	return diags
}

// checkParallelFanIn verifies DIP007: every parallel fan-out node has a matching
// fan-in node, and vice versa. Matching means the ParallelConfig.Targets set
// equals the FanInConfig.Sources set (order-insensitive).
func checkParallelFanIn(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	type nodeTargets struct {
		node   *ir.Node
		sorted []string
	}

	var parallels []nodeTargets
	var fanIns []nodeTargets

	for _, n := range w.Nodes {
		switch cfg := n.Config.(type) {
		case ir.ParallelConfig:
			sorted := make([]string, len(cfg.Targets))
			copy(sorted, cfg.Targets)
			sort.Strings(sorted)
			parallels = append(parallels, nodeTargets{node: n, sorted: sorted})
		case ir.FanInConfig:
			sorted := make([]string, len(cfg.Sources))
			copy(sorted, cfg.Sources)
			sort.Strings(sorted)
			fanIns = append(fanIns, nodeTargets{node: n, sorted: sorted})
		}
	}

	// For each parallel node, check there's a matching fan-in.
	for _, p := range parallels {
		found := false
		for _, f := range fanIns {
			if slicesEqual(p.sorted, f.sorted) {
				found = true
				break
			}
		}
		if !found {
			diags = append(diags, Diagnostic{
				Code:     DIP007,
				Severity: SeverityError,
				Message:  fmt.Sprintf("parallel node %q has targets %v but no matching fan_in node", p.node.ID, p.sorted),
				Location: p.node.Source,
				Help:     fmt.Sprintf("add a fan_in node with sources: %v", p.sorted),
			})
		}
	}

	// For each fan-in node, check there's a matching parallel.
	for _, f := range fanIns {
		found := false
		for _, p := range parallels {
			if slicesEqual(f.sorted, p.sorted) {
				found = true
				break
			}
		}
		if !found {
			diags = append(diags, Diagnostic{
				Code:     DIP007,
				Severity: SeverityError,
				Message:  fmt.Sprintf("fan_in node %q has sources %v but no matching parallel node", f.node.ID, f.sorted),
				Location: f.node.Source,
				Help:     fmt.Sprintf("add a parallel node with targets: %v", f.sorted),
			})
		}
	}

	return diags
}

// checkNoDuplicateNodes verifies DIP008: no two nodes share the same ID.
func checkNoDuplicateNodes(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	seen := make(map[string]*ir.Node)

	for _, n := range w.Nodes {
		if first, ok := seen[n.ID]; ok {
			diags = append(diags, Diagnostic{
				Code:     DIP008,
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate node ID %q", n.ID),
				Location: n.Source,
				Help:     fmt.Sprintf("first declaration at %s:%d:%d", locFile(first.Source), first.Source.Line, first.Source.Column),
			})
		} else {
			seen[n.ID] = n
		}
	}
	return diags
}

// checkNoDuplicateEdges verifies DIP009: no two edges are identical.
// The dedup key is (From, To, Condition.Raw). Two edges with the same
// endpoints but different conditions are conditional branches, not duplicates.
func checkNoDuplicateEdges(w *ir.Workflow) []Diagnostic {
	type edgeKey struct {
		from, to, condRaw string
	}

	var diags []Diagnostic
	seen := make(map[edgeKey]*ir.Edge)

	for _, e := range w.Edges {
		condRaw := ""
		if e.Condition != nil {
			condRaw = e.Condition.Raw
		}
		key := edgeKey{from: e.From, to: e.To, condRaw: condRaw}
		if first, ok := seen[key]; ok {
			diags = append(diags, Diagnostic{
				Code:     DIP009,
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate edge from %q to %q", e.From, e.To),
				Location: e.Source,
				Help:     fmt.Sprintf("first declaration at %s:%d:%d", locFile(first.Source), first.Source.Line, first.Source.Column),
			})
		} else {
			seen[key] = e
		}
	}
	return diags
}

// --- Helpers ---

// buildNodeSet returns a set of all node IDs in the workflow.
func buildNodeSet(w *ir.Workflow) map[string]bool {
	set := make(map[string]bool, len(w.Nodes))
	for _, n := range w.Nodes {
		set[n.ID] = true
	}
	return set
}

// closestNodeID returns the node ID most similar to the given name,
// or "" if no node is within edit distance 2.
func closestNodeID(w *ir.Workflow, name string) string {
	bestDist := 3 // threshold: Levenshtein ≤ 2
	bestID := ""
	for _, n := range w.Nodes {
		d := levenshtein(name, n.ID)
		if d < bestDist {
			bestDist = d
			bestID = n.ID
		}
	}
	return bestID
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two rows for space efficiency.
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			m := ins
			if del < m {
				m = del
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// slicesEqual returns true if two sorted string slices are element-wise equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// locFile returns the file from a SourceLocation, defaulting to "<unknown>".
func locFile(loc ir.SourceLocation) string {
	if loc.File == "" {
		return "<unknown>"
	}
	return loc.File
}
