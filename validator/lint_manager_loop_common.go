// lint_manager_loop_common.go contains pure-logic manager_loop lint checks that
// do not require filesystem access. These compile for both wasm and non-wasm.

package validator

import (
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

// appendManagerLoopDiags emits all DIP135/136/137 diagnostics for one node.
// The file-existence check (checkManagerLoopRefExists) is NOT called here;
// callers that have filesystem access (non-wasm) add it separately.
func appendManagerLoopDiags(diags []Diagnostic, n *ir.Node, cfg ir.ManagerLoopConfig) []Diagnostic {
	if d := checkManagerLoopMissingRef(n, cfg); d != nil {
		diags = append(diags, *d)
	}
	diags = append(diags, checkManagerLoopControl(n, cfg)...)
	if d := checkManagerLoopUnbounded(n, cfg); d != nil {
		diags = append(diags, *d)
	}
	return diags
}

// checkManagerLoopMissingRef emits DIP135 when subgraph_ref is empty.
// This is a pure-logic check — no filesystem access required.
func checkManagerLoopMissingRef(n *ir.Node, cfg ir.ManagerLoopConfig) *Diagnostic {
	if cfg.SubgraphRef != "" {
		return nil
	}
	return &Diagnostic{
		Code:     DIP135,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("manager_loop %q has no subgraph_ref", n.ID),
		Location: n.Source,
		Help:     "set subgraph_ref to the path of a .dip file defining the child pipeline",
	}
}

// checkManagerLoopControl emits DIP136 for each invalid control field.
func checkManagerLoopControl(n *ir.Node, cfg ir.ManagerLoopConfig) []Diagnostic {
	var out []Diagnostic
	if cfg.PollInterval < 0 {
		out = append(out, Diagnostic{
			Code:     DIP136,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("manager_loop %q poll_interval is negative (%s)", n.ID, cfg.PollInterval),
			Location: n.Source,
			Help:     "use a non-negative duration such as 30s; 0 means event-driven",
		})
	}
	if cfg.MaxCycles < 0 {
		out = append(out, Diagnostic{
			Code:     DIP136,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("manager_loop %q max_cycles is negative (%d)", n.ID, cfg.MaxCycles),
			Location: n.Source,
			Help:     "use a positive integer; 0 means unbounded",
		})
	}
	return out
}

// checkManagerLoopUnbounded emits DIP137 when both stop_condition and
// max_cycles are unset — supervision can run forever.
func checkManagerLoopUnbounded(n *ir.Node, cfg ir.ManagerLoopConfig) *Diagnostic {
	// Any non-zero MaxCycles (including invalid negative) indicates the user
	// expressed bounding intent; DIP136 owns the invalid-value diagnosis.
	hasMax := cfg.MaxCycles != 0
	if conditionPresent(cfg.StopCondition) || hasMax {
		return nil
	}
	return &Diagnostic{
		Code:     DIP137,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("manager_loop %q is unbounded: no stop_condition and no max_cycles", n.ID),
		Location: n.Source,
		Help:     "set stop_condition (e.g., stack.child.outcome = success) or max_cycles to bound supervision",
	}
}

// conditionPresent reports whether c carries a usable condition expression.
// Either a populated Raw text (typical for parser-produced workflows) or a
// non-nil Parsed AST (programmatic construction) counts as "present".
func conditionPresent(c *ir.Condition) bool {
	if c == nil {
		return false
	}
	return c.Raw != "" || c.Parsed != nil
}
