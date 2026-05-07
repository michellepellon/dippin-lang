package dipx

import (
	"github.com/2389-research/dippin-lang/ir"
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
	Workflow(refPath, relativeTo string) (*ir.Workflow, error)
}
