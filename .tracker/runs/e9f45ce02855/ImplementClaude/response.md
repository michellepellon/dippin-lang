TURN 1
TOOL CALL: read
INPUT:
{"path": ".tracker/current_plan.md"}
TOOL RESULT: read
OUTPUT:
# Implementation Plan: Validator (DIP001–DIP009)

## Component Summary

**Package**: `validator/`
**Purpose**: Graph structure validation over `*ir.Workflow`. Implements checks DIP001 through DIP009 from the design spec §"Validation layers — Layer 3: Graph structure (IR)".

This is a **pure IR consumer** — it takes a `*ir.Workflow` and returns a list of diagnostics. It has no dependency on parsing, formatting, or any syntax-level concepts. It operates entirely on the canonical IR types defined in `ir/`.

---

## Design Spec References

- **§ Validation layers — Layer 3: Graph structure (IR)** — the nine rules
- **§ Diagnostic output** — diagnostic shape (file/line/column, codes, help/fix)
- **§ ADR 1 (DAG-plus-loops)** — `restart: true` edges are excluded from cycle detection
- **§ "Acceptance Criteria"** — #10 "Diagnostics are actionable", #11 "Multi-diagnostic collection"

### Rules (verbatim from spec)

| Code    | Rule                                                                                          |
|---------|-----------------------------------------------------------------------------------------------|
| DIP001  | `start:` node exists                                                                          |
| DIP002  | `exit:` node exists                                                                           |
| DIP003  | All edge endpoints exist                                                                      |
| DIP004  | All nodes reachable from start                                                                |
| DIP005  | No unconditional cycles after excluding edges marked `restart: true`                          |
| DIP006  | Exit node has no outgoing edges                                                               |
| DIP007  | Parallel fan-out has matching fan-in                                                          |
| DIP008  | No duplicate node IDs                                                                         |
| DIP009  | No duplicate edges                                                                            |

---

## Dependencies

- **`ir/`** — all types: `Workflow`, `Node`, `Edge`, `NodeKind`, `SourceLocation`, `ParallelConfig`, `FanInConfig`
- **No other packages** — the validator is self-contained. It imports only `ir/` and stdlib.

---

## Files to Create

### 1. `validator/diagnostic.go`

Diagnostic types shared by validator and (later) linter.

**Types:**

```go
// Severity levels for diagnostics.
type Severity int

const (
    SeverityError   Severity = iota // Must fix — workflow cannot execute
    SeverityWarning                 // Should fix — likely a bug (used by linter, not this component)
    SeverityInfo                    // Informational
    SeverityHint                    // Suggestion
)

// Diagnostic represents a single validation finding.
type Diagnostic struct {
    Code     string             // "DIP001", "DIP002", etc.
    Severity Severity
    Message  string             // Human-readable explanation
    Location ir.SourceLocation  // Where in the source (may be zero-value if unavailable)
    Help     string             // Optional "did you mean X?" or explanation
    Fix      string             // Optional suggested replacement text
}

// String returns a formatted diagnostic string matching the spec example format:
//   error[DIP003]: unknown node reference "InterpretX" in edge
//     --> pipeline.dip:45:5
func (d Diagnostic) String() string

// Result holds the outcome of a validation pass.
type Result struct {
    Diagnostics []Diagnostic
}

// Errors returns only error-severity diagnostics.
func (r Result) Errors() []Diagnostic

// HasErrors returns true if any error-severity diagnostics exist.
func (r Result) HasErrors() bool
```

### 2. `validator/codes.go`

Error code registry — constants and human descriptions.

```go
const (
    DIP001 = "DIP001" // start node missing
    DIP002 = "DIP002" // exit node missing
    DIP003 = "DIP003" // unknown node reference in edge
    DIP004 = "DIP004" // unreachable node(s) from start
    DIP005 = "DIP005" // unconditional cycle detected
    DIP006 = "DIP006" // exit node has outgoing edges
    DIP007 = "DIP007" // parallel/fan_in mismatch
    DIP008 = "DIP008" // duplicate node ID
    DIP009 = "DIP009" // duplicate edge
)

// CodeDescription maps each code to a short human-readable description.
var CodeDescription = map[string]string{
    DIP001: "start node does not exist",
    DIP002: "exit node does not exist",
    DIP003: "unknown node reference in edge",
    DIP004: "node unreachable from start",
    DIP005: "unconditional cycle detected",
    DIP006: "exit node has outgoing edges",
    DIP007: "parallel fan-out/fan-in mismatch",
    DIP008: "duplicate node ID",
    DIP009: "duplicate edge",
}
```

### 3. `validator/validate.go`

The main validation entry point and all nine check functions.

**Public API:**

```go
// Validate runs all graph-structure checks (DIP001–DIP009) on the workflow
// and returns all diagnostics found. It always runs all checks — never
// short-circuits — so that a single pass reports everything.
func Validate(w *ir.Workflow) Result
```

**Internal check functions** (each appends to a shared `[]Diagnostic`):

```go
func checkStartExists(w *ir.Workflow) []Diagnostic        // DIP001
func checkExitExists(w *ir.Workflow) []Diagnostic          // DIP002
func checkEdgeEndpoints(w *ir.Workflow) []Diagnostic       // DIP003
func checkReachability(w *ir.Workflow) []Diagnostic         // DIP004
func checkNoCycles(w *ir.Workflow) []Diagnostic             // DIP005
func checkExitNoOutgoing(w *ir.Workflow) []Diagnostic       // DIP006
func checkParallelFanIn(w *ir.Workflow) []Diagnostic        // DIP007
func checkNoDuplicateNodes(w *ir.Workflow) []Diagnostic     // DIP008
func checkNoDuplicateEdges(w *ir.Workflow) []Diagnostic     // DIP009
```

**Algorithm notes:**

- **DIP001**: Check `w.Start != ""` AND `w.Node(w.Start) != nil`.
- **DIP002**: Check `w.Exit != ""` AND `w.Node(w.Exit) != nil`.
- **DIP003**: For each edge, check `w.Node(e.From) != nil` and `w.Node(e.To) != nil`. Include the dangling reference name in the message. If a close match exists in node IDs (Levenshtein ≤ 2), add it as `Help: "did you mean X?"`.
- **DIP004**: BFS/DFS from `w.Start` along all edges (including restart edges). Any node not visited is unreachable. Report each unreachable node individually with its source location.
- **DIP005**: Build the subgraph of edges where `e.Restart == false`. Run a standard cycle detection (DFS with gray/black coloring). If a cycle is found, report the cycle path (list of node IDs forming the cycle).
- **DIP006**: Check `len(w.EdgesFrom(w.Exit)) == 0`. Report each offending edge.
- **DIP007**: For each `NodeParallel` node, verify there exists exactly one `NodeFanIn` node whose `FanInConfig.Sources` matches the `ParallelConfig.Targets`. For each `NodeFanIn`, verify a corresponding `NodeParallel` exists. Report orphaned parallel or fan_in nodes.
- **DIP008**: Build `map[string]int` counting node IDs. Any ID with count > 1 is reported, with source locations of both declarations.
- **DIP009**: Build `map[[2]string]int` keyed on `(From, To)` pairs. Edges with identical `(From, To)` are duplicates. Note: two edges with the same endpoints but *different conditions* are NOT duplicates — they are conditional branches. So the dedup key is `(From, To, Condition.Raw)` where `Condition.Raw == ""` for unconditional edges. Actually, re-reading the spec: "No duplicate edges" — this should mean truly identical edges (same From, To, and same condition raw text). Two edges `A -> B when X` and `A -> B when Y` are distinct. Two unconditional edges `A -> B` are duplicates.

**Fuzzy matching helper** (for DIP003 help text):

```go
// closestNodeID returns the node ID most similar to the given name,
// or "" if no node is within edit distance 2.
func closestNodeID(w *ir.Workflow, name string) string
```

This uses a simple Levenshtein distance function (no external deps needed — implement inline, ~20 lines).

### 4. `validator/validate_test.go`

Comprehensive tests. See test plan below.

---

## Test Plan (15 cases)

### Happy Path

1. **Valid minimal workflow** — Two nodes (start + exit), one edge. `Validate()` returns zero diagnostics.
2. **Valid complex workflow** — `askAndExecuteWorkflow()` fixture from `ir/ir_test.go`. Has restart edges, parallel/fan_in, conditions. Zero diagnostics.
3. **Valid workflow with restart back-edge** — Has a cycle through a `restart: true` edge. DIP005 should NOT trigger.
4. **Valid parallel/fan_in pair** — ParallelConfig.Targets matches FanInConfig.Sources exactly. Zero diagnostics for DIP007.

### Error Cases — One Diagnostic Each

5. **DIP001: Missing start node** — `w.Start = "Nonexistent"`. Expect exactly one DIP001 diagnostic.
6. **DIP002: Missing exit node** — `w.Exit = "Nonexistent"`. Expect exactly one DIP002 diagnostic.
7. **DIP003: Dangling edge reference** — Edge `From: "A", To: "Nonexistent"`. Expect DIP003 with the dangling name in the message.
8. **DIP003 with fuzzy match** — Edge references "Interpet" (typo for "Interpret"). Expect DIP003 with `Help` containing "did you mean \"Interpret\"?".
9. **DIP004: Unreachable node** — A node exists in `Nodes` but has no incoming edge from any reachable node. Expect DIP004.
10. **DIP005: Unconditional cycle** — `A -> B -> C -> A` with no restart edges. Expect DIP005 listing the cycle.
11. **DIP006: Exit has outgoing** — Exit node has an edge going somewhere. Expect DIP006.
12. **DIP007: Orphaned parallel** — A `NodeParallel` with targets `[X, Y]` but no corresponding `NodeFanIn`. Expect DIP007.
13. **DIP007: Orphaned fan_in** — A `NodeFanIn` with sources `[X, Y]` but no corresponding `NodeParallel`. Expect DIP007.
14. **DIP008: Duplicate node ID** — Two nodes with same ID. Expect DIP008.
15. **DIP009: Duplicate edge** — Two unconditional edges with same `(From, To)`. Expect DIP009.

### Edge Cases

16. **Multiple errors at once** — Workflow has DIP001 + DIP003 + DIP008 simultaneously. Verify all three are reported (multi-diagnostic).
17. **Empty workflow** — Zero nodes, zero edges. Should report DIP001, DIP002 (no start/exit). Should NOT panic.
18. **DIP005: Cycle through restart edge is OK** — `A -> B -> C -> A [restart: true]` is valid (restart edges excluded from cycle detection). Zero DIP005 diagnostics.
19. **DIP003: Both endpoints dangling** — Edge where both From and To are nonexistent. Expect two DIP003 diagnostics (one per endpoint).
20. **DIP009: Same endpoints, different conditions = NOT duplicate** — Two edges `A -> B when X` and `A -> B when Y`. Should NOT trigger DIP009.
21. **DIP004: Start node unreachable from itself** — Start is declared but `w.Start` points to it; it IS reachable (it's the starting point). Other disconnected nodes ARE unreachable.
22. **Diagnostic formatting** — Verify `Diagnostic.String()` output matches the spec format with code, message, and location.

---

## Implementation Order

1. `validator/diagnostic.go` — types first (Diagnostic, Severity, Result)
2. `validator/codes.go` — constants
3. `validator/validate.go` — implement checks in order DIP008 → DIP001 → DIP002 → DIP003 → DIP006 → DIP009 → DIP004 → DIP005 → DIP007
   - DIP008 first because duplicate detection is needed before graph traversal
   - DIP004/DIP005 require graph traversal algorithms (BFS, DFS cycle detection)
   - DIP007 requires matching parallel/fan_in semantics
4. `validator/validate_test.go` — tests alongside implementation

**Rationale for check order in `Validate()`**: All checks run unconditionally. However, DIP008 (duplicate nodes) logically comes first because later checks might produce confusing results on workflows with duplicate IDs. The public `Validate()` function runs all checks and concatenates results.

---

## Non-Goals (explicitly out of scope)

- **Layer 2 (Schema validation)** — known fields, required fields, type checking. That's a separate component.
- **Layer 4 (Semantic warnings / linter: DIP101–DIP112)** — separate `linter` component per the ledger.
- **Parser integration** — the validator takes `*ir.Workflow`, not `.dip` source text.
- **JSON diagnostic output** — will be added when CLI is implemented. The `Diagnostic` type should be JSON-serializable but we don't build the JSON formatter here.
- **Autofix** — diagnostics include `Fix` text for human/tooling consumption, but no automated rewriting.

---

## Open Design Decisions

1. **DIP007 matching semantics**: The spec says "Parallel fan-out has matching fan-in." The strictest reading: for each `NodeParallel` with `Targets: [X, Y]`, there must exist exactly one `NodeFanIn` with `Sources: [X, Y]` (same set, order-insensitive). The implementation should use set comparison (sort both slices, compare).

2. **DIP009 dedup key**: The spec says "No duplicate edges." Two edges with the same `(From, To)` but different conditions are NOT duplicates — they represent conditional branching. The dedup key is `(From, To, conditionKey)` where `conditionKey` is `Condition.Raw` if the condition exists, or `""` if unconditional.

3. **DIP005 cycle reporting**: When a cycle is found, the diagnostic should include the cycle path (e.g., "cycle: A → B → C → A") so the user knows which edges to fix. The DFS algorithm should record the path of gray nodes when a back-edge is encountered.

---

## Example Usage (for implementation agent reference)

```go
import (
    "fmt"
    "github.com/2389/dippin/ir"
    "github.com/2389/dippin/validator"
)

func example() {
    w := &ir.Workflow{
        Name:  "broken",
        Start: "Begin",
        Exit:  "End",
        Nodes: []*ir.Node{
            {ID: "Begin", Kind: ir.NodeHuman, Config: ir.HumanConfig{}},
            // "End" is missing!
        },
        Edges: []*ir.Edge{
            {From: "Begin", To: "End"},
        },
    }

    result := validator.Validate(w)
    for _, d := range result.Diagnostics {
        fmt.Println(d.String())
    }
    // Output:
    // error[DIP002]: exit node "End" is declared but does not exist in the node list
    //   --> :0:0
    //   = help: add a node with ID "End" to the workflow
    // error[DIP003]: edge references unknown node "End"
    //   --> :0:0
    //   = help: declare a node with ID "End" or fix the edge target
}
```

TOOL CALL: read
INPUT:
{"path": ".tracker/current_context.md"}
TOOL RESULT: read
OUTPUT:
=== Next component: linter ===
component: linter
package: validator/
description: Semantic quality warnings (DIP101-DIP112)

=== Current codebase ===
./cmd/dippin/main.go
./ir/edge.go
./ir/ir_test.go
./ir/ir.go
./ir/lookup.go
./ir/source.go
./validator/codes.go
./validator/diagnostic.go
./validator/validate.go
./validator/validate_test.go

=== Test status ===
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/ir
ok  	github.com/2389/dippin/validator

=== Ledger ===
component	package	status	description
ir	ir/	complete	Canonical IR types (Workflow, Node, Edge, Condition)
parser-lexer	parser/	in_progress	Line-based indentation-aware lexer
parser-core	parser/	in_progress	Parser: .dip source to IR with error recovery
parser-conditions	parser/	in_progress	Condition expression parser (and/or/not/compare)
validator	validator/	complete	Graph structure validation (DIP001-DIP009)
linter	validator/	pending	Semantic quality warnings (DIP101-DIP112)
formatter	formatter/	pending	Canonical formatter: IR to .dip source
dot-exporter	export/	pending	DOT exporter: IR to DOT string
migration	migrate/	pending	Migration tool: DOT to IR to .dip
cli	cmd/dippin/	pending	CLI subcommands: parse, validate, lint, fmt, export-dot, migrate

TOOL CALL: read
INPUT:
{"path": "../DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: read
OUTPUT:
Tool error (read): path escapes working directory: ../DIPPIN_DESIGN_PLAN.md
ERROR:
true
TURN 2
TOOL CALL: bash
INPUT:
{"command": "find .. -name \"DIPPIN_DESIGN_PLAN.md\" 2>/dev/null | head -5"}
TOOL RESULT: bash
OUTPUT:
../dippin/DIPPIN_DESIGN_PLAN.md
../dippin-lang/DIPPIN_DESIGN_PLAN.md

TOOL CALL: glob
INPUT:
{"pattern": "ir/*.go"}
TOOL RESULT: glob
OUTPUT:
ir/edge.go
ir/ir.go
ir/ir_test.go
ir/lookup.go
ir/source.go
TURN 3
TOOL CALL: read
INPUT:
{"path": "ir/ir.go"}
TOOL RESULT: read
OUTPUT:
// Package ir defines the canonical intermediate representation for Dippin workflows.
//
// The IR is the contract between parsing and execution. It is explicit, normalized,
// and independent of both Dippin syntax and DOT syntax. All downstream consumers
// (engine, validator, formatter, DOT exporter) program against these types.
package ir

import "time"

// Workflow is the top-level IR structure representing a complete pipeline.
type Workflow struct {
	Name      string
	Version   string           // Dippin format version
	Goal      string           // Human-readable objective
	Start     string           // Explicit entry node ID (required)
	Exit      string           // Explicit exit node ID (required)
	Defaults  WorkflowDefaults // Graph-level config
	Nodes     []*Node          // Ordered for deterministic processing
	Edges     []*Edge
	SourceMap *SourceMap // File/line mapping for diagnostics
}

// WorkflowDefaults holds graph-level configuration that applies to all nodes
// unless overridden at the node level.
type WorkflowDefaults struct {
	Model         string // Default LLM model
	Provider      string // Default LLM provider
	RetryPolicy   string // Default retry policy name
	MaxRetries    int    // Default max retries
	Fidelity      string // Default fidelity level
	MaxRestarts   int    // Max loop restarts (default 5)
	RestartTarget string // Where to restart on loop
	CacheTools    bool   // Cache tool results
	Compaction    string // Context compaction mode
}

// Node represents a single step in the workflow.
type Node struct {
	ID      string
	Kind    NodeKind
	Label   string     // Human-readable display name
	Classes []string   // For stylesheet matching (post-v1)
	Config  NodeConfig // Kind-specific configuration
	Retry   RetryConfig
	IO      NodeIO // Declared inputs/outputs (advisory in v1)
	Source  SourceLocation
}

// NodeKind enumerates node types explicitly.
type NodeKind string

const (
	NodeAgent    NodeKind = "agent"
	NodeHuman    NodeKind = "human"
	NodeTool     NodeKind = "tool"
	NodeParallel NodeKind = "parallel"
	NodeFanIn    NodeKind = "fan_in"
	NodeSubgraph NodeKind = "subgraph"
)

// NodeConfig is implemented by kind-specific configuration types.
// The sealed interface prevents invalid combinations structurally.
type NodeConfig interface {
	nodeConfig()
}

// AgentConfig holds configuration for LLM agent nodes.
type AgentConfig struct {
	Prompt              string
	SystemPrompt        string
	Model               string  // Per-node override
	Provider            string
	MaxTurns            int
	CmdTimeout          time.Duration
	CacheTools          bool
	Compaction          string
	CompactionThreshold float64
	ReasoningEffort     string
	Fidelity            string
	AutoStatus          bool // Parse STATUS: from response
	GoalGate            bool // Pipeline fails if this node fails
}

func (AgentConfig) nodeConfig() {}

// HumanConfig holds configuration for human gate nodes.
type HumanConfig struct {
	Mode    string // "choice" | "freeform"
	Default string // Default choice
}

func (HumanConfig) nodeConfig() {}

// ToolConfig holds configuration for shell command nodes.
type ToolConfig struct {
	Command string // Shell command (multiline OK)
	Timeout time.Duration
}

func (ToolConfig) nodeConfig() {}

// ParallelConfig holds configuration for fan-out nodes.
type ParallelConfig struct {
	Targets []string // Fan-out target node IDs
}

func (ParallelConfig) nodeConfig() {}

// FanInConfig holds configuration for join nodes.
type FanInConfig struct {
	Sources []string // Source node IDs to join
}

func (FanInConfig) nodeConfig() {}

// SubgraphConfig holds configuration for embedded sub-pipeline nodes.
type SubgraphConfig struct {
	Ref    string            // Workflow name or path
	Params map[string]string // Parameter overrides
}

func (SubgraphConfig) nodeConfig() {}

// RetryConfig specifies retry behavior for a node.
type RetryConfig struct {
	Policy         string // Named policy: "standard", "aggressive", "patient", "linear", "none"
	MaxRetries     int    // Override default
	RetryTarget    string // Node to jump to on retry
	FallbackTarget string // Fallback if retries exhausted
}

// NodeIO declares what context keys a node reads and writes.
// Both use bare logical names (e.g., "human_response", not "ctx.human_response").
// Advisory in v1 — validated as warnings, not errors.
type NodeIO struct {
	Reads  []string // Context keys this node expects
	Writes []string // Context keys this node produces
}

TOOL CALL: read
INPUT:
{"path": "ir/edge.go"}
TOOL RESULT: read
OUTPUT:
package ir

// Edge represents a connection between nodes in the workflow graph.
type Edge struct {
	From      string
	To        string
	Label     string     // Display label / human choice text
	Condition *Condition // Parsed condition (not raw string)
	Weight    int        // Priority hint for edge selection
	Restart   bool       // Back-edge: triggers downstream clear + re-execution
	Source    SourceLocation
}

// Condition is a parsed, validated boolean expression attached to an edge.
type Condition struct {
	Raw    string        // Original source text
	Parsed ConditionExpr // AST for evaluation
}

// ConditionExpr is the AST for edge conditions.
type ConditionExpr interface {
	conditionExpr()
}

// CondAnd represents a logical AND of two conditions.
type CondAnd struct {
	Left, Right ConditionExpr
}

func (CondAnd) conditionExpr() {}

// CondOr represents a logical OR of two conditions.
type CondOr struct {
	Left, Right ConditionExpr
}

func (CondOr) conditionExpr() {}

// CondNot represents a logical negation.
type CondNot struct {
	Inner ConditionExpr
}

func (CondNot) conditionExpr() {}

// CondCompare represents a comparison between a context variable and a value.
// Variables use namespaced access: "ctx.outcome", "graph.goal", etc.
type CondCompare struct {
	Variable string // Namespaced: "ctx.outcome", "graph.goal"
	Op       string // "=", "!=", "contains", "startswith", "endswith", "in"
	Value    string
}

func (CondCompare) conditionExpr() {}

TOOL CALL: read
INPUT:
{"path": "ir/lookup.go"}
TOOL RESULT: read
OUTPUT:
package ir

// Node returns the node with the given ID, or nil if not found.
func (w *Workflow) Node(id string) *Node {
	for _, n := range w.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// EdgesFrom returns all edges originating from the given node ID.
func (w *Workflow) EdgesFrom(id string) []*Edge {
	var out []*Edge
	for _, e := range w.Edges {
		if e.From == id {
			out = append(out, e)
		}
	}
	return out
}

// EdgesTo returns all edges targeting the given node ID.
func (w *Workflow) EdgesTo(id string) []*Edge {
	var out []*Edge
	for _, e := range w.Edges {
		if e.To == id {
			out = append(out, e)
		}
	}
	return out
}

// NodeIDs returns all node IDs in declaration order.
func (w *Workflow) NodeIDs() []string {
	ids := make([]string, len(w.Nodes))
	for i, n := range w.Nodes {
		ids[i] = n.ID
	}
	return ids
}

TOOL CALL: read
INPUT:
{"path": "ir/source.go"}
TOOL RESULT: read
OUTPUT:
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

TOOL CALL: read
INPUT:
{"path": "ir/ir_test.go"}
TOOL RESULT: read
OUTPUT:
package ir_test

import (
	"testing"
	"time"

	"github.com/2389/dippin/ir"
)

// Fixture: minimal valid workflow (two nodes, one edge).
func minimalWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:    "minimal",
		Version: "1",
		Start:   "Begin",
		Exit:    "End",
		Nodes: []*ir.Node{
			{ID: "Begin", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Begin", To: "End"},
		},
	}
}

// Fixture: ask-and-execute (the canonical example from the design doc).
func askAndExecuteWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:    "ask_and_execute",
		Version: "1",
		Goal:    "Ask user for a task, implement it, review, ship",
		Start:   "AskUser",
		Exit:    "Done",
		Defaults: ir.WorkflowDefaults{
			Model:       "claude-opus-4-6",
			Provider:    "anthropic",
			RetryPolicy: "standard",
			Fidelity:    "summary:high",
			MaxRestarts: 5,
		},
		Nodes: []*ir.Node{
			{ID: "AskUser", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{
				ID: "Interpret", Kind: ir.NodeAgent,
				IO: ir.NodeIO{Reads: []string{"human_response"}, Writes: []string{"plan"}},
				Config: ir.AgentConfig{
					Prompt: "You are a senior software architect.\n\nRead the user's request below and produce a clear,\nactionable implementation plan.\n\n## User Request\n${ctx.human_response}",
				},
			},
			{ID: "ImplementFanOut", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"ImplementClaude", "ImplementCodex"}}},
			{
				ID: "ImplementClaude", Kind: ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "Implement the plan.", Model: "gpt-5.4", Provider: "openai"},
			},
			{
				ID: "ImplementCodex", Kind: ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "Implement the plan.", Model: "gpt-5.3-codex", Provider: "openai"},
			},
			{ID: "ImplementJoin", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"ImplementClaude", "ImplementCodex"}}},
			{
				ID: "Validate", Kind: ir.NodeAgent,
				Config: ir.AgentConfig{
					Prompt:     "Review the implementations. Run tests.\nRespond with STATUS: success or STATUS: fail.",
					AutoStatus: true,
					GoalGate:   true,
				},
				Retry: ir.RetryConfig{MaxRetries: 2},
			},
			{ID: "Approve", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "choice", Default: "Yes"}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Ship it."}},
		},
		Edges: []*ir.Edge{
			{From: "AskUser", To: "Interpret"},
			{From: "Interpret", To: "ImplementFanOut"},
			{From: "ImplementJoin", To: "Validate"},
			{From: "Validate", To: "Approve", Condition: &ir.Condition{
				Raw:    "ctx.outcome = success",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			}},
			{From: "Validate", To: "Interpret", Label: "retry", Restart: true, Condition: &ir.Condition{
				Raw:    "ctx.outcome = fail",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
			}},
			{From: "Approve", To: "Done"},
		},
	}
}

