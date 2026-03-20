package validator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/2389/dippin/ir"
)

// Lint runs all semantic quality checks (DIP101–DIP112) on the workflow
// and returns all diagnostics found. These are warnings, not errors —
// the workflow can still execute, but the findings indicate likely bugs
// or quality issues.
//
// Lint is independent of Validate. Callers should run both:
//
//	structureResult := validator.Validate(w)
//	lintResult := validator.Lint(w)
func Lint(w *ir.Workflow) Result {
	var diags []Diagnostic

	diags = append(diags, lintConditionalReachability(w)...)
	diags = append(diags, lintDefaultEdge(w)...)
	diags = append(diags, lintOverlappingConditions(w)...)
	diags = append(diags, lintUnboundedRetry(w)...)
	diags = append(diags, lintSuccessPath(w)...)
	diags = append(diags, lintUndefinedVariables(w)...)
	diags = append(diags, lintUnusedWrites(w)...)
	diags = append(diags, lintModelProvider(w)...)
	diags = append(diags, lintNamespaceCollisions(w)...)
	diags = append(diags, lintEmptyPrompts(w)...)
	diags = append(diags, lintToolTimeout(w)...)
	diags = append(diags, lintReadsWithoutUpstreamWrites(w)...)

	return Result{Diagnostics: diags}
}

// lintConditionalReachability checks DIP101: nodes that are only reachable
// through conditional edges may be unreachable at runtime if conditions are
// not satisfied. A node is flagged if ALL of its incoming edges are conditional
// (have a non-nil Condition), meaning there is no guaranteed path to it.
func lintConditionalReachability(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	// Build a map of incoming edges per node.
	incoming := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		incoming[e.To] = append(incoming[e.To], e)
	}

	for _, n := range w.Nodes {
		// Start node is always reachable by definition.
		if n.ID == w.Start {
			continue
		}
		edges := incoming[n.ID]
		if len(edges) == 0 {
			// No incoming edges at all — DIP004 handles this.
			continue
		}
		allConditional := true
		for _, e := range edges {
			if e.Condition == nil {
				allConditional = false
				break
			}
		}
		if allConditional {
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

		hasConditional := false
		hasUnconditional := false
		for _, e := range outgoing {
			if e.Condition != nil {
				hasConditional = true
			} else {
				hasUnconditional = true
			}
		}

		// Only flag if there are conditional edges but no unconditional fallback.
		if hasConditional && !hasUnconditional {
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

// lintOverlappingConditions checks DIP103: multiple edges from the same node
// with conditions that compare the same variable to the same value using "=".
// This indicates contradictory or duplicated routing logic.
func lintOverlappingConditions(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	// Group edges by source node.
	edgesBySource := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		if e.Condition != nil {
			edgesBySource[e.From] = append(edgesBySource[e.From], e)
		}
	}

	for from, edges := range edgesBySource {
		// Extract top-level equality comparisons from each edge condition.
		type condKey struct {
			variable string
			op       string
			value    string
		}

		seen := make(map[condKey]*ir.Edge)
		for _, e := range edges {
			comparisons := extractComparisons(e.Condition.Parsed)
			for _, cmp := range comparisons {
				key := condKey{variable: cmp.Variable, op: cmp.Op, value: cmp.Value}
				if first, ok := seen[key]; ok {
					diags = append(diags, Diagnostic{
						Code:     DIP103,
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("node %q has overlapping conditions: edges to %q and %q both check %s %s %s", from, first.To, e.To, cmp.Variable, cmp.Op, cmp.Value),
						Location: e.Source,
						Help:     "review the conditions to ensure they route to different targets for different states",
					})
				} else {
					seen[key] = e
				}
			}
		}
	}
	return diags
}

// extractComparisons recursively extracts all CondCompare nodes from a
// condition expression tree. This flattens AND/OR/NOT to find the leaf comparisons.
func extractComparisons(expr ir.ConditionExpr) []ir.CondCompare {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case ir.CondCompare:
		return []ir.CondCompare{e}
	case ir.CondAnd:
		return append(extractComparisons(e.Left), extractComparisons(e.Right)...)
	case ir.CondOr:
		return append(extractComparisons(e.Left), extractComparisons(e.Right)...)
	case ir.CondNot:
		return extractComparisons(e.Inner)
	default:
		return nil
	}
}

// lintUnboundedRetry checks DIP104: nodes with retry configuration that have
// no max_retries limit and no fallback target. This could cause infinite retry
// loops at runtime.
func lintUnboundedRetry(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		r := n.Retry
		// Only flag nodes that have some retry config but no bounds.
		hasRetryConfig := r.Policy != "" || r.RetryTarget != ""
		if hasRetryConfig && r.MaxRetries == 0 && r.FallbackTarget == "" {
			diags = append(diags, Diagnostic{
				Code:     DIP104,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has retry configuration but no max_retries or fallback_target", n.ID),
				Location: n.Source,
				Help:     "set max_retries to limit retries, or add a fallback_target for graceful degradation",
			})
		}
	}
	return diags
}

