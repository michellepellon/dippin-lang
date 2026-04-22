//go:build !wasm

package validator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/2389-research/dippin-lang/ir"
)

// lintManagerLoop emits DIP135 (ref missing or file does not exist), DIP136
// (invalid control field — negative poll_interval, max_cycles, or reserved
// steer_context delimiter), and DIP137 (unbounded supervision — neither
// stop_condition nor max_cycles set).
func lintManagerLoop(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.ManagerLoopConfig)
		if !ok {
			continue
		}
		diags = appendManagerLoopDiags(diags, n, cfg)
		if d := checkManagerLoopRefExists(n, cfg); d != nil {
			diags = append(diags, *d)
		}
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

// checkManagerLoopRefExists emits DIP135 when subgraph_ref points to a path
// that can be resolved but the file does not exist. Permission errors and other
// transient IO errors are silently ignored to avoid misreporting.
func checkManagerLoopRefExists(n *ir.Node, cfg ir.ManagerLoopConfig) *Diagnostic {
	if cfg.SubgraphRef == "" {
		return nil
	}
	resolved := resolveManagerLoopRef(n, cfg)
	if resolved == "" {
		return nil
	}
	if _, err := os.Stat(resolved); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			// Permission error or transient IO — don't mis-report as missing.
			return nil
		}
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
