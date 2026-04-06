// ABOUTME: DiskResolver loads referenced .dip files from the filesystem.
// ABOUTME: Used by the CLI for production subgraph resolution.
package flatten

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
)

// DiskResolver resolves subgraph refs by parsing .dip files from disk.
type DiskResolver struct{}

// Resolve parses the .dip file at refPath, resolved relative to relativeTo's directory.
func (r *DiskResolver) Resolve(refPath string, relativeTo string) (*ir.Workflow, error) {
	resolved := resolvePath(refPath, relativeTo)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", resolved, err)
	}
	p := parser.NewParser(string(data), resolved)
	w, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", resolved, err)
	}
	return w, nil
}

// resolvePath resolves refPath relative to the directory of relativeTo.
func resolvePath(refPath string, relativeTo string) string {
	if filepath.IsAbs(refPath) {
		return refPath
	}
	if relativeTo != "" {
		return filepath.Join(filepath.Dir(relativeTo), refPath)
	}
	return refPath
}