// lintSuccessPath checks DIP105: there must be at least one path from the
// start node to the exit node using only non-restart edges. If no such path
// exists, the workflow can never complete normally.
func lintSuccessPath(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Exit == "" {
		return nil
	}
	if w.Node(w.Start) == nil || w.Node(w.Exit) == nil {
		return nil
	}

	// BFS from start, following only non-restart edges.
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if !e.Restart {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}
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
		if curr == w.Exit {
			return nil // Found a path.
		}
		for _, next := range adj[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	return []Diagnostic{{
		Code:     DIP105,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("no forward path from start node %q to exit node %q (excluding restart edges)", w.Start, w.Exit),
		Help:     "ensure there is at least one non-restart path from start to exit",
	}}
}

// varRefPattern matches ${...} variable references in prompt text.
var varRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// knownNamespaces lists the valid namespace prefixes for variable references.
// Per §8.2 of the Dippin spec: ctx. (runtime context), graph. (workflow-level
// attributes), params. (module parameters for composition).
var knownNamespaces = map[string]bool{
	"ctx":    true,
	"graph":  true,
	"params": true,
}

// lintUndefinedVariables checks DIP106: ${variable} references in prompts
// must use known namespace prefixes (ctx., graph., params.). References without
// a recognized namespace are flagged.
func lintUndefinedVariables(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		prompt := nodePrompt(n)
		if prompt == "" {
			continue
		}
		matches := varRefPattern.FindAllStringSubmatch(prompt, -1)
		for _, m := range matches {
			varRef := m[1] // The captured group inside ${...}
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
	}
	return diags
}

// lintUnusedWrites checks DIP107: context keys declared in a node's writes:
// that are not referenced in any other node's reads:. These are dead outputs
// that may indicate unused work.
func lintUnusedWrites(w *ir.Workflow) []Diagnostic {
	// Collect all reads across all nodes.
	allReads := make(map[string]bool)
	for _, n := range w.Nodes {
		for _, key := range n.IO.Reads {
			allReads[key] = true
		}
	}

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

// knownModelProviders lists known valid model/provider combinations.
// This is a best-effort catalog — unknown combinations produce a warning,
// not an error, since new models may be added at any time.
var knownModelProviders = map[string]map[string]bool{
	"anthropic": {
		"claude-opus-4-6":        true,
		"claude-sonnet-4-6":      true,
		"claude-haiku-3-5":       true,
		"claude-opus-4-20250116": true,
	},
	"openai": {
		"gpt-5.4":       true,
		"gpt-5.3-codex": true,
		"gpt-4o":        true,
		"gpt-4o-mini":   true,
		"o3":            true,
		"o4-mini":       true,
	},
}

// lintModelProvider checks DIP108: model/provider combinations should be
// in the known catalog. Unknown combinations may indicate typos.
func lintModelProvider(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}

		model := cfg.Model
		provider := cfg.Provider

		// Use workflow defaults as fallback.
		if model == "" {
			model = w.Defaults.Model
		}
		if provider == "" {
			provider = w.Defaults.Provider
		}

		// Only check if both are specified.
		if model == "" || provider == "" {
			continue
		}

		providerModels, providerKnown := knownModelProviders[provider]
		if !providerKnown {
			diags = append(diags, Diagnostic{
				Code:     DIP108,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q uses unknown provider %q", n.ID, provider),
				Location: n.Source,
				Help:     fmt.Sprintf("known providers: %s", knownProviderList()),
			})
			continue
		}
		if !providerModels[model] {
			diags = append(diags, Diagnostic{
				Code:     DIP108,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q uses unknown model %q for provider %q", n.ID, model, provider),
				Location: n.Source,
				Help:     fmt.Sprintf("known models for %s: %s", provider, knownModelList(provider)),
			})
		}
	}
	return diags
}

