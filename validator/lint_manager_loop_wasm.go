//go:build wasm

package validator

import "github.com/2389-research/dippin-lang/ir"

// lintManagerLoop runs all pure-logic manager_loop checks on WASM.
// The file-existence check (DIP135 for nonexistent path) is skipped because
// os.Stat is not available in the WASM sandbox; all other checks (DIP135
// missing ref, DIP136 invalid control field, DIP137 unbounded) run normally.
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
