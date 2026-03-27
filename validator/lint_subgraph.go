//go:build !wasm

package validator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/2389-research/dippin-lang/ir"
)

// lintSubgraphRef checks DIP126: subgraph ref should point to a file that exists.
func lintSubgraphRef(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		if d := checkSubgraphRef(n); d != nil {
			diags = append(diags, *d)
		}
	}
	return diags
}

// checkSubgraphRef validates a single subgraph node's ref path.
func checkSubgraphRef(n *ir.Node) *Diagnostic {
	ref := subgraphRef(n)
	if ref == "" {
		return nil
	}
	return statSubgraphRef(n, ref)
}

// subgraphRef extracts the ref from a subgraph node, or "" if not applicable.
func subgraphRef(n *ir.Node) string {
	cfg, ok := n.Config.(ir.SubgraphConfig)
	if !ok || cfg.Ref == "" {
		return ""
	}
	// Skip relative refs without source file context — can't resolve.
	if !filepath.IsAbs(cfg.Ref) && n.Source.File == "" {
		return ""
	}
	return cfg.Ref
}

// statSubgraphRef checks if the resolved ref path exists on disk.
func statSubgraphRef(n *ir.Node, ref string) *Diagnostic {
	resolved := resolveRefPath(ref, n.Source.File)
	if _, err := os.Stat(resolved); err != nil {
		return &Diagnostic{
			Code:     DIP126,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("subgraph node %q references %q which does not exist", n.ID, ref),
			Location: n.Source,
			Help:     fmt.Sprintf("resolved path: %s", resolved),
		}
	}
	return nil
}

// resolveRefPath resolves a subgraph ref relative to the workflow's source file.
func resolveRefPath(ref, sourceFile string) string {
	if filepath.IsAbs(ref) {
		return ref
	}
	if sourceFile != "" {
		return filepath.Join(filepath.Dir(sourceFile), ref)
	}
	return ref
}
