package ir

// SourceLocation identifies a range in a source file for diagnostics.
type SourceLocation struct {
	File      string
	Line      int
	Column    int
	EndLine   int
	EndColumn int
}

// SourceMap preserves the mapping from IR elements back to source positions.
// After subgraph expansion, this is how you trace "where did this node come from?"
type SourceMap struct {
	Entries []SourceMapEntry
}

// SourceMapEntry maps an IR element identifier to its source location.
type SourceMapEntry struct {
	IRElement string // "node:MyNode", "edge:A->B"
	Location  SourceLocation
}
