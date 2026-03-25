// Package graph renders terminal-friendly ASCII DAG diagrams from IR workflows.
package graph

import (
	"github.com/2389-research/dippin-lang/ir"
)

// Options controls rendering behavior.
type Options struct {
	Compact bool
}

// LayerInfo exposes the layering for JSON output or testing.
type LayerInfo struct {
	Layers [][]string `json:"layers"`
}

// Render produces a human-readable ASCII DAG of the workflow.
func Render(w *ir.Workflow, opts Options) string {
	layers := assignLayers(w)
	if opts.Compact {
		return renderCompact(layers, w)
	}
	return renderFull(layers, w)
}

// Layers returns the topological layer assignment for the workflow.
func Layers(w *ir.Workflow) LayerInfo {
	return LayerInfo{Layers: assignLayers(w)}
}

// assignLayers performs longest-path layering from the start node.
func assignLayers(w *ir.Workflow) [][]string {
	adj := buildAdjacency(w)
	inDeg := buildInDegree(w, adj)
	dist := longestPaths(w, adj, inDeg)
	return groupByLayer(dist)
}

// buildAdjacency builds a forward adjacency list from edges and implicit connections.
// Restart (back) edges are excluded because they create cycles that prevent
// Kahn's algorithm from assigning layers to downstream nodes.
func buildAdjacency(w *ir.Workflow) map[string][]string {
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if e.Restart {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
	}
	addImplicitEdges(w, adj)
	return adj
}

// addImplicitEdges adds parallel/fan_in implicit edges to the adjacency list.
func addImplicitEdges(w *ir.Workflow, adj map[string][]string) {
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

// buildInDegree computes in-degree for each node.
func buildInDegree(w *ir.Workflow, adj map[string][]string) map[string]int {
	inDeg := make(map[string]int)
	for _, n := range w.Nodes {
		inDeg[n.ID] = 0
	}
	for _, neighbors := range adj {
		for _, to := range neighbors {
			inDeg[to]++
		}
	}
	return inDeg
}

// longestPaths computes longest-path distances using Kahn's algorithm.
func longestPaths(w *ir.Workflow, adj map[string][]string, inDeg map[string]int) map[string]int {
	dist := initDistances(w)
	dist[w.Start] = 0
	queue := collectZeroInDegree(inDeg)
	processQueue(queue, adj, inDeg, dist)
	return dist
}

// initDistances initializes all node distances to -1.
func initDistances(w *ir.Workflow) map[string]int {
	dist := make(map[string]int, len(w.Nodes))
	for _, n := range w.Nodes {
		dist[n.ID] = -1
	}
	return dist
}

// collectZeroInDegree returns all nodes with in-degree 0.
func collectZeroInDegree(inDeg map[string]int) []string {
	var queue []string
	for id, d := range inDeg {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	return queue
}

// processQueue runs BFS in topological order, updating longest-path distances.
func processQueue(queue []string, adj map[string][]string, inDeg map[string]int, dist map[string]int) {
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		queue = relaxNeighbors(cur, adj[cur], inDeg, dist, queue)
	}
}

// relaxNeighbors updates distances for neighbors and enqueues those with zero in-degree.
func relaxNeighbors(cur string, neighbors []string, inDeg map[string]int, dist map[string]int, queue []string) []string {
	for _, nb := range neighbors {
		if dist[cur]+1 > dist[nb] {
			dist[nb] = dist[cur] + 1
		}
		inDeg[nb]--
		if inDeg[nb] == 0 {
			queue = append(queue, nb)
		}
	}
	return queue
}

// groupByLayer groups nodes by their distance value, sorted by layer.
func groupByLayer(dist map[string]int) [][]string {
	maxLayer := findMaxLayer(dist)
	layers := make([][]string, maxLayer+1)
	for id, d := range dist {
		if d >= 0 {
			layers[d] = append(layers[d], id)
		}
	}
	return layers
}

// findMaxLayer returns the maximum distance value.
func findMaxLayer(dist map[string]int) int {
	max := 0
	for _, d := range dist {
		if d > max {
			max = d
		}
	}
	return max
}
