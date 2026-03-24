package validator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
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
	diags = append(diags, lintRetryPolicy(w)...)
	diags = append(diags, lintFidelity(w)...)
	diags = append(diags, lintGoalGateFallback(w)...)
	diags = append(diags, lintCompactionThreshold(w)...)
	diags = append(diags, lintOnResume(w)...)
	diags = append(diags, lintStylesheetRefs(w)...)

	return Result{Diagnostics: diags}
}

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

// lintOverlappingConditions checks DIP103: multiple edges from the same node
// with conditions that compare the same variable to the same value using "=".
// This indicates contradictory or duplicated routing logic.
func lintOverlappingConditions(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	edgesBySource := groupConditionalEdgesBySource(w)
	for from, edges := range edgesBySource {
		diags = append(diags, findOverlaps(from, edges)...)
	}
	return diags
}

// groupConditionalEdgesBySource groups edges with conditions by their source node.
func groupConditionalEdgesBySource(w *ir.Workflow) map[string][]*ir.Edge {
	edgesBySource := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		if e.Condition != nil {
			edgesBySource[e.From] = append(edgesBySource[e.From], e)
		}
	}
	return edgesBySource
}

// condKey identifies a unique condition comparison.
type condKey struct {
	variable string
	op       string
	value    string
}

// findOverlaps detects duplicate condition comparisons among edges from the same node.
func findOverlaps(from string, edges []*ir.Edge) []Diagnostic {
	var diags []Diagnostic
	seen := make(map[condKey]*ir.Edge)
	for _, e := range edges {
		comparisons := extractComparisons(e.Condition.Parsed)
		diags = append(diags, checkComparisonOverlaps(from, e, comparisons, seen)...)
	}
	return diags
}

// checkComparisonOverlaps checks a set of comparisons against previously seen ones.
func checkComparisonOverlaps(from string, e *ir.Edge, comparisons []ir.CondCompare, seen map[condKey]*ir.Edge) []Diagnostic {
	var diags []Diagnostic
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
	return diags
}

// extractComparisons recursively extracts all CondCompare nodes from a
// condition expression tree. This flattens AND/OR/NOT to find the leaf comparisons.
func extractComparisons(expr ir.ConditionExpr) []ir.CondCompare {
	switch e := expr.(type) {
	case ir.CondCompare:
		return []ir.CondCompare{e}
	case ir.CondAnd:
		return extractBinaryComparisons(e.Left, e.Right)
	case ir.CondOr:
		return extractBinaryComparisons(e.Left, e.Right)
	case ir.CondNot:
		return extractComparisons(e.Inner)
	default:
		return nil
	}
}

// extractBinaryComparisons extracts comparisons from both sides of a binary condition.
func extractBinaryComparisons(left, right ir.ConditionExpr) []ir.CondCompare {
	return append(extractComparisons(left), extractComparisons(right)...)
}

