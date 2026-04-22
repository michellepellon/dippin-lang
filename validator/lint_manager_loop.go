//go:build !wasm

package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// lintManagerLoop emits DIP135 (ref missing or file does not exist), DIP136
// (invalid control field — negative poll_interval or max_cycles), and DIP137
// (unbounded supervision — neither stop_condition nor max_cycles set).
func lintManagerLoop(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.ManagerLoopConfig)
		if !ok {
			continue
		}
		diags = appendManagerLoopDiags(diags, n, cfg)
	}
	return diags
}

// appendManagerLoopDiags emits all DIP135/136/137 diagnostics for a node.
func appendManagerLoopDiags(diags []Diagnostic, n *ir.Node, cfg ir.ManagerLoopConfig) []Diagnostic {
	if d := checkManagerLoopRef(n, cfg); d != nil {
		diags = append(diags, *d)
	}
	diags = append(diags, checkManagerLoopControl(n, cfg)...)
	if d := checkManagerLoopUnbounded(n, cfg); d != nil {
		diags = append(diags, *d)
	}
	return diags
}

// resolveManagerLoopRef returns the resolved file path for cfg.SubgraphRef,
// or "" when resolution isn't possible (empty ref, or relative ref with no
// source file for context). Delegates actual path resolution to resolveRefPath.
func resolveManagerLoopRef(n *ir.Node, cfg ir.ManagerLoopConfig) string {
	if cfg.SubgraphRef == "" {
		return ""
	}
	if !filepath.IsAbs(cfg.SubgraphRef) && n.Source.File == "" {
		return ""
	}
	return resolveRefPath(cfg.SubgraphRef, n.Source.File)
}

// checkManagerLoopRef emits DIP135 if subgraph_ref is empty or points to a
// nonexistent file (when the file path can be resolved).
func checkManagerLoopRef(n *ir.Node, cfg ir.ManagerLoopConfig) *Diagnostic {
	if cfg.SubgraphRef == "" {
		return &Diagnostic{
			Code:     DIP135,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("manager_loop %q has no subgraph_ref", n.ID),
			Location: n.Source,
			Help:     "set subgraph_ref to the path of a .dip file defining the child pipeline",
		}
	}
	resolved := resolveManagerLoopRef(n, cfg)
	if resolved == "" {
		return nil
	}
	if _, err := os.Stat(resolved); err != nil {
		return &Diagnostic{
			Code:     DIP135,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("manager_loop %q references %q which does not exist", n.ID, cfg.SubgraphRef),
			Location: n.Source,
			Help:     fmt.Sprintf("resolved path: %s", resolved),
		}
	}
	return nil
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
	out = append(out, checkManagerLoopSteerContextDelimiters(n, cfg)...)
	return out
}

// checkManagerLoopSteerContextDelimiters emits DIP136 when a steer_context key
// or value contains ',' or '=' — those characters are delimiters in the DOT
// flat-attr form and would corrupt round-trips through DOT export/migrate.
func checkManagerLoopSteerContextDelimiters(n *ir.Node, cfg ir.ManagerLoopConfig) []Diagnostic {
	var out []Diagnostic
	for k, v := range cfg.SteerContext {
		if strings.ContainsAny(k, ",=") || strings.ContainsAny(v, ",=") {
			out = append(out, Diagnostic{
				Code:     DIP136,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("manager_loop %q steer_context key/value contains reserved delimiter (',' or '='): %s=%s", n.ID, k, v),
				Location: n.Source,
				Help:     "steer_context keys and values must not contain ',' or '=' — those characters are used as delimiters in the DOT flat-attr form",
			})
		}
	}
	return out
}

// checkManagerLoopUnbounded emits DIP137 when both stop_condition and
// max_cycles are unset — supervision can run forever.
func checkManagerLoopUnbounded(n *ir.Node, cfg ir.ManagerLoopConfig) *Diagnostic {
	hasStop := cfg.StopCondition != nil && cfg.StopCondition.Raw != ""
	// Any non-zero MaxCycles (including invalid negative) indicates the user
	// expressed bounding intent; DIP136 owns the invalid-value diagnosis.
	hasMax := cfg.MaxCycles != 0
	if hasStop || hasMax {
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
