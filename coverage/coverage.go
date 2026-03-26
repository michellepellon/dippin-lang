// Package coverage analyzes workflow edge coverage and reachability.
//
// It examines tool nodes to determine whether their possible outputs are
// covered by outgoing edge conditions, checks that all nodes are reachable
// from start, and verifies that all reachable nodes have a path to exit.
package coverage

import (
	"regexp"
	"sort"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/simulate"
)

// Report is the top-level result of a coverage analysis.
type Report struct {
	Nodes        map[string]NodeCoverage `json:"nodes"`
	Reachability ReachabilityReport      `json:"reachability"`
	Termination  TerminationReport       `json:"termination"`
}

// NodeCoverage describes edge coverage for a single node.
type NodeCoverage struct {
	NodeID           string   `json:"node_id"`
	Status           string   `json:"status"` // "covered", "partial", "unknown", "no_conditions"
	DeclaredOutputs  []string `json:"declared_outputs,omitempty"`
	ExtractedOutputs []string `json:"extracted_outputs,omitempty"`
	EdgeConditions   []string `json:"edge_conditions"`
	MissingEdges     []string `json:"missing_edges,omitempty"`
	HasFallback      bool     `json:"has_fallback"`
}

// ReachabilityReport summarizes forward reachability from the start node.
type ReachabilityReport struct {
	TotalNodes       int      `json:"total_nodes"`
	ReachableNodes   int      `json:"reachable_nodes"`
	UnreachableNodes []string `json:"unreachable_nodes,omitempty"`
}

// TerminationReport summarizes whether all reachable nodes can reach exit.
type TerminationReport struct {
	AllPathsTerminate bool     `json:"all_paths_terminate"`
	SinkNodes         []string `json:"sink_nodes,omitempty"`
}

// outputPatterns matches printf/echo string literals in shell commands.
var outputPatterns = []*regexp.Regexp{
	regexp.MustCompile(`printf\s+'([^']+)'`),
	regexp.MustCompile(`printf\s+"([^"]+)"`),
	regexp.MustCompile(`printf\s+'%s'\s+'([^']+)'`),
	regexp.MustCompile(`echo\s+'([^']+)'`),
}

// Analyze runs coverage analysis on a workflow and returns a report.
func Analyze(w *ir.Workflow) *Report {
	// Parse edge conditions into AST form. Errors are non-fatal —
	// unparseable conditions stay with Parsed=nil and are skipped
	// by appendConditionValues (the default case returns out unchanged).
	_ = simulate.EnsureConditionsParsed(w)

	r := &Report{
		Nodes: make(map[string]NodeCoverage),
	}
	analyzeNodes(w, r)
	r.Reachability = analyzeReachability(w)
	r.Termination = analyzeTermination(w)
	return r
}

// analyzeNodes evaluates edge coverage for each tool node.
func analyzeNodes(w *ir.Workflow, r *Report) {
	for _, n := range w.Nodes {
		if _, ok := n.Config.(ir.ToolConfig); !ok {
			continue
		}
		r.Nodes[n.ID] = analyzeToolNode(w, n)
	}
}

// analyzeToolNode computes coverage for a single tool node.
func analyzeToolNode(w *ir.Workflow, n *ir.Node) NodeCoverage {
	cfg, ok := n.Config.(ir.ToolConfig)
	if !ok {
		return NodeCoverage{NodeID: n.ID}
	}
	edges := w.EdgesFrom(n.ID)
	conditions, hasFallback := collectEdgeConditions(edges)

	cov := NodeCoverage{
		NodeID:          n.ID,
		DeclaredOutputs: cfg.Outputs,
		EdgeConditions:  conditions,
		HasFallback:     hasFallback,
	}

	// Prefer declared outputs over regex extraction.
	outputs := cfg.Outputs
	if len(outputs) == 0 {
		outputs = extractToolOutputs(cfg.Command)
		cov.ExtractedOutputs = outputs
	}

	cov.MissingEdges = findMissingEdges(outputs, conditions)
	cov.Status = determineStatus(cov)
	return cov
}

// determineStatus assigns a coverage status based on the node's edges and outputs.
func determineStatus(cov NodeCoverage) string {
	if len(cov.EdgeConditions) == 0 {
		return "no_conditions"
	}
	if len(cov.ExtractedOutputs) == 0 {
		return "unknown"
	}
	if len(cov.MissingEdges) == 0 || cov.HasFallback {
		return "covered"
	}
	return "partial"
}

// findMissingEdges returns extracted outputs that have no matching edge condition.
func findMissingEdges(outputs, conditions []string) []string {
	condSet := toSet(conditions)
	var missing []string
	for _, o := range outputs {
		if !condSet[o] {
			missing = append(missing, o)
		}
	}
	return missing
}

// toSet converts a string slice to a map for O(1) lookups.
func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

