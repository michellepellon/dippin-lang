// ABOUTME: Flattens subgraph ref nodes by inlining referenced workflows.
// ABOUTME: Produces a single flat ir.Workflow with all refs resolved.
package flatten

import (
	"github.com/2389-research/dippin-lang/ir"
)

// Resolver loads a referenced workflow by path.
type Resolver interface {
	Resolve(refPath string, relativeTo string) (*ir.Workflow, error)
}

// Options controls flattening behavior.
type Options struct {
	MaxDepth int // default 10; catches circular refs
}

const defaultMaxDepth = 10

// Flatten resolves all subgraph ref nodes in w by inlining the referenced
// workflows. If w contains no subgraph refs, it returns a shallow copy.
func Flatten(w *ir.Workflow, resolve Resolver, opts Options) (*ir.Workflow, error) {
	if !hasSubgraphRefs(w) {
		return copyWorkflow(w), nil
	}
	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
	}
	return flattenRecursive(w, resolve, maxDepth, 0, nil)
}

// hasSubgraphRefs returns true if any node in w is a subgraph with a Ref.
func hasSubgraphRefs(w *ir.Workflow) bool {
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.SubgraphConfig)
		if ok && cfg.Ref != "" {
			return true
		}
	}
	return false
}

// copyWorkflow returns a shallow copy of w with fresh node and edge slices.
func copyWorkflow(w *ir.Workflow) *ir.Workflow {
	out := *w
	out.Nodes = make([]*ir.Node, len(w.Nodes))
	copy(out.Nodes, w.Nodes)
	out.Edges = make([]*ir.Edge, len(w.Edges))
	copy(out.Edges, w.Edges)
	return &out
}

// flattenRecursive is the placeholder for recursive subgraph inlining.
func flattenRecursive(w *ir.Workflow, _ Resolver, _ int, _ int, _ []string) (*ir.Workflow, error) {
	return copyWorkflow(w), nil // placeholder
}