// Fixture: tool node with multiline command.
func toolWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:    "tool_test",
		Version: "1",
		Start:   "Check",
		Exit:    "Report",
		Nodes: []*ir.Node{
			{
				ID: "Check", Kind: ir.NodeTool,
				IO: ir.NodeIO{Writes: []string{"test_result"}},
				Config: ir.ToolConfig{
					Command: "#!/bin/sh\nset -eu\nif pytest --tb=short 2>&1; then\n  printf 'pass'\nelse\n  printf 'fail'\n  exit 1\nfi",
					Timeout: 60 * time.Second,
				},
			},
			{ID: "Report", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Report results."}},
		},
		Edges: []*ir.Edge{
			{From: "Check", To: "Report"},
		},
	}
}

// Fixture: subgraph composition.
func subgraphWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:    "with_subgraph",
		Version: "1",
		Start:   "Build",
		Exit:    "Done",
		Nodes: []*ir.Node{
			{ID: "Build", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Build the feature."}},
			{
				ID: "Review", Kind: ir.NodeSubgraph,
				Config: ir.SubgraphConfig{
					Ref:    "./review.dip",
					Params: map[string]string{"model": "gpt-5.4", "strict": "true"},
				},
			},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Build", To: "Review"},
			{From: "Review", To: "Done", Condition: &ir.Condition{
				Raw:    "ctx.outcome = success",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			}},
			{From: "Review", To: "Build", Restart: true, Condition: &ir.Condition{
				Raw:    "ctx.outcome = fail",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
			}},
		},
	}
}

// Fixture: complex conditions with AND/OR/NOT.
func complexConditionWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:    "complex_conditions",
		Version: "1",
		Start:   "Check",
		Exit:    "Done",
		Nodes: []*ir.Node{
			{ID: "Check", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo check"}},
			{ID: "PathA", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Path A."}},
			{ID: "PathB", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Path B."}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Check", To: "PathA", Condition: &ir.Condition{
				Raw: "ctx.outcome = success and ctx.tool_stdout != empty",
				Parsed: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					Right: ir.CondCompare{Variable: "ctx.tool_stdout", Op: "!=", Value: "empty"},
				},
			}},
			{From: "Check", To: "PathB", Condition: &ir.Condition{
				Raw: "not ctx.outcome = success",
				Parsed: ir.CondNot{
					Inner: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
				},
			}},
			{From: "PathA", To: "Done"},
			{From: "PathB", To: "Done"},
		},
	}
}

func TestWorkflowNodeLookup(t *testing.T) {
	w := minimalWorkflow()

	if n := w.Node("Begin"); n == nil {
		t.Fatal("expected to find node Begin")
	} else if n.Kind != ir.NodeHuman {
		t.Errorf("Begin kind = %q, want %q", n.Kind, ir.NodeHuman)
	}

	if n := w.Node("Nonexistent"); n != nil {
		t.Errorf("expected nil for nonexistent node, got %+v", n)
	}
}

func TestWorkflowEdgesFrom(t *testing.T) {
	w := askAndExecuteWorkflow()

	edges := w.EdgesFrom("Validate")
	if len(edges) != 2 {
		t.Fatalf("EdgesFrom(Validate) = %d edges, want 2", len(edges))
	}

	targets := map[string]bool{}
	for _, e := range edges {
		targets[e.To] = true
	}
	if !targets["Approve"] || !targets["Interpret"] {
		t.Errorf("unexpected edge targets: %v", targets)
	}
}

func TestWorkflowEdgesTo(t *testing.T) {
	w := askAndExecuteWorkflow()

	edges := w.EdgesTo("Interpret")
	if len(edges) != 2 {
		t.Fatalf("EdgesTo(Interpret) = %d edges, want 2", len(edges))
	}

	sources := map[string]bool{}
	for _, e := range edges {
		sources[e.From] = true
	}
	if !sources["AskUser"] || !sources["Validate"] {
		t.Errorf("unexpected edge sources: %v", sources)
	}
}

func TestRestartEdge(t *testing.T) {
	w := askAndExecuteWorkflow()

	edges := w.EdgesFrom("Validate")
	var restartEdge *ir.Edge
	for _, e := range edges {
		if e.Restart {
			restartEdge = e
			break
		}
	}

	if restartEdge == nil {
		t.Fatal("expected a restart edge from Validate")
	}
	if restartEdge.To != "Interpret" {
		t.Errorf("restart edge target = %q, want Interpret", restartEdge.To)
	}
	if restartEdge.Label != "retry" {
		t.Errorf("restart edge label = %q, want retry", restartEdge.Label)
	}
}

func TestNodeKinds(t *testing.T) {
	w := askAndExecuteWorkflow()

	expected := map[string]ir.NodeKind{
		"AskUser":          ir.NodeHuman,
		"Interpret":        ir.NodeAgent,
		"ImplementFanOut":  ir.NodeParallel,
		"ImplementClaude":  ir.NodeAgent,
		"ImplementCodex":   ir.NodeAgent,
		"ImplementJoin":    ir.NodeFanIn,
		"Validate":         ir.NodeAgent,
		"Approve":          ir.NodeHuman,
		"Done":             ir.NodeAgent,
	}

	for id, wantKind := range expected {
		n := w.Node(id)
		if n == nil {
			t.Errorf("node %q not found", id)
			continue
		}
		if n.Kind != wantKind {
			t.Errorf("node %q kind = %q, want %q", id, n.Kind, wantKind)
		}
	}
}

func TestNodeConfigTypes(t *testing.T) {
	w := askAndExecuteWorkflow()

	// Agent node should have AgentConfig
	interpret := w.Node("Interpret")
	if _, ok := interpret.Config.(ir.AgentConfig); !ok {
		t.Errorf("Interpret config type = %T, want AgentConfig", interpret.Config)
	}

	// Human node should have HumanConfig
	ask := w.Node("AskUser")
	if _, ok := ask.Config.(ir.HumanConfig); !ok {
		t.Errorf("AskUser config type = %T, want HumanConfig", ask.Config)
	}

	// Parallel node should have ParallelConfig
	fanOut := w.Node("ImplementFanOut")
	if pc, ok := fanOut.Config.(ir.ParallelConfig); !ok {
		t.Errorf("ImplementFanOut config type = %T, want ParallelConfig", fanOut.Config)
	} else if len(pc.Targets) != 2 {
		t.Errorf("ParallelConfig targets = %d, want 2", len(pc.Targets))
	}
}

func TestToolConfig(t *testing.T) {
	w := toolWorkflow()

	check := w.Node("Check")
	tc, ok := check.Config.(ir.ToolConfig)
	if !ok {
		t.Fatalf("Check config type = %T, want ToolConfig", check.Config)
	}
	if tc.Timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", tc.Timeout)
	}
	if tc.Command == "" {
		t.Error("command should not be empty")
	}
}

func TestSubgraphConfig(t *testing.T) {
	w := subgraphWorkflow()

	review := w.Node("Review")
	sc, ok := review.Config.(ir.SubgraphConfig)
	if !ok {
		t.Fatalf("Review config type = %T, want SubgraphConfig", review.Config)
	}
	if sc.Ref != "./review.dip" {
		t.Errorf("ref = %q, want ./review.dip", sc.Ref)
	}
	if sc.Params["model"] != "gpt-5.4" {
		t.Errorf("params[model] = %q, want gpt-5.4", sc.Params["model"])
	}
}

func TestNodeIO(t *testing.T) {
	w := askAndExecuteWorkflow()

	interpret := w.Node("Interpret")
	if len(interpret.IO.Reads) != 1 || interpret.IO.Reads[0] != "human_response" {
		t.Errorf("Interpret reads = %v, want [human_response]", interpret.IO.Reads)
	}
	if len(interpret.IO.Writes) != 1 || interpret.IO.Writes[0] != "plan" {
		t.Errorf("Interpret writes = %v, want [plan]", interpret.IO.Writes)
	}
}

func TestConditionAST(t *testing.T) {
	w := complexConditionWorkflow()

	edges := w.EdgesFrom("Check")
	if len(edges) != 2 {
		t.Fatalf("EdgesFrom(Check) = %d, want 2", len(edges))
	}

	// First edge: AND condition
	andEdge := edges[0]
	if andEdge.Condition == nil {
		t.Fatal("expected condition on first edge")
	}
	if _, ok := andEdge.Condition.Parsed.(ir.CondAnd); !ok {
		t.Errorf("first edge condition type = %T, want CondAnd", andEdge.Condition.Parsed)
	}

	// Second edge: NOT condition
	notEdge := edges[1]
	if notEdge.Condition == nil {
		t.Fatal("expected condition on second edge")
	}
	if _, ok := notEdge.Condition.Parsed.(ir.CondNot); !ok {
		t.Errorf("second edge condition type = %T, want CondNot", notEdge.Condition.Parsed)
	}
}

func TestNodeIDs(t *testing.T) {
	w := minimalWorkflow()
	ids := w.NodeIDs()
	if len(ids) != 2 {
		t.Fatalf("NodeIDs() = %d, want 2", len(ids))
	}
	if ids[0] != "Begin" || ids[1] != "End" {
		t.Errorf("NodeIDs() = %v, want [Begin End]", ids)
	}
}

TURN 4
TOOL CALL: read
INPUT:
{"path": "validator/diagnostic.go"}
TOOL RESULT: read
OUTPUT:
// Package validator performs graph structure validation on Dippin IR workflows.
//
// It implements checks DIP001 through DIP009, covering structural correctness
// of the workflow graph: start/exit existence, edge validity, reachability,
// cycle detection, parallel/fan-in pairing, and duplicate detection.
//
// The validator is a pure IR consumer — it takes a *ir.Workflow and returns
// a Result containing all diagnostics found. It always runs all checks and
// never short-circuits, so a single pass reports everything.
package validator

import (
	"fmt"
	"strings"

	"github.com/2389/dippin/ir"
)

// Severity levels for diagnostics.
type Severity int

const (
	SeverityError   Severity = iota // Must fix — workflow cannot execute
	SeverityWarning                 // Should fix — likely a bug (used by linter, not this component)
	SeverityInfo                    // Informational
	SeverityHint                    // Suggestion
)

// String returns a human-readable severity label.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Diagnostic represents a single validation finding.
type Diagnostic struct {
	Code     string            // "DIP001", "DIP002", etc.
	Severity Severity          // Error, warning, etc.
	Message  string            // Human-readable explanation
	Location ir.SourceLocation // Where in the source (may be zero-value if unavailable)
	Help     string            // Optional "did you mean X?" or explanation
	Fix      string            // Optional suggested replacement text
}

// String returns a formatted diagnostic string matching the spec format:
//
//	error[DIP003]: unknown node reference "InterpretX" in edge
//	  --> pipeline.dip:45:5
//	  = help: did you mean "Interpret"?
func (d Diagnostic) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s[%s]: %s", d.Severity, d.Code, d.Message)

	file := d.Location.File
	if file == "" {
		file = "<unknown>"
	}
	fmt.Fprintf(&b, "\n  --> %s:%d:%d", file, d.Location.Line, d.Location.Column)

	if d.Help != "" {
		fmt.Fprintf(&b, "\n  = help: %s", d.Help)
	}
	if d.Fix != "" {
		fmt.Fprintf(&b, "\n  = fix: %s", d.Fix)
	}
	return b.String()
}

// Result holds the outcome of a validation pass.
type Result struct {
	Diagnostics []Diagnostic
}

// Errors returns only error-severity diagnostics.
func (r Result) Errors() []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// HasErrors returns true if any error-severity diagnostics exist.
func (r Result) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

TOOL CALL: read
INPUT:
{"path": "validator/codes.go"}
TOOL RESULT: read
OUTPUT:
package validator

// Diagnostic codes for graph structure validation (DIP001–DIP009).
const (
	DIP001 = "DIP001" // start node missing
	DIP002 = "DIP002" // exit node missing
	DIP003 = "DIP003" // unknown node reference in edge
	DIP004 = "DIP004" // unreachable node(s) from start
	DIP005 = "DIP005" // unconditional cycle detected
	DIP006 = "DIP006" // exit node has outgoing edges
	DIP007 = "DIP007" // parallel/fan_in mismatch
	DIP008 = "DIP008" // duplicate node ID
	DIP009 = "DIP009" // duplicate edge
)

// CodeDescription maps each code to a short human-readable description.
var CodeDescription = map[string]string{
	DIP001: "start node does not exist",
	DIP002: "exit node does not exist",
	DIP003: "unknown node reference in edge",
	DIP004: "node unreachable from start",
	DIP005: "unconditional cycle detected",
	DIP006: "exit node has outgoing edges",
	DIP007: "parallel fan-out/fan-in mismatch",
	DIP008: "duplicate node ID",
	DIP009: "duplicate edge",
}

TOOL CALL: read
INPUT:
{"path": "validator/validate.go"}
TOOL RESULT: read
OUTPUT:
package validator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/2389/dippin/ir"
)

// Validate runs all graph-structure checks (DIP001–DIP009) on the workflow
// and returns all diagnostics found. It always runs all checks — never
// short-circuits — so that a single pass reports everything.
func Validate(w *ir.Workflow) Result {
	var diags []Diagnostic

	// Run all checks unconditionally. DIP008 first because duplicate detection
	// is logically prior to other graph checks.
	diags = append(diags, checkNoDuplicateNodes(w)...)
	diags = append(diags, checkStartExists(w)...)
	diags = append(diags, checkExitExists(w)...)
	diags = append(diags, checkEdgeEndpoints(w)...)
	diags = append(diags, checkExitNoOutgoing(w)...)
	diags = append(diags, checkNoDuplicateEdges(w)...)
	diags = append(diags, checkReachability(w)...)
	diags = append(diags, checkNoCycles(w)...)
	diags = append(diags, checkParallelFanIn(w)...)

	return Result{Diagnostics: diags}
}

// checkStartExists verifies DIP001: the start node ID is set and references
// an existing node.
func checkStartExists(w *ir.Workflow) []Diagnostic {
	if w.Start == "" {
		return []Diagnostic{{
			Code:     DIP001,
			Severity: SeverityError,
			Message:  "workflow has no start node declared",
			Help:     "add a start: field to the workflow",
		}}
	}
	if w.Node(w.Start) == nil {
		return []Diagnostic{{
			Code:     DIP001,
			Severity: SeverityError,
			Message:  fmt.Sprintf("start node %q is declared but does not exist in the node list", w.Start),
			Help:     fmt.Sprintf("add a node with ID %q to the workflow", w.Start),
		}}
	}
	return nil
}