// lintUnboundedRetry checks DIP104: nodes with retry configuration that have
// no max_retries limit and no fallback target. This could cause infinite retry
// loops at runtime.
func lintUnboundedRetry(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if isUnboundedRetry(n.Retry) {
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

// isUnboundedRetry returns true if retry config exists but has no bounds.
func isUnboundedRetry(r ir.RetryConfig) bool {
	hasRetryConfig := r.Policy != "" || r.RetryTarget != ""
	return hasRetryConfig && r.MaxRetries == 0 && r.FallbackTarget == ""
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

// knownModelProviders lists known valid model/provider combinations.
// This is a best-effort catalog — unknown combinations produce a warning,
// not an error, since new models may be added at any time.
var knownModelProviders = map[string]map[string]bool{
	"anthropic": {
		"claude-opus-4-6":        true,
		"claude-sonnet-4-6":      true,
		"claude-haiku-4-5":       true,
		"claude-haiku-3-5":       true,
		"claude-opus-4-20250116": true,
	},
	"openai": {
		"gpt-5.4":       true,
		"gpt-5.3-codex": true,
		"gpt-5.2":       true,
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
		diags = append(diags, checkNodeModelProvider(w, n)...)
	}
	return diags
}

// checkNodeModelProvider validates the model/provider for a single node.
func checkNodeModelProvider(w *ir.Workflow, n *ir.Node) []Diagnostic {
	cfg, ok := n.Config.(ir.AgentConfig)
	if !ok {
		return nil
	}
	model, provider := resolveModelProvider(cfg, w)
	if model == "" || provider == "" {
		return nil
	}
	return validateModelProvider(n, model, provider)
}

// resolveModelProvider resolves model and provider using node config and workflow defaults.
func resolveModelProvider(cfg ir.AgentConfig, w *ir.Workflow) (model, provider string) {
	model = cfg.Model
	provider = cfg.Provider
	if model == "" {
		model = w.Defaults.Model
	}
	if provider == "" {
		provider = w.Defaults.Provider
	}
	return
}

// validateModelProvider checks if a model/provider combination is known.
func validateModelProvider(n *ir.Node, model, provider string) []Diagnostic {
	providerModels, providerKnown := knownModelProviders[provider]
	if !providerKnown {
		return []Diagnostic{{
			Code:     DIP108,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("node %q uses unknown provider %q", n.ID, provider),
			Location: n.Source,
			Help:     fmt.Sprintf("known providers: %s", knownProviderList()),
		}}
	}
	if !providerModels[model] {
		return []Diagnostic{{
			Code:     DIP108,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("node %q uses unknown model %q for provider %q", n.ID, model, provider),
			Location: n.Source,
			Help:     fmt.Sprintf("known models for %s: %s", provider, knownModelList(provider)),
		}}
	}
	return nil
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

	adj := buildForwardAdjacency(w)
	available := computeAvailableWrites(w, adj)
	return checkUnprovidedReads(w, available)
}

// buildForwardAdjacency builds a forward adjacency map for non-restart edges,
// including implicit edges from parallel and fan_in nodes.
func buildForwardAdjacency(w *ir.Workflow) map[string][]string {
	adj := buildNonRestartAdjacency(w)
	addParallelFanInEdges(adj, w)
	return adj
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

// validRetryPolicies is the set of retry policy names recognized by Tracker's engine.
var validRetryPolicies = map[string]bool{
	"standard":   true,
	"aggressive": true,
	"patient":    true,
	"linear":     true,
	"none":       true,
}

// lintRetryPolicy checks DIP113: retry_policy must be one of the known policy names.
func lintRetryPolicy(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	diags = append(diags, checkDefaultRetryPolicy(w)...)
	diags = append(diags, checkNodeRetryPolicies(w)...)
	return diags
}

// checkDefaultRetryPolicy checks the workflow-level default retry policy.
func checkDefaultRetryPolicy(w *ir.Workflow) []Diagnostic {
	if p := w.Defaults.RetryPolicy; p != "" && !validRetryPolicies[p] {
		return []Diagnostic{{
			Code:     DIP113,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("workflow default retry_policy %q is not a recognized policy name", p),
			Help:     "valid policies: standard, aggressive, patient, linear, none",
		}}
	}
	return nil
}

// checkNodeRetryPolicies checks per-node retry policies.
func checkNodeRetryPolicies(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if p := n.Retry.Policy; p != "" && !validRetryPolicies[p] {
			diags = append(diags, Diagnostic{
				Code:     DIP113,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has retry_policy %q which is not a recognized policy name", n.ID, p),
				Location: n.Source,
				Help:     "valid policies: standard, aggressive, patient, linear, none",
			})
		}
	}
	return diags
}

// validFidelityLevels is the set of fidelity levels recognized by Tracker's engine.
var validFidelityLevels = map[string]bool{
	"full":           true,
	"summary:high":   true,
	"summary:medium": true,
	"summary:low":    true,
	"compact":        true,
	"truncate":       true,
}

// lintFidelity checks DIP114: fidelity must be one of the known levels.
func lintFidelity(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	diags = append(diags, checkDefaultFidelity(w)...)
	diags = append(diags, checkNodeFidelity(w)...)
	return diags
}

// checkDefaultFidelity checks the workflow-level default fidelity.
func checkDefaultFidelity(w *ir.Workflow) []Diagnostic {
	if f := w.Defaults.Fidelity; f != "" && !validFidelityLevels[f] {
		return []Diagnostic{{
			Code:     DIP114,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("workflow default fidelity %q is not a recognized level", f),
			Help:     "valid levels: full, summary:high, summary:medium, summary:low, compact, truncate",
		}}
	}
	return nil
}

// checkNodeFidelity checks per-node fidelity levels.
func checkNodeFidelity(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if f := cfg.Fidelity; f != "" && !validFidelityLevels[f] {
			diags = append(diags, Diagnostic{
				Code:     DIP114,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has fidelity %q which is not a recognized level", n.ID, f),
				Location: n.Source,
				Help:     "valid levels: full, summary:high, summary:medium, summary:low, compact, truncate",
			})
		}
	}
	return diags
}

// lintGoalGateFallback checks DIP115: nodes with goal_gate: true should have
// a retry_target or fallback_target so the pipeline has a recovery path when
// the gate fails.
func lintGoalGateFallback(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if needsGoalGateFallback(n) {
			diags = append(diags, Diagnostic{
				Code:     DIP115,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has goal_gate: true but no retry_target or fallback_target", n.ID),
				Location: n.Source,
				Help:     "add retry_target or fallback_target so the pipeline can recover when the gate fails",
			})
		}
	}
	return diags
}

// needsGoalGateFallback returns true if a node has goal_gate but no recovery path.
func needsGoalGateFallback(n *ir.Node) bool {
	cfg, ok := n.Config.(ir.AgentConfig)
	if !ok || !cfg.GoalGate {
		return false
	}
	return n.Retry.RetryTarget == "" && n.Retry.FallbackTarget == ""
}

// lintCompactionThreshold checks DIP116: compaction_threshold must be in [0.0, 1.0].
func lintCompactionThreshold(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if isInvalidThreshold(cfg.CompactionThreshold) {
			diags = append(diags, Diagnostic{
				Code:     DIP116,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has compaction_threshold %.2f outside valid range [0.0, 1.0]", n.ID, cfg.CompactionThreshold),
				Location: n.Source,
				Help:     "compaction_threshold should be between 0.0 and 1.0",
			})
		}
	}
	return diags
}

