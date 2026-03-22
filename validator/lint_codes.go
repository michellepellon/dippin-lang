package validator

// Diagnostic codes for semantic quality warnings (DIP101–DIP115).
const (
	DIP101 = "DIP101" // unreachable nodes after conditional branches
	DIP102 = "DIP102" // routing node without default/unconditional edge
	DIP103 = "DIP103" // overlapping or contradictory conditions
	DIP104 = "DIP104" // unbounded retry loop
	DIP105 = "DIP105" // no success path to exit
	DIP106 = "DIP106" // undefined variable in prompt
	DIP107 = "DIP107" // unused context key (written but never read)
	DIP108 = "DIP108" // unknown model/provider combination
	DIP109 = "DIP109" // namespace collision in imports
	DIP110 = "DIP110" // empty prompt on agent node
	DIP111 = "DIP111" // tool command without timeout
	DIP112 = "DIP112" // reads key not in any upstream writes
	DIP113 = "DIP113" // invalid retry policy name
	DIP114 = "DIP114" // invalid fidelity level
	DIP115 = "DIP115" // goal_gate without retry/fallback target
	DIP116 = "DIP116" // invalid compaction threshold or on_resume value
	DIP117 = "DIP117" // stylesheet .class references undefined class
	DIP118 = "DIP118" // stylesheet #id references unknown node
)

func init() {
	// Extend CodeDescription with linter codes.
	CodeDescription[DIP101] = "unreachable node after conditional branches"
	CodeDescription[DIP102] = "routing node has no default/unconditional edge"
	CodeDescription[DIP103] = "overlapping or contradictory conditions"
	CodeDescription[DIP104] = "unbounded retry loop (no max_retries or fallback)"
	CodeDescription[DIP105] = "no success path from start to exit"
	CodeDescription[DIP106] = "undefined variable reference in prompt"
	CodeDescription[DIP107] = "unused context key (written but never read)"
	CodeDescription[DIP108] = "unknown model/provider combination"
	CodeDescription[DIP109] = "namespace collision in imports"
	CodeDescription[DIP110] = "empty prompt on agent node"
	CodeDescription[DIP111] = "tool command has no timeout"
	CodeDescription[DIP112] = "reads key not produced by any upstream writes"
	CodeDescription[DIP113] = "invalid retry policy name"
	CodeDescription[DIP114] = "invalid fidelity level"
	CodeDescription[DIP115] = "goal_gate node without retry or fallback target"
	CodeDescription[DIP116] = "invalid compaction threshold or on_resume value"
	CodeDescription[DIP117] = "stylesheet class references undefined class"
	CodeDescription[DIP118] = "stylesheet ID references unknown node"
}
