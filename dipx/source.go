package dipx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/simulate"
)

// Source loads workflows, whether from a .dip on disk (refs resolved against
// the filesystem) or from a .dipx bundle (refs resolved against the bundle root).
//
// Argument order matches flatten.Resolver.Resolve(refPath, relativeTo) for
// codebase consistency.
//
// Source is safe for concurrent reads. Returned *ir.Workflow values MUST be
// treated as read-only by callers.
type Source interface {
	Entry() *ir.Workflow
	Workflow(ctx context.Context, refPath, relativeTo string) (*ir.Workflow, error)
}

// Load opens either a .dip or a .dipx based on filename extension.
//
// Extension matching is case-insensitive and strict: anything other than
// .dip or .dipx (including extensionless paths and unknown suffixes like
// .txt or .zip) is rejected with ErrPathUnsafe.
func Load(ctx context.Context, path string) (Source, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".dipx":
		return Open(ctx, path)
	case ".dip":
		return loadDirSource(ctx, path)
	default:
		return nil, newError(ErrPathUnsafe, path, "unsupported file extension; expected .dip or .dipx", nil)
	}
}

// dirSource is a Source backed by .dip files on the local filesystem rooted at
// the entry file's directory. Subgraph refs are resolved lexically against the
// referring file and confirmed to remain under the entry's base directory.
type dirSource struct {
	entryPath string
	entry     *ir.Workflow
	baseDir   string
	mu        sync.Mutex
	cache     map[string]*ir.Workflow
}

func loadDirSource(ctx context.Context, entryPath string) (*dirSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(entryPath)
	if err != nil {
		return nil, err
	}
	wf, err := parseDipFile(abs)
	if err != nil {
		return nil, newError(ErrEntryParse, abs, "", err)
	}
	return &dirSource{
		entryPath: abs,
		entry:     wf,
		baseDir:   filepath.Dir(abs),
		cache:     map[string]*ir.Workflow{abs: wf},
	}, nil
}

// parseDipFile reads a .dip file from disk, parses it via parser.NewParser,
// and normalizes its conditions.
//
// SPEC NOTE: This is one of three parser.NewParser call sites within dipx.
// The three sites are:
//   - helpers.go parseAllWorkflows (consumes verifiedBytes from .dipx hash
//     verification, Open pathway).
//   - helpers.go parsePackSource (Pack pathway, trusted disk bytes).
//   - source.go parseDipFile (this function, dirSource pathway, trusted
//     disk bytes).
//
// The CI grep / TestInvariant_ParserNewParserSiteCount must allowlist all
// three sites. The verifiedBytes type-encoded ordering invariant applies
// only to the .dipx Open pathway and is not bypassed by this helper
// because dirSource never produces or consumes verifiedBytes.
func parseDipFile(path string) (*ir.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	wf, err := parser.NewParser(string(data), path).Parse()
	if err != nil {
		return nil, err
	}
	if err := simulate.EnsureConditionsParsed(wf); err != nil {
		return nil, err
	}
	return wf, nil
}

func (d *dirSource) Entry() *ir.Workflow { return d.entry }

func (d *dirSource) Workflow(ctx context.Context, refPath, relativeTo string) (*ir.Workflow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target, err := d.resolveDir(refPath, relativeTo)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if wf, ok := d.cache[target]; ok {
		return wf, nil
	}
	wf, err := parseDipFile(target)
	if err != nil {
		return nil, newError(ErrSubgraphParse, target, "", err)
	}
	d.cache[target] = wf
	return wf, nil
}

// resolveDir resolves refPath against relativeTo, then verifies the resulting
// path is still under the entry's base directory.
func (d *dirSource) resolveDir(refPath, relativeTo string) (string, error) {
	if filepath.IsAbs(refPath) {
		return "", newError(ErrPathUnsafe, refPath, "absolute ref", nil)
	}
	parentDir := filepath.Dir(relativeTo)
	target := filepath.Clean(filepath.Join(parentDir, refPath))
	rel, err := filepath.Rel(d.baseDir, target)
	if err != nil {
		return "", newError(ErrPathUnsafe, refPath, "ref escapes base directory", nil)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", newError(ErrPathUnsafe, refPath, "ref escapes base directory", nil)
	}
	return target, nil
}