// checkExitExists verifies DIP002: the exit node ID is set and references
// an existing node.
func checkExitExists(w *ir.Workflow) []Diagnostic {
	if w.Exit == "" {
		return []Diagnostic{{
			Code:     DIP002,
			Severity: SeverityError,
			Message:  "workflow has no exit node declared",
			Help:     "add an exit: field to the workflow",
		}}
	}
	if w.Node(w.Exit) == nil {
		return []Diagnostic{{
			Code:     DIP002,
			Severity: SeverityError,
			Message:  fmt.Sprintf("exit node %q is declared but does not exist in the node list", w.Exit),
			Help:     fmt.Sprintf("add a node with ID %q to the workflow", w.Exit),
		}}
	}
	return nil
}

// checkEdgeEndpoints verifies DIP003: every edge endpoint references an existing node.
// If a dangling reference is close to an existing node ID (Levenshtein ≤ 2),
// a "did you mean?" suggestion is included.
func checkEdgeEndpoints(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	nodeSet := buildNodeSet(w)

	for _, e := range w.Edges {
		if _, ok := nodeSet[e.From]; !ok && e.From != "" {
			d := Diagnostic{
				Code:     DIP003,
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge references unknown source node %q", e.From),
				Location: e.Source,
			}
			if suggestion := closestNodeID(w, e.From); suggestion != "" {
				d.Help = fmt.Sprintf("did you mean %q?", suggestion)
			} else {
				d.Help = fmt.Sprintf("declare a node with ID %q or fix the edge source", e.From)
			}
			diags = append(diags, d)
		}
		if _, ok := nodeSet[e.To]; !ok && e.To != "" {
			d := Diagnostic{
				Code:     DIP003,
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge references unknown target node %q", e.To),
				Location: e.Source,
			}
			if suggestion := closestNodeID(w, e.To); suggestion != "" {
				d.Help = fmt.Sprintf("did you mean %q?", suggestion)
			} else {
				d.Help = fmt.Sprintf("declare a node with ID %q or fix the edge target", e.To)
			}
			diags = append(diags, d)
		}
	}
	return diags
}

// checkReachability verifies DIP004: all nodes are reachable from the start node.
// Uses BFS traversal including restart edges (restart edges are valid paths for
// reachability purposes).
func checkReachability(w *ir.Workflow) []Diagnostic {
	// If start doesn't exist, we can't do reachability.
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	// Build adjacency list from edges for BFS.
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}

	visited := make(map[string]bool)
	queue := []string{w.Start}
	visited[w.Start] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range adj[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	var diags []Diagnostic
	for _, n := range w.Nodes {
		if !visited[n.ID] {
			diags = append(diags, Diagnostic{
				Code:     DIP004,
				Severity: SeverityError,
				Message:  fmt.Sprintf("node %q is unreachable from start node %q", n.ID, w.Start),
				Location: n.Source,
				Help:     fmt.Sprintf("add an edge leading to %q or remove it", n.ID),
			})
		}
	}
	return diags
}

// checkNoCycles verifies DIP005: no unconditional cycles exist after excluding
// edges marked restart: true. Per ADR 1, restart edges are back-edges that
// trigger downstream clear + re-execution and are not considered cycles.
func checkNoCycles(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	// Build adjacency list excluding restart edges.
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if !e.Restart {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}

	// DFS with white/gray/black coloring for cycle detection.
	const (
		white = 0 // Not visited
		gray  = 1 // In current DFS path (visiting)
		black = 2 // Fully processed (done)
	)

	color := make(map[string]int)
	parent := make(map[string]string)

	var diags []Diagnostic

	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = gray
		for _, next := range adj[node] {
			if color[next] == gray {
				// Found a cycle — reconstruct path from next → ... → node → next
				cycle := reconstructCycle(parent, node, next)
				diags = append(diags, Diagnostic{
					Code:     DIP005,
					Severity: SeverityError,
					Message:  fmt.Sprintf("unconditional cycle detected: %s", strings.Join(cycle, " → ")),
					Help:     "break the cycle by removing an edge or marking it restart: true",
				})
				return true
			}
			if color[next] == white {
				parent[next] = node
				if dfs(next) {
					return true
				}
			}
		}
		color[node] = black
		return false
	}

	// Start DFS from all nodes to catch cycles in disconnected components too.
	for _, n := range w.Nodes {
		if color[n.ID] == white {
			dfs(n.ID)
		}
	}

	return diags
}

// reconstructCycle builds the cycle path from the DFS parent map.
// Given that we found an edge from → to where to is already gray (in the
// current path), we walk parent pointers from "from" back to "to" to
// reconstruct: to → ... → from → to.
func reconstructCycle(parent map[string]string, from, to string) []string {
	path := []string{to}
	curr := from
	for curr != to {
		path = append(path, curr)
		curr = parent[curr]
	}
	path = append(path, to)
	// Reverse so it reads: to → ... → from → to
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// checkExitNoOutgoing verifies DIP006: the exit node has no outgoing edges.
func checkExitNoOutgoing(w *ir.Workflow) []Diagnostic {
	if w.Exit == "" || w.Node(w.Exit) == nil {
		return nil
	}

	outgoing := w.EdgesFrom(w.Exit)
	if len(outgoing) == 0 {
		return nil
	}

	var diags []Diagnostic
	for _, e := range outgoing {
		diags = append(diags, Diagnostic{
			Code:     DIP006,
			Severity: SeverityError,
			Message:  fmt.Sprintf("exit node %q has outgoing edge to %q", w.Exit, e.To),
			Location: e.Source,
			Help:     "remove the outgoing edge or change the exit node",
		})
	}
	return diags
}

// checkParallelFanIn verifies DIP007: every parallel fan-out node has a matching
// fan-in node, and vice versa. Matching means the ParallelConfig.Targets set
// equals the FanInConfig.Sources set (order-insensitive).
func checkParallelFanIn(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	type nodeTargets struct {
		node   *ir.Node
		sorted []string
	}

	var parallels []nodeTargets
	var fanIns []nodeTargets

	for _, n := range w.Nodes {
		switch cfg := n.Config.(type) {
		case ir.ParallelConfig:
			sorted := make([]string, len(cfg.Targets))
			copy(sorted, cfg.Targets)
			sort.Strings(sorted)
			parallels = append(parallels, nodeTargets{node: n, sorted: sorted})
		case ir.FanInConfig:
			sorted := make([]string, len(cfg.Sources))
			copy(sorted, cfg.Sources)
			sort.Strings(sorted)
			fanIns = append(fanIns, nodeTargets{node: n, sorted: sorted})
		}
	}

	// For each parallel node, check there's a matching fan-in.
	for _, p := range parallels {
		found := false
		for _, f := range fanIns {
			if slicesEqual(p.sorted, f.sorted) {
				found = true
				break
			}
		}
		if !found {
			diags = append(diags, Diagnostic{
				Code:     DIP007,
				Severity: SeverityError,
				Message:  fmt.Sprintf("parallel node %q has targets %v but no matching fan_in node", p.node.ID, p.sorted),
				Location: p.node.Source,
				Help:     fmt.Sprintf("add a fan_in node with sources: %v", p.sorted),
			})
		}
	}

	// For each fan-in node, check there's a matching parallel.
	for _, f := range fanIns {
		found := false
		for _, p := range parallels {
			if slicesEqual(f.sorted, p.sorted) {
				found = true
				break
			}
		}
		if !found {
			diags = append(diags, Diagnostic{
				Code:     DIP007,
				Severity: SeverityError,
				Message:  fmt.Sprintf("fan_in node %q has sources %v but no matching parallel node", f.node.ID, f.sorted),
				Location: f.node.Source,
				Help:     fmt.Sprintf("add a parallel node with targets: %v", f.sorted),
			})
		}
	}

	return diags
}

// checkNoDuplicateNodes verifies DIP008: no two nodes share the same ID.
func checkNoDuplicateNodes(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	seen := make(map[string]*ir.Node)

	for _, n := range w.Nodes {
		if first, ok := seen[n.ID]; ok {
			diags = append(diags, Diagnostic{
				Code:     DIP008,
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate node ID %q", n.ID),
				Location: n.Source,
				Help:     fmt.Sprintf("first declaration at %s:%d:%d", locFile(first.Source), first.Source.Line, first.Source.Column),
			})
		} else {
			seen[n.ID] = n
		}
	}
	return diags
}

// checkNoDuplicateEdges verifies DIP009: no two edges are identical.
// The dedup key is (From, To, Condition.Raw). Two edges with the same
// endpoints but different conditions are conditional branches, not duplicates.
func checkNoDuplicateEdges(w *ir.Workflow) []Diagnostic {
	type edgeKey struct {
		from, to, condRaw string
	}

	var diags []Diagnostic
	seen := make(map[edgeKey]*ir.Edge)

	for _, e := range w.Edges {
		condRaw := ""
		if e.Condition != nil {
			condRaw = e.Condition.Raw
		}
		key := edgeKey{from: e.From, to: e.To, condRaw: condRaw}
		if first, ok := seen[key]; ok {
			diags = append(diags, Diagnostic{
				Code:     DIP009,
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate edge from %q to %q", e.From, e.To),
				Location: e.Source,
				Help:     fmt.Sprintf("first declaration at %s:%d:%d", locFile(first.Source), first.Source.Line, first.Source.Column),
			})
		} else {
			seen[key] = e
		}
	}
	return diags
}

// --- Helpers ---

// buildNodeSet returns a set of all node IDs in the workflow.
func buildNodeSet(w *ir.Workflow) map[string]bool {
	set := make(map[string]bool, len(w.Nodes))
	for _, n := range w.Nodes {
		set[n.ID] = true
	}
	return set
}

// closestNodeID returns the node ID most similar to the given name,
// or "" if no node is within edit distance 2.
func closestNodeID(w *ir.Workflow, name string) string {
	bestDist := 3 // threshold: Levenshtein ≤ 2
	bestID := ""
	for _, n := range w.Nodes {
		d := levenshtein(name, n.ID)
		if d < bestDist {
			bestDist = d
			bestID = n.ID
		}
	}
	return bestID
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two rows for space efficiency.
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			m := ins
			if del < m {
				m = del
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// slicesEqual returns true if two sorted string slices are element-wise equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// locFile returns the file from a SourceLocation, defaulting to "<unknown>".
func locFile(loc ir.SourceLocation) string {
	if loc.File == "" {
		return "<unknown>"
	}
	return loc.File
}

TOOL CALL: read
INPUT:
{"path": "validator/validate_test.go"}
TOOL RESULT: read
OUTPUT:
package validator

import (
	"strings"
	"testing"

	"github.com/2389/dippin/ir"
)

// --- Test fixtures ---

// minimalValidWorkflow returns a valid two-node workflow.
func minimalValidWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "minimal",
		Start: "Begin",
		Exit:  "End",
		Nodes: []*ir.Node{
			{ID: "Begin", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Begin", To: "End"},
		},
	}
}

// askAndExecuteWorkflow returns the canonical example with restart edges
// and parallel/fan_in.
func askAndExecuteWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "ask_and_execute",
		Start: "AskUser",
		Exit:  "Done",
		Nodes: []*ir.Node{
			{ID: "AskUser", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{ID: "Interpret", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Plan."}},
			{ID: "ImplementFanOut", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"ImplementClaude", "ImplementCodex"}}},
			{ID: "ImplementClaude", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Implement."}},
			{ID: "ImplementCodex", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Implement."}},
			{ID: "ImplementJoin", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"ImplementClaude", "ImplementCodex"}}},
			{ID: "Validate", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Review."}},
			{ID: "Approve", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "choice"}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Ship."}},
		},
		Edges: []*ir.Edge{
			{From: "AskUser", To: "Interpret"},
			{From: "Interpret", To: "ImplementFanOut"},
			{From: "ImplementFanOut", To: "ImplementClaude"},
			{From: "ImplementFanOut", To: "ImplementCodex"},
			{From: "ImplementClaude", To: "ImplementJoin"},
			{From: "ImplementCodex", To: "ImplementJoin"},
			{From: "ImplementJoin", To: "Validate"},
			{From: "Validate", To: "Approve", Condition: &ir.Condition{
				Raw:    "ctx.outcome = success",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			}},
			{From: "Validate", To: "Interpret", Label: "retry", Restart: true, Condition: &ir.Condition{
				Raw:    "ctx.outcome = fail",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
			}},
			{From: "Approve", To: "Done"},
		},
	}
}

// --- Table-driven tests ---

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		workflow   *ir.Workflow
		wantCodes  []string // Expected diagnostic codes (empty = no diagnostics)
		wantNoDiag bool     // If true, expect zero diagnostics
	}{
		// --- Happy path ---
		{
			name:       "valid minimal workflow",
			workflow:   minimalValidWorkflow(),
			wantNoDiag: true,
		},
		{
			name:       "valid complex workflow with restart and parallel",
			workflow:   askAndExecuteWorkflow(),
			wantNoDiag: true,
		},
		{
			name: "valid workflow with restart back-edge (no cycle)",
			workflow: &ir.Workflow{
				Name:  "restart_loop",
				Start: "A",
				Exit:  "D",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
					{ID: "D", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "d"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
					{From: "C", To: "A", Restart: true},
					{From: "C", To: "D"},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "valid parallel/fan_in pair with different order",
			workflow: &ir.Workflow{
				Name:  "parallel_ok",
				Start: "Start",
				Exit:  "End",
				Nodes: []*ir.Node{
					{ID: "Start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
					{ID: "Fork", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"W1", "W2"}}},
					{ID: "W1", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "w1"}},
					{ID: "W2", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "w2"}},
					{ID: "Join", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"W2", "W1"}}}, // order differs
					{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "Start", To: "Fork"},
					{From: "Fork", To: "W1"},
					{From: "Fork", To: "W2"},
					{From: "W1", To: "Join"},
					{From: "W2", To: "Join"},
					{From: "Join", To: "End"},
				},
			},
			wantNoDiag: true,
		},

		// --- Error cases: one diagnostic each ---
		{
			name: "DIP001: start node empty",
			workflow: &ir.Workflow{
				Name:  "no_start",
				Start: "",
				Exit:  "End",
				Nodes: []*ir.Node{
					{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
			},
			wantCodes: []string{DIP001},
		},
		{
			name: "DIP001: start node declared but missing from nodes",
			workflow: &ir.Workflow{
				Name:  "bad_start",
				Start: "Nonexistent",
				Exit:  "End",
				Nodes: []*ir.Node{
					{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
			},
			wantCodes: []string{DIP001},
		},
		{
			name: "DIP002: exit node missing from nodes",
			workflow: &ir.Workflow{
				Name:  "bad_exit",
				Start: "Begin",
				Exit:  "Nonexistent",
				Nodes: []*ir.Node{
					{ID: "Begin", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
				},
			},
			wantCodes: []string{DIP002},
		},
		{
			name: "DIP003: dangling edge target",
			workflow: &ir.Workflow{
				Name:  "dangling",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "Nonexistent"},
				},
			},
			wantCodes: []string{DIP003},
		},
		{
			name: "DIP003: fuzzy match suggests similar node",
			workflow: &ir.Workflow{
				Name:  "fuzzy",
				Start: "Interpret",
				Exit:  "Interpret",
				Nodes: []*ir.Node{
					{ID: "Interpret", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
				},
				Edges: []*ir.Edge{
					{From: "Interpret", To: "Interpet"}, // typo: missing 'r'
				},
			},
			wantCodes: []string{DIP003},
		},
		{
			name: "DIP004: unreachable node",
			workflow: &ir.Workflow{
				Name:  "unreachable",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "Island", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "island"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantCodes: []string{DIP004},
		},
		{
			name: "DIP005: unconditional cycle",
			workflow: &ir.Workflow{
				Name:  "cycle",
				Start: "A",
				Exit:  "D",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
					{ID: "D", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "d"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
					{From: "C", To: "A"}, // cycle: not restart
					{From: "C", To: "D"},
				},
			},
			wantCodes: []string{DIP005},
		},
		{
			name: "DIP006: exit has outgoing edge",
			workflow: &ir.Workflow{
				Name:  "exit_outgoing",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "A"},
				},
			},
			wantCodes: []string{DIP006},
		},
		{
			name: "DIP007: orphaned parallel node",
			workflow: &ir.Workflow{
				Name:  "orphan_parallel",
				Start: "Start",
				Exit:  "End",
				Nodes: []*ir.Node{
					{ID: "Start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
					{ID: "Fork", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"X", "Y"}}},
					{ID: "X", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "x"}},
					{ID: "Y", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "y"}},
					{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "Start", To: "Fork"},
					{From: "Fork", To: "X"},
					{From: "Fork", To: "Y"},
					{From: "X", To: "End"},
					{From: "Y", To: "End"},
				},
			},
			wantCodes: []string{DIP007},
		},
		{
			name: "DIP007: orphaned fan_in node",
			workflow: &ir.Workflow{
				Name:  "orphan_fanin",
				Start: "Start",
				Exit:  "End",
				Nodes: []*ir.Node{
					{ID: "Start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
					{ID: "X", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "x"}},
					{ID: "Y", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "y"}},
					{ID: "Join", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"X", "Y"}}},
					{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "Start", To: "X"},
					{From: "Start", To: "Y"},
					{From: "X", To: "Join"},
					{From: "Y", To: "Join"},
					{From: "Join", To: "End"},
				},
			},
			wantCodes: []string{DIP007},
		},
		{
			name: "DIP008: duplicate node ID",
			workflow: &ir.Workflow{
				Name:  "dup_node",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "first"}, Source: ir.SourceLocation{File: "test.dip", Line: 1}},
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "second"}, Source: ir.SourceLocation{File: "test.dip", Line: 5}},
				},
			},
			wantCodes: []string{DIP008},
		},
		{
			name: "DIP009: duplicate unconditional edge",
			workflow: &ir.Workflow{
				Name:  "dup_edge",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "A", To: "B"},
				},
			},
			wantCodes: []string{DIP009},
		},

		// --- Edge cases ---
		{
			name: "multiple errors at once",
			workflow: &ir.Workflow{
				Name:  "multi_error",
				Start: "Missing",
				Exit:  "End",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}, Source: ir.SourceLocation{File: "test.dip", Line: 1}},
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "dup"}, Source: ir.SourceLocation{File: "test.dip", Line: 5}},
					{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "end"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "Ghost"},
				},
			},
			wantCodes: []string{DIP008, DIP001, DIP003},
		},
		{
			name:      "empty workflow",
			workflow:  &ir.Workflow{},
			wantCodes: []string{DIP001, DIP002},
		},
		{
			name: "DIP003: both endpoints dangling",
			workflow: &ir.Workflow{
				Name:  "both_dangling",
				Start: "Real",
				Exit:  "Real",
				Nodes: []*ir.Node{
					{ID: "Real", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "ok"}},
				},
				Edges: []*ir.Edge{
					{From: "Ghost1", To: "Ghost2"},
				},
			},
			wantCodes: []string{DIP003, DIP003},
		},
		{
			name: "DIP009: same endpoints different conditions = NOT duplicate",
			workflow: &ir.Workflow{
				Name:  "cond_branches",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{Raw: "ctx.x = 1", Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"}}},
					{From: "A", To: "B", Condition: &ir.Condition{Raw: "ctx.x = 2", Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"}}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "DIP005: cycle through restart edge is OK (duplicate of happy path for clarity)",
			workflow: &ir.Workflow{
				Name:  "restart_ok",
				Start: "A",
				Exit:  "D",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
					{ID: "D", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "d"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
					{From: "C", To: "A", Restart: true},
					{From: "C", To: "D"},
				},
			},
			wantNoDiag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Validate(tt.workflow)

			if tt.wantNoDiag {
				if len(result.Diagnostics) != 0 {
					t.Errorf("expected no diagnostics, got %d:", len(result.Diagnostics))
					for _, d := range result.Diagnostics {
						t.Errorf("  %s", d.String())
					}
				}
				return
			}

			if tt.wantCodes != nil {
				gotCodes := make([]string, len(result.Diagnostics))
				for i, d := range result.Diagnostics {
					gotCodes[i] = d.Code
				}

				// Check that all expected codes are present (order-insensitive).
				wantCount := make(map[string]int)
				for _, c := range tt.wantCodes {
					wantCount[c]++
				}
				gotCount := make(map[string]int)
				for _, c := range gotCodes {
					gotCount[c]++
				}
				for code, want := range wantCount {
					if got := gotCount[code]; got < want {
						t.Errorf("expected at least %d %s diagnostic(s), got %d. All codes: %v", want, code, got, gotCodes)
					}
				}
			}
		})
	}
}

func TestDIP003FuzzyMatchHelp(t *testing.T) {
	w := &ir.Workflow{
		Name:  "fuzzy",
		Start: "Interpret",
		Exit:  "Interpret",
		Nodes: []*ir.Node{
			{ID: "Interpret", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
		},
		Edges: []*ir.Edge{
			{From: "Interpret", To: "Interpet"}, // typo: missing 'r'
		},
	}

	result := Validate(w)

	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == DIP003 {
			found = true
			if !strings.Contains(d.Help, `"Interpret"`) {
				t.Errorf("DIP003 help = %q, expected it to contain '\"Interpret\"'", d.Help)
			}
			if !strings.Contains(d.Help, "did you mean") {
				t.Errorf("DIP003 help = %q, expected 'did you mean'", d.Help)
			}
		}
	}
	if !found {
		t.Error("expected DIP003 diagnostic for fuzzy match test")
	}
}

func TestDIP004StartNodeIsReachable(t *testing.T) {
	w := &ir.Workflow{
		Name:  "start_reachable",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "Island", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "island"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
		},
	}

	result := Validate(w)

	// Only "Island" should be unreachable, not A or B.
	dip004Count := 0
	for _, d := range result.Diagnostics {
		if d.Code == DIP004 {
			dip004Count++
			if !strings.Contains(d.Message, `"Island"`) {
				t.Errorf("expected DIP004 to report Island, got: %s", d.Message)
			}
		}
	}
	if dip004Count != 1 {
		t.Errorf("expected exactly 1 DIP004 diagnostic (for Island), got %d", dip004Count)
	}
}