// isInvalidThreshold returns true if threshold is set and outside [0, 1].
func isInvalidThreshold(t float64) bool {
	return t != 0 && (t < 0 || t > 1)
}

// validOnResumeValues is the set of valid on_resume values.
var validOnResumeValues = map[string]bool{
	"preserve": true,
	"degrade":  true,
}

// lintOnResume checks DIP116: on_resume must be "preserve" or "degrade",
// and should not be set without fidelity.
func lintOnResume(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	or := w.Defaults.OnResume
	if or == "" {
		return nil
	}
	if !validOnResumeValues[or] {
		diags = append(diags, Diagnostic{
			Code:     DIP116,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("workflow default on_resume %q is not a recognized value", or),
			Help:     "valid values: preserve, degrade",
		})
	}
	if w.Defaults.Fidelity == "" {
		diags = append(diags, Diagnostic{
			Code:     DIP116,
			Severity: SeverityWarning,
			Message:  "on_resume is set but fidelity is not configured",
			Help:     "set fidelity before configuring on_resume behavior",
		})
	}
	return diags
}

// lintStylesheetRefs checks DIP117/DIP118: stylesheet selectors must reference
// existing classes and node IDs.
func lintStylesheetRefs(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	classes, nodeIDs := collectClassesAndIDs(w)
	for _, rule := range w.Stylesheet {
		diags = append(diags, checkSelectorRef(rule, classes, nodeIDs)...)
	}
	return diags
}

// checkSelectorRef validates a single stylesheet rule's selector.
func checkSelectorRef(rule ir.StylesheetRule, classes, nodeIDs map[string]bool) []Diagnostic {
	switch rule.Selector.Kind {
	case "class":
		if !classes[rule.Selector.Value] {
			return []Diagnostic{{
				Code:     DIP117,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("stylesheet references class %q which is not declared on any node", rule.Selector.Value),
				Help:     "add class: " + rule.Selector.Value + " to a node declaration",
			}}
		}
	case "id":
		if !nodeIDs[rule.Selector.Value] {
			return []Diagnostic{{
				Code:     DIP118,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("stylesheet references node ID %q which does not exist", rule.Selector.Value),
				Help:     "check the node ID spelling or add the node",
			}}
		}
	}
	return nil
}

// collectClassesAndIDs builds sets of all declared classes and node IDs.
func collectClassesAndIDs(w *ir.Workflow) (map[string]bool, map[string]bool) {
	classes := make(map[string]bool)
	nodeIDs := make(map[string]bool)
	for _, n := range w.Nodes {
		nodeIDs[n.ID] = true
		for _, c := range n.Classes {
			classes[c] = true
		}
	}
	return classes, nodeIDs
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
