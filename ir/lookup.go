package ir

// Node returns the node with the given ID, or nil if not found.
func (w *Workflow) Node(id string) *Node {
	for _, n := range w.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// EdgesFrom returns all edges originating from the given node ID.
// This includes explicit edges from the workflow's Edges slice, as well as
// implicit edges defined by parallel fan-outs and fan-in joins.
// Redundant unconditional edges are de-duplicated.
func (w *Workflow) EdgesFrom(id string) []*Edge {
	var out []*Edge
	seenTargets := make(map[string]bool)

	for _, e := range w.Edges {
		if e.From == id {
			out = append(out, e)
			if e.Condition == nil {
				seenTargets[e.To] = true
			}
		}
	}

	// Include implicit edges from parallel fan-outs.
	node := w.Node(id)
	if node != nil {
		if cfg, ok := node.Config.(ParallelConfig); ok {
			for _, target := range cfg.Targets {
				if !seenTargets[target] {
					out = append(out, &Edge{From: id, To: target})
					seenTargets[target] = true
				}
			}
		}
	}

	// A node may be a source for a fan-in join.
	for _, n := range w.Nodes {
		if cfg, ok := n.Config.(FanInConfig); ok {
			for _, src := range cfg.Sources {
				if src == id && !seenTargets[n.ID] {
					out = append(out, &Edge{From: id, To: n.ID})
					seenTargets[n.ID] = true
				}
			}
		}
	}

	return out
}

// EdgesTo returns all edges targeting the given node ID.
// This includes explicit edges from the workflow's Edges slice, as well as
// implicit edges defined by parallel fan-outs and fan-in joins.
// Redundant unconditional edges are de-duplicated.
func (w *Workflow) EdgesTo(id string) []*Edge {
	var out []*Edge
	seenSources := make(map[string]bool)

	for _, e := range w.Edges {
		if e.To == id {
			out = append(out, e)
			if e.Condition == nil {
				seenSources[e.From] = true
			}
		}
	}

	// Include implicit edges from parallel fan-outs.
	for _, n := range w.Nodes {
		if cfg, ok := n.Config.(ParallelConfig); ok {
			for _, target := range cfg.Targets {
				if target == id && !seenSources[n.ID] {
					out = append(out, &Edge{From: n.ID, To: id})
					seenSources[n.ID] = true
				}
			}
		}
	}

	// Include implicit edges from fan-in joins.
	node := w.Node(id)
	if node != nil {
		if cfg, ok := node.Config.(FanInConfig); ok {
			for _, src := range cfg.Sources {
				if !seenSources[src] {
					out = append(out, &Edge{From: src, To: id})
					seenSources[src] = true
				}
			}
		}
	}

	return out
}

// NodeIDs returns all node IDs in declaration order.
func (w *Workflow) NodeIDs() []string {
	ids := make([]string, len(w.Nodes))
	for i, n := range w.Nodes {
		ids[i] = n.ID
	}
	return ids
}
