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
