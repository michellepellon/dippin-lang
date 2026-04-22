//go:build wasm

package validator

import "github.com/2389-research/dippin-lang/ir"

// lintManagerLoop is a no-op on WASM — os.Stat is not available.
// Non-file-related checks (DIP136, DIP137) could run in theory, but we
// mirror the lint_subgraph_wasm pattern and skip entirely for simplicity.
func lintManagerLoop(_ *ir.Workflow) []Diagnostic { return nil }
