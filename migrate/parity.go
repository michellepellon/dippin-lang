package migrate

import (
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// Difference describes a structural difference between two workflows.
type Difference struct {
	Kind string // "node_missing", "node_extra", "edge_missing", "edge_extra",
	//                "config_mismatch", "kind_mismatch", "start_mismatch",
	//                "exit_mismatch", "defaults_mismatch"
	Message string // Human-readable description
	PathA   string // Location in workflow A (e.g., "node:Validate")
	PathB   string // Location in workflow B (may be empty)
}

// CheckParity compares two workflows for structural equivalence.
// It checks:
//   - Same node IDs and kinds
//   - Same edges (from/to/conditions)
//   - Same start/exit
//   - Compatible node configurations (prompt content modulo whitespace)
//   - Same graph-level defaults
func CheckParity(a, b *ir.Workflow) []Difference {
	var diffs []Difference

	// Start/exit.
	if a.Start != b.Start {
		diffs = append(diffs, Difference{
			Kind:    "start_mismatch",
			Message: fmt.Sprintf("start differs: %q vs %q", a.Start, b.Start),
			PathA:   "workflow.start",
			PathB:   "workflow.start",
		})
	}
	if a.Exit != b.Exit {
		diffs = append(diffs, Difference{
			Kind:    "exit_mismatch",
			Message: fmt.Sprintf("exit differs: %q vs %q", a.Exit, b.Exit),
			PathA:   "workflow.exit",
			PathB:   "workflow.exit",
		})
	}

	// Build node maps.
	aNodes := make(map[string]*ir.Node)
	for _, n := range a.Nodes {
		aNodes[n.ID] = n
	}
	bNodes := make(map[string]*ir.Node)
	for _, n := range b.Nodes {
		bNodes[n.ID] = n
	}

	// Check for missing / extra nodes.
	for id, na := range aNodes {
		nb, ok := bNodes[id]
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "node_missing",
				Message: fmt.Sprintf("node %q present in A but missing from B", id),
				PathA:   "node:" + id,
			})
			continue
		}
		// Kind mismatch.
		if na.Kind != nb.Kind {
			diffs = append(diffs, Difference{
				Kind:    "kind_mismatch",
				Message: fmt.Sprintf("node %q kind: %q vs %q", id, na.Kind, nb.Kind),
				PathA:   "node:" + id,
				PathB:   "node:" + id,
			})
		}
		// Config comparison (per kind).
		diffs = append(diffs, compareConfigs(id, na, nb)...)
	}

	for id := range bNodes {
		if _, ok := aNodes[id]; !ok {
			diffs = append(diffs, Difference{
				Kind:    "node_extra",
				Message: fmt.Sprintf("node %q present in B but missing from A", id),
				PathB:   "node:" + id,
			})
		}
	}

	// Check edges.
	aEdges := edgeSet(a.Edges)
	bEdges := edgeSet(b.Edges)

	for key := range aEdges {
		if _, ok := bEdges[key]; !ok {
			diffs = append(diffs, Difference{
				Kind:    "edge_missing",
				Message: fmt.Sprintf("edge %s present in A but missing from B", key),
				PathA:   "edge:" + key,
			})
		}
	}
	for key := range bEdges {
		if _, ok := aEdges[key]; !ok {
			diffs = append(diffs, Difference{
				Kind:    "edge_extra",
				Message: fmt.Sprintf("edge %s present in B but missing from A", key),
				PathB:   "edge:" + key,
			})
		}
	}

	// Compare defaults.
	diffs = append(diffs, compareDefaults(a.Defaults, b.Defaults)...)

	return diffs
}

// edgeKey produces a canonical string key for an edge including condition.
func edgeKey(e *ir.Edge) string {
	condStr := ""
	if e.Condition != nil {
		condStr = e.Condition.Raw
	}
	return fmt.Sprintf("%s->%s[%s]", e.From, e.To, condStr)
}

func edgeSet(edges []*ir.Edge) map[string]*ir.Edge {
	m := make(map[string]*ir.Edge, len(edges))
	for _, e := range edges {
		m[edgeKey(e)] = e
	}
	return m
}