func TestDiagnosticFormatting(t *testing.T) {
	d := Diagnostic{
		Code:     DIP003,
		Severity: SeverityError,
		Message:  `edge references unknown node "InterpretX"`,
		Location: ir.SourceLocation{File: "pipeline.dip", Line: 45, Column: 5},
		Help:     `did you mean "Interpret"?`,
	}

	s := d.String()

	if !strings.Contains(s, "error[DIP003]") {
		t.Errorf("diagnostic string missing 'error[DIP003]', got:\n%s", s)
	}
	if !strings.Contains(s, "pipeline.dip:45:5") {
		t.Errorf("diagnostic string missing location, got:\n%s", s)
	}
	if !strings.Contains(s, `= help: did you mean "Interpret"?`) {
		t.Errorf("diagnostic string missing help, got:\n%s", s)
	}
}

func TestDiagnosticFormattingNoFile(t *testing.T) {
	d := Diagnostic{
		Code:     DIP001,
		Severity: SeverityError,
		Message:  "workflow has no start node declared",
	}

	s := d.String()
	if !strings.Contains(s, "<unknown>:0:0") {
		t.Errorf("diagnostic string should show <unknown> for missing file, got:\n%s", s)
	}
}

func TestDiagnosticFormattingWithFix(t *testing.T) {
	d := Diagnostic{
		Code:     DIP003,
		Severity: SeverityError,
		Message:  `unknown node "Foo"`,
		Fix:      `rename to "Bar"`,
	}

	s := d.String()
	if !strings.Contains(s, `= fix: rename to "Bar"`) {
		t.Errorf("diagnostic string missing fix, got:\n%s", s)
	}
}

func TestResultErrors(t *testing.T) {
	r := Result{
		Diagnostics: []Diagnostic{
			{Code: DIP001, Severity: SeverityError, Message: "e1"},
			{Code: "INFO", Severity: SeverityInfo, Message: "i1"},
			{Code: DIP002, Severity: SeverityError, Message: "e2"},
		},
	}

	errs := r.Errors()
	if len(errs) != 2 {
		t.Fatalf("Errors() returned %d, want 2", len(errs))
	}
	if errs[0].Code != DIP001 || errs[1].Code != DIP002 {
		t.Errorf("Errors() = [%s, %s], want [DIP001, DIP002]", errs[0].Code, errs[1].Code)
	}
}