// extractToolOutputs finds printf/echo string literals in a shell command.
func extractToolOutputs(command string) []string {
	seen := make(map[string]bool)
	var outputs []string
	for _, pat := range outputPatterns {
		for _, m := range pat.FindAllStringSubmatch(command, -1) {
			val := m[1]
			if !seen[val] && !isFormatSpecifier(val) {
				seen[val] = true
				outputs = append(outputs, val)
			}
		}
	}
	return outputs
}

// isFormatSpecifier returns true if the string looks like a printf format string.
func isFormatSpecifier(s string) bool {
	return strings.HasPrefix(s, "%") && len(s) <= 3
}

// collectEdgeConditions extracts condition values from edges and detects fallbacks.
func collectEdgeConditions(edges []*ir.Edge) (conditions []string, hasFallback bool) {
	for _, e := range edges {
		if e.Condition == nil {
			hasFallback = true
			continue
		}
		conditions = appendConditionValues(conditions, e.Condition.Parsed)
	}
	return conditions, hasFallback
}

// appendConditionValues extracts comparison values from a condition expression tree.
func appendConditionValues(out []string, expr ir.ConditionExpr) []string {
	switch e := expr.(type) {
	case ir.CondCompare:
		return append(out, e.Value)
	case ir.CondAnd:
		out = appendConditionValues(out, e.Left)
		return appendConditionValues(out, e.Right)
	case ir.CondOr:
		out = appendConditionValues(out, e.Left)
		return appendConditionValues(out, e.Right)
	case ir.CondNot:
		return appendConditionValues(out, e.Inner)
	default:
		return out
	}
}

// analyzeReachability performs BFS from start to find unreachable nodes.
func analyzeReachability(w *ir.Workflow) ReachabilityReport {
	adj := buildForwardAdjacency(w)
	reachable := bfsFrom(w.Start, adj)

	report := ReachabilityReport{
		TotalNodes:     len(w.Nodes),
		ReachableNodes: len(reachable),
	}
	report.UnreachableNodes = findUnreachableNodes(w, reachable)
	return report
}

// findUnreachableNodes returns sorted IDs of nodes not in the reachable set.
func findUnreachableNodes(w *ir.Workflow, reachable map[string]bool) []string {
	var unreachable []string
	for _, n := range w.Nodes {
		if !reachable[n.ID] {
			unreachable = append(unreachable, n.ID)
		}
	}
	sort.Strings(unreachable)
	return unreachable
}

// analyzeTermination checks that all reachable nodes can reach exit.
func analyzeTermination(w *ir.Workflow) TerminationReport {
	fwd := buildForwardAdjacency(w)
	reachable := bfsFrom(w.Start, fwd)

	rev := buildReverseAdjacency(w)
	canReachExit := bfsFrom(w.Exit, rev)

	sinks := findSinkNodes(reachable, canReachExit)
	return TerminationReport{
		AllPathsTerminate: len(sinks) == 0,
		SinkNodes:         sinks,
	}
}

// findSinkNodes returns sorted IDs of nodes reachable from start but not from exit (reverse).
func findSinkNodes(reachable, canReachExit map[string]bool) []string {
	var sinks []string
	for id := range reachable {
		if !canReachExit[id] {
			sinks = append(sinks, id)
		}
	}
	sort.Strings(sinks)
	return sinks
}

// buildForwardAdjacency creates a forward adjacency map from all edges.
func buildForwardAdjacency(w *ir.Workflow) map[string][]string {
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	addImplicitEdges(adj, w)
	return adj
}

// buildReverseAdjacency creates a reverse adjacency map from all edges.
func buildReverseAdjacency(w *ir.Workflow) map[string][]string {
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		adj[e.To] = append(adj[e.To], e.From)
	}
	addReverseImplicitEdges(adj, w)
	return adj
}

// addImplicitEdges adds forward edges from parallel and fan_in nodes.
func addImplicitEdges(adj map[string][]string, w *ir.Workflow) {
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
}

// addReverseImplicitEdges adds reverse edges from parallel and fan_in nodes.
func addReverseImplicitEdges(adj map[string][]string, w *ir.Workflow) {
	for _, n := range w.Nodes {
		addReverseNodeEdges(adj, n)
	}
}

// addReverseNodeEdges adds reverse edges for a single parallel or fan_in node.
func addReverseNodeEdges(adj map[string][]string, n *ir.Node) {
	switch cfg := n.Config.(type) {
	case ir.ParallelConfig:
		for _, t := range cfg.Targets {
			adj[t] = append(adj[t], n.ID)
		}
	case ir.FanInConfig:
		adj[n.ID] = append(adj[n.ID], cfg.Sources...)
	}
}

// bfsFrom returns the set of nodes reachable from start via BFS.
func bfsFrom(start string, adj map[string][]string) map[string]bool {
	visited := make(map[string]bool)
	queue := []string{start}
	visited[start] = true

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
	return visited
}