// compareConfigs compares the configurations of two nodes with the same ID.
func compareConfigs(id string, a, b *ir.Node) []Difference {
	var diffs []Difference
	path := "node:" + id

	switch ac := a.Config.(type) {
	case ir.AgentConfig:
		bc, ok := b.Config.(ir.AgentConfig)
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q config type mismatch: AgentConfig vs %T", id, b.Config),
				PathA:   path,
				PathB:   path,
			})
			return diffs
		}
		// Compare prompts with whitespace tolerance.
		if !promptsEqual(ac.Prompt, bc.Prompt) {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q prompt differs", id),
				PathA:   path + ".prompt",
				PathB:   path + ".prompt",
			})
		}
		if ac.Model != bc.Model {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q model: %q vs %q", id, ac.Model, bc.Model),
				PathA:   path + ".model",
				PathB:   path + ".model",
			})
		}
		if ac.Provider != bc.Provider {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q provider: %q vs %q", id, ac.Provider, bc.Provider),
				PathA:   path + ".provider",
				PathB:   path + ".provider",
			})
		}
		if ac.GoalGate != bc.GoalGate {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q goal_gate: %v vs %v", id, ac.GoalGate, bc.GoalGate),
				PathA:   path + ".goal_gate",
				PathB:   path + ".goal_gate",
			})
		}
		if ac.AutoStatus != bc.AutoStatus {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q auto_status: %v vs %v", id, ac.AutoStatus, bc.AutoStatus),
				PathA:   path + ".auto_status",
				PathB:   path + ".auto_status",
			})
		}

	case ir.ToolConfig:
		bc, ok := b.Config.(ir.ToolConfig)
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q config type mismatch: ToolConfig vs %T", id, b.Config),
				PathA:   path,
				PathB:   path,
			})
			return diffs
		}
		if !promptsEqual(ac.Command, bc.Command) {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q command differs", id),
				PathA:   path + ".command",
				PathB:   path + ".command",
			})
		}

	case ir.HumanConfig:
		bc, ok := b.Config.(ir.HumanConfig)
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q config type mismatch: HumanConfig vs %T", id, b.Config),
				PathA:   path,
				PathB:   path,
			})
			return diffs
		}
		if ac.Mode != bc.Mode {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q mode: %q vs %q", id, ac.Mode, bc.Mode),
				PathA:   path + ".mode",
				PathB:   path + ".mode",
			})
		}

	case ir.ParallelConfig:
		bc, ok := b.Config.(ir.ParallelConfig)
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q config type mismatch: ParallelConfig vs %T", id, b.Config),
				PathA:   path,
				PathB:   path,
			})
			return diffs
		}
		if strings.Join(ac.Targets, ",") != strings.Join(bc.Targets, ",") {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q targets: %v vs %v", id, ac.Targets, bc.Targets),
				PathA:   path + ".targets",
				PathB:   path + ".targets",
			})
		}

	case ir.FanInConfig:
		bc, ok := b.Config.(ir.FanInConfig)
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q config type mismatch: FanInConfig vs %T", id, b.Config),
				PathA:   path,
				PathB:   path,
			})
			return diffs
		}
		if strings.Join(ac.Sources, ",") != strings.Join(bc.Sources, ",") {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q sources: %v vs %v", id, ac.Sources, bc.Sources),
				PathA:   path + ".sources",
				PathB:   path + ".sources",
			})
		}

	case ir.SubgraphConfig:
		bc, ok := b.Config.(ir.SubgraphConfig)
		if !ok {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q config type mismatch: SubgraphConfig vs %T", id, b.Config),
				PathA:   path,
				PathB:   path,
			})
			return diffs
		}
		if ac.Ref != bc.Ref {
			diffs = append(diffs, Difference{
				Kind:    "config_mismatch",
				Message: fmt.Sprintf("node %q ref: %q vs %q", id, ac.Ref, bc.Ref),
				PathA:   path + ".ref",
				PathB:   path + ".ref",
			})
		}
	}

	return diffs
}

// promptsEqual compares two strings with whitespace tolerance.
// Prompts that differ only in trailing whitespace per line and leading/trailing
// whitespace overall are considered equal.
func promptsEqual(a, b string) bool {
	return normalizeWhitespace(a) == normalizeWhitespace(b)
}

// compareDefaults reports differences between workflow defaults.
func compareDefaults(a, b ir.WorkflowDefaults) []Difference {
	var diffs []Difference

	if a.Model != b.Model {
		diffs = append(diffs, Difference{
			Kind:    "defaults_mismatch",
			Message: fmt.Sprintf("defaults.model: %q vs %q", a.Model, b.Model),
			PathA:   "defaults.model",
			PathB:   "defaults.model",
		})
	}
	if a.Provider != b.Provider {
		diffs = append(diffs, Difference{
			Kind:    "defaults_mismatch",
			Message: fmt.Sprintf("defaults.provider: %q vs %q", a.Provider, b.Provider),
			PathA:   "defaults.provider",
			PathB:   "defaults.provider",
		})
	}
	if a.MaxRetries != b.MaxRetries {
		diffs = append(diffs, Difference{
			Kind:    "defaults_mismatch",
			Message: fmt.Sprintf("defaults.max_retries: %d vs %d", a.MaxRetries, b.MaxRetries),
			PathA:   "defaults.max_retries",
			PathB:   "defaults.max_retries",
		})
	}
	if a.MaxRestarts != b.MaxRestarts {
		diffs = append(diffs, Difference{
			Kind:    "defaults_mismatch",
			Message: fmt.Sprintf("defaults.max_restarts: %d vs %d", a.MaxRestarts, b.MaxRestarts),
			PathA:   "defaults.max_restarts",
			PathB:   "defaults.max_restarts",
		})
	}
	if a.Fidelity != b.Fidelity {
		diffs = append(diffs, Difference{
			Kind:    "defaults_mismatch",
			Message: fmt.Sprintf("defaults.fidelity: %q vs %q", a.Fidelity, b.Fidelity),
			PathA:   "defaults.fidelity",
			PathB:   "defaults.fidelity",
		})
	}

	return diffs
}
