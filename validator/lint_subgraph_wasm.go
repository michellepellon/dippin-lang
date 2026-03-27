//go:build wasm

package validator

import "github.com/2389-research/dippin-lang/ir"

// lintSubgraphRef is a no-op on WASM — os.Stat is not available.
func lintSubgraphRef(_ *ir.Workflow) []Diagnostic { return nil }
