//go:build wasm

package validator

import "github.com/2389-research/dippin-lang/ir"

// lintToolSyntax is a no-op on WASM — bash is not available.
func lintToolSyntax(_ *ir.Workflow) []Diagnostic { return nil }

// lintToolBinary is a no-op on WASM — exec.LookPath is not available.
func lintToolBinary(_ *ir.Workflow) []Diagnostic { return nil }
