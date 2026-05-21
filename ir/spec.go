// ABOUTME: SpecRef carries a workflow-level reference to an external spec document.
// ABOUTME: Dippin does not load or interpret the spec; loader semantics live in the runtime.
package ir

// SpecRef references an external spec document that the workflow's nodes can
// declare alignment with via Node.Satisfies. The loader name is a key into a
// runtime-side plugin registry (e.g. "acai"); the path is resolved relative
// to the .dip file's directory.
type SpecRef struct {
	Loader string
	Path   string
}