// knownProviderList returns a sorted comma-separated list of known providers.
func knownProviderList() string {
	providers := make([]string, 0, len(knownModelProviders))
	for p := range knownModelProviders {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return strings.Join(providers, ", ")
}

// knownModelList returns a sorted comma-separated list of known models for a provider.
func knownModelList(provider string) string {
	models := knownModelProviders[provider]
	list := make([]string, 0, len(models))
	for m := range models {
		list = append(list, m)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}

// lintNamespaceCollisions checks DIP109: multiple subgraph nodes referencing
// the same file without different parameters could cause namespace collisions
// when expanded.
func lintNamespaceCollisions(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	type subgraphRef struct {
		node *ir.Node
		ref  string
	}

	seen := make(map[string]*ir.Node)
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.SubgraphConfig)
		if !ok {
			continue
		}
		if first, exists := seen[cfg.Ref]; exists {
			diags = append(diags, Diagnostic{
				Code:     DIP109,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("nodes %q and %q both reference subgraph %q, which may cause namespace collisions", first.ID, n.ID, cfg.Ref),
				Location: n.Source,
				Help:     "use distinct node IDs and ensure imported names do not collide after expansion",
			})
		} else {
			seen[cfg.Ref] = n
		}
	}
	return diags
}

// lintEmptyPrompts checks DIP110: agent nodes should have a non-empty prompt.
// An agent without a prompt has nothing to send to the LLM.
func lintEmptyPrompts(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if strings.TrimSpace(cfg.Prompt) == "" {
			diags = append(diags, Diagnostic{
				Code:     DIP110,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("agent node %q has an empty prompt", n.ID),
				Location: n.Source,
				Help:     "add a prompt: field with instructions for the LLM",
			})
		}
	}
	return diags
}

// lintToolTimeout checks DIP111: tool nodes should have a timeout configured.
// Without a timeout, a hanging tool command could block the entire pipeline.
func lintToolTimeout(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.ToolConfig)
		if !ok {
			continue
		}
		if cfg.Timeout == 0 {
			diags = append(diags, Diagnostic{
				Code:     DIP111,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("tool node %q has no timeout configured", n.ID),
				Location: n.Source,
				Help:     "add a timeout (e.g., timeout: 60s) to prevent hanging commands",
			})
		}
	}
	return diags
}

// lintReadsWithoutUpstreamWrites checks DIP112: keys in a node's reads: list
// should appear in the writes: list of at least one upstream node (reachable
// via forward edges from start). This uses a simple flow analysis — for each
// node, compute the set of keys written by upstream nodes, then check reads.
func lintReadsWithoutUpstreamWrites(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	// Build forward adjacency (non-restart edges).
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if !e.Restart {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}
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

	// Topological order via BFS (Kahn's algorithm).
	inDegree := make(map[string]int)
	for _, n := range w.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range w.Edges {
		if !e.Restart {
			inDegree[e.To]++
		}
	}
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

	queue := []string{}
	for _, n := range w.Nodes {
		if inDegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}

	// For each node, compute the set of keys available from upstream writes.
	available := make(map[string]map[string]bool)
	for _, n := range w.Nodes {
		available[n.ID] = make(map[string]bool)
	}

	var order []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)

		// Add this node's writes to what's available for downstream.
		n := w.Node(curr)
		if n != nil {
			for _, key := range n.IO.Writes {
				available[curr][key] = true
			}
		}

		for _, next := range adj[curr] {
			// Merge current node's available keys into the next node's available set.
			for key := range available[curr] {
				available[next][key] = true
			}
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

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
