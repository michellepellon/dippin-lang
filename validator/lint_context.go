package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// varRefPattern matches ${...} variable references in prompt text.
var varRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// lintUndefinedVariables checks DIP106: ${variable} references in prompts
// must use known namespace prefixes (ctx., graph., params.). References without
// a recognized namespace are flagged.
func lintUndefinedVariables(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		diags = append(diags, checkNodeVarRefs(n)...)
	}
	return diags
}

// checkNodeVarRefs checks variable references in a single node's prompt.
func checkNodeVarRefs(n *ir.Node) []Diagnostic {
	prompt := nodePrompt(n)
	if prompt == "" {
		return nil
	}
	var diags []Diagnostic
	matches := varRefPattern.FindAllStringSubmatch(prompt, -1)
	for _, m := range matches {
		varRef := m[1]
		parts := strings.SplitN(varRef, ".", 2)
		if len(parts) < 2 || !knownNamespaces[parts[0]] {
			diags = append(diags, Diagnostic{
				Code:     DIP106,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q references undefined variable ${%s}", n.ID, varRef),
				Location: n.Source,
				Help:     fmt.Sprintf("use a namespaced variable like ${ctx.%s}, ${graph.%s}, or ${params.%s}", varRef, varRef, varRef),
			})
		}
	}
	return diags
}

// lintUnusedWrites checks DIP107: context keys declared in a node's writes:
// that are not referenced in any other node's reads:. These are dead outputs
// that may indicate unused work.
func lintUnusedWrites(w *ir.Workflow) []Diagnostic {
	allReads := collectAllReads(w)

	var diags []Diagnostic
	for _, n := range w.Nodes {
		for _, key := range n.IO.Writes {
			if !allReads[key] {
				diags = append(diags, Diagnostic{
					Code:     DIP107,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("node %q writes context key %q but no node declares it in reads", n.ID, key),
					Location: n.Source,
					Help:     fmt.Sprintf("add reads: %s to a downstream node, or remove this write if unused", key),
				})
			}
		}
	}
	return diags
}

// collectAllReads gathers all read keys across all nodes.
func collectAllReads(w *ir.Workflow) map[string]bool {
	allReads := make(map[string]bool)
	for _, n := range w.Nodes {
		for _, key := range n.IO.Reads {
			allReads[key] = true
		}
	}
	return allReads
}

// lintReadsWithoutUpstreamWrites checks DIP112: keys in a node's reads: list
// should appear in the writes: list of at least one upstream node (reachable
// via forward edges from start). This uses a simple flow analysis — for each
// node, compute the set of keys written by upstream nodes, then check reads.
func lintReadsWithoutUpstreamWrites(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	adj := buildForwardAdjacency(w)
	available := computeAvailableWrites(w, adj)
	return checkUnprovidedReads(w, available)
}

// computeAvailableWrites performs topological traversal and computes
// which context keys are available at each node based on upstream writes.
func computeAvailableWrites(w *ir.Workflow, adj map[string][]string) map[string]map[string]bool {
	inDegree := computeInDegrees(w, adj)
	queue := findRootNodes(w, inDegree)
	available := initializeAvailable(w)

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		addNodeWrites(w, available, curr)
		queue = propagateAndEnqueue(adj, available, inDegree, curr, queue)
	}

	return available
}

// addNodeWrites adds a node's write keys to its available set.
func addNodeWrites(w *ir.Workflow, available map[string]map[string]bool, nodeID string) {
	if n := w.Node(nodeID); n != nil {
		for _, key := range n.IO.Writes {
			available[nodeID][key] = true
		}
	}
}

// propagateAndEnqueue propagates available keys to successors and enqueues those with zero in-degree.
func propagateAndEnqueue(adj map[string][]string, available map[string]map[string]bool, inDegree map[string]int, curr string, queue []string) []string {
	for _, next := range adj[curr] {
		propagateKeys(available, curr, next)
		inDegree[next]--
		if inDegree[next] == 0 {
			queue = append(queue, next)
		}
	}
	return queue
}

// computeInDegrees calculates the in-degree for each node considering
// both explicit edges and implicit parallel/fan_in edges.
func computeInDegrees(w *ir.Workflow, adj map[string][]string) map[string]int {
	inDegree := make(map[string]int)
	for _, n := range w.Nodes {
		inDegree[n.ID] = 0
	}
	countExplicitInDegrees(w, inDegree)
	countImplicitInDegrees(w, inDegree)
	return inDegree
}

// countExplicitInDegrees counts in-degrees from non-restart edges.
func countExplicitInDegrees(w *ir.Workflow, inDegree map[string]int) {
	for _, e := range w.Edges {
		if !e.Restart {
			inDegree[e.To]++
		}
	}
}

// countImplicitInDegrees counts in-degrees from parallel/fan_in implicit edges.
func countImplicitInDegrees(w *ir.Workflow, inDegree map[string]int) {
	for _, n := range w.Nodes {
		switch cfg := n.Config.(type) {
		case ir.ParallelConfig:
			for _, t := range cfg.Targets {
				inDegree[t]++
			}
		case ir.FanInConfig:
			inDegree[n.ID] += len(cfg.Sources)
		}
	}
}

// findRootNodes returns all nodes with in-degree 0 (entry points for traversal).
func findRootNodes(w *ir.Workflow, inDegree map[string]int) []string {
	var roots []string
	for _, n := range w.Nodes {
		if inDegree[n.ID] == 0 {
			roots = append(roots, n.ID)
		}
	}
	return roots
}

// initializeAvailable creates the availability map with empty sets for each node.
func initializeAvailable(w *ir.Workflow) map[string]map[string]bool {
	available := make(map[string]map[string]bool)
	for _, n := range w.Nodes {
		available[n.ID] = make(map[string]bool)
	}
	return available
}

// propagateKeys copies all available keys from source to destination node.
func propagateKeys(available map[string]map[string]bool, from, to string) {
	for key := range available[from] {
		available[to][key] = true
	}
}

// checkUnprovidedReads generates diagnostics for reads that have no upstream write.
func checkUnprovidedReads(w *ir.Workflow, available map[string]map[string]bool) []Diagnostic {
	var diags []Diagnostic

	for _, n := range w.Nodes {
		for _, key := range n.IO.Reads {
			if !available[n.ID][key] {
				diags = append(diags, Diagnostic{
					Code:     DIP112,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("node %q reads context key %q but no upstream node declares it in writes", n.ID, key),
					Location: n.Source,
					Help:     fmt.Sprintf("add writes: %s to an upstream node, or the key may be auto-injected at runtime", key),
				})
			}
		}
	}

	return diags
}

// nodePrompt extracts the prompt text from a node if it has one.
func nodePrompt(n *ir.Node) string {
	switch cfg := n.Config.(type) {
	case ir.AgentConfig:
		return cfg.Prompt
	default:
		return ""
	}
}