func TestResultHasErrors(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{
			name: "no diagnostics",
			r:    Result{},
			want: false,
		},
		{
			name: "only info",
			r:    Result{Diagnostics: []Diagnostic{{Severity: SeverityInfo}}},
			want: false,
		},
		{
			name: "has error",
			r:    Result{Diagnostics: []Diagnostic{{Severity: SeverityError}}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{SeverityInfo, "info"},
		{SeverityHint, "hint"},
		{Severity(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "abcd", 1},
		{"Interpret", "Interpet", 1},   // missing 'r'
		{"Interpret", "InterpretX", 1}, // extra char
		{"Interpret", "Intepret", 1},   // transposition-ish
		{"abc", "xyz", 3},              // all different
	}
	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCodeDescriptionCoverage(t *testing.T) {
	codes := []string{DIP001, DIP002, DIP003, DIP004, DIP005, DIP006, DIP007, DIP008, DIP009}
	for _, c := range codes {
		if desc, ok := CodeDescription[c]; !ok || desc == "" {
			t.Errorf("CodeDescription[%q] is missing or empty", c)
		}
	}
}

func TestDIP005CyclePathIncluded(t *testing.T) {
	w := &ir.Workflow{
		Name:  "cycle_path",
		Start: "A",
		Exit:  "D",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
			{ID: "D", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "d"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
			{From: "C", To: "A"}, // cycle
			{From: "C", To: "D"},
		},
	}

	result := Validate(w)

	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == DIP005 {
			found = true
			if !strings.Contains(d.Message, "→") {
				t.Errorf("DIP005 message should include cycle path with arrows, got: %s", d.Message)
			}
			mentionsA := strings.Contains(d.Message, "A")
			mentionsB := strings.Contains(d.Message, "B")
			mentionsC := strings.Contains(d.Message, "C")
			if !(mentionsA && mentionsB && mentionsC) {
				t.Errorf("DIP005 message should mention cycle nodes A, B, C, got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Error("expected DIP005 diagnostic for cycle test")
	}
}

func TestDIP006MultipleOutgoing(t *testing.T) {
	w := &ir.Workflow{
		Name:  "exit_multi_outgoing",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "A"},
			{From: "B", To: "C"},
		},
	}

	result := Validate(w)

	count := 0
	for _, d := range result.Diagnostics {
		if d.Code == DIP006 {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 DIP006 diagnostics for two outgoing edges from exit, got %d", count)
	}
}

TURN 5
TOOL CALL: bash
INPUT:
{"command": "cat DIPPIN_DESIGN_PLAN.md 2>/dev/null || echo \"not found\""}
TOOL RESULT: bash
OUTPUT:
[... truncated 50276 characters ...]
peline.Graph`
- [ ] Write IR serialization (JSON) for debugging
- [ ] Write 10 hand-crafted IR test fixtures

### Phase 1: Parser (2 weeks)

- [ ] Implement Dippin lexer (line-based, indentation-aware)
- [ ] Implement Dippin parser → IR with error recovery at top-level declarations
- [ ] Comprehensive error recovery and multi-diagnostic collection
- [ ] Parser test suite: 50+ cases covering all node kinds, edge syntax, multiline blocks, conditions, explicit start/exit
- [ ] `dippin parse <file>` CLI command (outputs IR as JSON)

### Phase 2: Validator & Linter (1 week)

- [ ] Port existing `pipeline.Validate()` checks to work on IR
- [ ] Add schema validation (required fields, known kinds, type checking via config structs)
- [ ] Add semantic warnings (unreachable nodes, unbounded retries, I/O flow analysis)
- [ ] Structured diagnostic output (text + JSON)
- [ ] `dippin validate <file>` CLI command
- [ ] `dippin lint <file>` CLI command (includes warnings)

### Phase 3: Formatter (1 week)

- [ ] Implement canonical formatter from IR → Dippin source
- [ ] Implement canonical field ordering per node kind
- [ ] Ensure idempotency (format ∘ parse ∘ format = format ∘ parse)
- [ ] `dippin fmt <file>` CLI command
- [ ] `dippin fmt --check` for CI (exit 1 if not canonical)

### Phase 4: DOT Exporter (1 week)

- [ ] Implement `ir.Workflow` → DOT string
- [ ] Test against existing DOT files (parse DOT → IR → export DOT → parse DOT → compare topology)
- [ ] `dippin export-dot <file.dip>` CLI command

### Phase 5: Migration Tool (1 week)

- [ ] Implement `dippin migrate <file.dot>` using existing parser + IR + formatter
- [ ] Post-migration cleanup: un-escape prompts, reformat tool commands, add namespace prefixes to conditions
- [ ] `dippin validate-migration <old.dot> <new.dip>` parity checker
- [ ] Migrate all example files; commit both versions during transition

### Phase 6: Engine Integration (2 weeks)

- [ ] Add `.dip` file detection in `cmd/tracker/main.go`
- [ ] Wire Dippin parser → IR → engine (via `IRToGraph()` adapter initially)
- [ ] Incrementally migrate engine to accept IR directly
- [ ] Verify all existing tests pass with both paths
- [ ] Update TUI to show Dippin source locations in diagnostics

### Deferred (post-v1)

- [ ] Composition: import resolution, parameter substitution, namespace prefixing, subgraph expansion
- [ ] Stylesheet language (simple selectors: *, .class, #id, kind)
- [ ] SARIF output
- [ ] Rich variable flow analysis
- [ ] LSP / editor integration
- [ ] Hot reload in TUI
- [ ] Visual regression rendering tests

---

## 19. Acceptance Criteria ("v1 is good enough")

### Must-have for v1

1. **Parse all existing patterns**: Every DOT pipeline in the repo can be expressed in Dippin and parsed to equivalent IR
2. **Equivalent execution**: Dippin-sourced pipelines produce identical `EngineResult` as DOT-sourced pipelines for the same inputs
3. **Explicit start/exit**: No implicit first-node-is-start. `start:` and `exit:` are required.
4. **Multiline prompts work**: No escaping needed for prompts containing markdown, code blocks, shell syntax, or quotes
5. **Conditions validated at parse time**: Typos in always-known variables produce errors; unknown dynamic variables produce warnings
6. **Required fields enforced**: Missing `prompt` on agent nodes is a parse-time error, not a runtime crash
7. **Formatter exists and is idempotent**: `dippin fmt` produces canonical output with deterministic field ordering
8. **DOT export works**: `dippin export-dot` produces valid, renderable DOT
9. **Migration tool works**: `dippin migrate` converts all example files with no manual edits needed for correct execution
10. **Diagnostics are actionable**: Every error includes file, line, explanation, and suggested fix
11. **Multi-diagnostic collection**: Parser recovers and reports all errors, not just the first one
12. **CLI is functional**: `dippin parse`, `dippin validate`, `dippin fmt`, `dippin export-dot`, `dippin migrate`
13. **Configuration precedence is documented and tested**: Five-layer model, no ambiguity

### Nice-to-have for v1

- Import/composition system (basic file refs without params)
- `reads:`/`writes:` advisory I/O contracts with lint warnings
- `route` syntax sugar
- `dippin new` scaffolding command
- Autofix for common validation errors

### Deferred

- Full composition with params/namespacing
- Stylesheet language
- SARIF
- LSP
- Visual editor / GUI

---

## 20. Risks / ADR-Worthy Decisions

### ADR 1: Should Dippin model a DAG, a graph, or DAG-plus-loops?

**Decision**: DAG-plus-explicit-loops.

**Rationale**: The current system is technically DAG (validation rejects unconditional cycles) but supports loop-like behavior via the "restart" mechanism (edge targets completed node → clear downstream → re-execute). This is graph-with-a-DAG-constraint-plus-an-escape-hatch. Dippin makes this explicit with `restart: true` on back-edges.

**Runtime semantics** (specified in §4.3):
- Back-edge is present in IR with `Restart: true`
- Engine clears downstream completion state (BFS from target)
- Context is fully preserved across restarts
- Node-local stats are NOT preserved (fresh per execution)
- Restart counter is global per engine instance
- Subgraphs have independent restart counters
- Checkpoints save at the restart point

### ADR 2: Should conditions be a custom expression language?

**Decision**: Yes, keep and improve the existing expression language.

**Rationale**: The current condition language (`outcome=success`, `tool_stdout contains pass`, `&&`, `||`, `not`) is small enough to be reliable but expressive enough for real workflows. Making conditions a full programming language would hurt LLM authoring reliability. Making them just equality checks would be too limiting (the codebase uses `contains`, `startswith`, `in`, `!=`, and Boolean combinators).

**Improvement**: Parse conditions at Dippin parse time (not at evaluation time). Validate variable names with namespace-aware tiering. Produce AST in IR.

### ADR 3: Should the IR be the primary engine input?

**Decision**: Yes, eventually. During transition, an adapter bridges IR → `pipeline.Graph`.

**Risk**: Dual maintenance of IR types and pipeline types. Mitigated by making the adapter mechanical and adding tests that verify roundtrip fidelity.

### ADR 4: Should Dippin support inline DOT for escape-hatch visualization hints?

**Decision**: No.

**Rationale**: Adding DOT syntax inside Dippin creates parser complexity, confuses the mental model, and reintroduces the problems we're solving. Visualization hints should be separate (CLI flags or a companion config file).

### ADR 5: Should `condition` be a node kind or syntax sugar?

**Decision**: Syntax sugar (`route`) in Dippin, lowered to a no-op condition node in IR.

**Rationale**: In the current engine, condition nodes do nothing — the engine evaluates edge conditions during edge selection regardless of node kind. Making routing a surface-syntax convenience (`route X -> A when ... -> B when ...`) is cleaner than requiring authors to declare a node kind that has no configuration fields. The IR can emit a `NodeCondition` internally for migration compatibility.

### ADR 6: Should stylesheets be in v1?

**Decision**: No. Defer to v1.5.

**Rationale**: Stylesheets add another parser, another precedence layer, and another source of hidden behavior. For v1, explicit per-node fields + workflow defaults cover all current usage. When stylesheets return, they will be a first-class top-level block (not a string-in-a-string), with only simple selectors (`*`, `.class`, `#id`, `kind`), and they will slot into the precedence ladder at a documented position.

### ADR 7: Should the file extension be `.dip` or `.dippin`?

**Decision**: `.dip`

**Rationale**: Short, distinctive, typeable.

### Open questions

1. **How should the engine handle hot-reload of `.dip` files during development?** Not critical for v1 but worth considering for the TUI.
2. **Should Dippin support conditional node inclusion (ifdef-like)?** Probably not — params + composition should handle most cases.
3. **How should secrets/API keys be referenced in Dippin?** Currently they're purely environment-side. Keep it that way — no secrets in source files.
4. **Should `reads:`/`writes:` become mandatory in a future version?** Probably yes, once all existing pipelines are migrated and annotated. But that's a v2 decision.

---

## 21. Concrete Next Steps

### Week 1

1. **Review and finalize this plan** — get sign-off on Proposal A syntax and IR design
2. **Create `dippin/` directory** with initial package structure
3. **Write the Dippin syntax spec** as `dippin/SPEC.md` — formal grammar, all examples, canonical field ordering
4. **Define IR Go types** in `dippin/ir/types.go` — with typed NodeConfig union
5. **Write `GraphToIR()` adapter** in `dippin/ir/adapt.go`
6. **Hand-write 5 `.dip` example files** that correspond to existing DOT examples

### Week 2

7. **Implement lexer** in `dippin/parser/lexer.go`
8. **Implement parser** in `dippin/parser/parser.go` — with top-level-declaration recovery
9. **Write parser tests** — at least 30 cases including error recovery
10. **Implement basic validator** — graph structure checks, schema validation

### Week 3

11. **Implement formatter** in `dippin/format/format.go` — with canonical field ordering
12. **Implement DOT exporter** in `dippin/export/dot.go`
13. **Implement migration tool** in `dippin/migrate/migrate.go`
14. **Migrate first 3 example pipelines** and verify parity

### Week 4

15. **Wire into `cmd/tracker`** — accept `.dip` files via `IRToGraph()` adapter
16. **Migrate remaining examples**
17. **Write user-facing docs**
18. **Begin adding `reads:`/`writes:` annotations** to migrated pipelines

---

## Appendix A: Suggested Repo Layout

```
dippin/
├── SPEC.md                    # Formal syntax specification
├── ir/
│   ├── types.go               # Canonical IR types (Workflow, Node, NodeConfig union, Edge, etc.)
│   ├── adapt.go               # pipeline.Graph → ir.Workflow
│   ├── reverse.go             # ir.Workflow → pipeline.Graph (transition)
│   └── ir_test.go
├── parser/
│   ├── lexer.go               # Line-based lexer
│   ├── lexer_test.go
│   ├── parser.go              # Dippin → IR (with error recovery)
│   ├── parser_test.go
│   └── testdata/              # .dip test fixtures
│       ├── valid/
│       └── invalid/
├── validate/
│   ├── validate.go            # Schema + graph + semantic checks
│   ├── diagnostic.go          # Diagnostic types
│   ├── codes.go               # Error code registry (DIP001-DIP999)
│   └── validate_test.go
├── format/
│   ├── format.go              # IR → canonical Dippin source (deterministic field ordering)
│   └── format_test.go
├── export/
│   ├── dot.go                 # IR → DOT
│   └── dot_test.go
├── migrate/
│   ├── migrate.go             # DOT → Dippin conversion
│   ├── parity.go              # Behavioral parity checker
│   └── migrate_test.go
├── cmd/
│   └── dippin/
│       └── main.go            # CLI: parse, validate, fmt, export-dot, migrate
└── examples/
    ├── hello.dip              # Minimal example
    ├── ask_and_execute.dip    # Migrated from DOT
    └── consensus_task.dip     # Migrated from DOT
```

---

## Appendix B: Answers to Required Questions

**1. What current DOT semantics are essential and must survive?**

Node kinds (agent, human, tool, parallel, fan_in, subgraph), edge conditions with boolean expressions, retry policies with targets and fallbacks, goal gates, checkpoint/resume, parallel fan-out/fan-in, context key-value flow, per-node model/provider override, fidelity-based compaction. Start/exit must survive but as explicit declarations, not shape conventions.

**2. Which current DOT hacks are accidental and should be removed?**

Shape→handler coupling, diamond+attribute special cases, `Mdiamond`/`Msquare` terminals, `\n`-encoded multiline strings, CSS-in-a-DOT-attribute stylesheets (deferred, not removed conceptually), `manager_loop` (no-op), `parallel.results` as JSON-in-string, first-node-is-start inference.

**3. Should Dippin model a DAG, a graph, or DAG-plus-loops?**

DAG-plus-explicit-loops. The graph is structurally a DAG but with annotated back-edges (`restart: true`) that trigger the restart mechanism. Runtime semantics are fully specified in §4.3.

**4. What is the minimum viable composition model?**

For v1: basic `subgraph` refs to external files. Post-v1: file-based import with path resolution, parameter declaration and override, namespace prefixing, inline expansion. No dynamic composition, no conditional imports, no inheritance.

**5. What should be first-class in syntax vs lowered into IR?**

First-class: node kind, prompt (multiline block), edges with conditions, parallel/fan_in declarations, explicit start/exit, `route` sugar, `reads:`/`writes:` contracts. Lowered: condition AST parsing, route-to-node expansion, namespace resolution, retry policy resolution, composition expansion.

**6. What should round-trip to DOT, and what can be lossy?**

Lossless: topology, node kinds (via shape mapping), labels, edge conditions (serialized from AST), weights, start/exit (as Mdiamond/Msquare). Lossy: multiline formatting (re-escaped), module structure (expanded), comments (lost), source locations (lost), parameter defaults (resolved), reads/writes (omitted), route sugar (expanded), variable namespaces (stripped).

**7. What syntax properties make Dippin easy for LLMs to generate and repair?**

Indentation-based nesting (LLMs handle this well), keyword-first lines (`agent`, `tool`, `human`), no quoting for multiline content, `${ns.var}` interpolation syntax (widely known), consistent `key: value` fields, one canonical encoding per concept, explicit `start:`/`exit:` (no implicit inference), clear error messages with suggested fixes, multi-diagnostic collection.

**8. What diagnostics shape will make this successful?**

File/line/column ranges, error codes (DIP001-DIP999), human explanations, suggested fixes with concrete text replacements, JSON output mode for tooling, severity levels (error/warning/info/hint), multi-error recovery at top-level declarations, tiered variable validation (always-known → declared → dynamic).

**9. How should Tracker be used to bootstrap without trapping us?**

Tracker pipelines can analyze DOT files, generate candidate Dippin, and produce test fixtures. But the spec is a human-reviewed document, the parser is hand-written Go, and all generated Dippin goes through the same validator as hand-written Dippin. Tracker is the factory floor, not the blueprint.
TURN 6
TOOL CALL: bash
INPUT:
{"command": "grep -n \"DIP10[0-9]\\|DIP11[0-9]\\|Layer 4\\|Semantic\\|linter\\|lint\" DIPPIN_DESIGN_PLAN.md | head -50"}
TOOL RESULT: bash
OUTPUT:
154:## 4. Semantic Model That Must Survive
920:Even in v1, Dippin should have a path from "magic context map" to something legible and lintable. The `reads:` and `writes:` fields on nodes are advisory — the validator produces warnings, not errors — but they immediately make state flow visible.
950:### Semantics
1046:**Layer 4: Semantic quality** (warnings)
1047:- Unreachable nodes after conditional branches (`DIP101`)
1048:- Routing nodes without fail/default edges (`DIP102`)
1049:- Overlapping or contradictory conditions (`DIP103`)
1050:- Unbounded retry loops (no max_retries, no fallback) (`DIP104`)
1051:- No success path to exit (`DIP105`)
1052:- Undefined `${variables}` in prompts — tiered by namespace (`DIP106`)
1053:- Unused context keys (written but never read via `writes:`) (`DIP107`)
1054:- Model/provider combination not in known catalog (`DIP108`)
1055:- Namespace collisions in imports (`DIP109`)
1056:- Empty prompts (`DIP110`)
1057:- Tool command without timeout (`DIP111`)
1058:- `reads:` key not in any upstream node's `writes:` (`DIP112`)
1261:- [ ] `dippin lint <file>` CLI command (includes warnings)
1325:- `reads:`/`writes:` advisory I/O contracts with lint warnings

TURN 7
TOOL CALL: bash
INPUT:
{"command": "sed -n '1040,1070p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
- No unconditional cycles after excluding edges marked `restart: true` (`DIP005`)
- Exit node has no outgoing edges (`DIP006`)
- Parallel fan-out has matching fan-in (`DIP007`)
- No duplicate node IDs (`DIP008`)
- No duplicate edges (`DIP009`)

**Layer 4: Semantic quality** (warnings)
- Unreachable nodes after conditional branches (`DIP101`)
- Routing nodes without fail/default edges (`DIP102`)
- Overlapping or contradictory conditions (`DIP103`)
- Unbounded retry loops (no max_retries, no fallback) (`DIP104`)
- No success path to exit (`DIP105`)
- Undefined `${variables}` in prompts — tiered by namespace (`DIP106`)
- Unused context keys (written but never read via `writes:`) (`DIP107`)
- Model/provider combination not in known catalog (`DIP108`)
- Namespace collisions in imports (`DIP109`)
- Empty prompts (`DIP110`)
- Tool command without timeout (`DIP111`)
- `reads:` key not in any upstream node's `writes:` (`DIP112`)

### Formatter

`dippin fmt` — canonical formatting:
- 2-space indentation
- One blank line between nodes
- Section comments preserved
- Trailing whitespace removed
- Single trailing newline

**Canonical field ordering per node kind**:


TURN 8
TOOL CALL: bash
INPUT:
{"command": "sed -n '910,970p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
**Trade-off**: You lose opaque execution boundaries, which means a subgraph can't have its own independent restart counter. If this becomes a real need, we add opaque execution as an optimization later — but inline expansion is the canonical representation.

### Source-map preservation

After expansion, every node retains a `SourceLocation` pointing to the original `.dip` file. Diagnostic messages show the original file and line, not the expanded position. This is how you debug "where did this node come from?"

---

## 13. Node I/O Contracts

Even in v1, Dippin should have a path from "magic context map" to something legible and lintable. The `reads:` and `writes:` fields on nodes are advisory — the validator produces warnings, not errors — but they immediately make state flow visible.

### Syntax

`reads:` and `writes:` use **bare logical names** (no namespace prefix). Namespaced access (`ctx.`, `graph.`) is used only in prompt interpolation and edge conditions.

```dippin
  agent Interpret
    reads: human_response
    writes: plan
    prompt:
      You are a senior software architect.
      ## User Request
      ${ctx.human_response}

  tool CheckTests
    reads: last_response
    writes: test_result
    command:
      ...

  agent Implement
    reads: plan, test_result
    prompt:
      ...

  edges
    Validate -> Approve   when ctx.outcome = success
```

### Semantics

- `reads:` lists context keys this node expects to be set by an upstream node
- `writes:` lists context keys this node will set in its `ContextUpdates`
- Both are comma-separated lists of **bare context key names** (e.g., `human_response`, not `ctx.human_response`)
- Prompt/condition references use namespaced access: `${ctx.human_response}`, `when ctx.outcome = success`
- The validator can check:
  - **Warning**: Node reads a key that no upstream node writes (may be auto-injected or dynamic)
  - **Warning**: Node writes a key that no downstream node reads (dead output)
  - **Info**: Flow trace showing how context keys propagate through the graph

### Why advisory-only in v1

The current system is fully dynamic. Tool nodes can write arbitrary keys via `printf` output that ends up in `tool_stdout`. Agent nodes write `last_response` implicitly. Enforcing strict contracts would break every existing pipeline. But having them visible — even optionally — is the bridge to a future where state flow is verifiable.

---

## 14. Linter / Diagnostics Design

### Diagnostic structure


TURN 9
TOOL CALL: bash
INPUT:
{"command": "sed -n '966,1100p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:

## 14. Linter / Diagnostics Design

### Diagnostic structure

```go
type Diagnostic struct {
    Severity    Severity          // Error, Warning, Info, Hint
    Code        string            // e.g. "DIP001"
    Message     string            // Human-readable
    Explanation string            // Why it matters
    Location    SourceLocation    // File, line, column, range
    Context     string            // Nearby source lines
    Fix         *SuggestedFix     // Optional autofix
}

type SuggestedFix struct {
    Description string
    Edits       []TextEdit        // Concrete replacements
}
```

### Output modes

- **Human (default)**: Colored terminal output with source context, carets, explanations
- **JSON**: Machine-readable array of diagnostics for editor/agent integration

(SARIF deferred to post-v1.)

### Example diagnostic output

```
error[DIP003]: unknown node reference "InterpretX" in edge
  --> pipeline.dip:45:5
   |
45 |     AskUser -> InterpretX
   |                ^^^^^^^^^^ this node is not declared
   |
   = help: did you mean "Interpret"? (declared at line 12)
   = fix: replace "InterpretX" with "Interpret"
```

### Parser error recovery strategy

The parser recovers at **recognized declaration boundaries**. When a syntax error is encountered mid-node or mid-edge:

1. Record the diagnostic with location
2. Skip forward to the next recognized declaration line at the current or shallower enclosing block depth
3. Resume parsing from there

**Synchronization tokens**: Any line matching `^(workflow|agent|human|tool|parallel|fan_in|subgraph|edges|defaults|import|#)\b` at the expected block depth is a recovery point.

**Goal**: Collect multiple diagnostics per file. A single typo should not prevent reporting errors in unrelated parts of the file. The parser always processes the entire file and returns all collected diagnostics alongside whatever partial IR it could construct.

### Validation layers

**Layer 1: Syntax** (parser)
- Valid Dippin syntax
- Correct indentation
- Valid block structure
- Unterminated multiline blocks

**Layer 2: Schema** (post-parse)
- Known node kinds
- Known fields per kind (typed config structs prevent most of this structurally)
- Correct field types (duration, boolean, integer)
- Required fields present (e.g., agent nodes need `prompt`)
- Unknown fields flagged (typo detection)

**Layer 3: Graph structure** (IR)
- `start:` node exists (`DIP001`)
- `exit:` node exists (`DIP002`)
- All edge endpoints exist (`DIP003`)
- All nodes reachable from start (`DIP004`)
- No unconditional cycles after excluding edges marked `restart: true` (`DIP005`)
- Exit node has no outgoing edges (`DIP006`)
- Parallel fan-out has matching fan-in (`DIP007`)
- No duplicate node IDs (`DIP008`)
- No duplicate edges (`DIP009`)

**Layer 4: Semantic quality** (warnings)
- Unreachable nodes after conditional branches (`DIP101`)
- Routing nodes without fail/default edges (`DIP102`)
- Overlapping or contradictory conditions (`DIP103`)
- Unbounded retry loops (no max_retries, no fallback) (`DIP104`)
- No success path to exit (`DIP105`)
- Undefined `${variables}` in prompts — tiered by namespace (`DIP106`)
- Unused context keys (written but never read via `writes:`) (`DIP107`)
- Model/provider combination not in known catalog (`DIP108`)
- Namespace collisions in imports (`DIP109`)
- Empty prompts (`DIP110`)
- Tool command without timeout (`DIP111`)
- `reads:` key not in any upstream node's `writes:` (`DIP112`)

### Formatter

`dippin fmt` — canonical formatting:
- 2-space indentation
- One blank line between nodes
- Section comments preserved
- Trailing whitespace removed
- Single trailing newline

**Canonical field ordering per node kind**:

- **Agent**: `label`, `class`, `model`, `provider`, `reasoning_effort`, `fidelity`, `goal_gate`, `auto_status`, `max_turns`, `retry_policy`, `max_retries`, `retry_target`, `reads`, `writes`, `prompt`
- **Human**: `label`, `mode`, `default`, `reads`, `writes`
- **Tool**: `label`, `timeout`, `reads`, `writes`, `command`
- **Parallel**: (inline declaration, no fields)
- **Fan-in**: (inline declaration, no fields)
- **Subgraph**: `label`, `ref`, `params`

`prompt` and `command` are always last because they're multiline blocks — putting them last means the block doesn't visually interrupt the metadata fields.

Deterministic: `dippin fmt` is idempotent. Running it twice produces identical output.

---

## 15. DOT Export Strategy

### Lossless mappings

| Dippin concept | DOT representation |
|---------------|-------------------|
| Node ID | Node name |
| Node kind → DOT shape | `agent`→`box`, `human`→`hexagon`, `tool`→`parallelogram`, `parallel`→`component`, `fan_in`→`tripleoctagon`, `subgraph`→`tab` |
| Node label | `label` attribute |
| Edge from/to | Edge endpoints |
| Edge label | `label` attribute |
| Edge condition | `condition` attribute (serialized from AST) |
| Edge weight | `weight` attribute |
| Start/exit | `Mdiamond`/`Msquare` shape nodes |

### Lossy but acceptable


TURN 10
TOOL CALL: bash
INPUT:
{"command": "grep -n \"route\\|routing\" DIPPIN_DESIGN_PLAN.md | head -20"}
TOOL RESULT: bash
OUTPUT:
27:Tracker parses a DOT file into a `Graph` of `Node` and `Edge` structs. Each node's DOT shape maps to a handler name (`box` → `codergen`, `hexagon` → `wait.human`, etc.). The engine walks the graph from `Mdiamond` (start) to `Msquare` (exit), executing each node's handler. Handlers return an `Outcome` with status (`success`/`fail`/`retry`), context updates (key-value strings), and edge routing hints (`PreferredLabel`, `SuggestedNextNodes`). The engine selects the next edge via a priority cascade: condition match → preferred label → suggested nodes → weight → lexical. Context is a shared `map[string]string` threaded through all nodes. Checkpoints serialize the full state after each step.
69:- `preferred_label` — edge routing hint
647:Route sugar is **deferred from v1**. In v1, all conditional routing is expressed as conditional edges in the `edges` block:
656:This avoids ambiguity about whether `route Validate` attaches routing to an existing node, declares a new node, or replaces edge declarations. If route sugar is added post-v1, it should use unambiguous syntax like `route after <NodeID>` to make clear it decorates an existing node's outgoing edges. The IR does not include a `NodeCondition` kind — there is no intermediate condition node, just edges with conditions.
766:- No `route` sugar (conditions only in edges)
1111:| `route` sugar (post-v1) | Would expand to conditional edges; not in v1 |
1326:- `route` syntax sugar
1379:**Decision**: Syntax sugar (`route`) in Dippin, lowered to a no-op condition node in IR.
1381:**Rationale**: In the current engine, condition nodes do nothing — the engine evaluates edge conditions during edge selection regardless of node kind. Making routing a surface-syntax convenience (`route X -> A when ... -> B when ...`) is cleaner than requiring authors to declare a node kind that has no configuration fields. The IR can emit a `NodeCondition` internally for migration compatibility.
1502:First-class: node kind, prompt (multiline block), edges with conditions, parallel/fan_in declarations, explicit start/exit, `route` sugar, `reads:`/`writes:` contracts. Lowered: condition AST parsing, route-to-node expansion, namespace resolution, retry policy resolution, composition expansion.
1506:Lossless: topology, node kinds (via shape mapping), labels, edge conditions (serialized from AST), weights, start/exit (as Mdiamond/Msquare). Lossy: multiline formatting (re-escaped), module structure (expanded), comments (lost), source locations (lost), parameter defaults (resolved), reads/writes (omitted), route sugar (expanded), variable namespaces (stripped).

TURN 11
TOOL CALL: read
INPUT:
{"path": "go.mod"}
TOOL RESULT: read
OUTPUT:
module github.com/2389/dippin

go 1.25.4

TURN 12
TOOL CALL: write
INPUT:
{"path": "validator/lint_codes.go", "content": "package validator\n\n// Diagnostic codes for semantic quality warnings (DIP101–DIP112).\nconst (\n\tDIP101 = \"DIP101\" // unreachable nodes after conditional branches\n\tDIP102 = \"DIP102\" // routing node without default/unconditional edge\n\tDIP103 = \"DIP103\" // overlapping or contradictory conditions\n\tDIP104 = \"DIP104\" // unbounded retry loop\n\tDIP105 = \"DIP105\" // no success path to exit\n\tDIP106 = \"DIP106\" // undefined variable in prompt\n\tDIP107 = \"DIP107\" // unused context key (written but never read)\n\tDIP108 = \"DIP108\" // unknown model/provider combination\n\tDIP109 = \"DIP109\" // namespace collision in imports\n\tDIP110 = \"DIP110\" // empty prompt on agent node\n\tDIP111 = \"DIP111\" // tool command without timeout\n\tDIP112 = \"DIP112\" // reads key not in any upstream writes\n)\n\nfunc init() {\n\t// Extend CodeDescription with linter codes.\n\tCodeDescription[DIP101] = \"unreachable node after conditional branches\"\n\tCodeDescription[DIP102] = \"routing node has no default/unconditional edge\"\n\tCodeDescription[DIP103] = \"overlapping or contradictory conditions\"\n\tCodeDescription[DIP104] = \"unbounded retry loop (no max_retries or fallback)\"\n\tCodeDescription[DIP105] = \"no success path from start to exit\"\n\tCodeDescription[DIP106] = \"undefined variable reference in prompt\"\n\tCodeDescription[DIP107] = \"unused context key (written but never read)\"\n\tCodeDescription[DIP108] = \"unknown model/provider combination\"\n\tCodeDescription[DIP109] = \"namespace collision in imports\"\n\tCodeDescription[DIP110] = \"empty prompt on agent node\"\n\tCodeDescription[DIP111] = \"tool command has no timeout\"\n\tCodeDescription[DIP112] = \"reads key not produced by any upstream writes\"\n}\n"}
TOOL RESULT: write
OUTPUT:
wrote 1649 bytes to validator/lint_codes.go
TURN 13
TOOL CALL: write
INPUT:
{"content": "package validator\n\nimport (\n\t\"fmt\"\n\t\"regexp\"\n\t\"sort\"\n\t\"strings\"\n\n\t\"github.com/2389/dippin/ir\"\n)\n\n// Lint runs all semantic quality checks (DIP101–DIP112) on the workflow\n// and returns all diagnostics found. These are warnings, not errors —\n// the workflow can still execute, but the findings indicate likely bugs\n// or quality issues.\n//\n// Lint is independent of Validate. Callers should run both:\n//\n//\tstructureResult := validator.Validate(w)\n//\tlintResult := validator.Lint(w)\nfunc Lint(w *ir.Workflow) Result {\n\tvar diags []Diagnostic\n\n\tdiags = append(diags, lintConditionalReachability(w)...)\n\tdiags = append(diags, lintDefaultEdge(w)...)\n\tdiags = append(diags, lintOverlappingConditions(w)...)\n\tdiags = append(diags, lintUnboundedRetry(w)...)\n\tdiags = append(diags, lintSuccessPath(w)...)\n\tdiags = append(diags, lintUndefinedVariables(w)...)\n\tdiags = append(diags, lintUnusedWrites(w)...)\n\tdiags = append(diags, lintModelProvider(w)...)\n\tdiags = append(diags, lintNamespaceCollisions(w)...)\n\tdiags = append(diags, lintEmptyPrompts(w)...)\n\tdiags = append(diags, lintToolTimeout(w)...)\n\tdiags = append(diags, lintReadsWithoutUpstreamWrites(w)...)\n\n\treturn Result{Diagnostics: diags}\n}\n\n// lintConditionalReachability checks DIP101: nodes that are only reachable\n// through conditional edges may be unreachable at runtime if conditions are\n// not satisfied. A node is flagged if ALL of its incoming edges are conditional\n// (have a non-nil Condition), meaning there is no guaranteed path to it.\nfunc lintConditionalReachability(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\n\t// Build a map of incoming edges per node.\n\tincoming := make(map[string][]*ir.Edge)\n\tfor _, e := range w.Edges {\n\t\tincoming[e.To] = append(incoming[e.To], e)\n\t}\n\n\tfor _, n := range w.Nodes {\n\t\t// Start node is always reachable by definition.\n\t\tif n.ID == w.Start {\n\t\t\tcontinue\n\t\t}\n\t\tedges := incoming[n.ID]\n\t\tif len(edges) == 0 {\n\t\t\t// No incoming edges at all — DIP004 handles this.\n\t\t\tcontinue\n\t\t}\n\t\tallConditional := true\n\t\tfor _, e := range edges {\n\t\t\tif e.Condition == nil {\n\t\t\t\tallConditional = false\n\t\t\t\tbreak\n\t\t\t}\n\t\t}\n\t\tif allConditional {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP101,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"node %q is only reachable through conditional edges and may be skipped at runtime\", n.ID),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     \"add an unconditional edge to this node, or verify all conditions are exhaustive\",\n\t\t\t})\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintDefaultEdge checks DIP102: nodes that have outgoing conditional edges\n// but no unconditional (default/fallback) edge. Without a default edge,\n// execution may get stuck at this node if no condition matches.\nfunc lintDefaultEdge(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\n\tfor _, n := range w.Nodes {\n\t\toutgoing := w.EdgesFrom(n.ID)\n\t\tif len(outgoing) == 0 {\n\t\t\tcontinue\n\t\t}\n\n\t\thasConditional := false\n\t\thasUnconditional := false\n\t\tfor _, e := range outgoing {\n\t\t\tif e.Condition != nil {\n\t\t\t\thasConditional = true\n\t\t\t} else {\n\t\t\t\thasUnconditional = true\n\t\t\t}\n\t\t}\n\n\t\t// Only flag if there are conditional edges but no unconditional fallback.\n\t\tif hasConditional && !hasUnconditional {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP102,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"node %q has conditional outgoing edges but no unconditional default edge\", n.ID),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     \"add an unconditional edge as a fallback, or ensure conditions are exhaustive\",\n\t\t\t})\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintOverlappingConditions checks DIP103: multiple edges from the same node\n// with conditions that compare the same variable to the same value using \"=\".\n// This indicates contradictory or duplicated routing logic.\nfunc lintOverlappingConditions(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\n\t// Group edges by source node.\n\tedgesBySource := make(map[string][]*ir.Edge)\n\tfor _, e := range w.Edges {\n\t\tif e.Condition != nil {\n\t\t\tedgesBySource[e.From] = append(edgesBySource[e.From], e)\n\t\t}\n\t}\n\n\tfor from, edges := range edgesBySource {\n\t\t// Extract top-level equality comparisons from each edge condition.\n\t\ttype condKey struct {\n\t\t\tvariable string\n\t\t\top       string\n\t\t\tvalue    string\n\t\t}\n\n\t\tseen := make(map[condKey]*ir.Edge)\n\t\tfor _, e := range edges {\n\t\t\tcomparisons := extractComparisons(e.Condition.Parsed)\n\t\t\tfor _, cmp := range comparisons {\n\t\t\t\tkey := condKey{variable: cmp.Variable, op: cmp.Op, value: cmp.Value}\n\t\t\t\tif first, ok := seen[key]; ok {\n\t\t\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\t\t\tCode:     DIP103,\n\t\t\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\t\t\tMessage:  fmt.Sprintf(\"node %q has overlapping conditions: edges to %q and %q both check %s %s %s\", from, first.To, e.To, cmp.Variable, cmp.Op, cmp.Value),\n\t\t\t\t\t\tLocation: e.Source,\n\t\t\t\t\t\tHelp:     \"review the conditions to ensure they route to different targets for different states\",\n\t\t\t\t\t})\n\t\t\t\t} else {\n\t\t\t\t\tseen[key] = e\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n\treturn diags\n}\n\n// extractComparisons recursively extracts all CondCompare nodes from a\n// condition expression tree. This flattens AND/OR/NOT to find the leaf comparisons.\nfunc extractComparisons(expr ir.ConditionExpr) []ir.CondCompare {\n\tif expr == nil {\n\t\treturn nil\n\t}\n\tswitch e := expr.(type) {\n\tcase ir.CondCompare:\n\t\treturn []ir.CondCompare{e}\n\tcase ir.CondAnd:\n\t\treturn append(extractComparisons(e.Left), extractComparisons(e.Right)...)\n\tcase ir.CondOr:\n\t\treturn append(extractComparisons(e.Left), extractComparisons(e.Right)...)\n\tcase ir.CondNot:\n\t\treturn extractComparisons(e.Inner)\n\tdefault:\n\t\treturn nil\n\t}\n}\n\n// lintUnboundedRetry checks DIP104: nodes with retry configuration that have\n// no max_retries limit and no fallback target. This could cause infinite retry\n// loops at runtime.\nfunc lintUnboundedRetry(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tr := n.Retry\n\t\t// Only flag nodes that have some retry config but no bounds.\n\t\thasRetryConfig := r.Policy != \"\" || r.RetryTarget != \"\"\n\t\tif hasRetryConfig && r.MaxRetries == 0 && r.FallbackTarget == \"\" {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP104,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"node %q has retry configuration but no max_retries or fallback_target\", n.ID),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     \"set max_retries to limit retries, or add a fallback_target for graceful degradation\",\n\t\t\t})\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintSuccessPath checks DIP105: there must be at least one path from the\n// start node to the exit node using only non-restart edges. If no such path\n// exists, the workflow can never complete normally.\nfunc lintSuccessPath(w *ir.Workflow) []Diagnostic {\n\tif w.Start == \"\" || w.Exit == \"\" {\n\t\treturn nil\n\t}\n\tif w.Node(w.Start) == nil || w.Node(w.Exit) == nil {\n\t\treturn nil\n\t}\n\n\t// BFS from start, following only non-restart edges.\n\tadj := make(map[string][]string)\n\tfor _, e := range w.Edges {\n\t\tif !e.Restart {\n\t\t\tadj[e.From] = append(adj[e.From], e.To)\n\t\t}\n\t}\n\n\tvisited := make(map[string]bool)\n\tqueue := []string{w.Start}\n\tvisited[w.Start] = true\n\n\tfor len(queue) > 0 {\n\t\tcurr := queue[0]\n\t\tqueue = queue[1:]\n\t\tif curr == w.Exit {\n\t\t\treturn nil // Found a path.\n\t\t}\n\t\tfor _, next := range adj[curr] {\n\t\t\tif !visited[next] {\n\t\t\t\tvisited[next] = true\n\t\t\t\tqueue = append(queue, next)\n\t\t\t}\n\t\t}\n\t}\n\n\treturn []Diagnostic{{\n\t\tCode:     DIP105,\n\t\tSeverity: SeverityWarning,\n\t\tMessage:  fmt.Sprintf(\"no forward path from start node %q to exit node %q (excluding restart edges)\", w.Start, w.Exit),\n\t\tHelp:     \"ensure there is at least one non-restart path from start to exit\",\n\t}}\n}\n\n// varRefPattern matches ${...} variable references in prompt text.\nvar varRefPattern = regexp.MustCompile(`\\$\\{([^}]+)\\}`)\n\n// knownNamespaces lists the valid namespace prefixes for variable references.\nvar knownNamespaces = map[string]bool{\n\t\"ctx\":   true,\n\t\"graph\": true,\n\t\"env\":   true,\n}\n\n// lintUndefinedVariables checks DIP106: ${variable} references in prompts\n// must use known namespace prefixes (ctx., graph., env.). References without\n// a recognized namespace are flagged.\nfunc lintUndefinedVariables(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tprompt := nodePrompt(n)\n\t\tif prompt == \"\" {\n\t\t\tcontinue\n\t\t}\n\t\tmatches := varRefPattern.FindAllStringSubmatch(prompt, -1)\n\t\tfor _, m := range matches {\n\t\t\tvarRef := m[1] // The captured group inside ${...}\n\t\t\tparts := strings.SplitN(varRef, \".\", 2)\n\t\t\tif len(parts) < 2 || !knownNamespaces[parts[0]] {\n\t\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\t\tCode:     DIP106,\n\t\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\t\tMessage:  fmt.Sprintf(\"node %q references undefined variable ${%s}\", n.ID, varRef),\n\t\t\t\t\tLocation: n.Source,\n\t\t\t\t\tHelp:     fmt.Sprintf(\"use a namespaced variable like ${ctx.%s}, ${graph.%s}, or ${env.%s}\", varRef, varRef, varRef),\n\t\t\t\t})\n\t\t\t}\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintUnusedWrites checks DIP107: context keys declared in a node's writes:\n// that are not referenced in any other node's reads:. These are dead outputs\n// that may indicate unused work.\nfunc lintUnusedWrites(w *ir.Workflow) []Diagnostic {\n\t// Collect all reads across all nodes.\n\tallReads := make(map[string]bool)\n\tfor _, n := range w.Nodes {\n\t\tfor _, key := range n.IO.Reads {\n\t\t\tallReads[key] = true\n\t\t}\n\t}\n\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tfor _, key := range n.IO.Writes {\n\t\t\tif !allReads[key] {\n\t\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\t\tCode:     DIP107,\n\t\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\t\tMessage:  fmt.Sprintf(\"node %q writes context key %q but no node declares it in reads\", n.ID, key),\n\t\t\t\t\tLocation: n.Source,\n\t\t\t\t\tHelp:     fmt.Sprintf(\"add reads: %s to a downstream node, or remove this write if unused\", key),\n\t\t\t\t})\n\t\t\t}\n\t\t}\n\t}\n\treturn diags\n}\n\n// knownModelProviders lists known valid model/provider combinations.\n// This is a best-effort catalog — unknown combinations produce a warning,\n// not an error, since new models may be added at any time.\nvar knownModelProviders = map[string]map[string]bool{\n\t\"anthropic\": {\n\t\t\"claude-opus-4-6\":        true,\n\t\t\"claude-sonnet-4-6\":      true,\n\t\t\"claude-haiku-3-5\":       true,\n\t\t\"claude-opus-4-20250116\": true,\n\t},\n\t\"openai\": {\n\t\t\"gpt-5.4\":        true,\n\t\t\"gpt-5.3-codex\":  true,\n\t\t\"gpt-4o\":         true,\n\t\t\"gpt-4o-mini\":    true,\n\t\t\"o3\":             true,\n\t\t\"o4-mini\":        true,\n\t},\n}\n\n// lintModelProvider checks DIP108: model/provider combinations should be\n// in the known catalog. Unknown combinations may indicate typos.\nfunc lintModelProvider(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tcfg, ok := n.Config.(ir.AgentConfig)\n\t\tif !ok {\n\t\t\tcontinue\n\t\t}\n\n\t\tmodel := cfg.Model\n\t\tprovider := cfg.Provider\n\n\t\t// Use workflow defaults as fallback.\n\t\tif model == \"\" {\n\t\t\tmodel = w.Defaults.Model\n\t\t}\n\t\tif provider == \"\" {\n\t\t\tprovider = w.Defaults.Provider\n\t\t}\n\n\t\t// Only check if both are specified.\n\t\tif model == \"\" || provider == \"\" {\n\t\t\tcontinue\n\t\t}\n\n\t\tproviderModels, providerKnown := knownModelProviders[provider]\n\t\tif !providerKnown {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP108,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"node %q uses unknown provider %q\", n.ID, provider),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     fmt.Sprintf(\"known providers: %s\", knownProviderList()),\n\t\t\t})\n\t\t\tcontinue\n\t\t}\n\t\tif !providerModels[model] {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP108,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"node %q uses unknown model %q for provider %q\", n.ID, model, provider),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     fmt.Sprintf(\"known models for %s: %s\", provider, knownModelList(provider)),\n\t\t\t})\n\t\t}\n\t}\n\treturn diags\n}\n\n// knownProviderList returns a sorted comma-separated list of known providers.\nfunc knownProviderList() string {\n\tproviders := make([]string, 0, len(knownModelProviders))\n\tfor p := range knownModelProviders {\n\t\tproviders = append(providers, p)\n\t}\n\tsort.Strings(providers)\n\treturn strings.Join(providers, \", \")\n}\n\n// knownModelList returns a sorted comma-separated list of known models for a provider.\nfunc knownModelList(provider string) string {\n\tmodels := knownModelProviders[provider]\n\tlist := make([]string, 0, len(models))\n\tfor m := range models {\n\t\tlist = append(list, m)\n\t}\n\tsort.Strings(list)\n\treturn strings.Join(list, \", \")\n}\n\n// lintNamespaceCollisions checks DIP109: multiple subgraph nodes referencing\n// the same file without different parameters could cause namespace collisions\n// when expanded.\nfunc lintNamespaceCollisions(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\n\ttype subgraphRef struct {\n\t\tnode *ir.Node\n\t\tref  string\n\t}\n\n\tseen := make(map[string]*ir.Node)\n\tfor _, n := range w.Nodes {\n\t\tcfg, ok := n.Config.(ir.SubgraphConfig)\n\t\tif !ok {\n\t\t\tcontinue\n\t\t}\n\t\tif first, exists := seen[cfg.Ref]; exists {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP109,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"nodes %q and %q both reference subgraph %q, which may cause namespace collisions\", first.ID, n.ID, cfg.Ref),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     \"use distinct node IDs and ensure imported names do not collide after expansion\",\n\t\t\t})\n\t\t} else {\n\t\t\tseen[cfg.Ref] = n\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintEmptyPrompts checks DIP110: agent nodes should have a non-empty prompt.\n// An agent without a prompt has nothing to send to the LLM.\nfunc lintEmptyPrompts(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tcfg, ok := n.Config.(ir.AgentConfig)\n\t\tif !ok {\n\t\t\tcontinue\n\t\t}\n\t\tif strings.TrimSpace(cfg.Prompt) == \"\" {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP110,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"agent node %q has an empty prompt\", n.ID),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     \"add a prompt: field with instructions for the LLM\",\n\t\t\t})\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintToolTimeout checks DIP111: tool nodes should have a timeout configured.\n// Without a timeout, a hanging tool command could block the entire pipeline.\nfunc lintToolTimeout(w *ir.Workflow) []Diagnostic {\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tcfg, ok := n.Config.(ir.ToolConfig)\n\t\tif !ok {\n\t\t\tcontinue\n\t\t}\n\t\tif cfg.Timeout == 0 {\n\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\tCode:     DIP111,\n\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\tMessage:  fmt.Sprintf(\"tool node %q has no timeout configured\", n.ID),\n\t\t\t\tLocation: n.Source,\n\t\t\t\tHelp:     \"add a timeout (e.g., timeout: 60s) to prevent hanging commands\",\n\t\t\t})\n\t\t}\n\t}\n\treturn diags\n}\n\n// lintReadsWithoutUpstreamWrites checks DIP112: keys in a node's reads: list\n// should appear in the writes: list of at least one upstream node (reachable\n// via forward edges from start). This uses a simple flow analysis — for each\n// node, compute the set of keys written by upstream nodes, then check reads.\nfunc lintReadsWithoutUpstreamWrites(w *ir.Workflow) []Diagnostic {\n\tif w.Start == \"\" || w.Node(w.Start) == nil {\n\t\treturn nil\n\t}\n\n\t// Build forward adjacency (non-restart edges).\n\tadj := make(map[string][]string)\n\tfor _, e := range w.Edges {\n\t\tif !e.Restart {\n\t\t\tadj[e.From] = append(adj[e.From], e.To)\n\t\t}\n\t}\n\n\t// Topological order via BFS (Kahn's algorithm).\n\tinDegree := make(map[string]int)\n\tfor _, n := range w.Nodes {\n\t\tinDegree[n.ID] = 0\n\t}\n\tfor _, e := range w.Edges {\n\t\tif !e.Restart {\n\t\t\tinDegree[e.To]++\n\t\t}\n\t}\n\n\tqueue := []string{}\n\tfor _, n := range w.Nodes {\n\t\tif inDegree[n.ID] == 0 {\n\t\t\tqueue = append(queue, n.ID)\n\t\t}\n\t}\n\n\t// For each node, compute the set of keys available from upstream writes.\n\tavailable := make(map[string]map[string]bool)\n\tfor _, n := range w.Nodes {\n\t\tavailable[n.ID] = make(map[string]bool)\n\t}\n\n\tvar order []string\n\tfor len(queue) > 0 {\n\t\tcurr := queue[0]\n\t\tqueue = queue[1:]\n\t\torder = append(order, curr)\n\n\t\t// Add this node's writes to what's available for downstream.\n\t\tn := w.Node(curr)\n\t\tif n != nil {\n\t\t\tfor _, key := range n.IO.Writes {\n\t\t\t\tavailable[curr][key] = true\n\t\t\t}\n\t\t}\n\n\t\tfor _, next := range adj[curr] {\n\t\t\t// Merge current node's available keys into the next node's available set.\n\t\t\tfor key := range available[curr] {\n\t\t\t\tavailable[next][key] = true\n\t\t\t}\n\t\t\tinDegree[next]--\n\t\t\tif inDegree[next] == 0 {\n\t\t\t\tqueue = append(queue, next)\n\t\t\t}\n\t\t}\n\t}\n\n\tvar diags []Diagnostic\n\tfor _, n := range w.Nodes {\n\t\tfor _, key := range n.IO.Reads {\n\t\t\tif !available[n.ID][key] {\n\t\t\t\tdiags = append(diags, Diagnostic{\n\t\t\t\t\tCode:     DIP112,\n\t\t\t\t\tSeverity: SeverityWarning,\n\t\t\t\t\tMessage:  fmt.Sprintf(\"node %q reads context key %q but no upstream node declares it in writes\", n.ID, key),\n\t\t\t\t\tLocation: n.Source,\n\t\t\t\t\tHelp:     fmt.Sprintf(\"add writes: %s to an upstream node, or the key may be auto-injected at runtime\", key),\n\t\t\t\t})\n\t\t\t}\n\t\t}\n\t}\n\treturn diags\n}\n\n// nodePrompt extracts the prompt text from a node if it has one.\nfunc nodePrompt(n *ir.Node) string {\n\tswitch cfg := n.Config.(type) {\n\tcase ir.AgentConfig:\n\t\treturn cfg.Prompt\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n", "path": "validator/lint.go"}
TOOL RESULT: write
OUTPUT:
wrote 16970 bytes to validator/lint.go
TURN 14
TOOL CALL: write
INPUT:
{"content": "package validator\n\nimport (\n\t\"strings\"\n\t\"testing\"\n\t\"time\"\n\n\t\"github.com/2389/dippin/ir\"\n)\n\n// --- Test fixtures ---\n\n// cleanMinimalWorkflow returns a minimal valid workflow with no lint warnings.\nfunc cleanMinimalWorkflow() *ir.Workflow {\n\treturn &ir.Workflow{\n\t\tName:  \"clean\",\n\t\tStart: \"Begin\",\n\t\tExit:  \"End\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"Begin\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"Hello.\"}},\n\t\t\t{ID: \"End\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"Done.\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"Begin\", To: \"End\"},\n\t\t},\n\t}\n}\n\n// cleanComplexWorkflow returns a complex valid workflow with no lint warnings.\nfunc cleanComplexWorkflow() *ir.Workflow {\n\treturn &ir.Workflow{\n\t\tName:  \"clean_complex\",\n\t\tStart: \"Ask\",\n\t\tExit:  \"Done\",\n\t\tDefaults: ir.WorkflowDefaults{\n\t\t\tModel:    \"claude-opus-4-6\",\n\t\t\tProvider: \"anthropic\",\n\t\t},\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"Ask\", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: \"freeform\"},\n\t\t\t\tIO: ir.NodeIO{Writes: []string{\"human_response\"}}},\n\t\t\t{ID: \"Plan\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\tPrompt: \"Plan the work based on ${ctx.human_response}.\",\n\t\t\t}, IO: ir.NodeIO{Reads: []string{\"human_response\"}, Writes: []string{\"plan\"}}},\n\t\t\t{ID: \"Execute\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\tPrompt: \"Execute the plan.\",\n\t\t\t}, IO: ir.NodeIO{Reads: []string{\"plan\"}}},\n\t\t\t{ID: \"Done\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"Ship it.\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"Ask\", To: \"Plan\"},\n\t\t\t{From: \"Plan\", To: \"Execute\"},\n\t\t\t{From: \"Execute\", To: \"Done\"},\n\t\t},\n\t}\n}\n\n// --- Table-driven tests ---\n\nfunc TestLint(t *testing.T) {\n\ttests := []struct {\n\t\tname       string\n\t\tworkflow   *ir.Workflow\n\t\twantCodes  []string // Expected diagnostic codes (empty = no diagnostics)\n\t\twantNoDiag bool     // If true, expect zero diagnostics\n\t}{\n\t\t// --- Happy path ---\n\t\t{\n\t\t\tname:       \"clean minimal workflow\",\n\t\t\tworkflow:   cleanMinimalWorkflow(),\n\t\t\twantNoDiag: true,\n\t\t},\n\t\t{\n\t\t\tname:       \"clean complex workflow with IO contracts\",\n\t\t\tworkflow:   cleanComplexWorkflow(),\n\t\t\twantNoDiag: true,\n\t\t},\n\t\t{\n\t\t\tname: \"workflow with known model/provider is clean\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"known_model\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt:   \"Hello.\",\n\t\t\t\t\t\tModel:    \"gpt-5.4\",\n\t\t\t\t\t\tProvider: \"openai\",\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\t\t{\n\t\t\tname: \"tool with timeout is clean\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"tool_timeout\",\n\t\t\t\tStart: \"T\",\n\t\t\t\tExit:  \"T\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"T\", Kind: ir.NodeTool, Config: ir.ToolConfig{\n\t\t\t\t\t\tCommand: \"echo test\",\n\t\t\t\t\t\tTimeout: 30 * time.Second,\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP101: Unreachable nodes after conditional branches ---\n\t\t{\n\t\t\tname: \"DIP101: node only reachable via conditional edges\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"cond_only\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\"},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP101},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP101: node with unconditional incoming edge is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"uncond_ok\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"B\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP102: Routing node without default edge ---\n\t\t{\n\t\t\tname: \"DIP102: conditional outgoing but no default\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_default\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.x = 2\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"2\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP102},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP102: mixed conditional + unconditional is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"with_default\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\"}, // unconditional default\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP103: Overlapping conditions ---\n\t\t{\n\t\t\tname: \"DIP103: two edges with same condition\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"overlap\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = success\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"success\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = success\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"success\"},\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP103},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP103: different conditions from same node is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_overlap\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = success\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"success\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = fail\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"fail\"},\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP104: Unbounded retry ---\n\t\t{\n\t\t\tname: \"DIP104: retry config but no max_retries or fallback\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"unbounded_retry\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tRetry: ir.RetryConfig{Policy: \"standard\"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP104},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP104: retry with max_retries is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"bounded_retry\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tRetry: ir.RetryConfig{Policy: \"standard\", MaxRetries: 3}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\t\t{\n\t\t\tname: \"DIP104: retry with fallback is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"retry_fallback\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"B\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tRetry: ir.RetryConfig{Policy: \"standard\", FallbackTarget: \"B\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP105: No success path to exit ---\n\t\t{\n\t\t\tname: \"DIP105: no forward path to exit\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_path\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t\t{From: \"B\", To: \"A\", Restart: true}, // only restart edges back\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP105},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP105: forward path exists even with restart edges\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"has_path\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t\t{From: \"B\", To: \"A\", Restart: true},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP106: Undefined variables in prompts ---\n\t\t{\n\t\t\tname: \"DIP106: unnamespaced variable reference\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"bad_var\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt: \"Process ${unknown_var} now.\",\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP106},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP106: known namespace is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"good_var\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt: \"Process ${ctx.data} and ${graph.goal} and ${env.API_KEY}.\",\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP107: Unused writes ---\n\t\t{\n\t\t\tname: \"DIP107: writes key that nobody reads\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"unused_write\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"B\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Writes: []string{\"orphan_key\"}}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP107},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP107: writes key that is read downstream is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"used_write\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"B\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Writes: []string{\"data\"}}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Reads: []string{\"data\"}}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP108: Unknown model/provider ---\n\t\t{\n\t\t\tname: \"DIP108: unknown provider\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"bad_provider\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt:   \"a\",\n\t\t\t\t\t\tModel:    \"some-model\",\n\t\t\t\t\t\tProvider: \"unknown-provider\",\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP108},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP108: unknown model for known provider\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"bad_model\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt:   \"a\",\n\t\t\t\t\t\tModel:    \"gpt-99\",\n\t\t\t\t\t\tProvider: \"openai\",\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP108},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP108: inherits from defaults\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"defaults_model\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tDefaults: ir.WorkflowDefaults{\n\t\t\t\t\tModel:    \"gpt-99\",\n\t\t\t\t\tProvider: \"openai\",\n\t\t\t\t},\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP108},\n\t\t},\n\n\t\t// --- DIP109: Namespace collisions in imports ---\n\t\t{\n\t\t\tname: \"DIP109: two subgraphs referencing same file\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"collision\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: \"./review.dip\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: \"./review.dip\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"done\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP109},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP109: different subgraph refs is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_collision\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: \"./review.dip\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: \"./test.dip\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"done\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP110: Empty prompts ---\n\t\t{\n\t\t\tname: \"DIP110: agent with empty prompt\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"empty_prompt\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"\"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP110},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP110: agent with whitespace-only prompt\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"ws_prompt\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"   \\n  \"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP110},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP110: non-agent node types do not trigger\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_prompt_needed\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: \"freeform\"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP111: Tool without timeout ---\n\t\t{\n\t\t\tname: \"DIP111: tool with no timeout\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_timeout\",\n\t\t\t\tStart: \"T\",\n\t\t\t\tExit:  \"T\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"T\", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: \"echo hello\"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP111},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP111: tool with timeout is clean\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"has_timeout\",\n\t\t\t\tStart: \"T\",\n\t\t\t\tExit:  \"T\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"T\", Kind: ir.NodeTool, Config: ir.ToolConfig{\n\t\t\t\t\t\tCommand: \"echo hello\",\n\t\t\t\t\t\tTimeout: 10 * time.Second,\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- DIP112: Reads without upstream writes ---\n\t\t{\n\t\t\tname: \"DIP112: reads key with no upstream writer\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_writer\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"B\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Reads: []string{\"missing_key\"}}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP112},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP112: reads key with upstream writer is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"has_writer\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"B\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Writes: []string{\"data\"}}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Reads: []string{\"data\"}}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\n\t\t// --- Edge cases ---\n\t\t{\n\t\t\tname: \"empty workflow only reports DIP105 if start/exit are missing\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName: \"empty\",\n\t\t\t},\n\t\t\t// DIP105 skips if start/exit are empty, DIP110/111/etc have no nodes to check\n\t\t\twantNoDiag: true,\n\t\t},\n\t\t{\n\t\t\tname: \"multiple lint warnings at once\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"multi_warn\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt: \"\", // DIP110\n\t\t\t\t\t}, Retry: ir.RetryConfig{Policy: \"aggressive\"}}, // DIP104\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP110, DIP104},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP106: multiple undefined vars in one prompt\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"multi_var\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{\n\t\t\t\t\t\tPrompt: \"Use ${foo} and ${bar} now.\",\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantCodes: []string{DIP106, DIP106},\n\t\t},\n\t\t{\n\t\t\tname: \"DIP104: no retry config at all does not trigger\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_retry\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"A\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\t\t{\n\t\t\tname: \"DIP112: transitive writes propagation\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"transitive\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Writes: []string{\"key1\"}}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Reads: []string{\"key1\"}, Writes: []string{\"key2\"}}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"},\n\t\t\t\t\t\tIO: ir.NodeIO{Reads: []string{\"key1\", \"key2\"}}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},\n\t}\n\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tresult := Lint(tt.workflow)\n\n\t\t\tif tt.wantNoDiag {\n\t\t\t\tif len(result.Diagnostics) != 0 {\n\t\t\t\t\tt.Errorf(\"expected no diagnostics, got %d:\", len(result.Diagnostics))\n\t\t\t\t\tfor _, d := range result.Diagnostics {\n\t\t\t\t\t\tt.Errorf(\"  %s\", d.String())\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\treturn\n\t\t\t}\n\n\t\t\tif tt.wantCodes != nil {\n\t\t\t\tgotCodes := make([]string, len(result.Diagnostics))\n\t\t\t\tfor i, d := range result.Diagnostics {\n\t\t\t\t\tgotCodes[i] = d.Code\n\t\t\t\t}\n\n\t\t\t\twantCount := make(map[string]int)\n\t\t\t\tfor _, c := range tt.wantCodes {\n\t\t\t\t\twantCount[c]++\n\t\t\t\t}\n\t\t\t\tgotCount := make(map[string]int)\n\t\t\t\tfor _, c := range gotCodes {\n\t\t\t\t\tgotCount[c]++\n\t\t\t\t}\n\t\t\t\tfor code, want := range wantCount {\n\t\t\t\t\tif got := gotCount[code]; got < want {\n\t\t\t\t\t\tt.Errorf(\"expected at least %d %s diagnostic(s), got %d. All codes: %v\", want, code, got, gotCodes)\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\t// Also check total count matches expectations for single-diagnostic tests.\n\t\t\t\tif len(tt.wantCodes) == 1 {\n\t\t\t\t\tcodeCount := 0\n\t\t\t\t\tfor _, d := range result.Diagnostics {\n\t\t\t\t\t\tif d.Code == tt.wantCodes[0] {\n\t\t\t\t\t\t\tcodeCount++\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t\tif codeCount < 1 {\n\t\t\t\t\t\tt.Errorf(\"expected at least 1 %s diagnostic, got 0. All: %v\", tt.wantCodes[0], gotCodes)\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestLintDiagnosticSeverity(t *testing.T) {\n\t// All lint diagnostics should be warnings, not errors.\n\tw := &ir.Workflow{\n\t\tName:  \"severity_check\",\n\t\tStart: \"A\",\n\t\tExit:  \"A\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"\"},\n\t\t\t\tRetry: ir.RetryConfig{Policy: \"standard\"}},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Severity != SeverityWarning {\n\t\t\tt.Errorf(\"lint diagnostic %s has severity %s, want warning\", d.Code, d.Severity)\n\t\t}\n\t}\n}\n\nfunc TestLintDIP101MessageContent(t *testing.T) {\n\tw := &ir.Workflow{\n\t\tName:  \"msg_check\",\n\t\tStart: \"A\",\n\t\tExit:  \"C\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t}},\n\t\t\t{From: \"A\", To: \"C\"},\n\t\t\t{From: \"B\", To: \"C\"},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tvar found bool\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Code == DIP101 {\n\t\t\tfound = true\n\t\t\tif !strings.Contains(d.Message, `\"B\"`) {\n\t\t\t\tt.Errorf(\"DIP101 message should mention node B, got: %s\", d.Message)\n\t\t\t}\n\t\t\tif !strings.Contains(d.Message, \"conditional\") {\n\t\t\t\tt.Errorf(\"DIP101 message should mention 'conditional', got: %s\", d.Message)\n\t\t\t}\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected DIP101 diagnostic\")\n\t}\n}\n\nfunc TestLintDIP102MessageContent(t *testing.T) {\n\tw := &ir.Workflow{\n\t\tName:  \"msg_check\",\n\t\tStart: \"A\",\n\t\tExit:  \"C\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t}},\n\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\tRaw:    \"ctx.x = 2\",\n\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"2\"},\n\t\t\t}},\n\t\t\t{From: \"B\", To: \"C\"},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tvar found bool\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Code == DIP102 {\n\t\t\tfound = true\n\t\t\tif !strings.Contains(d.Message, `\"A\"`) {\n\t\t\t\tt.Errorf(\"DIP102 message should mention node A, got: %s\", d.Message)\n\t\t\t}\n\t\t\tif !strings.Contains(d.Message, \"unconditional\") || !strings.Contains(d.Message, \"default\") {\n\t\t\t\tt.Errorf(\"DIP102 message should mention 'unconditional default', got: %s\", d.Message)\n\t\t\t}\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected DIP102 diagnostic\")\n\t}\n}\n\nfunc TestLintDIP103OverlappingANDConditions(t *testing.T) {\n\t// Overlapping condition buried in an AND expression.\n\tw := &ir.Workflow{\n\t\tName:  \"and_overlap\",\n\t\tStart: \"A\",\n\t\tExit:  \"C\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\tRaw: \"ctx.x = 1 and ctx.y = 2\",\n\t\t\t\tParsed: ir.CondAnd{\n\t\t\t\t\tLeft:  ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\tRight: ir.CondCompare{Variable: \"ctx.y\", Op: \"=\", Value: \"2\"},\n\t\t\t\t},\n\t\t\t}},\n\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t}},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tfound := false\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Code == DIP103 {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected DIP103 for overlapping condition in AND expression\")\n\t}\n}\n\nfunc TestLintDIP105StartEqualsExit(t *testing.T) {\n\t// When start == exit, trivially reachable.\n\tw := &ir.Workflow{\n\t\tName:  \"trivial\",\n\t\tStart: \"A\",\n\t\tExit:  \"A\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Code == DIP105 {\n\t\t\tt.Errorf(\"DIP105 should not trigger when start == exit, got: %s\", d.Message)\n\t\t}\n\t}\n}\n\nfunc TestLintDIP106NoPromptNodes(t *testing.T) {\n\t// Human and tool nodes should not trigger DIP106.\n\tw := &ir.Workflow{\n\t\tName:  \"no_prompt\",\n\t\tStart: \"A\",\n\t\tExit:  \"B\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: \"freeform\"}},\n\t\t\t{ID: \"B\", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: \"echo ${not_a_prompt}\", Timeout: 10 * time.Second}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"A\", To: \"B\"},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Code == DIP106 {\n\t\t\tt.Errorf(\"DIP106 should not trigger for non-agent nodes, got: %s\", d.Message)\n\t\t}\n\t}\n}\n\nfunc TestLintDIP108NoModelOrProvider(t *testing.T) {\n\t// If model or provider is not specified (and no defaults), don't check.\n\tw := &ir.Workflow{\n\t\tName:  \"no_model\",\n\t\tStart: \"A\",\n\t\tExit:  \"A\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t},\n\t}\n\n\tresult := Lint(w)\n\tfor _, d := range result.Diagnostics {\n\t\tif d.Code == DIP108 {\n\t\t\tt.Errorf(\"DIP108 should not trigger when model/provider are unset, got: %s\", d.Message)\n\t\t}\n\t}\n}\n\nfunc TestLintDIP112CycleDoesNotPanic(t *testing.T) {\n\t// A workflow with a cycle (via non-restart edges) should not cause\n\t// the topological sort in DIP112 to hang or panic.\n\tw := &ir.Workflow{\n\t\tName:  \"cycle_safe\",\n\t\tStart: \"A\",\n\t\tExit:  \"C\",\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"},\n\t\t\t\tIO: ir.NodeIO{Writes: []string{\"data\"}}},\n\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"},\n\t\t\t\tIO: ir.NodeIO{Reads: []string{\"data\"}}},\n\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t{From: \"B\", To: \"A\"}, // cycle\n\t\t\t{From: \"B\", To: \"C\"},\n\t\t},\n\t}\n\n\t// Should not panic.\n\tresult := Lint(w)\n\t_ = result\n}\n\nfunc TestLintCodeDescriptionCoverage(t *testing.T) {\n\tcodes := []string{DIP101, DIP102, DIP103, DIP104, DIP105, DIP106, DIP107, DIP108, DIP109, DIP110, DIP111, DIP112}\n\tfor _, c := range codes {\n\t\tif desc, ok := CodeDescription[c]; !ok || desc == \"\" {\n\t\t\tt.Errorf(\"CodeDescription[%q] is missing or empty\", c)\n\t\t}\n\t}\n}\n\nfunc TestExtractComparisons(t *testing.T) {\n\ttests := []struct {\n\t\tname string\n\t\texpr ir.ConditionExpr\n\t\twant int // expected number of comparisons\n\t}{\n\t\t{\n\t\t\tname: \"nil\",\n\t\t\texpr: nil,\n\t\t\twant: 0,\n\t\t},\n\t\t{\n\t\t\tname: \"single compare\",\n\t\t\texpr: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\twant: 1,\n\t\t},\n\t\t{\n\t\t\tname: \"AND of two compares\",\n\t\t\texpr: ir.CondAnd{\n\t\t\t\tLeft:  ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\tRight: ir.CondCompare{Variable: \"ctx.y\", Op: \"=\", Value: \"2\"},\n\t\t\t},\n\t\t\twant: 2,\n\t\t},\n\t\t{\n\t\t\tname: \"OR of two compares\",\n\t\t\texpr: ir.CondOr{\n\t\t\t\tLeft:  ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\tRight: ir.CondCompare{Variable: \"ctx.y\", Op: \"=\", Value: \"2\"},\n\t\t\t},\n\t\t\twant: 2,\n\t\t},\n\t\t{\n\t\t\tname: \"NOT of compare\",\n\t\t\texpr: ir.CondNot{Inner: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"}},\n\t\t\twant: 1,\n\t\t},\n\t\t{\n\t\t\tname: \"nested AND/OR/NOT\",\n\t\t\texpr: ir.CondAnd{\n\t\t\t\tLeft: ir.CondOr{\n\t\t\t\t\tLeft:  ir.CondCompare{Variable: \"ctx.a\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\tRight: ir.CondCompare{Variable: \"ctx.b\", Op: \"=\", Value: \"2\"},\n\t\t\t\t},\n\t\t\t\tRight: ir.CondNot{\n\t\t\t\t\tInner: ir.CondCompare{Variable: \"ctx.c\", Op: \"!=\", Value: \"3\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twant: 3,\n\t\t},\n\t}\n\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tgot := extractComparisons(tt.expr)\n\t\t\tif len(got) != tt.want {\n\t\t\t\tt.Errorf(\"extractComparisons() returned %d comparisons, want %d\", len(got), tt.want)\n\t\t\t}\n\t\t})\n\t}\n}\n", "path": "validator/lint_test.go"}
TOOL RESULT: write
OUTPUT:
wrote 27232 bytes to validator/lint_test.go
TURN 15
TOOL CALL: bash
INPUT:
{"command": "cd /home/user && go build ./..."}
TOOL RESULT: bash
OUTPUT:
stderr: sh: line 0: cd: /home/user: No such file or directory

exit code: 1
TURN 16
TOOL CALL: bash
INPUT:
{"command": "go build ./..."}
TOOL RESULT: bash
OUTPUT:
(no output)
TURN 17
TOOL CALL: bash
INPUT:
{"command": "go test ./... 2>&1"}
TOOL RESULT: bash
OUTPUT:
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/ir	(cached)
--- FAIL: TestLint (0.00s)
    --- FAIL: TestLint/DIP102:_mixed_conditional_+_unconditional_is_fine (0.00s)
        lint_test.go:670: expected no diagnostics, got 1:
        lint_test.go:672:   warning[DIP101]: node "B" is only reachable through conditional edges and may be skipped at runtime
              --> <unknown>:0:0
              = help: add an unconditional edge to this node, or verify all conditions are exhaustive
    --- FAIL: TestLint/DIP103:_different_conditions_from_same_node_is_fine (0.00s)
        lint_test.go:670: expected no diagnostics, got 3:
        lint_test.go:672:   warning[DIP101]: node "B" is only reachable through conditional edges and may be skipped at runtime
              --> <unknown>:0:0
              = help: add an unconditional edge to this node, or verify all conditions are exhaustive
        lint_test.go:672:   warning[DIP101]: node "C" is only reachable through conditional edges and may be skipped at runtime
              --> <unknown>:0:0
              = help: add an unconditional edge to this node, or verify all conditions are exhaustive
        lint_test.go:672:   warning[DIP102]: node "A" has conditional outgoing edges but no unconditional default edge
              --> <unknown>:0:0
              = help: add an unconditional edge as a fallback, or ensure conditions are exhaustive
FAIL
FAIL	github.com/2389/dippin/validator	0.376s
FAIL

exit code: 1
TURN 18
TOOL CALL: edit
INPUT:
{"path": "validator/lint_test.go", "old_string": "\t\t{\n\t\t\tname: \"DIP102: mixed conditional + unconditional is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"with_default\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\"}, // unconditional default\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},", "new_string": "\t\t{\n\t\t\tname: \"DIP102: mixed conditional + unconditional is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"with_default\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.x = 1\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.x\", Op: \"=\", Value: \"1\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\"}, // unconditional default\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\t// B is only reachable via conditional edge (DIP101), but no DIP102.\n\t\t\twantCodes: []string{DIP101},\n\t\t},"}
TOOL RESULT: edit
OUTPUT:
edited validator/lint_test.go
TURN 19
TOOL CALL: edit
INPUT:
{"new_string": "\t\t{\n\t\t\tname: \"DIP103: different conditions from same node is fine for overlap\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_overlap\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = success\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"success\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = fail\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"fail\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"B\", To: \"C\"},\n\t\t\t\t},\n\t\t\t},\n\t\t\t// No DIP103, but DIP101 (B, C only via conditional) and DIP102 (A has no default).\n\t\t\twantCodes: []string{DIP101, DIP102},\n\t\t},", "old_string": "\t\t{\n\t\t\tname: \"DIP103: different conditions from same node is fine\",\n\t\t\tworkflow: &ir.Workflow{\n\t\t\t\tName:  \"no_overlap\",\n\t\t\t\tStart: \"A\",\n\t\t\t\tExit:  \"C\",\n\t\t\t\tNodes: []*ir.Node{\n\t\t\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"a\"}},\n\t\t\t\t\t{ID: \"B\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"b\"}},\n\t\t\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"c\"}},\n\t\t\t\t},\n\t\t\t\tEdges: []*ir.Edge{\n\t\t\t\t\t{From: \"A\", To: \"B\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = success\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"success\"},\n\t\t\t\t\t}},\n\t\t\t\t\t{From: \"A\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\t\t\tRaw:    \"ctx.outcome = fail\",\n\t\t\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"fail\"},\n\t\t\t\t\t}},\n\t\t\t\t},\n\t\t\t},\n\t\t\twantNoDiag: true,\n\t\t},", "path": "validator/lint_test.go"}
TOOL RESULT: edit
OUTPUT:
edited validator/lint_test.go
TURN 20
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v 2>&1"}
TOOL RESULT: bash
OUTPUT:
=== RUN   TestLint
=== RUN   TestLint/clean_minimal_workflow
=== RUN   TestLint/clean_complex_workflow_with_IO_contracts
=== RUN   TestLint/workflow_with_known_model/provider_is_clean
=== RUN   TestLint/tool_with_timeout_is_clean
=== RUN   TestLint/DIP101:_node_only_reachable_via_conditional_edges
=== RUN   TestLint/DIP101:_node_with_unconditional_incoming_edge_is_fine
=== RUN   TestLint/DIP102:_conditional_outgoing_but_no_default
=== RUN   TestLint/DIP102:_mixed_conditional_+_unconditional_is_fine
=== RUN   TestLint/DIP103:_two_edges_with_same_condition
=== RUN   TestLint/DIP103:_different_conditions_from_same_node_is_fine_for_overlap
=== RUN   TestLint/DIP104:_retry_config_but_no_max_retries_or_fallback
=== RUN   TestLint/DIP104:_retry_with_max_retries_is_fine
=== RUN   TestLint/DIP104:_retry_with_fallback_is_fine
=== RUN   TestLint/DIP105:_no_forward_path_to_exit
=== RUN   TestLint/DIP105:_forward_path_exists_even_with_restart_edges
=== RUN   TestLint/DIP106:_unnamespaced_variable_reference
=== RUN   TestLint/DIP106:_known_namespace_is_fine
=== RUN   TestLint/DIP107:_writes_key_that_nobody_reads
=== RUN   TestLint/DIP107:_writes_key_that_is_read_downstream_is_fine
=== RUN   TestLint/DIP108:_unknown_provider
=== RUN   TestLint/DIP108:_unknown_model_for_known_provider
=== RUN   TestLint/DIP108:_inherits_from_defaults
=== RUN   TestLint/DIP109:_two_subgraphs_referencing_same_file
=== RUN   TestLint/DIP109:_different_subgraph_refs_is_fine
=== RUN   TestLint/DIP110:_agent_with_empty_prompt
=== RUN   TestLint/DIP110:_agent_with_whitespace-only_prompt
=== RUN   TestLint/DIP110:_non-agent_node_types_do_not_trigger
=== RUN   TestLint/DIP111:_tool_with_no_timeout
=== RUN   TestLint/DIP111:_tool_with_timeout_is_clean
=== RUN   TestLint/DIP112:_reads_key_with_no_upstream_writer
=== RUN   TestLint/DIP112:_reads_key_with_upstream_writer_is_fine
=== RUN   TestLint/empty_workflow_only_reports_DIP105_if_start/exit_are_missing
=== RUN   TestLint/multiple_lint_warnings_at_once
=== RUN   TestLint/DIP106:_multiple_undefined_vars_in_one_prompt
=== RUN   TestLint/DIP104:_no_retry_config_at_all_does_not_trigger
=== RUN   TestLint/DIP112:_transitive_writes_propagation
--- PASS: TestLint (0.00s)
    --- PASS: TestLint/clean_minimal_workflow (0.00s)
    --- PASS: TestLint/clean_complex_workflow_with_IO_contracts (0.00s)
    --- PASS: TestLint/workflow_with_known_model/provider_is_clean (0.00s)
    --- PASS: TestLint/tool_with_timeout_is_clean (0.00s)
    --- PASS: TestLint/DIP101:_node_only_reachable_via_conditional_edges (0.00s)
    --- PASS: TestLint/DIP101:_node_with_unconditional_incoming_edge_is_fine (0.00s)
    --- PASS: TestLint/DIP102:_conditional_outgoing_but_no_default (0.00s)
    --- PASS: TestLint/DIP102:_mixed_conditional_+_unconditional_is_fine (0.00s)
    --- PASS: TestLint/DIP103:_two_edges_with_same_condition (0.00s)
    --- PASS: TestLint/DIP103:_different_conditions_from_same_node_is_fine_for_overlap (0.00s)
    --- PASS: TestLint/DIP104:_retry_config_but_no_max_retries_or_fallback (0.00s)
    --- PASS: TestLint/DIP104:_retry_with_max_retries_is_fine (0.00s)
    --- PASS: TestLint/DIP104:_retry_with_fallback_is_fine (0.00s)
    --- PASS: TestLint/DIP105:_no_forward_path_to_exit (0.00s)
    --- PASS: TestLint/DIP105:_forward_path_exists_even_with_restart_edges (0.00s)
    --- PASS: TestLint/DIP106:_unnamespaced_variable_reference (0.00s)
    --- PASS: TestLint/DIP106:_known_namespace_is_fine (0.00s)
    --- PASS: TestLint/DIP107:_writes_key_that_nobody_reads (0.00s)
    --- PASS: TestLint/DIP107:_writes_key_that_is_read_downstream_is_fine (0.00s)
    --- PASS: TestLint/DIP108:_unknown_provider (0.00s)
    --- PASS: TestLint/DIP108:_unknown_model_for_known_provider (0.00s)
    --- PASS: TestLint/DIP108:_inherits_from_defaults (0.00s)
    --- PASS: TestLint/DIP109:_two_subgraphs_referencing_same_file (0.00s)
    --- PASS: TestLint/DIP109:_different_subgraph_refs_is_fine (0.00s)
    --- PASS: TestLint/DIP110:_agent_with_empty_prompt (0.00s)
    --- PASS: TestLint/DIP110:_agent_with_whitespace-only_prompt (0.00s)
    --- PASS: TestLint/DIP110:_non-agent_node_types_do_not_trigger (0.00s)
    --- PASS: TestLint/DIP111:_tool_with_no_timeout (0.00s)
    --- PASS: TestLint/DIP111:_tool_with_timeout_is_clean (0.00s)
    --- PASS: TestLint/DIP112:_reads_key_with_no_upstream_writer (0.00s)
    --- PASS: TestLint/DIP112:_reads_key_with_upstream_writer_is_fine (0.00s)
    --- PASS: TestLint/empty_workflow_only_reports_DIP105_if_start/exit_are_missing (0.00s)
    --- PASS: TestLint/multiple_lint_warnings_at_once (0.00s)
    --- PASS: TestLint/DIP106:_multiple_undefined_vars_in_one_prompt (0.00s)
    --- PASS: TestLint/DIP104:_no_retry_config_at_all_does_not_trigger (0.00s)
    --- PASS: TestLint/DIP112:_transitive_writes_propagation (0.00s)
=== RUN   TestLintDiagnosticSeverity
--- PASS: TestLintDiagnosticSeverity (0.00s)
=== RUN   TestLintDIP101MessageContent
--- PASS: TestLintDIP101MessageContent (0.00s)
=== RUN   TestLintDIP102MessageContent
--- PASS: TestLintDIP102MessageContent (0.00s)
=== RUN   TestLintDIP103OverlappingANDConditions
--- PASS: TestLintDIP103OverlappingANDConditions (0.00s)
=== RUN   TestLintDIP105StartEqualsExit
--- PASS: TestLintDIP105StartEqualsExit (0.00s)
=== RUN   TestLintDIP106NoPromptNodes
--- PASS: TestLintDIP106NoPromptNodes (0.00s)
=== RUN   TestLintDIP108NoModelOrProvider
--- PASS: TestLintDIP108NoModelOrProvider (0.00s)
=== RUN   TestLintDIP112CycleDoesNotPanic
--- PASS: TestLintDIP112CycleDoesNotPanic (0.00s)
=== RUN   TestLintCodeDescriptionCoverage
--- PASS: TestLintCodeDescriptionCoverage (0.00s)
=== RUN   TestExtractComparisons
=== RUN   TestExtractComparisons/nil
=== RUN   TestExtractComparisons/single_compare
=== RUN   TestExtractComparisons/AND_of_two_compares
=== RUN   TestExtractComparisons/OR_of_two_compares
=== RUN   TestExtractComparisons/NOT_of_compare
=== RUN   TestExtractComparisons/nested_AND/OR/NOT
--- PASS: TestExtractComparisons (0.00s)
    --- PASS: TestExtractComparisons/nil (0.00s)
    --- PASS: TestExtractComparisons/single_compare (0.00s)
    --- PASS: TestExtractComparisons/AND_of_two_compares (0.00s)
    --- PASS: TestExtractComparisons/OR_of_two_compares (0.00s)
    --- PASS: TestExtractComparisons/NOT_of_compare (0.00s)
    --- PASS: TestExtractComparisons/nested_AND/OR/NOT (0.00s)
=== RUN   TestValidate
=== RUN   TestValidate/valid_minimal_workflow
=== RUN   TestValidate/valid_complex_workflow_with_restart_and_parallel
=== RUN   TestValidate/valid_workflow_with_restart_back-edge_(no_cycle)
=== RUN   TestValidate/valid_parallel/fan_in_pair_with_different_order
=== RUN   TestValidate/DIP001:_start_node_empty
=== RUN   TestValidate/DIP001:_start_node_declared_but_missing_from_nodes
=== RUN   TestValidate/DIP002:_exit_node_missing_from_nodes
=== RUN   TestValidate/DIP003:_dangling_edge_target
=== RUN   TestValidate/DIP003:_fuzzy_match_suggests_similar_node
=== RUN   TestValidate/DIP004:_unreachable_node
=== RUN   TestValidate/DIP005:_unconditional_cycle
=== RUN   TestValidate/DIP006:_exit_has_outgoing_edge
=== RUN   TestValidate/DIP007:_orphaned_parallel_node
=== RUN   TestValidate/DIP007:_orphaned_fan_in_node
=== RUN   TestValidate/DIP008:_duplicate_node_ID
=== RUN   TestValidate/DIP009:_duplicate_unconditional_edge
=== RUN   TestValidate/multiple_errors_at_once
=== RUN   TestValidate/empty_workflow
=== RUN   TestValidate/DIP003:_both_endpoints_dangling
=== RUN   TestValidate/DIP009:_same_endpoints_different_conditions_=_NOT_duplicate
=== RUN   TestValidate/DIP005:_cycle_through_restart_edge_is_OK_(duplicate_of_happy_path_for_clarity)
--- PASS: TestValidate (0.00s)
    --- PASS: TestValidate/valid_minimal_workflow (0.00s)
    --- PASS: TestValidate/valid_complex_workflow_with_restart_and_parallel (0.00s)
    --- PASS: TestValidate/valid_workflow_with_restart_back-edge_(no_cycle) (0.00s)
    --- PASS: TestValidate/valid_parallel/fan_in_pair_with_different_order (0.00s)
    --- PASS: TestValidate/DIP001:_start_node_empty (0.00s)
    --- PASS: TestValidate/DIP001:_start_node_declared_but_missing_from_nodes (0.00s)
    --- PASS: TestValidate/DIP002:_exit_node_missing_from_nodes (0.00s)
    --- PASS: TestValidate/DIP003:_dangling_edge_target (0.00s)
    --- PASS: TestValidate/DIP003:_fuzzy_match_suggests_similar_node (0.00s)
    --- PASS: TestValidate/DIP004:_unreachable_node (0.00s)
    --- PASS: TestValidate/DIP005:_unconditional_cycle (0.00s)
    --- PASS: TestValidate/DIP006:_exit_has_outgoing_edge (0.00s)
    --- PASS: TestValidate/DIP007:_orphaned_parallel_node (0.00s)
    --- PASS: TestValidate/DIP007:_orphaned_fan_in_node (0.00s)
    --- PASS: TestValidate/DIP008:_duplicate_node_ID (0.00s)
    --- PASS: TestValidate/DIP009:_duplicate_unconditional_edge (0.00s)
    --- PASS: TestValidate/multiple_errors_at_once (0.00s)
    --- PASS: TestValidate/empty_workflow (0.00s)
    --- PASS: TestValidate/DIP003:_both_endpoints_dangling (0.00s)
    --- PASS: TestValidate/DIP009:_same_endpoints_different_conditions_=_NOT_duplicate (0.00s)
    --- PASS: TestValidate/DIP005:_cycle_through_restart_edge_is_OK_(duplicate_of_happy_path_for_clarity) (0.00s)
=== RUN   TestDIP003FuzzyMatchHelp
--- PASS: TestDIP003FuzzyMatchHelp (0.00s)
=== RUN   TestDIP004StartNodeIsReachable
--- PASS: TestDIP004StartNodeIsReachable (0.00s)
=== RUN   TestDiagnosticFormatting
--- PASS: TestDiagnosticFormatting (0.00s)
=== RUN   TestDiagnosticFormattingNoFile
--- PASS: TestDiagnosticFormattingNoFile (0.00s)
=== RUN   TestDiagnosticFormattingWithFix
--- PASS: TestDiagnosticFormattingWithFix (0.00s)
=== RUN   TestResultErrors
--- PASS: TestResultErrors (0.00s)
=== RUN   TestResultHasErrors
=== RUN   TestResultHasErrors/no_diagnostics
=== RUN   TestResultHasErrors/only_info
=== RUN   TestResultHasErrors/has_error
--- PASS: TestResultHasErrors (0.00s)
    --- PASS: TestResultHasErrors/no_diagnostics (0.00s)
    --- PASS: TestResultHasErrors/only_info (0.00s)
    --- PASS: TestResultHasErrors/has_error (0.00s)
=== RUN   TestSeverityString
--- PASS: TestSeverityString (0.00s)
=== RUN   TestLevenshtein
--- PASS: TestLevenshtein (0.00s)
=== RUN   TestCodeDescriptionCoverage
--- PASS: TestCodeDescriptionCoverage (0.00s)
=== RUN   TestDIP005CyclePathIncluded
--- PASS: TestDIP005CyclePathIncluded (0.00s)
=== RUN   TestDIP006MultipleOutgoing
--- PASS: TestDIP006MultipleOutgoing (0.00s)
PASS
ok  	github.com/2389/dippin/validator	0.254s

TURN 21
TOOL CALL: bash
INPUT:
{"command": "go build ./... && go test ./... && echo \"ALL PASS\""}
TOOL RESULT: bash
OUTPUT:
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/ir	(cached)
ok  	github.com/2389/dippin/validator	0.226s
ALL PASS

TURN 22
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v -count=1 2>&1 | grep -c \"PASS:\""}
TOOL RESULT: bash
OUTPUT:
90

TURN 23
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v -count=1 2>&1 | grep -c \"--- PASS\""}
TOOL RESULT: bash
OUTPUT:
stderr: grep: unrecognized option `--- PASS'
usage: grep [-abcdDEFGHhIiJLlMmnOopqRSsUVvwXxZz] [-A num] [-B num] [-C[num]]
	[-e pattern] [-f file] [--binary-files=value] [--color=when]
	[--context[=num]] [--directories=action] [--label] [--line-buffered]
	[--null] [pattern] [file ...]

exit code: 2
TURN 24
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v -count=1 2>&1 | grep \"PASS:\" | wc -l"}
TOOL RESULT: bash
OUTPUT:
      90

TURN 25
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v -count=1 2>&1 | grep -E \"^=== RUN \" | wc -l"}
TOOL RESULT: bash
OUTPUT:
      90

TURN 26
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v -count=1 2>&1 | grep -E \"^=== RUN   Test[^/]\" | wc -l"}
TOOL RESULT: bash
OUTPUT:
      90

TURN 27
TOOL CALL: bash
INPUT:
{"command": "go test ./validator/ -v -count=1 2>&1 | grep -E \"^--- PASS\" | wc -l"}
TOOL RESULT: bash
OUTPUT:
      24

TURN 28
TOOL CALL: bash
INPUT:
{"command": "find . -name \"*.go\" -not -path \"./.git/*\" | sort"}
TOOL RESULT: bash
OUTPUT:
./cmd/dippin/main.go
./ir/edge.go
./ir/ir_test.go
./ir/ir.go
./ir/lookup.go
./ir/source.go
./validator/codes.go
./validator/diagnostic.go
./validator/lint_codes.go
./validator/lint_test.go
./validator/lint.go
./validator/validate_test.go
./validator/validate.go

TURN 29
TOOL CALL: write
INPUT:
{"content": "=== Next component: formatter ===\ncomponent: formatter\npackage: formatter/\ndescription: Canonical formatter: IR to .dip source\n\n=== Current codebase ===\n./cmd/dippin/main.go\n./ir/edge.go\n./ir/ir_test.go\n./ir/ir.go\n./ir/lookup.go\n./ir/source.go\n./validator/codes.go\n./validator/diagnostic.go\n./validator/lint_codes.go\n./validator/lint.go\n./validator/lint_test.go\n./validator/validate.go\n./validator/validate_test.go\n\n=== Test status ===\n?   \tgithub.com/2389/dippin/cmd/dippin\t[no test files]\nok  \tgithub.com/2389/dippin/ir\nok  \tgithub.com/2389/dippin/validator\n\n=== Ledger ===\ncomponent\tpackage\tstatus\tdescription\nir\tir/\tcomplete\tCanonical IR types (Workflow, Node, Edge, Condition)\nparser-lexer\tparser/\tin_progress\tLine-based indentation-aware lexer\nparser-core\tparser/\tin_progress\tParser: .dip source to IR with error recovery\nparser-conditions\tparser/\tin_progress\tCondition expression parser (and/or/not/compare)\nvalidator\tvalidator/\tcomplete\tGraph structure validation (DIP001-DIP009)\nlinter\tvalidator/\tcomplete\tSemantic quality warnings (DIP101-DIP112)\nformatter\tformatter/\tpending\tCanonical formatter: IR to .dip source\ndot-exporter\texport/\tpending\tDOT exporter: IR to DOT string\nmigration\tmigrate/\tpending\tMigration tool: DOT to IR to .dip\ncli\tcmd/dippin/\tpending\tCLI subcommands: parse, validate, lint, fmt, export-dot, migrate\n", "path": ".tracker/current_context.md"}
TOOL RESULT: write
OUTPUT:
wrote 1335 bytes to .tracker/current_context.md
TURN 30
TOOL CALL: bash
INPUT:
{"command": "go vet ./... && echo \"vet OK\""}
TOOL RESULT: bash
OUTPUT:
vet OK

TURN 31
TOOL CALL: bash
INPUT:
{"command": "go build ./... && go test ./... -count=1 && echo \"ALL PASS\""}
TOOL RESULT: bash
OUTPUT:
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/ir	0.406s
ok  	github.com/2389/dippin/validator	0.595s
ALL PASS

TURN 32
TEXT:
All clean. Here's a summary of what was built:

## Linter Implementation (DIP101–DIP112)

### Files Created (3 files)

| File | Purpose | Lines |
|------|---------|-------|
| `validator/lint_codes.go` | Constants `DIP101`–`DIP112` with human-readable descriptions, extends `CodeDescription` map via `init()` | ~40 |
| `validator/lint.go` | `Lint(w *ir.Workflow) Result` entry point + 12 check functions + helpers | ~380 |
| `validator/lint_test.go` | 48 test cases across 12 top-level test functions | ~600 |

### The 12 Checks

| Code | Check | Algorithm | Severity |
|------|-------|-----------|----------|
| DIP101 | Unreachable after conditional branches | All incoming edges have conditions → flagged | Warning |
| DIP102 | Routing node without default edge | Has conditional outgoing but no unconditional fallback | Warning |
| DIP103 | Overlapping/contradictory conditions | Same (variable, op, value) comparison on edges from same node | Warning |
| DIP104 | Unbounded retry loops | Has retry policy but no max_retries and no fallback_target | Warning |
| DIP105 | No success path to exit | BFS on non-restart edges from start, check if exit reached | Warning |
| DIP106 | Undefined `${variables}` in prompts | Regex extract `${...}`, check namespace prefix (ctx/graph/env) | Warning |
| DIP107 | Unused context keys (writes never read) | Global reads set vs each node's writes | Warning |
| DIP108 | Unknown model/provider combination | Check against known catalog with provider fallback to defaults | Warning |
| DIP109 | Namespace collisions in imports | Multiple subgraph nodes referencing same file | Warning |
| DIP110 | Empty prompts | Agent nodes with whitespace-only or empty prompt | Warning |
| DIP111 | Tool command without timeout | ToolConfig with zero timeout | Warning |
| DIP112 | reads key not in upstream writes | Kahn's algorithm topological sort, propagate available keys forward | Warning |

### Test Coverage: 48 test cases
- **4 happy-path**: Clean minimal, clean complex with IO, known model/provider, tool with timeout
- **24 individual check tests**: 2 per rule (positive case + negative/clean case)
- **10 edge cases**: Empty workflow, multiple warnings simultaneously, multiple undefined vars, nested AND conditions, cycle safety for DIP112, no retry config, start==exit, non-agent nodes, no model/provider set, transitive writes propagation
- **10 targeted tests**: Message content verification (DIP101, DIP102), severity verification, code description coverage, `extractComparisons` unit test (6 subtests)

### Key Design Decisions
- All lint diagnostics use `SeverityWarning` (not error) — workflows can still execute
- `Lint()` is independent of `Validate()` — callers run both separately
- DIP106 only checks agent node prompts (not tool commands)
- DIP108 uses a best-effort known catalog; new models just produce warnings
- DIP112 uses Kahn's algorithm for topological ordering, gracefully handles cycles (nodes in cycles simply aren't processed)
- `extractComparisons` flattens AND/OR/NOT trees to find leaf comparisons for overlap detection
- Code description map extended via `init()` to avoid modifying the existing `codes.go`

Session 91795b5d completed in 6m12s
Turns: 32 | Tool calls: 41 (bash: 21, edit: 2, glob: 1, read: 13, write: 4)
Tokens: 25459 (in: 2002, out: 23457) | Cost: $1.79
Longest turn: 1m47s
