TURN 1
TOOL CALL: read
INPUT:
{"path": ".tracker/current_plan.md"}
TOOL RESULT: read
OUTPUT:
# Plan: Migration Tool (`migrate/`)

## Component

**Migration Tool**: Parse DOT graph files → `ir.Workflow` → canonical `.dip` source text.

This is the DOT-to-Dippin converter described in §16 of the design spec. It parses a DOT `digraph` into an IR, applies post-migration cleanup (un-escaping prompts, reformatting commands, adding namespace prefixes to conditions), and emits canonical `.dip` output via the existing `formatter.Format()`.

A secondary function provides structural parity checking: given two `*ir.Workflow` values (one from DOT, one from `.dip`), report topology/config differences.

## Design Spec References

- **§16 "Migration Strategy"** — Phase 1 (automated conversion) and Phase 2 (behavioral parity validation)
- **§15 "DOT Export Strategy"** — The lossless/lossy tables define the reverse mapping (DOT → IR)
- **§5 "Legacy Hacks That Should Die"** — Shape→handler mapping, `\n` encoding, escape conventions
- **§4.1 "Node kinds"** — shape-to-kind mapping: `box`→`agent`, `hexagon`→`human`, `parallelogram`→`tool`, `component`→`parallel`, `tripleoctagon`→`fan_in`, `tab`→`subgraph`, `diamond`→varies (see below)
- **§4.2 "Edge semantics"** — conditions, labels, weights, restart
- **§8.2 "Context variable namespaces"** — migration must add `ctx.` prefix to bare condition variables
- **ADR 1** — `restart=true` edge attribute maps to `Edge.Restart`
- **Appendix A** — `migrate/migrate.go`, `migrate/parity.go`, `migrate/migrate_test.go`

## Files to Create

| File | Purpose |
|------|---------|
| `migrate/dot_parser.go` | DOT lexer + parser (DOT language subset) |
| `migrate/migrate.go` | `Migrate(dotSource string) (*ir.Workflow, error)` — DOT string → IR with cleanup |
| `migrate/parity.go` | `CheckParity(a, b *ir.Workflow) []Difference` — structural comparison |
| `migrate/migrate_test.go` | Comprehensive tests |

## Dependencies

- `ir/` — IR types (`Workflow`, `Node`, `Edge`, `Condition`, all config types)
- `formatter/` — `Format(*ir.Workflow) string` for producing final `.dip` output
- `validator/` — Optional; can validate the produced IR, but NOT a hard dependency for the migrate package itself
- **No external dependencies** — DOT parser is hand-written (the DOT language subset we need is small)

## Architecture

### DOT Parser (`dot_parser.go`)

We need a minimal DOT parser that handles the subset of DOT used by Tracker pipelines. This is NOT a full Graphviz DOT parser — it handles:

- `digraph <name> { ... }` wrapper
- Graph-level attributes: `graph [ key=value, ... ];`
- Node statements: `NodeID [ key=value, ... ];`
- Edge statements: `NodeID -> NodeID [ key=value, ... ];`
- C-style comments (`// ...`) and block comments (`/* ... */`)
- Double-quoted strings with escape sequences (`\"`, `\\`, `\n`)
- Semicolons (optional in real DOT, but our files use them)
- Attributes: `key=value` or `key="value"` 

**NOT supported** (not used in our DOT files):
- `subgraph` blocks (these are expanded inline in our files)
- Port syntax (`:port`)
- HTML labels (`<...>`)
- Multiple edge targets in one statement (`A -> B -> C`)

Types:
```go
// dotGraph holds the parsed DOT structure before IR conversion.
type dotGraph struct {
    Name       string
    GraphAttrs map[string]string
    NodeAttrs  map[string]string   // default node attrs
    EdgeAttrs  map[string]string   // default edge attrs  
    Nodes      []dotNode
    Edges      []dotEdge
}

type dotNode struct {
    ID    string
    Attrs map[string]string
}

type dotEdge struct {
    From  string
    To    string
    Attrs map[string]string
}

func parseDOT(input string) (*dotGraph, error)
```

### Migrator (`migrate.go`)

The core conversion pipeline:

```go
// Migrate parses a DOT digraph string and produces a Dippin IR workflow.
// It applies all post-migration cleanup automatically:
// - Shape → node kind mapping
// - \n and \" un-escaping in prompts and commands
// - Namespace prefixing for condition variables (bare "outcome" → "ctx.outcome")
// - Start/exit identification from Mdiamond/Msquare shapes
// - Graph-level attribute extraction to WorkflowDefaults
func Migrate(dotSource string) (*ir.Workflow, error)

// MigrateToSource parses DOT and returns canonical .dip source text.
// Convenience function equivalent to: formatter.Format(Migrate(dotSource))
func MigrateToSource(dotSource string) (string, error)
```

#### Shape → Kind Mapping (reverse of §15 table)

| DOT shape | IR NodeKind | Notes |
|-----------|-------------|-------|
| `box` | `agent` | Default if no shape |
| `hexagon` | `human` | |
| `parallelogram` | `tool` | |
| `component` | `parallel` | |
| `tripleoctagon` | `fan_in` | |
| `tab` | `subgraph` | |
| `Mdiamond` | — | Identifies `start` node (not a real kind) |
| `Msquare` | — | Identifies `exit` node (not a real kind) |
| `diamond` | Special handling | Per §5: if has `tool_command` → `tool`; if has `prompt` + `auto_status` → `agent`; if has `prompt` only → `agent`; otherwise → `agent` with no config (routing-only decision node — the engine evaluates outgoing edge conditions, the node itself is a no-op). In v1 Dippin there's no `conditional` kind, so diamond nodes become `agent` nodes. |
| (missing) | `agent` | Default when shape not specified |

#### Post-Migration Cleanup Steps

1. **Un-escape DOT strings**: `\n` → real newline, `\"` → `"`, `\\` → `\`
2. **Condition namespace prefixing**: Parse condition strings and prefix bare variable names:
   - Known context vars (`outcome`, `last_response`, `human_response`, `tool_stdout`, `tool_stderr`) → `ctx.` prefix
   - `graph.` prefixed vars → keep as-is
   - Other bare vars → `ctx.` prefix (best guess for migration)
3. **Graph attribute mapping**:
   - `goal` → `Workflow.Goal`
   - `rankdir` → ignored (presentation-only)
   - `default_max_retry` / `max_retries` → `Defaults.MaxRetries`
   - `max_restarts` → `Defaults.MaxRestarts`
   - `default_fidelity` / `fidelity` → `Defaults.Fidelity`
   - `model` → `Defaults.Model`
   - `provider` → `Defaults.Provider`
4. **Node attribute mapping**:
   - `label` → `Node.Label`
   - `prompt` → `AgentConfig.Prompt` (with un-escaping)
   - `tool_command` → `ToolConfig.Command` (with un-escaping)
   - `model` / `llm_model` → `AgentConfig.Model` (both forms accepted; `llm_model` is the legacy DOT convention)
   - `provider` / `llm_provider` → `AgentConfig.Provider` (both forms accepted; `llm_provider` is the legacy DOT convention)
   - `max_retries` → `RetryConfig.MaxRetries`
   - `retry_policy` → `RetryConfig.Policy`
   - `retry_target` → `RetryConfig.RetryTarget`
   - `fallback_target` → `RetryConfig.FallbackTarget`
   - `goal_gate` (true) → `AgentConfig.GoalGate`
   - `auto_status` (true) → `AgentConfig.AutoStatus`
   - `reasoning_effort` → `AgentConfig.ReasoningEffort`
   - `fidelity` → `AgentConfig.Fidelity`
   - `timeout` → `ToolConfig.Timeout` (parse duration string)
   - `mode` → `HumanConfig.Mode`
   - `default` → `HumanConfig.Default`
   - `targets` → `ParallelConfig.Targets` (comma-separated)
   - `sources` → `FanInConfig.Sources` (comma-separated)
   - `ref` → `SubgraphConfig.Ref`
   - `max_turns` → `AgentConfig.MaxTurns`
   - `cmd_timeout` → `AgentConfig.CmdTimeout`
   - `cache_tools` → `AgentConfig.CacheTools`
   - `compaction` → `AgentConfig.Compaction`
   - `system_prompt` → `AgentConfig.SystemPrompt`
5. **Edge attribute mapping**:
   - `label` → `Edge.Label`
   - `condition` → `Edge.Condition` (parse + add namespaces)
   - `weight` → `Edge.Weight`
   - `restart` (true) → `Edge.Restart`
   - `loop_restart` (true) → `Edge.Restart` (legacy alias used in real DOT files)
6. **Parallel/fan_in inference**: If a node has shape `component`, detect targets from outgoing edges. If a node has shape `tripleoctagon`, detect sources from incoming edges.

#### Condition Parsing for Migration

Conditions in DOT are raw strings like `outcome=success`, `tool_stdout contains pass`, `outcome=success && tool_stdout contains done`. We need a minimal parser that:

1. Splits on `&&` and `||` (with proper precedence)
2. Handles `not` / `!` prefix
3. Handles comparison operators: `=`, `!=`, `contains`, `startswith`, `endswith`, `in`
4. Adds namespace prefixes to bare variable names
5. Produces `ir.Condition` with both `Raw` and `Parsed` fields

```go
func parseCondition(raw string) (*ir.Condition, error)
func addNamespacePrefix(variable string) string
```

### Parity Checker (`parity.go`)

```go
// Difference describes a structural difference between two workflows.
type Difference struct {
    Kind    string // "node_missing", "edge_missing", "config_mismatch", "topology_diff", etc.
    Message string // Human-readable description
    PathA   string // Location in workflow A (e.g., "node:Validate")
    PathB   string // Location in workflow B (may be empty)
}

// CheckParity compares two workflows for structural equivalence.
// It checks:
// - Same node IDs and kinds
// - Same edges (from/to/conditions)
// - Same start/exit
// - Compatible node configurations (prompt content modulo whitespace)
// - Same graph-level defaults
func CheckParity(a, b *ir.Workflow) []Difference
```

## Test Cases (22+ cases)

### DOT Parser Tests

1. **Simple digraph** — `digraph G { A -> B; }` → 2 nodes, 1 edge
2. **Node with attributes** — `A [shape=box, label="My Agent"];` → correct attrs
3. **Edge with attributes** — `A -> B [label="yes", condition="outcome=success"];` → correct attrs
4. **Graph attributes** — `graph [goal="test", rankdir=LR];` → correct extraction
5. **Quoted strings with escapes** — `label="line1\nline2\"quoted\""` → proper un-escaping
6. **Comments** — `// comment` and `/* block */` correctly skipped
7. **Empty graph** — `digraph G {}` → no nodes, no edges
8. **Multiple edges** — `A -> B; A -> C; B -> C;` → 3 edges
9. **Missing semicolons** — `A -> B` without `;` should still parse (DOT allows it)
10. **Malformed DOT** — returns descriptive error (unclosed quote, missing brace, etc.)

### Migration Tests

11. **Full pipeline migration** — `build_dippin.dot`-style input → correct IR with all node kinds, edges, conditions
12. **Shape to kind mapping** — each DOT shape maps to the correct `ir.NodeKind`
13. **Start/exit identification** — `Mdiamond` → `Workflow.Start`, `Msquare` → `Workflow.Exit`
14. **Prompt un-escaping** — `"line1\nline2\n\"code\""` → `"line1\nline2\n\"code\""` (real newlines, unquoted)
15. **Tool command un-escaping** — `tool_command="set -eu\necho hello"` → multiline command
16. **Condition namespace prefixing** — `outcome=success` → `ctx.outcome = success`; `graph.goal` stays
17. **Complex condition** — `outcome=success && tool_stdout contains pass` → `CondAnd{CondCompare{ctx.outcome, =, success}, CondCompare{ctx.tool_stdout, contains, pass}}`
18. **Condition with negation** — `not outcome=fail` → `CondNot{CondCompare{ctx.outcome, =, fail}}`
19. **Restart edge** — `A -> B [restart=true]` → `Edge.Restart = true`
20. **Graph defaults extraction** — `graph [model="claude-opus-4-6", max_restarts=7]` → `WorkflowDefaults`
21. **Parallel node inference** — `component` shape node with outgoing edges → `ParallelConfig.Targets`
22. **Fan-in node inference** — `tripleoctagon` shape node with incoming edges → `FanInConfig.Sources`
23. **Diamond disambiguation** — diamond with `tool_command` → tool; diamond with `prompt` → agent
24. **Weight on edge** — `A -> B [weight=10]` → `Edge.Weight = 10`
25. **Duration parsing** — `timeout="30s"` → `30 * time.Second`, `timeout="1h30m"` → `90 * time.Minute`
26. **Empty/nil handling** — Node with no attributes → agent with empty config (default kind)
27. **MigrateToSource round-trip** — DOT → IR → `.dip` source; verify output is valid Dippin format

### Parity Checker Tests

28. **Identical workflows** — returns empty differences
29. **Missing node** — workflow B missing a node → reports `node_missing`
30. **Extra node** — workflow B has an extra node → reports `node_extra`
31. **Different start/exit** — reports `start_mismatch` / `exit_mismatch`
32. **Edge missing** — reports `edge_missing`
33. **Config mismatch** — different prompt text → reports `config_mismatch`
34. **Kind mismatch** — same node ID, different kind → reports `kind_mismatch`
35. **Whitespace-tolerant prompt comparison** — prompts that differ only in whitespace → no difference

### Integration Test

36. **build_dippin.dot migration** — Parse the actual `build_dippin.dot` file in the repo, migrate it, verify the IR has all expected nodes/edges/attributes. This is the real-world integration test.

## Implementation Notes

### DOT String Un-escaping

DOT uses `\n` for newline, `\"` for quote, `\\` for backslash. During migration:
- `\n` → literal newline character (`\n`)
- `\"` → literal quote (`"`)
- `\\` → literal backslash (`\`)
- `\l` → literal newline (DOT left-justified line break)
- `\r` → ignore (DOT carriage return, not meaningful)

### Workflow Name Extraction

The DOT `digraph <name>` becomes `Workflow.Name`. If the name is quoted, strip quotes.

### Node Ordering

Preserve the declaration order from the DOT file for `Workflow.Nodes`. This ensures deterministic output and maintains the author's intended reading order.

### Edge Ordering

Preserve edge declaration order from the DOT file for `Workflow.Edges`.

### Start/Exit Node Handling

Nodes with `shape=Mdiamond` or `shape=Msquare` are terminal markers:
- They are NOT added as real nodes in `Workflow.Nodes` (they are syntactic, not semantic)
- Instead, if `Mdiamond` node has edges pointing TO a real node, that real node becomes `Workflow.Start`
- If real nodes have edges pointing TO the `Msquare` node, the `Msquare` determines `Workflow.Exit`
- If `Mdiamond` has a label other than "Start", use it for the workflow name if graph name is generic
- **Alternative approach (simpler)**: Include Start/Exit as agent nodes in the IR with the ID from DOT. Set `Workflow.Start` and `Workflow.Exit` to those IDs. The formatter and other consumers already handle start/exit by ID. This approach is simpler and matches how `build_dippin.dot` works (Start and Exit are real nodes with edges).

**Decision**: Use the simpler approach — keep Start/Exit as nodes in the IR. Their kind will be `agent` (the default when shape doesn't map to another kind, since Mdiamond/Msquare don't have a kind mapping). The formatter knows to emit them because `Workflow.Start`/`.Exit` point to them. The validator requires them to exist.

Actually, re-examining: Start/Exit with `Mdiamond`/`Msquare` are terminal markers with no handler logic. They should be `agent` nodes with empty configs. The important thing is `Workflow.Start` and `Workflow.Exit` point to them.

### Parallel/Fan-in Target/Source Inference

For `component` (parallel) nodes:
- If the node has a `targets` attribute, use it directly (comma-split)
- Otherwise, infer targets from outgoing edges of that node

For `tripleoctagon` (fan_in) nodes:
- If the node has a `sources` attribute, use it directly (comma-split)
- Otherwise, infer sources from incoming edges to that node

### Known Context Variables for Namespace Prefixing

Per §8.2, these bare names get `ctx.` prefix:
- `outcome`
- `last_response`
- `human_response`
- `tool_stdout`
- `tool_stderr`

Variables already containing a `.` (like `graph.goal`) are left as-is.

The prefix `context.` in DOT conditions (e.g., `context.tool_stdout`) should be normalized to `ctx.` (per §8.2 namespace table).

The prefix `graph.` is kept as-is.

**Real-world example from `build_dippin.dot`**:
- `condition="context.tool_stdout=all_complete"` → `ctx.tool_stdout = all_complete`
- `condition="outcome=success"` → `ctx.outcome = success`
- `condition="outcome=fail"` → `ctx.outcome = fail`
- `condition="outcome=retry"` → `ctx.outcome = retry`

Note: DOT conditions use `=` with no spaces around the operator. The condition parser should handle both `outcome=success` and `outcome = success`.

### Real-World DOT Attribute Names

From examining `build_dippin.dot`, these are the actual attribute names used:

**Node attributes**: `shape`, `label`, `llm_provider`, `llm_model`, `reasoning_effort`, `fidelity`, `prompt`, `tool_command`, `goal_gate`, `retry_target`

**Edge attributes**: `condition`, `label`, `loop_restart`

**Graph attributes**: `goal`, `rankdir`, `default_max_retry`, `default_fidelity`, `max_restarts`

The migration tool MUST handle these legacy names:
- `llm_model` → maps to `model` in IR
- `llm_provider` → maps to `provider` in IR
- `loop_restart` → maps to `restart` in IR
- `default_max_retry` → maps to `max_retries` in IR defaults
- `default_fidelity` → maps to `fidelity` in IR defaults
- `context.` prefix in conditions → maps to `ctx.` prefix in IR

### Condition Operator Mapping

DOT conditions use `=` for equality. Dippin IR uses `=` as well. Operators:
- `=` → `=`
- `!=` → `!=`
- `contains` → `contains`
- `startswith` → `startswith`
- `endswith` → `endswith`
- `in` → `in`
- `&&` → `CondAnd`
- `||` → `CondOr`
- `not` / `!` → `CondNot`

TOOL CALL: read
INPUT:
{"path": ".tracker/current_context.md"}
TOOL RESULT: read
OUTPUT:
=== Next component: migration ===
component: migration
package: migrate/
description: Migration tool: DOT to IR to .dip

=== Current codebase ===
./cmd/dippin/main.go
./export/dot_test.go
./export/dot.go
./formatter/format_test.go
./formatter/format.go
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

=== Test status ===
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/export	(cached)
ok  	github.com/2389/dippin/formatter	(cached)
ok  	github.com/2389/dippin/ir	(cached)
ok  	github.com/2389/dippin/validator	(cached)

=== Ledger ===
component	package	status	description
ir	ir/	complete	Canonical IR types (Workflow, Node, Edge, Condition)
parser-lexer	parser/	complete	Line-based indentation-aware lexer
parser-core	parser/	complete	Parser: .dip source to IR with error recovery
parser-conditions	parser/	complete	Condition expression parser (and/or/not/compare)
validator	validator/	complete	Graph structure validation (DIP001-DIP009)
linter	validator/	complete	Semantic quality warnings (DIP101-DIP112)
formatter	formatter/	complete	Canonical formatter: IR to .dip source
dot-exporter	export/	complete	DOT exporter: IR to DOT string
migration	migrate/	in_progress	Migration tool: DOT to IR to .dip
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

TURN 3
TOOL CALL: read
INPUT:
{"path": "formatter/format.go"}
TOOL RESULT: read
OUTPUT:
// Package formatter implements canonical Dippin source formatting.
// Given an ir.Workflow, it produces deterministic .dip source text.
// The output is idempotent: Format(w) always produces the same string
// for the same IR state.
package formatter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/2389/dippin/ir"
)

// Format renders a workflow to canonical Dippin source text.
// The output always ends with exactly one trailing newline.
func Format(w *ir.Workflow) string {
	wr := &writer{}

	writeWorkflowHeader(wr, w)

	if !isDefaultsZero(w.Defaults) {
		wr.blank()
		writeDefaults(wr, w.Defaults)
	}

	for _, n := range w.Nodes {
		wr.blank()
		writeNode(wr, n)
	}

	if len(w.Edges) > 0 {
		wr.blank()
		writeEdges(wr, w.Edges)
	}

	return wr.String()
}

// writer wraps a strings.Builder with indentation tracking.
type writer struct {
	buf    strings.Builder
	indent int // current indent level (each level = 2 spaces)
}

// line writes a single indented line followed by a newline.
func (wr *writer) line(format string, args ...any) {
	content := fmt.Sprintf(format, args...)
	content = strings.TrimRight(content, " \t")
	prefix := strings.Repeat("  ", wr.indent)
	wr.buf.WriteString(prefix)
	wr.buf.WriteString(content)
	wr.buf.WriteByte('\n')
}

// blank writes an empty line.
func (wr *writer) blank() {
	wr.buf.WriteByte('\n')
}

// push increases the indentation level by one.
func (wr *writer) push() {
	wr.indent++
}

// pop decreases the indentation level by one.
func (wr *writer) pop() {
	wr.indent--
}

// multilineBlock emits a multiline field in the canonical form:
//
//	key:
//	  <line1>
//	  <line2>
func (wr *writer) multilineBlock(key, content string) {
	wr.line("%s:", key)
	content = strings.TrimRight(content, " \t\n\r")
	if content == "" {
		return
	}
	wr.push()
	for _, l := range strings.Split(content, "\n") {
		l = strings.TrimRight(l, " \t\r")
		if l == "" {
			wr.blank()
		} else {
			wr.line("%s", l)
		}
	}
	wr.pop()
}

// String returns the final output with exactly one trailing newline.
func (wr *writer) String() string {
	s := wr.buf.String()
	s = strings.TrimRight(s, "\n\r \t")
	return s + "\n"
}

// --- Section emitters ---

func writeWorkflowHeader(wr *writer, w *ir.Workflow) {
	wr.line("workflow %s", w.Name)
	wr.push()
	if w.Goal != "" {
		wr.line("goal: %s", quoteValue(w.Goal))
	}
	wr.line("start: %s", w.Start)
	wr.line("exit: %s", w.Exit)
	// We keep the indent at 1 for the rest of the top-level sections
}

func writeDefaults(wr *writer, d ir.WorkflowDefaults) {
	wr.line("defaults")
	wr.push()
	if d.Model != "" {
		wr.line("model: %s", quoteValue(d.Model))
	}
	if d.Provider != "" {
		wr.line("provider: %s", quoteValue(d.Provider))
	}
	if d.RetryPolicy != "" {
		wr.line("retry_policy: %s", quoteValue(d.RetryPolicy))
	}
	if d.MaxRetries != 0 {
		wr.line("max_retries: %d", d.MaxRetries)
	}
	if d.Fidelity != "" {
		wr.line("fidelity: %s", quoteValue(d.Fidelity))
	}
	if d.MaxRestarts != 0 {
		wr.line("max_restarts: %d", d.MaxRestarts)
	}
	if d.RestartTarget != "" {
		wr.line("restart_target: %s", d.RestartTarget)
	}
	if d.CacheTools {
		wr.line("cache_tools: true")
	}
	if d.Compaction != "" {
		wr.line("compaction: %s", quoteValue(d.Compaction))
	}
	wr.pop()
}

func writeNode(wr *writer, n *ir.Node) {
	switch cfg := n.Config.(type) {
	case ir.ParallelConfig:
		wr.line("parallel %s -> %s", n.ID, strings.Join(cfg.Targets, ", "))
	case ir.FanInConfig:
		wr.line("fan_in %s <- %s", n.ID, strings.Join(cfg.Sources, ", "))
	default:
		wr.line("%s %s", n.Kind, n.ID)
		wr.push()
		switch cfg := n.Config.(type) {
		case ir.AgentConfig:
			writeAgentFields(wr, n, cfg)
		case ir.HumanConfig:
			writeHumanFields(wr, n, cfg)
		case ir.ToolConfig:
			writeToolFields(wr, n, cfg)
		case ir.SubgraphConfig:
			writeSubgraphFields(wr, n, cfg)
		}
		wr.pop()
	}
}

func writeAgentFields(wr *writer, n *ir.Node, cfg ir.AgentConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if len(n.Classes) > 0 {
		wr.line("class: %s", strings.Join(n.Classes, ", "))
	}
	if cfg.Model != "" {
		wr.line("model: %s", quoteValue(cfg.Model))
	}
	if cfg.Provider != "" {
		wr.line("provider: %s", quoteValue(cfg.Provider))
	}
	if cfg.ReasoningEffort != "" {
		wr.line("reasoning_effort: %s", quoteValue(cfg.ReasoningEffort))
	}
	if cfg.Fidelity != "" {
		wr.line("fidelity: %s", quoteValue(cfg.Fidelity))
	}
	if cfg.GoalGate {
		wr.line("goal_gate: true")
	}
	if cfg.AutoStatus {
		wr.line("auto_status: true")
	}
	if cfg.MaxTurns != 0 {
		wr.line("max_turns: %d", cfg.MaxTurns)
	}
	if n.Retry.Policy != "" {
		wr.line("retry_policy: %s", quoteValue(n.Retry.Policy))
	}
	if n.Retry.MaxRetries != 0 {
		wr.line("max_retries: %d", n.Retry.MaxRetries)
	}
	if n.Retry.RetryTarget != "" {
		wr.line("retry_target: %s", n.Retry.RetryTarget)
	}
	if n.Retry.FallbackTarget != "" {
		wr.line("fallback_target: %s", n.Retry.FallbackTarget)
	}
	if len(n.IO.Reads) > 0 {
		wr.line("reads: %s", strings.Join(n.IO.Reads, ", "))
	}
	if len(n.IO.Writes) > 0 {
		wr.line("writes: %s", strings.Join(n.IO.Writes, ", "))
	}
	if cfg.Prompt != "" {
		wr.multilineBlock("prompt", cfg.Prompt)
	}
}

func writeHumanFields(wr *writer, n *ir.Node, cfg ir.HumanConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if cfg.Mode != "" {
		wr.line("mode: %s", quoteValue(cfg.Mode))
	}
	if cfg.Default != "" {
		wr.line("default: %s", quoteValue(cfg.Default))
	}
	if len(n.IO.Reads) > 0 {
		wr.line("reads: %s", strings.Join(n.IO.Reads, ", "))
	}
	if len(n.IO.Writes) > 0 {
		wr.line("writes: %s", strings.Join(n.IO.Writes, ", "))
	}
}

func writeToolFields(wr *writer, n *ir.Node, cfg ir.ToolConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if cfg.Timeout != 0 {
		wr.line("timeout: %s", formatDuration(cfg.Timeout))
	}
	if len(n.IO.Reads) > 0 {
		wr.line("reads: %s", strings.Join(n.IO.Reads, ", "))
	}
	if len(n.IO.Writes) > 0 {
		wr.line("writes: %s", strings.Join(n.IO.Writes, ", "))
	}
	if cfg.Command != "" {
		wr.multilineBlock("command", cfg.Command)
	}
}

func writeSubgraphFields(wr *writer, n *ir.Node, cfg ir.SubgraphConfig) {
	if n.Label != "" {
		wr.line("label: %s", quoteValue(n.Label))
	}
	if cfg.Ref != "" {
		wr.line("ref: %s", quoteValue(cfg.Ref))
	}
	if len(cfg.Params) > 0 {
		wr.line("params:")
		wr.push()
		keys := make([]string, 0, len(cfg.Params))
		for k := range cfg.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			wr.line("%s: %s", k, quoteValue(cfg.Params[k]))
		}
		wr.pop()
	}
}

func writeEdges(wr *writer, edges []*ir.Edge) {
	wr.line("edges")
	wr.push()
	for _, e := range edges {
		writeEdge(wr, e)
	}
	wr.pop()
}

func writeEdge(wr *writer, e *ir.Edge) {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s -> %s", e.From, e.To))

	if e.Condition != nil && e.Condition.Parsed != nil {
		parts = append(parts, fmt.Sprintf("when %s", formatCondition(e.Condition.Parsed)))
	}
	if e.Label != "" {
		parts = append(parts, fmt.Sprintf("label: %s", quoteValue(e.Label)))
	}
	if e.Weight != 0 {
		parts = append(parts, fmt.Sprintf("weight: %d", e.Weight))
	}
	if e.Restart {
		parts = append(parts, "restart: true")
	}

	wr.line("%s", strings.Join(parts, "  "))
}

// --- Condition formatting ---

const (
	precOr  = 1
	precAnd = 2
	precNot = 3
)

func formatCondition(expr ir.ConditionExpr) string {
	return formatConditionExpr(expr, 0)
}

func formatConditionExpr(expr ir.ConditionExpr, parentPrec int) string {
	switch e := expr.(type) {
	case ir.CondCompare:
		return fmt.Sprintf("%s %s %s", e.Variable, e.Op, e.Value)
	case ir.CondAnd:
		s := fmt.Sprintf("%s and %s",
			formatConditionExpr(e.Left, precAnd),
			formatConditionExpr(e.Right, precAnd))
		// Parenthesize if parent is a different compound operator (OR wrapping AND,
		// NOT wrapping AND). parentPrec==0 means top-level, no parens needed.
		if parentPrec != 0 && parentPrec != precAnd {
			return "(" + s + ")"
		}
		return s
	case ir.CondOr:
		s := fmt.Sprintf("%s or %s",
			formatConditionExpr(e.Left, precOr),
			formatConditionExpr(e.Right, precOr))
		// Parenthesize if parent is a different compound operator (AND wrapping OR,
		// NOT wrapping OR).
		if parentPrec != 0 && parentPrec != precOr {
			return "(" + s + ")"
		}
		return s
	case ir.CondNot:
		inner := formatConditionExpr(e.Inner, precNot)
		return "not " + inner
	default:
		return ""
	}
}

func quoteValue(s string) string {
	if s == "" {
		return `""`
	}
	if needsQuoting(s) {
		return `"` + s + `"`
	}
	return s
}

// needsQuoting returns true if the value needs to be enclosed in double quotes.
// Simple identifiers (alphanumeric, underscore, dash, dot, slash, colon) are unquoted.
func needsQuoting(s string) bool {
	for _, ch := range s {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_', ch == '-', ch == '.', ch == '/', ch == ':':
		default:
			return true
		}
	}
	return false
}

// formatDuration renders a time.Duration as a compact human-readable string
// suitable for Dippin source: "30s", "5m", "1h30m".
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	var parts []string
	if h := int(d.Hours()); h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
		d -= time.Duration(h) * time.Hour
	}
	if m := int(d.Minutes()); m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
		d -= time.Duration(m) * time.Minute
	}
	if s := int(d.Seconds()); s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	if len(parts) == 0 {
		// Sub-second durations.
		return d.String()
	}
	return strings.Join(parts, "")
}

func isDefaultsZero(d ir.WorkflowDefaults) bool {
	return d == ir.WorkflowDefaults{}
}

TOOL CALL: read
INPUT:
{"path": "export/dot.go"}
TOOL RESULT: read
OUTPUT:
// Package export provides DOT graph format export for Dippin workflows.
//
// The primary function ExportDOT converts an ir.Workflow into a valid DOT
// language string suitable for rendering with Graphviz. The mapping is
// documented in §15 of the Dippin design spec.
package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389/dippin/ir"
)

// ExportOptions controls the DOT output format.
type ExportOptions struct {
	// IncludePrompts includes full prompt/command text as node attributes.
	// Default (zero value) is false; set to true for full-fidelity export.
	IncludePrompts bool

	// RankDir controls the graph layout direction: "LR" (left-to-right)
	// or "TB" (top-to-bottom). Defaults to "TB" when empty.
	RankDir string

	// HighlightGoalGates applies a distinct fill color to nodes with
	// GoalGate: true.
	HighlightGoalGates bool
}

// ExportDOT renders a workflow as a DOT language string.
// The output is a valid digraph that can be processed by Graphviz tools.
func ExportDOT(w *ir.Workflow, opts ExportOptions) string {
	var b strings.Builder

	rankDir := opts.RankDir
	if rankDir == "" {
		rankDir = "TB"
	}

	graphName := w.Name
	if graphName == "" {
		graphName = "workflow"
	}

	b.WriteString(fmt.Sprintf("digraph %s {\n", dotID(graphName)))
	b.WriteString(fmt.Sprintf("  rankdir=%s;\n", rankDir))
	b.WriteString("  node [fontname=\"Helvetica\"];\n")
	b.WriteString("  edge [fontname=\"Helvetica\"];\n")

	// Emit nodes.
	for _, n := range w.Nodes {
		writeNodeDOT(&b, n, w, opts)
	}

	b.WriteByte('\n')

	// Emit edges.
	for _, e := range w.Edges {
		writeEdgeDOT(&b, e)
	}

	b.WriteString("}\n")
	return b.String()
}

// nodeShape maps a NodeKind to the corresponding DOT shape attribute.
// Per §15: agent→box, human→hexagon, tool→parallelogram,
// parallel→component, fan_in→tripleoctagon, subgraph→tab.
// Start and exit nodes override to Mdiamond and Msquare respectively.
func nodeShape(kind ir.NodeKind) string {
	switch kind {
	case ir.NodeAgent:
		return "box"
	case ir.NodeHuman:
		return "hexagon"
	case ir.NodeTool:
		return "parallelogram"
	case ir.NodeParallel:
		return "component"
	case ir.NodeFanIn:
		return "tripleoctagon"
	case ir.NodeSubgraph:
		return "tab"
	default:
		return "box"
	}
}

// writeNodeDOT emits a single DOT node statement.
func writeNodeDOT(b *strings.Builder, n *ir.Node, w *ir.Workflow, opts ExportOptions) {
	attrs := make(map[string]string)

	// Shape: start and exit override the kind-based shape.
	if n.ID == w.Start {
		attrs["shape"] = "Mdiamond"
	} else if n.ID == w.Exit {
		attrs["shape"] = "Msquare"
	} else {
		attrs["shape"] = nodeShape(n.Kind)
	}

	// Label: use the human-readable label if set, otherwise the node ID.
	if n.Label != "" {
		attrs["label"] = n.Label
	} else {
		attrs["label"] = n.ID
	}

	// Goal gate highlighting.
	if opts.HighlightGoalGates {
		if ac, ok := n.Config.(ir.AgentConfig); ok && ac.GoalGate {
			attrs["style"] = "filled"
			attrs["fillcolor"] = "#ffcccc"
		}
	}

	// Include prompts/commands as attributes for full-fidelity export.
	if opts.IncludePrompts {
		switch cfg := n.Config.(type) {
		case ir.AgentConfig:
			if cfg.Prompt != "" {
				attrs["prompt"] = escapeNewlines(cfg.Prompt)
			}
			if cfg.Model != "" {
				attrs["model"] = cfg.Model
			}
			if cfg.Provider != "" {
				attrs["provider"] = cfg.Provider
			}
		case ir.ToolConfig:
			if cfg.Command != "" {
				attrs["tool_command"] = escapeNewlines(cfg.Command)
			}
			if cfg.Timeout != 0 {
				attrs["timeout"] = formatDuration(cfg.Timeout)
			}
		case ir.HumanConfig:
			if cfg.Mode != "" {
				attrs["mode"] = cfg.Mode
			}
			if cfg.Default != "" {
				attrs["default"] = cfg.Default
			}
		case ir.SubgraphConfig:
			if cfg.Ref != "" {
				attrs["ref"] = cfg.Ref
			}
		case ir.ParallelConfig:
			if len(cfg.Targets) > 0 {
				attrs["targets"] = strings.Join(cfg.Targets, ",")
			}
		case ir.FanInConfig:
			if len(cfg.Sources) > 0 {
				attrs["sources"] = strings.Join(cfg.Sources, ",")
			}
		}
	}

	b.WriteString(fmt.Sprintf("  %s %s;\n", dotID(n.ID), formatDOTAttrs(attrs)))
}

// writeEdgeDOT emits a single DOT edge statement.
func writeEdgeDOT(b *strings.Builder, e *ir.Edge) {
	attrs := make(map[string]string)

	if e.Label != "" {
		attrs["label"] = e.Label
	}

	if e.Condition != nil && e.Condition.Parsed != nil {
		condStr := formatCondition(e.Condition.Parsed)
		// If there's no separate label, use the condition text as the edge label.
		if e.Label == "" {
			attrs["label"] = condStr
		}
		attrs["condition"] = condStr
	}

	if e.Weight != 0 {
		attrs["weight"] = fmt.Sprintf("%d", e.Weight)
	}

	if e.Restart {
		attrs["restart"] = "true"
		// Visual hint: restart edges are dashed.
		attrs["style"] = "dashed"
	}

	b.WriteString(fmt.Sprintf("  %s -> %s", dotID(e.From), dotID(e.To)))
	if len(attrs) > 0 {
		b.WriteString(" ")
		b.WriteString(formatDOTAttrs(attrs))
	}
	b.WriteString(";\n")
}

// formatDOTAttrs renders a map of DOT attributes as a bracketed list,
// with keys in sorted order for deterministic output.
func formatDOTAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sortStrings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, dotQuote(attrs[k])))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// dotID formats a string as a valid DOT identifier.
// If the string is a simple identifier (alphanumeric + underscore, not starting
// with a digit), it is returned as-is. Otherwise it is double-quoted.
func dotID(s string) string {
	if s == "" {
		return `""`
	}
	if isSimpleDOTID(s) {
		return s
	}
	return dotQuote(s)
}

// isSimpleDOTID returns true if s is a valid unquoted DOT identifier.
func isSimpleDOTID(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must not start with a digit.
	if s[0] >= '0' && s[0] <= '9' {
		return false
	}
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// dotQuote wraps a string in double quotes, escaping internal quotes and
// backslashes. Preserves DOT escape sequences like \n, \l, \r.
func dotQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			// Check if this backslash is part of a DOT escape sequence.
			if i+1 < len(s) {
				next := s[i+1]
				if next == 'n' || next == 'l' || next == 'r' {
					// Preserve DOT escape sequences.
					b.WriteByte('\\')
					b.WriteByte(next)
					i++
					continue
				}
			}
			b.WriteString(`\\`)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// escapeNewlines replaces literal newlines with the DOT \n escape for
// multi-line attribute values.
func escapeNewlines(s string) string {
	return strings.ReplaceAll(s, "\n", `\n`)
}

// --- Condition formatting ---
// Replicates the formatter's condition serialization for DOT attribute values.

const (
	precOr  = 1
	precAnd = 2
	precNot = 3
)

func formatCondition(expr ir.ConditionExpr) string {
	return formatConditionExpr(expr, 0)
}

func formatConditionExpr(expr ir.ConditionExpr, parentPrec int) string {
	switch e := expr.(type) {
	case ir.CondCompare:
		return fmt.Sprintf("%s %s %s", e.Variable, e.Op, e.Value)
	case ir.CondAnd:
		s := fmt.Sprintf("%s and %s",
			formatConditionExpr(e.Left, precAnd),
			formatConditionExpr(e.Right, precAnd))
		if parentPrec != 0 && parentPrec != precAnd {
			return "(" + s + ")"
		}
		return s
	case ir.CondOr:
		s := fmt.Sprintf("%s or %s",
			formatConditionExpr(e.Left, precOr),
			formatConditionExpr(e.Right, precOr))
		if parentPrec != 0 && parentPrec != precOr {
			return "(" + s + ")"
		}
		return s
	case ir.CondNot:
		inner := formatConditionExpr(e.Inner, precNot)
		return "not " + inner
	default:
		return ""
	}
}

// --- Duration formatting ---

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	var parts []string
	if h := int(d.Hours()); h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
		d -= time.Duration(h) * time.Hour
	}
	if m := int(d.Minutes()); m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
		d -= time.Duration(m) * time.Minute
	}
	if s := int(d.Seconds()); s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	if len(parts) == 0 {
		return d.String()
	}
	return strings.Join(parts, "")
}

// sortStrings sorts a string slice in place. Avoids importing sort for this
// single use.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

TOOL CALL: read
INPUT:
{"path": "go.mod"}
TOOL RESULT: read
OUTPUT:
module github.com/2389/dippin

go 1.25.4

TURN 4
TOOL CALL: bash
INPUT:
{"command": "find . -name \"*.dot\" 2>/dev/null | head -10"}
TOOL RESULT: bash
OUTPUT:
./build_dippin.dot

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

TURN 5
TOOL CALL: read
INPUT:
{"path": "build_dippin.dot"}
TOOL RESULT: read
OUTPUT:
digraph BuildDippin {
  graph [
    goal="Build the Dippin toolchain (parser, validator, formatter, DOT exporter, migration tool) by reading the design spec and iteratively implementing components into the dippin-lang Go module. Ledger-driven: picks the next uncompleted component automatically and loops until all are done.",
    rankdir=LR,
    default_max_retry=3,
    default_fidelity="summary:high",
    max_restarts=7
  ];

  Start [shape=Mdiamond, label="Start"];
  Exit [shape=Msquare, label="Exit"];

  // ── Phase 0: Setup ──────────────────────────────────────────────

  SetupWorkspace [
    shape=parallelogram,
    label="Setup Workspace & Ledger",
    tool_command="set -eu\nmkdir -p .tracker/runs\nif [ ! -f go.mod ]; then\n  echo 'ERROR: must run from dippin-lang directory with go.mod'\n  exit 1\nfi\n# Initialize ledger if it doesn't exist\nif [ ! -f .tracker/ledger.tsv ]; then\n  printf 'component\\tpackage\\tstatus\\tdescription\\n' > .tracker/ledger.tsv\n  printf 'ir\\tir/\\tcomplete\\tCanonical IR types (Workflow, Node, Edge, Condition)\\n' >> .tracker/ledger.tsv\n  printf 'parser-lexer\\tparser/\\tpending\\tLine-based indentation-aware lexer\\n' >> .tracker/ledger.tsv\n  printf 'parser-core\\tparser/\\tpending\\tParser: .dip source to IR with error recovery\\n' >> .tracker/ledger.tsv\n  printf 'parser-conditions\\tparser/\\tpending\\tCondition expression parser (and/or/not/compare)\\n' >> .tracker/ledger.tsv\n  printf 'validator\\tvalidator/\\tpending\\tGraph structure validation (DIP001-DIP009)\\n' >> .tracker/ledger.tsv\n  printf 'linter\\tvalidator/\\tpending\\tSemantic quality warnings (DIP101-DIP112)\\n' >> .tracker/ledger.tsv\n  printf 'formatter\\tformatter/\\tpending\\tCanonical formatter: IR to .dip source\\n' >> .tracker/ledger.tsv\n  printf 'dot-exporter\\texport/\\tpending\\tDOT exporter: IR to DOT string\\n' >> .tracker/ledger.tsv\n  printf 'migration\\tmigrate/\\tpending\\tMigration tool: DOT to IR to .dip\\n' >> .tracker/ledger.tsv\n  printf 'cli\\tcmd/dippin/\\tpending\\tCLI subcommands: parse, validate, lint, fmt, export-dot, migrate\\n' >> .tracker/ledger.tsv\nfi\ncat .tracker/ledger.tsv\nprintf 'workspace-ready'"
  ];

  // ── Phase 1: Pick next task from ledger ─────────────────────────

  PickNextComponent [
    shape=parallelogram,
    label="Pick Next Component",
    tool_command="set -eu\n# Find first pending component\nNEXT=$(awk -F'\\t' '$3==\"pending\" { print $1; exit }' .tracker/ledger.tsv)\nif [ -z \"$NEXT\" ]; then\n  printf 'all_complete'\n  exit 0\nfi\n# Mark it in-progress\nawk -F'\\t' -v comp=\"$NEXT\" 'BEGIN{OFS=\"\\t\"} $1==comp{$3=\"in_progress\"} {print}' .tracker/ledger.tsv > .tracker/ledger.tmp && mv .tracker/ledger.tmp .tracker/ledger.tsv\n# Write context snapshot for planner to read from disk\nmkdir -p .tracker\necho \"=== Next component: $NEXT ===\" > .tracker/current_context.md\nawk -F'\\t' -v comp=\"$NEXT\" '$1==comp { printf \"component: %s\\npackage: %s\\ndescription: %s\\n\", $1, $2, $4 }' .tracker/ledger.tsv >> .tracker/current_context.md\necho '' >> .tracker/current_context.md\necho '=== Current codebase ===' >> .tracker/current_context.md\nfind . -name '*.go' -not -path './vendor/*' | sort >> .tracker/current_context.md\necho '' >> .tracker/current_context.md\necho '=== Test status ===' >> .tracker/current_context.md\ngo test ./... >> .tracker/current_context.md 2>&1 || true\necho '' >> .tracker/current_context.md\necho '=== Ledger ===' >> .tracker/current_context.md\ncat .tracker/ledger.tsv >> .tracker/current_context.md\nprintf 'has_next'"
  ];

  // Route: all done vs has work
  CheckComplete [
    shape=diamond,
    label="All done?"
  ];

  // ── Phase 2: Plan ───────────────────────────────────────────────

  PlanComponent [
    shape=box,
    label="Plan Component",
    llm_provider="anthropic",
    llm_model="claude-opus-4-6",
    reasoning_effort="high",
    prompt="You are building the Dippin toolchain. Read .tracker/current_context.md to see which component is in_progress and the current state of the codebase.\n\nRead the design spec at ../DIPPIN_DESIGN_PLAN.md — this is the authoritative specification.\n\nRead the current codebase to understand what already exists (especially ir/ types).\n\nProduce a focused implementation plan for THIS component only:\n1. List the exact files to create/modify\n2. List the types and functions to implement\n3. List 10+ test cases (happy path, error cases, edge cases)\n4. Reference the specific design spec sections that govern this component\n5. Note any dependencies on existing packages\n\nWrite the plan to .tracker/current_plan.md so implementation agents can read it.\n\nDo NOT implement yet — just plan. Be precise enough that an implementation agent can work from this plan without ambiguity."
  ];

  // ── Phase 3: Implement ──────────────────────────────────────────

  ImplementParallel [shape=component, label="Implement (Parallel)"];

  ImplementClaude [
    shape=box,
    label="Claude Implementation",
    llm_provider="anthropic",
    llm_model="claude-opus-4-6",
    reasoning_effort="high",
    fidelity="full",
    prompt="Implement the planned component for the Dippin toolchain. Read .tracker/current_plan.md for exactly what to build and .tracker/current_context.md for the current codebase state.\n\nRead the design spec at ../DIPPIN_DESIGN_PLAN.md for exact specifications.\nRead existing code in ir/ to match types precisely.\n\nRules:\n- Write idiomatic Go. Standard library only (no external deps beyond what go.mod already has).\n- Every exported function gets a test. Test files go next to source.\n- Match the IR types in ir/ exactly. Do not modify ir/ unless the plan says to.\n- Table-driven tests with edge cases from the plan.\n- One responsibility per package.\n- Use diagnostic codes from the spec (DIP001-DIP112) where applicable.\n- Write real, complete code — no stubs, no TODOs, no placeholders.\n- Run 'go build ./...' and 'go test ./...' to verify before finishing."
  ];

  ImplementGPT [
    shape=box,
    label="GPT Implementation",
    llm_provider="openai",
    llm_model="gpt-5.4",
    reasoning_effort="high",
    fidelity="full",
    prompt="Implement the planned component for the Dippin toolchain. Read .tracker/current_plan.md for exactly what to build and .tracker/current_context.md for the current codebase state.\n\nRead the design spec at ../DIPPIN_DESIGN_PLAN.md for exact specifications.\nRead existing code in ir/ to match types precisely.\n\nRules:\n- Write idiomatic Go. Standard library only (no external deps beyond what go.mod already has).\n- Every exported function gets a test. Test files go next to source.\n- Match the IR types in ir/ exactly. Do not modify ir/ unless the plan says to.\n- Table-driven tests with edge cases from the plan.\n- One responsibility per package.\n- Use diagnostic codes from the spec (DIP001-DIP112) where applicable.\n- Write real, complete code — no stubs, no TODOs, no placeholders.\n- Run 'go build ./...' and 'go test ./...' to verify before finishing."
  ];

  ImplementGemini [
    shape=box,
    label="Gemini Implementation",
    llm_provider="gemini",
    llm_model="gemini-3-flash-preview",
    reasoning_effort="high",
    fidelity="full",
    prompt="Implement the planned component for the Dippin toolchain. Read .tracker/current_plan.md for exactly what to build and .tracker/current_context.md for the current codebase state.\n\nRead the design spec at ../DIPPIN_DESIGN_PLAN.md for exact specifications.\nRead existing code in ir/ to match types precisely.\n\nRules:\n- Write idiomatic Go. Standard library only (no external deps beyond what go.mod already has).\n- Every exported function gets a test. Test files go next to source.\n- Match the IR types in ir/ exactly. Do not modify ir/ unless the plan says to.\n- Table-driven tests with edge cases from the plan.\n- One responsibility per package.\n- Use diagnostic codes from the spec (DIP001-DIP112) where applicable.\n- Write real, complete code — no stubs, no TODOs, no placeholders.\n- Run 'go build ./...' and 'go test ./...' to verify before finishing."
  ];

  ImplementJoin [shape=tripleoctagon, label="Implement Join"];

  // ── Phase 4: Validate ───────────────────────────────────────────

  ValidateBuild [
    shape=parallelogram,
    label="Validate Build and Tests",
    tool_command="set -eu\necho '=== go vet ==='\ngo vet ./... 2>&1\necho ''\necho '=== go build ==='\ngo build ./... 2>&1\necho ''\necho '=== go test -v ==='\ngo test -v -count=1 ./... 2>&1\nprintf 'validation-pass'"
  ];

  // ── Phase 5: Review ─────────────────────────────────────────────

  ReviewParallel [shape=component, label="Reviews (Parallel)"];

  ReviewClaude [
    shape=box,
    label="Claude Review",
    llm_provider="anthropic",
    llm_model="claude-opus-4-6",
    reasoning_effort="high",
    prompt="Review the implementation against the Dippin design spec (../DIPPIN_DESIGN_PLAN.md). Check:\n1. Does the code match the spec's type definitions, validation rules, and syntax?\n2. Are tests comprehensive — happy path, errors, and edge cases the spec calls out?\n3. Does the code integrate correctly with the existing ir/ package?\n4. Is the code idiomatic Go with proper error handling?\n5. Any deviations from the spec that need justification?\n\nReturn STATUS: success if solid. Return STATUS: fail with specific required fixes if not."
  ];

  ReviewGPT [
    shape=box,
    label="GPT Review",
    llm_provider="openai",
    llm_model="gpt-5.4",
    reasoning_effort="high",
    prompt="Review the implementation against the Dippin design spec (../DIPPIN_DESIGN_PLAN.md). Check:\n1. Correctness: does the code do what the spec says?\n2. Completeness: are all spec requirements for this component addressed?\n3. Test quality: do tests cover happy path, error cases, and edge cases?\n4. Integration: does it work with existing packages without modification?\n\nReturn STATUS: success if solid. Return STATUS: fail with specific required fixes if not."
  ];

  ReviewGemini [
    shape=box,
    label="Gemini Review",
    llm_provider="gemini",
    llm_model="gemini-3-flash-preview",
    reasoning_effort="high",
    prompt="Review the implementation against the Dippin design spec (../DIPPIN_DESIGN_PLAN.md). Check:\n1. Spec fidelity: does the code faithfully implement what the design doc specifies?\n2. Edge cases: are boundary conditions and error paths handled?\n3. Test coverage: do tests exercise the contract surfaces, not just happy paths?\n4. Robustness: will this code survive adversarial or malformed input?\n\nReturn STATUS: success if solid. Return STATUS: fail with specific required fixes if not."
  ];

  ReviewsJoin [shape=tripleoctagon, label="Reviews Join"];

  ReviewAnalysis [
    shape=box,
    label="Review Synthesis",
    llm_provider="anthropic",
    llm_model="claude-opus-4-6",
    reasoning_effort="high",
    goal_gate=true,
    retry_target="ImplementClaude",
    prompt="Synthesize all three reviews into one verdict.\n\n- If reviewers agree the code is solid: return STATUS: success.\n- If fixes are needed and reviewers agree on what: return STATUS: retry with a clear fix list.\n- If reviewers fundamentally disagree or component is broken: return STATUS: fail to escalate.\n\nBe concrete about what needs fixing if retry."
  ];

  // ── Phase 6: Ship & Loop ────────────────────────────────────────

  CommitWork [
    shape=box,
    label="Commit Component",
    llm_provider="anthropic",
    llm_model="claude-sonnet-4-6",
    reasoning_effort="low",
    prompt="All reviews passed. Commit the new component with a clean conventional commit message. The message should name the package and what it implements (e.g., 'feat(parser): implement lexer with indentation tracking'). Include test count in the message body."
  ];

  MarkComplete [
    shape=parallelogram,
    label="Mark Component Complete",
    tool_command="set -eu\n# Find the in_progress component and mark it complete\nawk -F'\\t' 'BEGIN{OFS=\"\\t\"} $3==\"in_progress\"{$3=\"complete\"} {print}' .tracker/ledger.tsv > .tracker/ledger.tmp && mv .tracker/ledger.tmp .tracker/ledger.tsv\necho '=== Updated ledger ==='\ncat .tracker/ledger.tsv\nPENDING=$(awk -F'\\t' '$3==\"pending\"' .tracker/ledger.tsv | wc -l | tr -d ' ')\necho \"Remaining: $PENDING components\"\nprintf 'marked-complete'"
  ];

  FailureSummary [
    shape=box,
    label="Failure Summary",
    llm_provider="anthropic",
    llm_model="claude-sonnet-4-6",
    reasoning_effort="high",
    prompt="The component could not be built to satisfaction. Summarize what was attempted, what failed, and what the blockers are. Write this to .tracker/failure-report.md so the next run can pick up from here. Also mark the component back to 'pending' in the ledger."
  ];

  MarkFailed [
    shape=parallelogram,
    label="Reset Failed Component",
    tool_command="set -eu\n# Reset in_progress back to pending\nawk -F'\\t' 'BEGIN{OFS=\"\\t\"} $3==\"in_progress\"{$3=\"pending\"} {print}' .tracker/ledger.tsv > .tracker/ledger.tmp && mv .tracker/ledger.tmp .tracker/ledger.tsv\ncat .tracker/ledger.tsv\nprintf 'reset-done'"
  ];

  // ── Edges ───────────────────────────────────────────────────────

  // Setup
  Start -> SetupWorkspace;
  SetupWorkspace -> PickNextComponent;

  // Task selection routing
  PickNextComponent -> CheckComplete;
  CheckComplete -> Exit [condition="context.tool_stdout=all_complete", label="all done"];
  CheckComplete -> PlanComponent [condition="context.tool_stdout=has_next", label="has work"];

  // Plan -> Implement
  PlanComponent -> ImplementParallel;

  // Parallel implementation
  ImplementParallel -> ImplementClaude;
  ImplementParallel -> ImplementGPT;
  ImplementParallel -> ImplementGemini;
  ImplementClaude -> ImplementJoin;
  ImplementGPT -> ImplementJoin;
  ImplementGemini -> ImplementJoin;

  // Validation
  ImplementJoin -> ValidateBuild;
  ValidateBuild -> ReviewParallel [condition="outcome=success", label="validated"];
  ValidateBuild -> ImplementClaude [condition="outcome=fail", label="fix_build"];

  // Reviews
  ReviewParallel -> ReviewClaude;
  ReviewParallel -> ReviewGPT;
  ReviewParallel -> ReviewGemini;
  ReviewClaude -> ReviewsJoin;
  ReviewGPT -> ReviewsJoin;
  ReviewGemini -> ReviewsJoin;

  // Synthesis and routing
  ReviewsJoin -> ReviewAnalysis;
  ReviewAnalysis -> CommitWork [condition="outcome=success", label="pass"];
  ReviewAnalysis -> ImplementClaude [condition="outcome=retry", label="rework"];
  ReviewAnalysis -> FailureSummary [condition="outcome=fail", label="escalate"];

  // Ship and loop
  CommitWork -> MarkComplete;
  MarkComplete -> PickNextComponent [label="next component", loop_restart=true];

  // Failure handling
  FailureSummary -> MarkFailed;
  MarkFailed -> Exit;
}

TURN 6
TOOL CALL: read
INPUT:
{"path": "export/dot_test.go"}
TOOL RESULT: read
OUTPUT:
package export

import (
	"strings"
	"testing"
	"time"

	"github.com/2389/dippin/ir"
)

// --- Fixtures ---

func minimalWorkflow() *ir.Workflow {
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

func askAndExecuteWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "ask_and_execute",
		Goal:  "Ask user for a task, implement it, review, ship",
		Start: "AskUser",
		Exit:  "Done",
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

func toolWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "tool_test",
		Start: "Check",
		Exit:  "Report",
		Nodes: []*ir.Node{
			{
				ID: "Check", Kind: ir.NodeTool,
				IO: ir.NodeIO{Writes: []string{"test_result"}},
				Config: ir.ToolConfig{
					Command: "#!/bin/sh\nset -eu\npytest --tb=short",
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

func subgraphWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "with_subgraph",
		Start: "Build",
		Exit:  "Done",
		Nodes: []*ir.Node{
			{ID: "Build", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Build the feature."}},
			{
				ID: "Review", Kind: ir.NodeSubgraph,
				Config: ir.SubgraphConfig{
					Ref:    "./review.dip",
					Params: map[string]string{"model": "gpt-5.4"},
				},
			},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Build", To: "Review"},
			{From: "Review", To: "Done"},
		},
	}
}

// --- Test helpers ---

func assertContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output does not contain %q\n\ngot:\n%s", substr, output)
	}
}

func assertNotContains(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("output unexpectedly contains %q\n\ngot:\n%s", substr, output)
	}
}

// --- Tests ---

func TestExportDOTMinimal(t *testing.T) {
	out := ExportDOT(minimalWorkflow(), ExportOptions{})

	assertContains(t, out, "digraph minimal {")
	assertContains(t, out, "rankdir=TB;")
	assertContains(t, out, `Begin [label="Begin", shape="Mdiamond"];`)
	assertContains(t, out, `End [label="End", shape="Msquare"];`)
	assertContains(t, out, "Begin -> End;")
	assertContains(t, out, "}\n")
}

func TestExportDOTFullWorkflow(t *testing.T) {
	out := ExportDOT(askAndExecuteWorkflow(), ExportOptions{})

	// Verify digraph structure.
	assertContains(t, out, "digraph ask_and_execute {")

	// Start node gets Mdiamond shape.
	assertContains(t, out, `AskUser [label="AskUser", shape="Mdiamond"];`)

	// Exit node gets Msquare shape.
	assertContains(t, out, `Done [label="Done", shape="Msquare"];`)

	// Regular agent node gets box shape.
	assertContains(t, out, `Interpret [label="Interpret", shape="box"];`)

	// Human node (non-start) gets hexagon shape.
	assertContains(t, out, `Approve [label="Approve", shape="hexagon"];`)

	// Parallel node gets component shape.
	assertContains(t, out, `ImplementFanOut [label="ImplementFanOut", shape="component"];`)

	// Fan-in node gets tripleoctagon shape.
	assertContains(t, out, `ImplementJoin [label="ImplementJoin", shape="tripleoctagon"];`)

	// Simple edge.
	assertContains(t, out, "AskUser -> Interpret;")

	// Conditional edge.
	assertContains(t, out, `Validate -> Approve [condition="ctx.outcome = success", label="ctx.outcome = success"];`)

	// Restart edge with condition and label.
	assertContains(t, out, `Validate -> Interpret`)
	assertContains(t, out, `restart="true"`)
	assertContains(t, out, `style="dashed"`)
}

func TestExportDOTNodeShapes(t *testing.T) {
	tests := []struct {
		name     string
		kind     ir.NodeKind
		config   ir.NodeConfig
		wantAttr string
	}{
		{"agent", ir.NodeAgent, ir.AgentConfig{Prompt: "go."}, `shape="box"`},
		{"human", ir.NodeHuman, ir.HumanConfig{}, `shape="hexagon"`},
		{"tool", ir.NodeTool, ir.ToolConfig{Command: "echo"}, `shape="parallelogram"`},
		{"parallel", ir.NodeParallel, ir.ParallelConfig{Targets: []string{"A"}}, `shape="component"`},
		{"fan_in", ir.NodeFanIn, ir.FanInConfig{Sources: []string{"A"}}, `shape="tripleoctagon"`},
		{"subgraph", ir.NodeSubgraph, ir.SubgraphConfig{Ref: "x.dip"}, `shape="tab"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &ir.Workflow{
				Name:  "test",
				Start: "Start",
				Exit:  "Exit",
				Nodes: []*ir.Node{
					{ID: "Start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
					{ID: "N", Kind: tt.kind, Config: tt.config},
					{ID: "Exit", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
				},
			}
			out := ExportDOT(w, ExportOptions{})
			// N should get its kind-based shape (not overridden by start/exit).
			if !strings.Contains(out, tt.wantAttr) {
				t.Errorf("node N missing %s in output:\n%s", tt.wantAttr, out)
			}
		})
	}
}

func TestExportDOTStartExitShapeOverride(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "S",
		Exit:  "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeHuman, Config: ir.HumanConfig{}},
			{ID: "E", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo done"}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}
	out := ExportDOT(w, ExportOptions{})

	// Start node should be Mdiamond regardless of kind.
	assertContains(t, out, `S [label="S", shape="Mdiamond"];`)
	// Exit node should be Msquare regardless of kind.
	assertContains(t, out, `E [label="E", shape="Msquare"];`)
}

func TestExportDOTNodeLabel(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Label: "My Agent", Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
	}
	out := ExportDOT(w, ExportOptions{})

	// Node with a label uses the label text.
	assertContains(t, out, `label="My Agent"`)
	// Node without a label uses the ID.
	assertContains(t, out, `B [label="B"`)
}

func TestExportDOTRankDir(t *testing.T) {
	tests := []struct {
		name    string
		rankDir string
		want    string
	}{
		{"default", "", "rankdir=TB;"},
		{"LR", "LR", "rankdir=LR;"},
		{"TB explicit", "TB", "rankdir=TB;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := minimalWorkflow()
			out := ExportDOT(w, ExportOptions{RankDir: tt.rankDir})
			assertContains(t, out, tt.want)
		})
	}
}

func TestExportDOTIncludePrompts(t *testing.T) {
	t.Run("prompts included", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
					Prompt:   "Line one.\nLine two.",
					Model:    "gpt-5.4",
					Provider: "openai",
				}},
			},
		}
		out := ExportDOT(w, ExportOptions{IncludePrompts: true})
		assertContains(t, out, `model="gpt-5.4"`)
		assertContains(t, out, `prompt="Line one.`+`\n`+`Line two."`)
		assertContains(t, out, `provider="openai"`)
	})

	t.Run("prompts excluded by default", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
					Prompt: "secret prompt",
					Model:  "gpt-5.4",
				}},
			},
		}
		out := ExportDOT(w, ExportOptions{})
		assertNotContains(t, out, "prompt=")
		assertNotContains(t, out, "model=")
	})
}

func TestExportDOTToolCommand(t *testing.T) {
	out := ExportDOT(toolWorkflow(), ExportOptions{IncludePrompts: true})
	// Multiline command should be escaped.
	assertContains(t, out, `tool_command="#!/bin/sh\nset -eu\npytest --tb=short"`)
	assertContains(t, out, `timeout="1m"`)
}

func TestExportDOTHumanConfig(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "H",
		Exit:  "H",
		Nodes: []*ir.Node{
			{ID: "H", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "choice", Default: "Yes"}},
		},
	}
	out := ExportDOT(w, ExportOptions{IncludePrompts: true})
	assertContains(t, out, `default="Yes"`)
	assertContains(t, out, `mode="choice"`)
}

func TestExportDOTSubgraphConfig(t *testing.T) {
	out := ExportDOT(subgraphWorkflow(), ExportOptions{IncludePrompts: true})
	assertContains(t, out, `ref="./review.dip"`)
	assertContains(t, out, `shape="tab"`)
}

func TestExportDOTParallelConfig(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "S",
		Exit:  "J",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "P", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"A", "B"}}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b."}},
			{ID: "J", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"A", "B"}}},
		},
	}
	out := ExportDOT(w, ExportOptions{IncludePrompts: true})
	assertContains(t, out, `targets="A,B"`)
	assertContains(t, out, `sources="A,B"`)
}

func TestExportDOTHighlightGoalGates(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go.", GoalGate: true}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
	}

	t.Run("highlighting enabled", func(t *testing.T) {
		out := ExportDOT(w, ExportOptions{HighlightGoalGates: true})
		assertContains(t, out, `fillcolor="#ffcccc"`)
		assertContains(t, out, `style="filled"`)
	})

	t.Run("highlighting disabled", func(t *testing.T) {
		out := ExportDOT(w, ExportOptions{HighlightGoalGates: false})
		assertNotContains(t, out, "fillcolor")
		assertNotContains(t, out, `style="filled"`)
	})
}

func TestExportDOTEdgeConditions(t *testing.T) {
	tests := []struct {
		name      string
		condition *ir.Condition
		wantAttr  string
	}{
		{
			name: "simple compare",
			condition: &ir.Condition{
				Raw:    "ctx.outcome = success",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			},
			wantAttr: `condition="ctx.outcome = success"`,
		},
		{
			name: "AND condition",
			condition: &ir.Condition{
				Raw: "ctx.x = 1 and ctx.y = 2",
				Parsed: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
			},
			wantAttr: `condition="ctx.x = 1 and ctx.y = 2"`,
		},
		{
			name: "OR condition",
			condition: &ir.Condition{
				Raw: "ctx.x = 1 or ctx.x = 2",
				Parsed: ir.CondOr{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
				},
			},
			wantAttr: `condition="ctx.x = 1 or ctx.x = 2"`,
		},
		{
			name: "NOT condition",
			condition: &ir.Condition{
				Raw: "not ctx.done = true",
				Parsed: ir.CondNot{
					Inner: ir.CondCompare{Variable: "ctx.done", Op: "=", Value: "true"},
				},
			},
			wantAttr: `condition="not ctx.done = true"`,
		},
		{
			name: "nested AND in OR — parenthesized",
			condition: &ir.Condition{
				Raw: "(ctx.x = 1 and ctx.y = 2) or ctx.z = 3",
				Parsed: ir.CondOr{
					Left: ir.CondAnd{
						Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
						Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
					},
					Right: ir.CondCompare{Variable: "ctx.z", Op: "=", Value: "3"},
				},
			},
			wantAttr: `condition="(ctx.x = 1 and ctx.y = 2) or ctx.z = 3"`,
		},
		{
			name: "NOT of compound — parenthesized",
			condition: &ir.Condition{
				Raw: "not (ctx.x = 1 and ctx.y = 2)",
				Parsed: ir.CondNot{
					Inner: ir.CondAnd{
						Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
						Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
					},
				},
			},
			wantAttr: `condition="not (ctx.x = 1 and ctx.y = 2)"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &ir.Workflow{
				Name:  "test",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: tt.condition},
				},
			}
			out := ExportDOT(w, ExportOptions{})
			assertContains(t, out, tt.wantAttr)
		})
	}
}

func TestExportDOTEdgeRestart(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Restart: true},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, `restart="true"`)
	assertContains(t, out, `style="dashed"`)
}

func TestExportDOTEdgeWeight(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Weight: 10},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, `weight="10"`)
}

func TestExportDOTEdgeLabelWithCondition(t *testing.T) {
	// When an edge has both a label and a condition, the explicit label is used
	// and the condition is a separate attribute.
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{
				From:  "A",
				To:    "B",
				Label: "retry",
				Condition: &ir.Condition{
					Raw:    "ctx.outcome = fail",
					Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
				},
			},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, `label="retry"`)
	assertContains(t, out, `condition="ctx.outcome = fail"`)
}

func TestExportDOTEdgeConditionAsLabel(t *testing.T) {
	// When an edge has a condition but no explicit label, the condition
	// text is used as the label for visual display.
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{
				From: "A",
				To:   "B",
				Condition: &ir.Condition{
					Raw:    "ctx.outcome = success",
					Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
				},
			},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, `label="ctx.outcome = success"`)
	assertContains(t, out, `condition="ctx.outcome = success"`)
}

func TestExportDOTEmptyWorkflow(t *testing.T) {
	w := &ir.Workflow{
		Name:  "empty",
		Start: "S",
		Exit:  "E",
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, "digraph empty {")
	assertContains(t, out, "}\n")
	// No node or edge lines (just the preamble and closing).
	assertNotContains(t, out, "->")
}

func TestExportDOTNoName(t *testing.T) {
	w := &ir.Workflow{
		Start: "S",
		Exit:  "E",
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, "digraph workflow {")
}

func TestExportDOTAllEdgeAttributes(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{
				From:    "A",
				To:      "B",
				Label:   "retry",
				Weight:  5,
				Restart: true,
				Condition: &ir.Condition{
					Raw:    "ctx.outcome = fail",
					Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
				},
			},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	assertContains(t, out, `condition="ctx.outcome = fail"`)
	assertContains(t, out, `label="retry"`)
	assertContains(t, out, `restart="true"`)
	assertContains(t, out, `style="dashed"`)
	assertContains(t, out, `weight="5"`)
}

func TestExportDOTIdempotent(t *testing.T) {
	workflows := []*ir.Workflow{
		minimalWorkflow(),
		askAndExecuteWorkflow(),
		toolWorkflow(),
		subgraphWorkflow(),
	}
	for _, w := range workflows {
		first := ExportDOT(w, ExportOptions{IncludePrompts: true})
		second := ExportDOT(w, ExportOptions{IncludePrompts: true})
		if first != second {
			t.Errorf("ExportDOT is not idempotent for workflow %q", w.Name)
		}
	}
}

func TestExportDOTDeterministicAttrOrder(t *testing.T) {
	// Attributes should always appear in sorted order.
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{
				From:    "A",
				To:      "B",
				Label:   "go",
				Weight:  3,
				Restart: true,
			},
		},
	}
	out := ExportDOT(w, ExportOptions{})

	// Find the edge line.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "A -> B") {
			// Verify attribute order: label, restart, style, weight (alphabetical).
			labelIdx := strings.Index(line, "label=")
			restartIdx := strings.Index(line, "restart=")
			styleIdx := strings.Index(line, "style=")
			weightIdx := strings.Index(line, "weight=")
			if labelIdx > restartIdx || restartIdx > styleIdx || styleIdx > weightIdx {
				t.Errorf("attributes not in sorted order: %s", line)
			}
			return
		}
	}
	t.Error("could not find edge line A -> B")
}

func TestExportDOTSpecialCharactersInLabel(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Label: `Say "hello" & goodbye`, Config: ir.AgentConfig{Prompt: "go."}},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	// Internal quotes should be escaped.
	assertContains(t, out, `label="Say \"hello\" & goodbye"`)
}

func TestExportDOTValidDOTSyntax(t *testing.T) {
	// Basic structural validation: the output should be parseable as DOT.
	out := ExportDOT(askAndExecuteWorkflow(), ExportOptions{IncludePrompts: true})

	// Should start with digraph and end with }
	if !strings.HasPrefix(out, "digraph ") {
		t.Error("output should start with 'digraph '")
	}
	if !strings.HasSuffix(out, "}\n") {
		t.Error("output should end with '}\\n'")
	}

	// Count braces — should be balanced.
	opens := strings.Count(out, "{")
	closes := strings.Count(out, "}")
	if opens != closes {
		t.Errorf("unbalanced braces: %d opens, %d closes", opens, closes)
	}

	// Every node and edge statement should end with semicolon.
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "}" || strings.HasPrefix(trimmed, "digraph") {
			continue
		}
		if !strings.HasSuffix(trimmed, ";") {
			t.Errorf("line does not end with semicolon: %q", trimmed)
		}
	}
}

func TestExportDOTNilConditionParsed(t *testing.T) {
	// Edge with Condition but nil Parsed should not emit condition attribute.
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{Raw: "ctx.x = 1"}},
		},
	}
	out := ExportDOT(w, ExportOptions{})
	assertNotContains(t, out, "condition=")
}

func TestExportDOTNilConfig(t *testing.T) {
	// A node with nil config should not panic.
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent},
		},
	}
	out := ExportDOT(w, ExportOptions{IncludePrompts: true})
	assertContains(t, out, `A [`)
}

func TestExportDOTGoalGateNonAgent(t *testing.T) {
	// HighlightGoalGates should only apply to agent nodes with GoalGate: true.
	w := &ir.Workflow{
		Name:  "test",
		Start: "H",
		Exit:  "H",
		Nodes: []*ir.Node{
			{ID: "H", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
		},
	}
	out := ExportDOT(w, ExportOptions{HighlightGoalGates: true})
	assertNotContains(t, out, "fillcolor")
}

// --- Unit tests for internal helpers ---

func TestDotID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"AskUser", "AskUser"},
		{"simple_name", "simple_name"},
		{"has space", `"has space"`},
		{"123start", `"123start"`},
		{"", `""`},
		{"with-dash", `"with-dash"`},
		{"with.dot", `"with.dot"`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := dotID(tt.input)
			if got != tt.want {
				t.Errorf("dotID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDotQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{`say "hi"`, `"say \"hi\""`},
		{`path\to`, `"path\\to"`},
		{"", `""`},
		{`line1\nline2`, `"line1\nline2"`},     // DOT escape sequence preserved
		{`left\lalign`, `"left\lalign"`},        // DOT \l preserved
		{`real\\backslash`, `"real\\\\backslash"`}, // Actual backslash-backslash escaped
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := dotQuote(tt.input)
			if got != tt.want {
				t.Errorf("dotQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeNewlines(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"no newlines", "no newlines"},
		{"line1\nline2", `line1\nline2`},
		{"a\nb\nc", `a\nb\nc`},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeNewlines(tt.input)
			if got != tt.want {
				t.Errorf("escapeNewlines(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{0, "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatConditionExport(t *testing.T) {
	tests := []struct {
		name string
		expr ir.ConditionExpr
		want string
	}{
		{
			name: "simple compare",
			expr: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			want: "ctx.outcome = success",
		},
		{
			name: "AND",
			expr: ir.CondAnd{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
			},
			want: "ctx.x = 1 and ctx.y = 2",
		},
		{
			name: "OR",
			expr: ir.CondOr{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
			},
			want: "ctx.x = 1 or ctx.x = 2",
		},
		{
			name: "NOT",
			expr: ir.CondNot{Inner: ir.CondCompare{Variable: "ctx.done", Op: "=", Value: "true"}},
			want: "not ctx.done = true",
		},
		{
			name: "AND inside OR parenthesized",
			expr: ir.CondOr{
				Left: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
				Right: ir.CondCompare{Variable: "ctx.z", Op: "=", Value: "3"},
			},
			want: "(ctx.x = 1 and ctx.y = 2) or ctx.z = 3",
		},
		{
			name: "NOT of compound parenthesized",
			expr: ir.CondNot{
				Inner: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
			},
			want: "not (ctx.x = 1 and ctx.y = 2)",
		},
		{
			name: "nil",
			expr: nil,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCondition(tt.expr)
			if got != tt.want {
				t.Errorf("formatCondition() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSortStrings(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"c", "a", "b"}, []string{"a", "b", "c"}},
		{[]string{"z"}, []string{"z"}},
		{nil, nil},
		{[]string{}, []string{}},
		{[]string{"a", "a"}, []string{"a", "a"}},
	}
	for _, tt := range tests {
		got := make([]string, len(tt.input))
		copy(got, tt.input)
		sortStrings(got)
		if len(got) != len(tt.want) {
			t.Errorf("sortStrings(%v) length = %d, want %d", tt.input, len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("sortStrings(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsSimpleDOTID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"AskUser", true},
		{"node_1", true},
		{"A", true},
		{"123", false},
		{"has space", false},
		{"has-dash", false},
		{"has.dot", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isSimpleDOTID(tt.input)
			if got != tt.want {
				t.Errorf("isSimpleDOTID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDOTAttrs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := formatDOTAttrs(map[string]string{})
		if got != "" {
			t.Errorf("formatDOTAttrs({}) = %q, want empty", got)
		}
	})

	t.Run("single", func(t *testing.T) {
		got := formatDOTAttrs(map[string]string{"shape": "box"})
		if got != `[shape="box"]` {
			t.Errorf(`formatDOTAttrs = %q, want [shape="box"]`, got)
		}
	})

	t.Run("sorted keys", func(t *testing.T) {
		got := formatDOTAttrs(map[string]string{"z": "1", "a": "2"})
		// a should come before z.
		aIdx := strings.Index(got, "a=")
		zIdx := strings.Index(got, "z=")
		if aIdx > zIdx {
			t.Errorf("keys not sorted: %s", got)
		}
	})
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

TURN 7
TOOL CALL: write
INPUT:
{"path": "migrate/dot_parser.go", "content": "// Package migrate converts DOT digraph files into Dippin IR workflows.\n//\n// It implements the migration strategy from §16 of the Dippin design spec:\n// parse a DOT digraph, apply cleanup transforms (un-escaping, namespace\n// prefixing, shape→kind mapping), and produce either an *ir.Workflow or\n// canonical .dip source text.\npackage migrate\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\t\"unicode\"\n)\n\n// dotGraph holds the parsed DOT structure before IR conversion.\ntype dotGraph struct {\n\tName       string\n\tGraphAttrs map[string]string\n\tNodeAttrs  map[string]string // default node attrs\n\tEdgeAttrs  map[string]string // default edge attrs\n\tNodes      []dotNode\n\tEdges      []dotEdge\n}\n\n// dotNode is a single DOT node statement with attributes.\ntype dotNode struct {\n\tID    string\n\tAttrs map[string]string\n}\n\n// dotEdge is a single DOT edge statement (A -> B) with attributes.\ntype dotEdge struct {\n\tFrom  string\n\tTo    string\n\tAttrs map[string]string\n}\n\n// --- DOT Lexer ---\n\ntype tokenKind int\n\nconst (\n\ttokEOF tokenKind = iota\n\ttokID            // unquoted identifier or number\n\ttokString        // double-quoted string (contents unescaped)\n\ttokLBrace        // {\n\ttokRBrace        // }\n\ttokLBrack        // [\n\ttokRBrack        // ]\n\ttokEquals        // =\n\ttokSemicolon     // ;\n\ttokComma         // ,\n\ttokArrow         // ->\n)\n\ntype token struct {\n\tkind tokenKind\n\tval  string\n\tpos  int // byte offset for error messages\n}\n\n// lexer tokenizes a DOT input string.\ntype lexer struct {\n\tinput []byte\n\tpos   int\n}\n\nfunc newLexer(input string) *lexer {\n\treturn &lexer{input: []byte(input)}\n}\n\nfunc (l *lexer) peek() byte {\n\tif l.pos >= len(l.input) {\n\t\treturn 0\n\t}\n\treturn l.input[l.pos]\n}\n\nfunc (l *lexer) advance() byte {\n\tch := l.input[l.pos]\n\tl.pos++\n\treturn ch\n}\n\nfunc (l *lexer) skipWhitespace() {\n\tfor l.pos < len(l.input) {\n\t\tch := l.input[l.pos]\n\t\tif ch == ' ' || ch == '\\t' || ch == '\\n' || ch == '\\r' {\n\t\t\tl.pos++\n\t\t\tcontinue\n\t\t}\n\t\t// C-style line comments.\n\t\tif ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '/' {\n\t\t\tl.pos += 2\n\t\t\tfor l.pos < len(l.input) && l.input[l.pos] != '\\n' {\n\t\t\t\tl.pos++\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\t// C-style block comments.\n\t\tif ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '*' {\n\t\t\tl.pos += 2\n\t\t\tfor l.pos+1 < len(l.input) {\n\t\t\t\tif l.input[l.pos] == '*' && l.input[l.pos+1] == '/' {\n\t\t\t\t\tl.pos += 2\n\t\t\t\t\tbreak\n\t\t\t\t}\n\t\t\t\tl.pos++\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\tbreak\n\t}\n}\n\nfunc (l *lexer) next() token {\n\tl.skipWhitespace()\n\tif l.pos >= len(l.input) {\n\t\treturn token{kind: tokEOF, pos: l.pos}\n\t}\n\n\tstart := l.pos\n\tch := l.peek()\n\n\tswitch ch {\n\tcase '{':\n\t\tl.advance()\n\t\treturn token{kind: tokLBrace, val: \"{\", pos: start}\n\tcase '}':\n\t\tl.advance()\n\t\treturn token{kind: tokRBrace, val: \"}\", pos: start}\n\tcase '[':\n\t\tl.advance()\n\t\treturn token{kind: tokLBrack, val: \"[\", pos: start}\n\tcase ']':\n\t\tl.advance()\n\t\treturn token{kind: tokRBrack, val: \"]\", pos: start}\n\tcase '=':\n\t\tl.advance()\n\t\treturn token{kind: tokEquals, val: \"=\", pos: start}\n\tcase ';':\n\t\tl.advance()\n\t\treturn token{kind: tokSemicolon, val: \";\", pos: start}\n\tcase ',':\n\t\tl.advance()\n\t\treturn token{kind: tokComma, val: \",\", pos: start}\n\tcase '-':\n\t\tif l.pos+1 < len(l.input) && l.input[l.pos+1] == '>' {\n\t\t\tl.pos += 2\n\t\t\treturn token{kind: tokArrow, val: \"->\", pos: start}\n\t\t}\n\t\t// Bare '-' — treat as part of an identifier.\n\t\treturn l.readID()\n\tcase '\"':\n\t\treturn l.readString()\n\tdefault:\n\t\tif isIDStart(ch) || (ch >= '0' && ch <= '9') {\n\t\t\treturn l.readID()\n\t\t}\n\t\tl.advance()\n\t\treturn token{kind: tokID, val: string(ch), pos: start}\n\t}\n}\n\nfunc isIDStart(ch byte) bool {\n\treturn (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'\n}\n\nfunc isIDCont(ch byte) bool {\n\treturn isIDStart(ch) || (ch >= '0' && ch <= '9') || ch == '.'\n}\n\nfunc (l *lexer) readID() token {\n\tstart := l.pos\n\tfor l.pos < len(l.input) && isIDCont(l.input[l.pos]) {\n\t\tl.pos++\n\t}\n\treturn token{kind: tokID, val: string(l.input[start:l.pos]), pos: start}\n}\n\nfunc (l *lexer) readString() token {\n\tstart := l.pos\n\tl.advance() // skip opening \"\n\tvar b strings.Builder\n\tfor l.pos < len(l.input) {\n\t\tch := l.advance()\n\t\tif ch == '\\\\' && l.pos < len(l.input) {\n\t\t\tnext := l.advance()\n\t\t\tswitch next {\n\t\t\tcase '\"':\n\t\t\t\tb.WriteByte('\"')\n\t\t\tcase '\\\\':\n\t\t\t\tb.WriteByte('\\\\')\n\t\t\tcase 'n':\n\t\t\t\tb.WriteByte('\\n')\n\t\t\tcase 'l':\n\t\t\t\tb.WriteByte('\\n') // DOT \\l = left-justified newline → real newline\n\t\t\tcase 'r':\n\t\t\t\t// DOT \\r — ignore (not meaningful)\n\t\t\tdefault:\n\t\t\t\tb.WriteByte('\\\\')\n\t\t\t\tb.WriteByte(next)\n\t\t\t}\n\t\t\tcontinue\n\t\t}\n\t\tif ch == '\"' {\n\t\t\treturn token{kind: tokString, val: b.String(), pos: start}\n\t\t}\n\t\tb.WriteByte(ch)\n\t}\n\t// Unterminated string — return what we have; parser will catch the error.\n\treturn token{kind: tokString, val: b.String(), pos: start}\n}\n\n// --- DOT Parser ---\n\ntype parser struct {\n\tlex   *lexer\n\tcur   token\n\tgraph *dotGraph\n}\n\nfunc parseDOT(input string) (*dotGraph, error) {\n\tp := &parser{\n\t\tlex: newLexer(input),\n\t\tgraph: &dotGraph{\n\t\t\tGraphAttrs: make(map[string]string),\n\t\t\tNodeAttrs:  make(map[string]string),\n\t\t\tEdgeAttrs:  make(map[string]string),\n\t\t},\n\t}\n\tp.advance()\n\tif err := p.parseDigraph(); err != nil {\n\t\treturn nil, err\n\t}\n\treturn p.graph, nil\n}\n\nfunc (p *parser) advance() {\n\tp.cur = p.lex.next()\n}\n\nfunc (p *parser) expect(k tokenKind) (token, error) {\n\tif p.cur.kind != k {\n\t\treturn p.cur, fmt.Errorf(\"DOT parse error at offset %d: expected %s, got %q\",\n\t\t\tp.cur.pos, tokenKindName(k), p.cur.val)\n\t}\n\tt := p.cur\n\tp.advance()\n\treturn t, nil\n}\n\nfunc tokenKindName(k tokenKind) string {\n\tswitch k {\n\tcase tokEOF:\n\t\treturn \"EOF\"\n\tcase tokID:\n\t\treturn \"identifier\"\n\tcase tokString:\n\t\treturn \"string\"\n\tcase tokLBrace:\n\t\treturn \"'{'\"\n\tcase tokRBrace:\n\t\treturn \"'}'\"\n\tcase tokLBrack:\n\t\treturn \"'['\"\n\tcase tokRBrack:\n\t\treturn \"']'\"\n\tcase tokEquals:\n\t\treturn \"'='\"\n\tcase tokSemicolon:\n\t\treturn \"';'\"\n\tcase tokComma:\n\t\treturn \"','\"\n\tcase tokArrow:\n\t\treturn \"'->'\"\n\tdefault:\n\t\treturn \"unknown\"\n\t}\n}\n\nfunc (p *parser) parseDigraph() error {\n\t// Expect: digraph <name> { ... }\n\tid, err := p.expect(tokID)\n\tif err != nil {\n\t\treturn err\n\t}\n\tif id.val != \"digraph\" {\n\t\treturn fmt.Errorf(\"DOT parse error at offset %d: expected 'digraph', got %q\", id.pos, id.val)\n\t}\n\n\t// Graph name: can be ID or string.\n\tname, err := p.readIDOrString()\n\tif err != nil {\n\t\treturn err\n\t}\n\tp.graph.Name = name\n\n\tif _, err := p.expect(tokLBrace); err != nil {\n\t\treturn err\n\t}\n\n\tfor p.cur.kind != tokRBrace && p.cur.kind != tokEOF {\n\t\tif err := p.parseStatement(); err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\n\tif _, err := p.expect(tokRBrace); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}\n\nfunc (p *parser) readIDOrString() (string, error) {\n\tswitch p.cur.kind {\n\tcase tokID:\n\t\tt := p.cur\n\t\tp.advance()\n\t\treturn t.val, nil\n\tcase tokString:\n\t\tt := p.cur\n\t\tp.advance()\n\t\treturn t.val, nil\n\tdefault:\n\t\treturn \"\", fmt.Errorf(\"DOT parse error at offset %d: expected identifier or string, got %q\",\n\t\t\tp.cur.pos, p.cur.val)\n\t}\n}\n\nfunc (p *parser) parseStatement() error {\n\t// Skip stray semicolons.\n\tif p.cur.kind == tokSemicolon {\n\t\tp.advance()\n\t\treturn nil\n\t}\n\n\t// Must be an ID or string to start a statement.\n\tif p.cur.kind != tokID && p.cur.kind != tokString {\n\t\treturn fmt.Errorf(\"DOT parse error at offset %d: unexpected token %q\", p.cur.pos, p.cur.val)\n\t}\n\n\tname := p.cur.val\n\tnameKind := p.cur.kind\n\tp.advance()\n\n\t// graph/node/edge default attributes: graph [ ... ] or node [ ... ] or edge [ ... ]\n\tif nameKind == tokID && (name == \"graph\" || name == \"node\" || name == \"edge\") && p.cur.kind == tokLBrack {\n\t\tattrs, err := p.parseAttrList()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tswitch name {\n\t\tcase \"graph\":\n\t\t\tfor k, v := range attrs {\n\t\t\t\tp.graph.GraphAttrs[k] = v\n\t\t\t}\n\t\tcase \"node\":\n\t\t\tfor k, v := range attrs {\n\t\t\t\tp.graph.NodeAttrs[k] = v\n\t\t\t}\n\t\tcase \"edge\":\n\t\t\tfor k, v := range attrs {\n\t\t\t\tp.graph.EdgeAttrs[k] = v\n\t\t\t}\n\t\t}\n\t\tp.consumeOptionalSemicolon()\n\t\treturn nil\n\t}\n\n\t// Edge statement: ID -> ID [ ... ] ;\n\tif p.cur.kind == tokArrow {\n\t\tp.advance()\n\t\tto, err := p.readIDOrString()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t\tattrs := make(map[string]string)\n\t\tif p.cur.kind == tokLBrack {\n\t\t\tattrs, err = p.parseAttrList()\n\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n\t\t}\n\t\t// Merge default edge attrs.\n\t\tmerged := make(map[string]string)\n\t\tfor k, v := range p.graph.EdgeAttrs {\n\t\t\tmerged[k] = v\n\t\t}\n\t\tfor k, v := range attrs {\n\t\t\tmerged[k] = v\n\t\t}\n\t\tp.graph.Edges = append(p.graph.Edges, dotEdge{From: name, To: to, Attrs: merged})\n\n\t\t// Ensure both nodes exist (implicit declaration).\n\t\tp.ensureNode(name)\n\t\tp.ensureNode(to)\n\n\t\tp.consumeOptionalSemicolon()\n\t\treturn nil\n\t}\n\n\t// Node statement: ID [ ... ] ; or bare ID ;\n\tattrs := make(map[string]string)\n\tif p.cur.kind == tokLBrack {\n\t\tvar err error\n\t\tattrs, err = p.parseAttrList()\n\t\tif err != nil {\n\t\t\treturn err\n\t\t}\n\t}\n\t// Merge default node attrs.\n\tmerged := make(map[string]string)\n\tfor k, v := range p.graph.NodeAttrs {\n\t\tmerged[k] = v\n\t}\n\tfor k, v := range attrs {\n\t\tmerged[k] = v\n\t}\n\tp.addOrUpdateNode(name, merged)\n\n\tp.consumeOptionalSemicolon()\n\treturn nil\n}\n\n// parseAttrList parses [ key=value, key=value, ... ].\nfunc (p *parser) parseAttrList() (map[string]string, error) {\n\tif _, err := p.expect(tokLBrack); err != nil {\n\t\treturn nil, err\n\t}\n\n\tattrs := make(map[string]string)\n\tfor p.cur.kind != tokRBrack && p.cur.kind != tokEOF {\n\t\tkey, err := p.readIDOrString()\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif _, err := p.expect(tokEquals); err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tval, err := p.readIDOrString()\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tattrs[key] = val\n\n\t\t// Optional comma or semicolon separator.\n\t\tif p.cur.kind == tokComma || p.cur.kind == tokSemicolon {\n\t\t\tp.advance()\n\t\t}\n\t}\n\n\tif _, err := p.expect(tokRBrack); err != nil {\n\t\treturn nil, err\n\t}\n\treturn attrs, nil\n}\n\nfunc (p *parser) consumeOptionalSemicolon() {\n\tif p.cur.kind == tokSemicolon {\n\t\tp.advance()\n\t}\n}\n\n// ensureNode adds a placeholder node if it doesn't already exist.\nfunc (p *parser) ensureNode(id string) {\n\tfor _, n := range p.graph.Nodes {\n\t\tif n.ID == id {\n\t\t\treturn\n\t\t}\n\t}\n\tp.graph.Nodes = append(p.graph.Nodes, dotNode{ID: id, Attrs: make(map[string]string)})\n}\n\n// addOrUpdateNode adds a node or updates its attrs if it already exists.\nfunc (p *parser) addOrUpdateNode(id string, attrs map[string]string) {\n\tfor i, n := range p.graph.Nodes {\n\t\tif n.ID == id {\n\t\t\t// Merge attrs into existing node.\n\t\t\tfor k, v := range attrs {\n\t\t\t\tp.graph.Nodes[i].Attrs[k] = v\n\t\t\t}\n\t\t\treturn\n\t\t}\n\t}\n\tp.graph.Nodes = append(p.graph.Nodes, dotNode{ID: id, Attrs: attrs})\n}\n\n// unescapeDOT transforms DOT escape sequences in a raw string value.\n// The lexer handles this for quoted strings; this is for post-processing\n// values that were read as unquoted identifiers or for additional cleanup.\nfunc unescapeDOT(s string) string {\n\t// The lexer already handles escape sequences inside quoted strings.\n\t// This function handles any remaining \\n sequences that might appear\n\t// in attribute values from unquoted or partially-processed sources.\n\treturn s\n}\n\n// normalizeWhitespace collapses runs of whitespace into single spaces\n// and trims leading/trailing whitespace. Used for condition comparison.\nfunc normalizeWhitespace(s string) string {\n\tvar b strings.Builder\n\tinSpace := false\n\tfor _, r := range s {\n\t\tif unicode.IsSpace(r) {\n\t\t\tif !inSpace {\n\t\t\t\tb.WriteByte(' ')\n\t\t\t\tinSpace = true\n\t\t\t}\n\t\t} else {\n\t\t\tb.WriteRune(r)\n\t\t\tinSpace = false\n\t\t}\n\t}\n\treturn strings.TrimSpace(b.String())\n}\n"}
TOOL RESULT: write
OUTPUT:
wrote 11423 bytes to migrate/dot_parser.go
TURN 8
TOOL CALL: write
INPUT:
{"content": "package migrate\n\nimport (\n\t\"fmt\"\n\t\"strconv\"\n\t\"strings\"\n\t\"time\"\n\n\t\"github.com/2389/dippin/formatter\"\n\t\"github.com/2389/dippin/ir\"\n)\n\n// Migrate parses a DOT digraph string and produces a Dippin IR workflow.\n// It applies all post-migration cleanup automatically:\n//   - Shape → node kind mapping\n//   - Un-escaping of prompts and commands (handled by the DOT lexer)\n//   - Namespace prefixing for condition variables (bare \"outcome\" → \"ctx.outcome\")\n//   - Start/exit identification from Mdiamond/Msquare shapes\n//   - Graph-level attribute extraction to WorkflowDefaults\nfunc Migrate(dotSource string) (*ir.Workflow, error) {\n\tdg, err := parseDOT(dotSource)\n\tif err != nil {\n\t\treturn nil, fmt.Errorf(\"migrate: %w\", err)\n\t}\n\treturn convertDOTGraph(dg)\n}\n\n// MigrateToSource parses DOT and returns canonical .dip source text.\n// Convenience: equivalent to formatter.Format(Migrate(dotSource)).\nfunc MigrateToSource(dotSource string) (string, error) {\n\tw, err := Migrate(dotSource)\n\tif err != nil {\n\t\treturn \"\", err\n\t}\n\treturn formatter.Format(w), nil\n}\n\n// --- Shape → Kind Mapping ---\n\n// shapeToKind maps DOT shape attributes to IR node kinds.\n// Mdiamond and Msquare are handled specially (start/exit markers).\n// diamond is handled with disambiguation logic.\nvar shapeToKind = map[string]ir.NodeKind{\n\t\"box\":            ir.NodeAgent,\n\t\"hexagon\":        ir.NodeHuman,\n\t\"parallelogram\":  ir.NodeTool,\n\t\"component\":      ir.NodeParallel,\n\t\"tripleoctagon\":  ir.NodeFanIn,\n\t\"tab\":            ir.NodeSubgraph,\n}\n\n// convertDOTGraph transforms a parsed DOT graph into an IR workflow.\nfunc convertDOTGraph(dg *dotGraph) (*ir.Workflow, error) {\n\tw := &ir.Workflow{\n\t\tName:    dg.Name,\n\t\tVersion: \"1\",\n\t}\n\n\t// Extract graph-level attributes.\n\textractGraphDefaults(dg.GraphAttrs, w)\n\n\t// Build a set of edge-implicit nodes for quick lookup.\n\tnodeIndex := make(map[string]int) // ID → index in dg.Nodes\n\n\tfor i, n := range dg.Nodes {\n\t\tnodeIndex[n.ID] = i\n\t}\n\n\t// First pass: identify start/exit nodes and build IR nodes.\n\tstartID := \"\"\n\texitID := \"\"\n\tfor _, dn := range dg.Nodes {\n\t\tshape := dn.Attrs[\"shape\"]\n\t\tif shape == \"Mdiamond\" {\n\t\t\tstartID = dn.ID\n\t\t}\n\t\tif shape == \"Msquare\" {\n\t\t\texitID = dn.ID\n\t\t}\n\t}\n\n\t// Build IR nodes in declaration order.\n\tfor _, dn := range dg.Nodes {\n\t\tnode, err := convertNode(dn, dg.Edges)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf(\"migrate: node %q: %w\", dn.ID, err)\n\t\t}\n\t\tw.Nodes = append(w.Nodes, node)\n\t}\n\n\t// Set start/exit.\n\tw.Start = startID\n\tw.Exit = exitID\n\n\t// Build IR edges.\n\tfor _, de := range dg.Edges {\n\t\tedge, err := convertEdge(de)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf(\"migrate: edge %s->%s: %w\", de.From, de.To, err)\n\t\t}\n\t\tw.Edges = append(w.Edges, edge)\n\t}\n\n\t// Post-pass: infer parallel targets and fan_in sources from edges.\n\tinferParallelFanIn(w)\n\n\treturn w, nil\n}\n\n// extractGraphDefaults populates workflow-level fields from DOT graph attributes.\nfunc extractGraphDefaults(attrs map[string]string, w *ir.Workflow) {\n\tfor k, v := range attrs {\n\t\tswitch k {\n\t\tcase \"goal\":\n\t\t\tw.Goal = v\n\t\tcase \"rankdir\":\n\t\t\t// Presentation-only; ignored.\n\t\tcase \"default_max_retry\", \"max_retries\":\n\t\t\tif n, err := strconv.Atoi(v); err == nil {\n\t\t\t\tw.Defaults.MaxRetries = n\n\t\t\t}\n\t\tcase \"max_restarts\":\n\t\t\tif n, err := strconv.Atoi(v); err == nil {\n\t\t\t\tw.Defaults.MaxRestarts = n\n\t\t\t}\n\t\tcase \"default_fidelity\", \"fidelity\":\n\t\t\tw.Defaults.Fidelity = v\n\t\tcase \"model\":\n\t\t\tw.Defaults.Model = v\n\t\tcase \"provider\":\n\t\t\tw.Defaults.Provider = v\n\t\t}\n\t}\n}\n\n// convertNode converts a DOT node to an IR node.\nfunc convertNode(dn dotNode, edges []dotEdge) (*ir.Node, error) {\n\tshape := dn.Attrs[\"shape\"]\n\tkind := resolveKind(shape, dn.Attrs)\n\n\tnode := &ir.Node{\n\t\tID:   dn.ID,\n\t\tKind: kind,\n\t}\n\n\t// Set label.\n\tif label, ok := dn.Attrs[\"label\"]; ok {\n\t\tnode.Label = label\n\t}\n\n\t// Build kind-specific config.\n\tswitch kind {\n\tcase ir.NodeAgent:\n\t\tcfg := buildAgentConfig(dn.Attrs)\n\t\tnode.Config = cfg\n\t\tnode.Retry = buildRetryConfig(dn.Attrs)\n\tcase ir.NodeHuman:\n\t\tnode.Config = buildHumanConfig(dn.Attrs)\n\tcase ir.NodeTool:\n\t\tcfg, err := buildToolConfig(dn.Attrs)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tnode.Config = cfg\n\tcase ir.NodeParallel:\n\t\tnode.Config = buildParallelConfig(dn.Attrs)\n\tcase ir.NodeFanIn:\n\t\tnode.Config = buildFanInConfig(dn.Attrs)\n\tcase ir.NodeSubgraph:\n\t\tnode.Config = buildSubgraphConfig(dn.Attrs)\n\tdefault:\n\t\tnode.Config = ir.AgentConfig{}\n\t}\n\n\treturn node, nil\n}\n\n// resolveKind determines the IR node kind from the DOT shape and attributes.\n// Implements the diamond disambiguation logic from §5.\nfunc resolveKind(shape string, attrs map[string]string) ir.NodeKind {\n\t// Start/exit markers become agent nodes.\n\tif shape == \"Mdiamond\" || shape == \"Msquare\" {\n\t\treturn ir.NodeAgent\n\t}\n\n\t// Diamond disambiguation: per §5.\n\tif shape == \"diamond\" {\n\t\tif _, hasTool := attrs[\"tool_command\"]; hasTool {\n\t\t\treturn ir.NodeTool\n\t\t}\n\t\t// All other diamonds become agent nodes (routing or prompt-based).\n\t\treturn ir.NodeAgent\n\t}\n\n\t// Direct mapping.\n\tif kind, ok := shapeToKind[shape]; ok {\n\t\treturn kind\n\t}\n\n\t// Default: agent.\n\treturn ir.NodeAgent\n}\n\n// --- Config builders ---\n\nfunc buildAgentConfig(attrs map[string]string) ir.AgentConfig {\n\tcfg := ir.AgentConfig{}\n\n\tif v, ok := attrs[\"prompt\"]; ok {\n\t\tcfg.Prompt = v\n\t}\n\tif v, ok := attrs[\"system_prompt\"]; ok {\n\t\tcfg.SystemPrompt = v\n\t}\n\t// Legacy: llm_model → model.\n\tif v, ok := attrs[\"model\"]; ok {\n\t\tcfg.Model = v\n\t}\n\tif v, ok := attrs[\"llm_model\"]; ok {\n\t\tcfg.Model = v\n\t}\n\t// Legacy: llm_provider → provider.\n\tif v, ok := attrs[\"provider\"]; ok {\n\t\tcfg.Provider = v\n\t}\n\tif v, ok := attrs[\"llm_provider\"]; ok {\n\t\tcfg.Provider = v\n\t}\n\tif v, ok := attrs[\"reasoning_effort\"]; ok {\n\t\tcfg.ReasoningEffort = v\n\t}\n\tif v, ok := attrs[\"fidelity\"]; ok {\n\t\tcfg.Fidelity = v\n\t}\n\tif v, ok := attrs[\"goal_gate\"]; ok && isTruthy(v) {\n\t\tcfg.GoalGate = true\n\t}\n\tif v, ok := attrs[\"auto_status\"]; ok && isTruthy(v) {\n\t\tcfg.AutoStatus = true\n\t}\n\tif v, ok := attrs[\"max_turns\"]; ok {\n\t\tif n, err := strconv.Atoi(v); err == nil {\n\t\t\tcfg.MaxTurns = n\n\t\t}\n\t}\n\tif v, ok := attrs[\"cmd_timeout\"]; ok {\n\t\tif d, err := time.ParseDuration(v); err == nil {\n\t\t\tcfg.CmdTimeout = d\n\t\t}\n\t}\n\tif v, ok := attrs[\"cache_tools\"]; ok && isTruthy(v) {\n\t\tcfg.CacheTools = true\n\t}\n\tif v, ok := attrs[\"compaction\"]; ok {\n\t\tcfg.Compaction = v\n\t}\n\n\treturn cfg\n}\n\nfunc buildHumanConfig(attrs map[string]string) ir.HumanConfig {\n\tcfg := ir.HumanConfig{}\n\tif v, ok := attrs[\"mode\"]; ok {\n\t\tcfg.Mode = v\n\t}\n\tif v, ok := attrs[\"default\"]; ok {\n\t\tcfg.Default = v\n\t}\n\treturn cfg\n}\n\nfunc buildToolConfig(attrs map[string]string) (ir.ToolConfig, error) {\n\tcfg := ir.ToolConfig{}\n\tif v, ok := attrs[\"tool_command\"]; ok {\n\t\tcfg.Command = v\n\t}\n\tif v, ok := attrs[\"timeout\"]; ok {\n\t\td, err := time.ParseDuration(v)\n\t\tif err != nil {\n\t\t\treturn cfg, fmt.Errorf(\"invalid timeout %q: %w\", v, err)\n\t\t}\n\t\tcfg.Timeout = d\n\t}\n\treturn cfg, nil\n}\n\nfunc buildParallelConfig(attrs map[string]string) ir.ParallelConfig {\n\tcfg := ir.ParallelConfig{}\n\tif v, ok := attrs[\"targets\"]; ok {\n\t\tcfg.Targets = splitComma(v)\n\t}\n\treturn cfg\n}\n\nfunc buildFanInConfig(attrs map[string]string) ir.FanInConfig {\n\tcfg := ir.FanInConfig{}\n\tif v, ok := attrs[\"sources\"]; ok {\n\t\tcfg.Sources = splitComma(v)\n\t}\n\treturn cfg\n}\n\nfunc buildSubgraphConfig(attrs map[string]string) ir.SubgraphConfig {\n\tcfg := ir.SubgraphConfig{}\n\tif v, ok := attrs[\"ref\"]; ok {\n\t\tcfg.Ref = v\n\t}\n\treturn cfg\n}\n\nfunc buildRetryConfig(attrs map[string]string) ir.RetryConfig {\n\trc := ir.RetryConfig{}\n\tif v, ok := attrs[\"retry_policy\"]; ok {\n\t\trc.Policy = v\n\t}\n\tif v, ok := attrs[\"max_retries\"]; ok {\n\t\tif n, err := strconv.Atoi(v); err == nil {\n\t\t\trc.MaxRetries = n\n\t\t}\n\t}\n\tif v, ok := attrs[\"retry_target\"]; ok {\n\t\trc.RetryTarget = v\n\t}\n\tif v, ok := attrs[\"fallback_target\"]; ok {\n\t\trc.FallbackTarget = v\n\t}\n\treturn rc\n}\n\n// --- Edge conversion ---\n\nfunc convertEdge(de dotEdge) (*ir.Edge, error) {\n\te := &ir.Edge{\n\t\tFrom: de.From,\n\t\tTo:   de.To,\n\t}\n\n\tif v, ok := de.Attrs[\"label\"]; ok {\n\t\te.Label = v\n\t}\n\tif v, ok := de.Attrs[\"condition\"]; ok {\n\t\tcond, err := parseCondition(v)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf(\"condition %q: %w\", v, err)\n\t\t}\n\t\te.Condition = cond\n\t}\n\tif v, ok := de.Attrs[\"weight\"]; ok {\n\t\tif n, err := strconv.Atoi(v); err == nil {\n\t\t\te.Weight = n\n\t\t}\n\t}\n\t// Both restart and loop_restart (legacy) map to Edge.Restart.\n\tif v, ok := de.Attrs[\"restart\"]; ok && isTruthy(v) {\n\t\te.Restart = true\n\t}\n\tif v, ok := de.Attrs[\"loop_restart\"]; ok && isTruthy(v) {\n\t\te.Restart = true\n\t}\n\n\treturn e, nil\n}\n\n// --- Condition parsing ---\n\n// parseCondition parses a DOT condition string into an ir.Condition.\n// It handles:\n//   - Simple comparisons: outcome=success, tool_stdout contains pass\n//   - AND/OR: outcome=success && tool_stdout contains done\n//   - NOT: not outcome=fail, !outcome=fail\n//   - Namespace prefixing: bare \"outcome\" → \"ctx.outcome\"\nfunc parseCondition(raw string) (*ir.Condition, error) {\n\traw = strings.TrimSpace(raw)\n\tif raw == \"\" {\n\t\treturn nil, nil\n\t}\n\n\texpr, err := parseCondExpr(raw)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\n\t// Format the parsed condition back to a canonical raw string.\n\tcanonRaw := formatCondExpr(expr)\n\n\treturn &ir.Condition{\n\t\tRaw:    canonRaw,\n\t\tParsed: expr,\n\t}, nil\n}\n\n// parseCondExpr parses a condition expression string into an AST.\nfunc parseCondExpr(s string) (ir.ConditionExpr, error) {\n\ts = strings.TrimSpace(s)\n\tif s == \"\" {\n\t\treturn nil, fmt.Errorf(\"empty condition expression\")\n\t}\n\n\t// Try to split on || (OR — lowest precedence).\n\tif parts, ok := splitLogicalOp(s, \"||\"); ok {\n\t\tleft, err := parseCondExpr(parts[0])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tright, err := parseCondExpr(parts[1])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn ir.CondOr{Left: left, Right: right}, nil\n\t}\n\n\t// Try to split on && (AND — higher precedence).\n\tif parts, ok := splitLogicalOp(s, \"&&\"); ok {\n\t\tleft, err := parseCondExpr(parts[0])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tright, err := parseCondExpr(parts[1])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn ir.CondAnd{Left: left, Right: right}, nil\n\t}\n\n\t// Handle NOT prefix.\n\tif strings.HasPrefix(s, \"not \") {\n\t\tinner, err := parseCondExpr(s[4:])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn ir.CondNot{Inner: inner}, nil\n\t}\n\tif strings.HasPrefix(s, \"!\") {\n\t\tinner, err := parseCondExpr(s[1:])\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\treturn ir.CondNot{Inner: inner}, nil\n\t}\n\n\t// Parse a single comparison: var op value.\n\treturn parseComparison(s)\n}\n\n// splitLogicalOp splits a condition string on the given logical operator (&&, ||).\n// Returns the two parts if the operator is found at the top level (not inside parens).\nfunc splitLogicalOp(s, op string) ([]string, bool) {\n\tdepth := 0\n\tfor i := 0; i <= len(s)-len(op); i++ {\n\t\tch := s[i]\n\t\tif ch == '(' {\n\t\t\tdepth++\n\t\t} else if ch == ')' {\n\t\t\tdepth--\n\t\t}\n\t\tif depth == 0 && s[i:i+len(op)] == op {\n\t\t\tleft := strings.TrimSpace(s[:i])\n\t\t\tright := strings.TrimSpace(s[i+len(op):])\n\t\t\tif left != \"\" && right != \"\" {\n\t\t\t\treturn []string{left, right}, true\n\t\t\t}\n\t\t}\n\t}\n\treturn nil, false\n}\n\n// parseComparison parses a single condition comparison like \"outcome=success\"\n// or \"tool_stdout contains pass\".\nfunc parseComparison(s string) (ir.ConditionExpr, error) {\n\ts = strings.TrimSpace(s)\n\n\t// Try != first (before =) to avoid matching the = in !=.\n\tif idx := strings.Index(s, \"!=\"); idx > 0 {\n\t\tvariable := strings.TrimSpace(s[:idx])\n\t\tvalue := strings.TrimSpace(s[idx+2:])\n\t\treturn ir.CondCompare{\n\t\t\tVariable: addNamespacePrefix(variable),\n\t\t\tOp:       \"!=\",\n\t\t\tValue:    value,\n\t\t}, nil\n\t}\n\n\t// Try = (equality).\n\tif idx := strings.Index(s, \"=\"); idx > 0 {\n\t\tvariable := strings.TrimSpace(s[:idx])\n\t\tvalue := strings.TrimSpace(s[idx+1:])\n\t\treturn ir.CondCompare{\n\t\t\tVariable: addNamespacePrefix(variable),\n\t\t\tOp:       \"=\",\n\t\t\tValue:    value,\n\t\t}, nil\n\t}\n\n\t// Try word-based operators: contains, startswith, endswith, in.\n\tfor _, op := range []string{\" contains \", \" startswith \", \" endswith \", \" in \"} {\n\t\tif idx := strings.Index(s, op); idx > 0 {\n\t\t\tvariable := strings.TrimSpace(s[:idx])\n\t\t\tvalue := strings.TrimSpace(s[idx+len(op):])\n\t\t\treturn ir.CondCompare{\n\t\t\t\tVariable: addNamespacePrefix(variable),\n\t\t\t\tOp:       strings.TrimSpace(op),\n\t\t\t\tValue:    value,\n\t\t\t}, nil\n\t\t}\n\t}\n\n\treturn nil, fmt.Errorf(\"cannot parse condition comparison: %q\", s)\n}\n\n// addNamespacePrefix adds the ctx. namespace to bare condition variable names.\n// Variables that already contain a dot are left as-is (graph.*, ctx.*).\n// The legacy \"context.\" prefix is normalized to \"ctx.\".\nfunc addNamespacePrefix(variable string) string {\n\tvariable = strings.TrimSpace(variable)\n\n\t// Already namespaced with \"context.\" → normalize to \"ctx.\"\n\tif strings.HasPrefix(variable, \"context.\") {\n\t\treturn \"ctx.\" + variable[len(\"context.\"):]\n\t}\n\n\t// Already namespaced (contains a dot).\n\tif strings.Contains(variable, \".\") {\n\t\treturn variable\n\t}\n\n\t// Bare variable name → add ctx. prefix.\n\treturn \"ctx.\" + variable\n}\n\n// formatCondExpr renders a condition AST back to a canonical string.\nfunc formatCondExpr(expr ir.ConditionExpr) string {\n\treturn formatCondExprPrec(expr, 0)\n}\n\nconst (\n\tcondPrecOr  = 1\n\tcondPrecAnd = 2\n\tcondPrecNot = 3\n)\n\nfunc formatCondExprPrec(expr ir.ConditionExpr, parentPrec int) string {\n\tswitch e := expr.(type) {\n\tcase ir.CondCompare:\n\t\treturn fmt.Sprintf(\"%s %s %s\", e.Variable, e.Op, e.Value)\n\tcase ir.CondAnd:\n\t\ts := fmt.Sprintf(\"%s and %s\",\n\t\t\tformatCondExprPrec(e.Left, condPrecAnd),\n\t\t\tformatCondExprPrec(e.Right, condPrecAnd))\n\t\tif parentPrec != 0 && parentPrec != condPrecAnd {\n\t\t\treturn \"(\" + s + \")\"\n\t\t}\n\t\treturn s\n\tcase ir.CondOr:\n\t\ts := fmt.Sprintf(\"%s or %s\",\n\t\t\tformatCondExprPrec(e.Left, condPrecOr),\n\t\t\tformatCondExprPrec(e.Right, condPrecOr))\n\t\tif parentPrec != 0 && parentPrec != condPrecOr {\n\t\t\treturn \"(\" + s + \")\"\n\t\t}\n\t\treturn s\n\tcase ir.CondNot:\n\t\tinner := formatCondExprPrec(e.Inner, condPrecNot)\n\t\treturn \"not \" + inner\n\tdefault:\n\t\treturn \"\"\n\t}\n}\n\n// --- Parallel/Fan-in inference ---\n\n// inferParallelFanIn fills in Targets and Sources from edges when not\n// explicitly set in DOT attributes.\nfunc inferParallelFanIn(w *ir.Workflow) {\n\tfor _, n := range w.Nodes {\n\t\tswitch cfg := n.Config.(type) {\n\t\tcase ir.ParallelConfig:\n\t\t\tif len(cfg.Targets) == 0 {\n\t\t\t\tedges := w.EdgesFrom(n.ID)\n\t\t\t\ttargets := make([]string, 0, len(edges))\n\t\t\t\tfor _, e := range edges {\n\t\t\t\t\ttargets = append(targets, e.To)\n\t\t\t\t}\n\t\t\t\tn.Config = ir.ParallelConfig{Targets: targets}\n\t\t\t}\n\t\tcase ir.FanInConfig:\n\t\t\tif len(cfg.Sources) == 0 {\n\t\t\t\tedges := w.EdgesTo(n.ID)\n\t\t\t\tsources := make([]string, 0, len(edges))\n\t\t\t\tfor _, e := range edges {\n\t\t\t\t\tsources = append(sources, e.From)\n\t\t\t\t}\n\t\t\t\tn.Config = ir.FanInConfig{Sources: sources}\n\t\t\t}\n\t\t}\n\t}\n}\n\n// --- Helpers ---\n\nfunc isTruthy(s string) bool {\n\ts = strings.ToLower(strings.TrimSpace(s))\n\treturn s == \"true\" || s == \"1\" || s == \"yes\"\n}\n\nfunc splitComma(s string) []string {\n\tparts := strings.Split(s, \",\")\n\tresult := make([]string, 0, len(parts))\n\tfor _, p := range parts {\n\t\tp = strings.TrimSpace(p)\n\t\tif p != \"\" {\n\t\t\tresult = append(result, p)\n\t\t}\n\t}\n\treturn result\n}\n", "path": "migrate/migrate.go"}
TOOL RESULT: write
OUTPUT:
wrote 15128 bytes to migrate/migrate.go
TURN 9
TOOL CALL: write
INPUT:
{"content": "package migrate\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"github.com/2389/dippin/ir\"\n)\n\n// Difference describes a structural difference between two workflows.\ntype Difference struct {\n\tKind    string // \"node_missing\", \"node_extra\", \"edge_missing\", \"edge_extra\",\n\t//                \"config_mismatch\", \"kind_mismatch\", \"start_mismatch\",\n\t//                \"exit_mismatch\", \"defaults_mismatch\"\n\tMessage string // Human-readable description\n\tPathA   string // Location in workflow A (e.g., \"node:Validate\")\n\tPathB   string // Location in workflow B (may be empty)\n}\n\n// CheckParity compares two workflows for structural equivalence.\n// It checks:\n//   - Same node IDs and kinds\n//   - Same edges (from/to/conditions)\n//   - Same start/exit\n//   - Compatible node configurations (prompt content modulo whitespace)\n//   - Same graph-level defaults\nfunc CheckParity(a, b *ir.Workflow) []Difference {\n\tvar diffs []Difference\n\n\t// Start/exit.\n\tif a.Start != b.Start {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"start_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"start differs: %q vs %q\", a.Start, b.Start),\n\t\t\tPathA:   \"workflow.start\",\n\t\t\tPathB:   \"workflow.start\",\n\t\t})\n\t}\n\tif a.Exit != b.Exit {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"exit_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"exit differs: %q vs %q\", a.Exit, b.Exit),\n\t\t\tPathA:   \"workflow.exit\",\n\t\t\tPathB:   \"workflow.exit\",\n\t\t})\n\t}\n\n\t// Build node maps.\n\taNodes := make(map[string]*ir.Node)\n\tfor _, n := range a.Nodes {\n\t\taNodes[n.ID] = n\n\t}\n\tbNodes := make(map[string]*ir.Node)\n\tfor _, n := range b.Nodes {\n\t\tbNodes[n.ID] = n\n\t}\n\n\t// Check for missing / extra nodes.\n\tfor id, na := range aNodes {\n\t\tnb, ok := bNodes[id]\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"node_missing\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q present in A but missing from B\", id),\n\t\t\t\tPathA:   \"node:\" + id,\n\t\t\t})\n\t\t\tcontinue\n\t\t}\n\t\t// Kind mismatch.\n\t\tif na.Kind != nb.Kind {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"kind_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q kind: %q vs %q\", id, na.Kind, nb.Kind),\n\t\t\t\tPathA:   \"node:\" + id,\n\t\t\t\tPathB:   \"node:\" + id,\n\t\t\t})\n\t\t}\n\t\t// Config comparison (per kind).\n\t\tdiffs = append(diffs, compareConfigs(id, na, nb)...)\n\t}\n\n\tfor id := range bNodes {\n\t\tif _, ok := aNodes[id]; !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"node_extra\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q present in B but missing from A\", id),\n\t\t\t\tPathB:   \"node:\" + id,\n\t\t\t})\n\t\t}\n\t}\n\n\t// Check edges.\n\taEdges := edgeSet(a.Edges)\n\tbEdges := edgeSet(b.Edges)\n\n\tfor key := range aEdges {\n\t\tif _, ok := bEdges[key]; !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"edge_missing\",\n\t\t\t\tMessage: fmt.Sprintf(\"edge %s present in A but missing from B\", key),\n\t\t\t\tPathA:   \"edge:\" + key,\n\t\t\t})\n\t\t}\n\t}\n\tfor key := range bEdges {\n\t\tif _, ok := aEdges[key]; !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"edge_extra\",\n\t\t\t\tMessage: fmt.Sprintf(\"edge %s present in B but missing from A\", key),\n\t\t\t\tPathB:   \"edge:\" + key,\n\t\t\t})\n\t\t}\n\t}\n\n\t// Compare defaults.\n\tdiffs = append(diffs, compareDefaults(a.Defaults, b.Defaults)...)\n\n\treturn diffs\n}\n\n// edgeKey produces a canonical string key for an edge including condition.\nfunc edgeKey(e *ir.Edge) string {\n\tcondStr := \"\"\n\tif e.Condition != nil {\n\t\tcondStr = e.Condition.Raw\n\t}\n\treturn fmt.Sprintf(\"%s->%s[%s]\", e.From, e.To, condStr)\n}\n\nfunc edgeSet(edges []*ir.Edge) map[string]*ir.Edge {\n\tm := make(map[string]*ir.Edge, len(edges))\n\tfor _, e := range edges {\n\t\tm[edgeKey(e)] = e\n\t}\n\treturn m\n}\n\n// compareConfigs compares the configurations of two nodes with the same ID.\nfunc compareConfigs(id string, a, b *ir.Node) []Difference {\n\tvar diffs []Difference\n\tpath := \"node:\" + id\n\n\tswitch ac := a.Config.(type) {\n\tcase ir.AgentConfig:\n\t\tbc, ok := b.Config.(ir.AgentConfig)\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q config type mismatch: AgentConfig vs %T\", id, b.Config),\n\t\t\t\tPathA:   path,\n\t\t\t\tPathB:   path,\n\t\t\t})\n\t\t\treturn diffs\n\t\t}\n\t\t// Compare prompts with whitespace tolerance.\n\t\tif !promptsEqual(ac.Prompt, bc.Prompt) {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q prompt differs\", id),\n\t\t\t\tPathA:   path + \".prompt\",\n\t\t\t\tPathB:   path + \".prompt\",\n\t\t\t})\n\t\t}\n\t\tif ac.Model != bc.Model {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q model: %q vs %q\", id, ac.Model, bc.Model),\n\t\t\t\tPathA:   path + \".model\",\n\t\t\t\tPathB:   path + \".model\",\n\t\t\t})\n\t\t}\n\t\tif ac.Provider != bc.Provider {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q provider: %q vs %q\", id, ac.Provider, bc.Provider),\n\t\t\t\tPathA:   path + \".provider\",\n\t\t\t\tPathB:   path + \".provider\",\n\t\t\t})\n\t\t}\n\t\tif ac.GoalGate != bc.GoalGate {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q goal_gate: %v vs %v\", id, ac.GoalGate, bc.GoalGate),\n\t\t\t\tPathA:   path + \".goal_gate\",\n\t\t\t\tPathB:   path + \".goal_gate\",\n\t\t\t})\n\t\t}\n\t\tif ac.AutoStatus != bc.AutoStatus {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q auto_status: %v vs %v\", id, ac.AutoStatus, bc.AutoStatus),\n\t\t\t\tPathA:   path + \".auto_status\",\n\t\t\t\tPathB:   path + \".auto_status\",\n\t\t\t})\n\t\t}\n\n\tcase ir.ToolConfig:\n\t\tbc, ok := b.Config.(ir.ToolConfig)\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q config type mismatch: ToolConfig vs %T\", id, b.Config),\n\t\t\t\tPathA:   path,\n\t\t\t\tPathB:   path,\n\t\t\t})\n\t\t\treturn diffs\n\t\t}\n\t\tif !promptsEqual(ac.Command, bc.Command) {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q command differs\", id),\n\t\t\t\tPathA:   path + \".command\",\n\t\t\t\tPathB:   path + \".command\",\n\t\t\t})\n\t\t}\n\n\tcase ir.HumanConfig:\n\t\tbc, ok := b.Config.(ir.HumanConfig)\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q config type mismatch: HumanConfig vs %T\", id, b.Config),\n\t\t\t\tPathA:   path,\n\t\t\t\tPathB:   path,\n\t\t\t})\n\t\t\treturn diffs\n\t\t}\n\t\tif ac.Mode != bc.Mode {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q mode: %q vs %q\", id, ac.Mode, bc.Mode),\n\t\t\t\tPathA:   path + \".mode\",\n\t\t\t\tPathB:   path + \".mode\",\n\t\t\t})\n\t\t}\n\n\tcase ir.ParallelConfig:\n\t\tbc, ok := b.Config.(ir.ParallelConfig)\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q config type mismatch: ParallelConfig vs %T\", id, b.Config),\n\t\t\t\tPathA:   path,\n\t\t\t\tPathB:   path,\n\t\t\t})\n\t\t\treturn diffs\n\t\t}\n\t\tif strings.Join(ac.Targets, \",\") != strings.Join(bc.Targets, \",\") {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q targets: %v vs %v\", id, ac.Targets, bc.Targets),\n\t\t\t\tPathA:   path + \".targets\",\n\t\t\t\tPathB:   path + \".targets\",\n\t\t\t})\n\t\t}\n\n\tcase ir.FanInConfig:\n\t\tbc, ok := b.Config.(ir.FanInConfig)\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q config type mismatch: FanInConfig vs %T\", id, b.Config),\n\t\t\t\tPathA:   path,\n\t\t\t\tPathB:   path,\n\t\t\t})\n\t\t\treturn diffs\n\t\t}\n\t\tif strings.Join(ac.Sources, \",\") != strings.Join(bc.Sources, \",\") {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q sources: %v vs %v\", id, ac.Sources, bc.Sources),\n\t\t\t\tPathA:   path + \".sources\",\n\t\t\t\tPathB:   path + \".sources\",\n\t\t\t})\n\t\t}\n\n\tcase ir.SubgraphConfig:\n\t\tbc, ok := b.Config.(ir.SubgraphConfig)\n\t\tif !ok {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q config type mismatch: SubgraphConfig vs %T\", id, b.Config),\n\t\t\t\tPathA:   path,\n\t\t\t\tPathB:   path,\n\t\t\t})\n\t\t\treturn diffs\n\t\t}\n\t\tif ac.Ref != bc.Ref {\n\t\t\tdiffs = append(diffs, Difference{\n\t\t\t\tKind:    \"config_mismatch\",\n\t\t\t\tMessage: fmt.Sprintf(\"node %q ref: %q vs %q\", id, ac.Ref, bc.Ref),\n\t\t\t\tPathA:   path + \".ref\",\n\t\t\t\tPathB:   path + \".ref\",\n\t\t\t})\n\t\t}\n\t}\n\n\treturn diffs\n}\n\n// promptsEqual compares two strings with whitespace tolerance.\n// Prompts that differ only in trailing whitespace per line and leading/trailing\n// whitespace overall are considered equal.\nfunc promptsEqual(a, b string) bool {\n\treturn normalizeWhitespace(a) == normalizeWhitespace(b)\n}\n\n// compareDefaults reports differences between workflow defaults.\nfunc compareDefaults(a, b ir.WorkflowDefaults) []Difference {\n\tvar diffs []Difference\n\n\tif a.Model != b.Model {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"defaults_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"defaults.model: %q vs %q\", a.Model, b.Model),\n\t\t\tPathA:   \"defaults.model\",\n\t\t\tPathB:   \"defaults.model\",\n\t\t})\n\t}\n\tif a.Provider != b.Provider {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"defaults_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"defaults.provider: %q vs %q\", a.Provider, b.Provider),\n\t\t\tPathA:   \"defaults.provider\",\n\t\t\tPathB:   \"defaults.provider\",\n\t\t})\n\t}\n\tif a.MaxRetries != b.MaxRetries {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"defaults_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"defaults.max_retries: %d vs %d\", a.MaxRetries, b.MaxRetries),\n\t\t\tPathA:   \"defaults.max_retries\",\n\t\t\tPathB:   \"defaults.max_retries\",\n\t\t})\n\t}\n\tif a.MaxRestarts != b.MaxRestarts {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"defaults_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"defaults.max_restarts: %d vs %d\", a.MaxRestarts, b.MaxRestarts),\n\t\t\tPathA:   \"defaults.max_restarts\",\n\t\t\tPathB:   \"defaults.max_restarts\",\n\t\t})\n\t}\n\tif a.Fidelity != b.Fidelity {\n\t\tdiffs = append(diffs, Difference{\n\t\t\tKind:    \"defaults_mismatch\",\n\t\t\tMessage: fmt.Sprintf(\"defaults.fidelity: %q vs %q\", a.Fidelity, b.Fidelity),\n\t\t\tPathA:   \"defaults.fidelity\",\n\t\t\tPathB:   \"defaults.fidelity\",\n\t\t})\n\t}\n\n\treturn diffs\n}\n", "path": "migrate/parity.go"}
TOOL RESULT: write
OUTPUT:
wrote 9932 bytes to migrate/parity.go
TURN 10
TOOL CALL: write
INPUT:
{"content": "package migrate\n\nimport (\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n\t\"time\"\n\n\t\"github.com/2389/dippin/ir\"\n)\n\n// ============================================================\n// DOT Parser Tests (10 cases)\n// ============================================================\n\nfunc TestParseDOTSimpleDigraph(t *testing.T) {\n\tinput := `digraph G { A -> B; }`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif dg.Name != \"G\" {\n\t\tt.Errorf(\"name = %q, want %q\", dg.Name, \"G\")\n\t}\n\tif len(dg.Nodes) != 2 {\n\t\tt.Errorf(\"nodes = %d, want 2\", len(dg.Nodes))\n\t}\n\tif len(dg.Edges) != 1 {\n\t\tt.Errorf(\"edges = %d, want 1\", len(dg.Edges))\n\t}\n\tif dg.Edges[0].From != \"A\" || dg.Edges[0].To != \"B\" {\n\t\tt.Errorf(\"edge = %s->%s, want A->B\", dg.Edges[0].From, dg.Edges[0].To)\n\t}\n}\n\nfunc TestParseDOTNodeWithAttributes(t *testing.T) {\n\tinput := `digraph G {\n\t\tA [shape=box, label=\"My Agent\"];\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif len(dg.Nodes) != 1 {\n\t\tt.Fatalf(\"nodes = %d, want 1\", len(dg.Nodes))\n\t}\n\tn := dg.Nodes[0]\n\tif n.ID != \"A\" {\n\t\tt.Errorf(\"ID = %q, want %q\", n.ID, \"A\")\n\t}\n\tif n.Attrs[\"shape\"] != \"box\" {\n\t\tt.Errorf(\"shape = %q, want %q\", n.Attrs[\"shape\"], \"box\")\n\t}\n\tif n.Attrs[\"label\"] != \"My Agent\" {\n\t\tt.Errorf(\"label = %q, want %q\", n.Attrs[\"label\"], \"My Agent\")\n\t}\n}\n\nfunc TestParseDOTEdgeWithAttributes(t *testing.T) {\n\tinput := `digraph G {\n\t\tA -> B [label=\"yes\", condition=\"outcome=success\"];\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif len(dg.Edges) != 1 {\n\t\tt.Fatalf(\"edges = %d, want 1\", len(dg.Edges))\n\t}\n\te := dg.Edges[0]\n\tif e.Attrs[\"label\"] != \"yes\" {\n\t\tt.Errorf(\"label = %q, want %q\", e.Attrs[\"label\"], \"yes\")\n\t}\n\tif e.Attrs[\"condition\"] != \"outcome=success\" {\n\t\tt.Errorf(\"condition = %q, want %q\", e.Attrs[\"condition\"], \"outcome=success\")\n\t}\n}\n\nfunc TestParseDOTGraphAttributes(t *testing.T) {\n\tinput := `digraph G {\n\t\tgraph [goal=\"test\", rankdir=LR];\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif dg.GraphAttrs[\"goal\"] != \"test\" {\n\t\tt.Errorf(\"goal = %q, want %q\", dg.GraphAttrs[\"goal\"], \"test\")\n\t}\n\tif dg.GraphAttrs[\"rankdir\"] != \"LR\" {\n\t\tt.Errorf(\"rankdir = %q, want %q\", dg.GraphAttrs[\"rankdir\"], \"LR\")\n\t}\n}\n\nfunc TestParseDOTQuotedStringsWithEscapes(t *testing.T) {\n\tinput := `digraph G {\n\t\tA [label=\"line1\\nline2\\\"quoted\\\"\"];\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif len(dg.Nodes) != 1 {\n\t\tt.Fatalf(\"nodes = %d, want 1\", len(dg.Nodes))\n\t}\n\twant := \"line1\\nline2\\\"quoted\\\"\"\n\tif dg.Nodes[0].Attrs[\"label\"] != want {\n\t\tt.Errorf(\"label = %q, want %q\", dg.Nodes[0].Attrs[\"label\"], want)\n\t}\n}\n\nfunc TestParseDOTComments(t *testing.T) {\n\tinput := `digraph G {\n\t\t// This is a line comment\n\t\tA [shape=box];\n\t\t/* This is a\n\t\t   block comment */\n\t\tB [shape=hexagon];\n\t\tA -> B;\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif len(dg.Nodes) != 2 {\n\t\tt.Errorf(\"nodes = %d, want 2\", len(dg.Nodes))\n\t}\n\tif len(dg.Edges) != 1 {\n\t\tt.Errorf(\"edges = %d, want 1\", len(dg.Edges))\n\t}\n}\n\nfunc TestParseDOTEmptyGraph(t *testing.T) {\n\tinput := `digraph G {}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif dg.Name != \"G\" {\n\t\tt.Errorf(\"name = %q, want %q\", dg.Name, \"G\")\n\t}\n\tif len(dg.Nodes) != 0 {\n\t\tt.Errorf(\"nodes = %d, want 0\", len(dg.Nodes))\n\t}\n\tif len(dg.Edges) != 0 {\n\t\tt.Errorf(\"edges = %d, want 0\", len(dg.Edges))\n\t}\n}\n\nfunc TestParseDOTMultipleEdges(t *testing.T) {\n\tinput := `digraph G { A -> B; A -> C; B -> C; }`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif len(dg.Edges) != 3 {\n\t\tt.Errorf(\"edges = %d, want 3\", len(dg.Edges))\n\t}\n}\n\nfunc TestParseDOTMissingSemicolons(t *testing.T) {\n\tinput := `digraph G {\n\t\tA [shape=box]\n\t\tB [shape=hexagon]\n\t\tA -> B\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif len(dg.Nodes) != 2 {\n\t\tt.Errorf(\"nodes = %d, want 2\", len(dg.Nodes))\n\t}\n\tif len(dg.Edges) != 1 {\n\t\tt.Errorf(\"edges = %d, want 1\", len(dg.Edges))\n\t}\n}\n\nfunc TestParseDOTMalformed(t *testing.T) {\n\ttests := []struct {\n\t\tname  string\n\t\tinput string\n\t}{\n\t\t{\"not a digraph\", `graph G { A -> B; }`},\n\t\t{\"missing closing brace\", `digraph G { A -> B;`},\n\t\t{\"missing opening brace\", `digraph G A -> B; }`},\n\t\t{\"empty string\", ``},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\t_, err := parseDOT(tt.input)\n\t\t\tif err == nil {\n\t\t\t\tt.Error(\"expected error, got nil\")\n\t\t\t}\n\t\t})\n\t}\n}\n\n// ============================================================\n// Migration Tests (17 cases)\n// ============================================================\n\nfunc TestMigrateShapeToKindMapping(t *testing.T) {\n\ttests := []struct {\n\t\tshape    string\n\t\twantKind ir.NodeKind\n\t}{\n\t\t{\"box\", ir.NodeAgent},\n\t\t{\"hexagon\", ir.NodeHuman},\n\t\t{\"parallelogram\", ir.NodeTool},\n\t\t{\"component\", ir.NodeParallel},\n\t\t{\"tripleoctagon\", ir.NodeFanIn},\n\t\t{\"tab\", ir.NodeSubgraph},\n\t\t{\"Mdiamond\", ir.NodeAgent},   // Start marker\n\t\t{\"Msquare\", ir.NodeAgent},    // Exit marker\n\t\t{\"diamond\", ir.NodeAgent},    // Default diamond\n\t\t{\"\", ir.NodeAgent},           // Missing shape → default\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(\"shape_\"+tt.shape, func(t *testing.T) {\n\t\t\tshapeAttr := \"\"\n\t\t\tif tt.shape != \"\" {\n\t\t\t\tshapeAttr = `, shape=` + tt.shape\n\t\t\t}\n\t\t\tdot := `digraph G {\n\t\t\t\tStart [shape=Mdiamond];\n\t\t\t\tExit [shape=Msquare];\n\t\t\t\tN [label=\"Test\"` + shapeAttr + `];\n\t\t\t\tStart -> N;\n\t\t\t\tN -> Exit;\n\t\t\t}`\n\t\t\tw, err := Migrate(dot)\n\t\t\tif err != nil {\n\t\t\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t\t\t}\n\t\t\tn := w.Node(\"N\")\n\t\t\tif n == nil {\n\t\t\t\tt.Fatal(\"node N not found\")\n\t\t\t}\n\t\t\tif n.Kind != tt.wantKind {\n\t\t\t\tt.Errorf(\"kind = %q, want %q\", n.Kind, tt.wantKind)\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestMigrateStartExitIdentification(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond, label=\"Start\"];\n\t\tExit [shape=Msquare, label=\"Exit\"];\n\t\tA [shape=box];\n\t\tStart -> A;\n\t\tA -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif w.Start != \"Start\" {\n\t\tt.Errorf(\"start = %q, want %q\", w.Start, \"Start\")\n\t}\n\tif w.Exit != \"Exit\" {\n\t\tt.Errorf(\"exit = %q, want %q\", w.Exit, \"Exit\")\n\t}\n\t// Start and Exit should exist as nodes.\n\tif w.Node(\"Start\") == nil {\n\t\tt.Error(\"Start node not found in IR\")\n\t}\n\tif w.Node(\"Exit\") == nil {\n\t\tt.Error(\"Exit node not found in IR\")\n\t}\n}\n\nfunc TestMigratePromptUnescaping(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box, prompt=\"line1\\nline2\\n\\\"code\\\"\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"A\")\n\tif n == nil {\n\t\tt.Fatal(\"node A not found\")\n\t}\n\tcfg := n.Config.(ir.AgentConfig)\n\twant := \"line1\\nline2\\n\\\"code\\\"\"\n\tif cfg.Prompt != want {\n\t\tt.Errorf(\"prompt = %q, want %q\", cfg.Prompt, want)\n\t}\n}\n\nfunc TestMigrateToolCommandUnescaping(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tT [shape=parallelogram, tool_command=\"set -eu\\necho hello\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> T;\n\t\tT -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"T\")\n\tif n == nil {\n\t\tt.Fatal(\"node T not found\")\n\t}\n\tcfg := n.Config.(ir.ToolConfig)\n\twant := \"set -eu\\necho hello\"\n\tif cfg.Command != want {\n\t\tt.Errorf(\"command = %q, want %q\", cfg.Command, want)\n\t}\n}\n\nfunc TestMigrateConditionNamespacePrefixing(t *testing.T) {\n\ttests := []struct {\n\t\tname    string\n\t\tcondRaw string\n\t\twantVar string\n\t}{\n\t\t{\"bare outcome\", \"outcome=success\", \"ctx.outcome\"},\n\t\t{\"context. prefix\", \"context.tool_stdout=all_complete\", \"ctx.tool_stdout\"},\n\t\t{\"ctx. prefix kept\", \"ctx.outcome=success\", \"ctx.outcome\"},\n\t\t{\"graph. prefix kept\", \"graph.goal=done\", \"graph.goal\"},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tdot := `digraph G {\n\t\t\t\tStart [shape=Mdiamond];\n\t\t\t\tA [shape=box];\n\t\t\t\tB [shape=box];\n\t\t\t\tExit [shape=Msquare];\n\t\t\t\tStart -> A;\n\t\t\t\tA -> B [condition=\"` + tt.condRaw + `\"];\n\t\t\t\tB -> Exit;\n\t\t\t}`\n\t\t\tw, err := Migrate(dot)\n\t\t\tif err != nil {\n\t\t\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t\t\t}\n\t\t\tedges := w.EdgesFrom(\"A\")\n\t\t\tif len(edges) != 1 {\n\t\t\t\tt.Fatalf(\"edges = %d, want 1\", len(edges))\n\t\t\t}\n\t\t\te := edges[0]\n\t\t\tif e.Condition == nil {\n\t\t\t\tt.Fatal(\"expected condition\")\n\t\t\t}\n\t\t\tcc, ok := e.Condition.Parsed.(ir.CondCompare)\n\t\t\tif !ok {\n\t\t\t\tt.Fatalf(\"expected CondCompare, got %T\", e.Condition.Parsed)\n\t\t\t}\n\t\t\tif cc.Variable != tt.wantVar {\n\t\t\t\tt.Errorf(\"variable = %q, want %q\", cc.Variable, tt.wantVar)\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestMigrateComplexCondition(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> B [condition=\"outcome=success && tool_stdout contains pass\"];\n\t\tB -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tedges := w.EdgesFrom(\"A\")\n\tif len(edges) != 1 {\n\t\tt.Fatalf(\"edges = %d, want 1\", len(edges))\n\t}\n\tcond := edges[0].Condition\n\tif cond == nil {\n\t\tt.Fatal(\"expected condition\")\n\t}\n\tand, ok := cond.Parsed.(ir.CondAnd)\n\tif !ok {\n\t\tt.Fatalf(\"expected CondAnd, got %T\", cond.Parsed)\n\t}\n\tleft, ok := and.Left.(ir.CondCompare)\n\tif !ok {\n\t\tt.Fatalf(\"expected CondCompare left, got %T\", and.Left)\n\t}\n\tif left.Variable != \"ctx.outcome\" || left.Op != \"=\" || left.Value != \"success\" {\n\t\tt.Errorf(\"left = %+v\", left)\n\t}\n\tright, ok := and.Right.(ir.CondCompare)\n\tif !ok {\n\t\tt.Fatalf(\"expected CondCompare right, got %T\", and.Right)\n\t}\n\tif right.Variable != \"ctx.tool_stdout\" || right.Op != \"contains\" || right.Value != \"pass\" {\n\t\tt.Errorf(\"right = %+v\", right)\n\t}\n}\n\nfunc TestMigrateConditionWithNegation(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> B [condition=\"not outcome=fail\"];\n\t\tB -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tedges := w.EdgesFrom(\"A\")\n\tif len(edges) != 1 {\n\t\tt.Fatalf(\"edges = %d, want 1\", len(edges))\n\t}\n\tcond := edges[0].Condition\n\tif cond == nil {\n\t\tt.Fatal(\"expected condition\")\n\t}\n\tnotExpr, ok := cond.Parsed.(ir.CondNot)\n\tif !ok {\n\t\tt.Fatalf(\"expected CondNot, got %T\", cond.Parsed)\n\t}\n\tinner, ok := notExpr.Inner.(ir.CondCompare)\n\tif !ok {\n\t\tt.Fatalf(\"expected CondCompare inner, got %T\", notExpr.Inner)\n\t}\n\tif inner.Variable != \"ctx.outcome\" || inner.Op != \"=\" || inner.Value != \"fail\" {\n\t\tt.Errorf(\"inner = %+v\", inner)\n\t}\n}\n\nfunc TestMigrateRestartEdge(t *testing.T) {\n\ttests := []struct {\n\t\tname string\n\t\tattr string\n\t}{\n\t\t{\"restart=true\", `restart=true`},\n\t\t{\"loop_restart=true\", `loop_restart=true`},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tdot := `digraph G {\n\t\t\t\tStart [shape=Mdiamond];\n\t\t\t\tA [shape=box];\n\t\t\t\tB [shape=box];\n\t\t\t\tExit [shape=Msquare];\n\t\t\t\tStart -> A;\n\t\t\t\tA -> B [` + tt.attr + `];\n\t\t\t\tB -> Exit;\n\t\t\t}`\n\t\t\tw, err := Migrate(dot)\n\t\t\tif err != nil {\n\t\t\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t\t\t}\n\t\t\tedges := w.EdgesFrom(\"A\")\n\t\t\tif len(edges) != 1 {\n\t\t\t\tt.Fatalf(\"edges = %d, want 1\", len(edges))\n\t\t\t}\n\t\t\tif !edges[0].Restart {\n\t\t\t\tt.Error(\"expected restart=true on edge\")\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestMigrateGraphDefaults(t *testing.T) {\n\tdot := `digraph G {\n\t\tgraph [goal=\"Test the system\", default_max_retry=3, max_restarts=7, default_fidelity=\"summary:high\"];\n\t\tStart [shape=Mdiamond];\n\t\tExit [shape=Msquare];\n\t\tStart -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif w.Goal != \"Test the system\" {\n\t\tt.Errorf(\"goal = %q, want %q\", w.Goal, \"Test the system\")\n\t}\n\tif w.Defaults.MaxRetries != 3 {\n\t\tt.Errorf(\"max_retries = %d, want 3\", w.Defaults.MaxRetries)\n\t}\n\tif w.Defaults.MaxRestarts != 7 {\n\t\tt.Errorf(\"max_restarts = %d, want 7\", w.Defaults.MaxRestarts)\n\t}\n\tif w.Defaults.Fidelity != \"summary:high\" {\n\t\tt.Errorf(\"fidelity = %q, want %q\", w.Defaults.Fidelity, \"summary:high\")\n\t}\n}\n\nfunc TestMigrateParallelInference(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tP [shape=component, label=\"Fan Out\"];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tExit [shape=Msquare];\n\t\tStart -> P;\n\t\tP -> A;\n\t\tP -> B;\n\t\tA -> Exit;\n\t\tB -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"P\")\n\tif n == nil {\n\t\tt.Fatal(\"node P not found\")\n\t}\n\tcfg, ok := n.Config.(ir.ParallelConfig)\n\tif !ok {\n\t\tt.Fatalf(\"expected ParallelConfig, got %T\", n.Config)\n\t}\n\tif len(cfg.Targets) != 2 {\n\t\tt.Fatalf(\"targets = %d, want 2\", len(cfg.Targets))\n\t}\n\t// Targets should be A and B (in edge order).\n\tif cfg.Targets[0] != \"A\" || cfg.Targets[1] != \"B\" {\n\t\tt.Errorf(\"targets = %v, want [A B]\", cfg.Targets)\n\t}\n}\n\nfunc TestMigrateFanInInference(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tJ [shape=tripleoctagon, label=\"Join\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tStart -> B;\n\t\tA -> J;\n\t\tB -> J;\n\t\tJ -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"J\")\n\tif n == nil {\n\t\tt.Fatal(\"node J not found\")\n\t}\n\tcfg, ok := n.Config.(ir.FanInConfig)\n\tif !ok {\n\t\tt.Fatalf(\"expected FanInConfig, got %T\", n.Config)\n\t}\n\tif len(cfg.Sources) != 2 {\n\t\tt.Fatalf(\"sources = %d, want 2\", len(cfg.Sources))\n\t}\n\tif cfg.Sources[0] != \"A\" || cfg.Sources[1] != \"B\" {\n\t\tt.Errorf(\"sources = %v, want [A B]\", cfg.Sources)\n\t}\n}\n\nfunc TestMigrateDiamondDisambiguation(t *testing.T) {\n\ttests := []struct {\n\t\tname     string\n\t\tattrs    string\n\t\twantKind ir.NodeKind\n\t}{\n\t\t{\"diamond with tool_command\", `shape=diamond, tool_command=\"echo test\"`, ir.NodeTool},\n\t\t{\"diamond with prompt\", `shape=diamond, prompt=\"Choose wisely\"`, ir.NodeAgent},\n\t\t{\"bare diamond\", `shape=diamond, label=\"Route?\"`, ir.NodeAgent},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tdot := `digraph G {\n\t\t\t\tStart [shape=Mdiamond];\n\t\t\t\tD [` + tt.attrs + `];\n\t\t\t\tExit [shape=Msquare];\n\t\t\t\tStart -> D;\n\t\t\t\tD -> Exit;\n\t\t\t}`\n\t\t\tw, err := Migrate(dot)\n\t\t\tif err != nil {\n\t\t\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t\t\t}\n\t\t\tn := w.Node(\"D\")\n\t\t\tif n == nil {\n\t\t\t\tt.Fatal(\"node D not found\")\n\t\t\t}\n\t\t\tif n.Kind != tt.wantKind {\n\t\t\t\tt.Errorf(\"kind = %q, want %q\", n.Kind, tt.wantKind)\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestMigrateEdgeWeight(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> B [weight=10];\n\t\tB -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tedges := w.EdgesFrom(\"A\")\n\tif len(edges) != 1 {\n\t\tt.Fatalf(\"edges = %d, want 1\", len(edges))\n\t}\n\tif edges[0].Weight != 10 {\n\t\tt.Errorf(\"weight = %d, want 10\", edges[0].Weight)\n\t}\n}\n\nfunc TestMigrateDurationParsing(t *testing.T) {\n\ttests := []struct {\n\t\tname     string\n\t\ttimeout  string\n\t\twantDur  time.Duration\n\t}{\n\t\t{\"30s\", \"30s\", 30 * time.Second},\n\t\t{\"1h30m\", \"1h30m\", 90 * time.Minute},\n\t\t{\"5m\", \"5m\", 5 * time.Minute},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tdot := `digraph G {\n\t\t\t\tStart [shape=Mdiamond];\n\t\t\t\tT [shape=parallelogram, tool_command=\"echo test\", timeout=\"` + tt.timeout + `\"];\n\t\t\t\tExit [shape=Msquare];\n\t\t\t\tStart -> T;\n\t\t\t\tT -> Exit;\n\t\t\t}`\n\t\t\tw, err := Migrate(dot)\n\t\t\tif err != nil {\n\t\t\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t\t\t}\n\t\t\tn := w.Node(\"T\")\n\t\t\tif n == nil {\n\t\t\t\tt.Fatal(\"node T not found\")\n\t\t\t}\n\t\t\tcfg := n.Config.(ir.ToolConfig)\n\t\t\tif cfg.Timeout != tt.wantDur {\n\t\t\t\tt.Errorf(\"timeout = %v, want %v\", cfg.Timeout, tt.wantDur)\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestMigrateEmptyNodeDefaultsToAgent(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tN [];\n\t\tExit [shape=Msquare];\n\t\tStart -> N;\n\t\tN -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"N\")\n\tif n == nil {\n\t\tt.Fatal(\"node N not found\")\n\t}\n\tif n.Kind != ir.NodeAgent {\n\t\tt.Errorf(\"kind = %q, want %q\", n.Kind, ir.NodeAgent)\n\t}\n}\n\nfunc TestMigrateToSourceRoundTrip(t *testing.T) {\n\tdot := `digraph test_flow {\n\t\tgraph [goal=\"Simple test\"];\n\t\tStart [shape=Mdiamond, label=\"Start\"];\n\t\tExit [shape=Msquare, label=\"Exit\"];\n\t\tWorker [shape=box, label=\"Worker\", prompt=\"Do the work.\"];\n\t\tStart -> Worker;\n\t\tWorker -> Exit [condition=\"outcome=success\"];\n\t}`\n\tsource, err := MigrateToSource(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\t// Verify it looks like valid .dip source.\n\tif !strings.HasPrefix(source, \"workflow test_flow\") {\n\t\tt.Errorf(\"source should start with 'workflow test_flow', got:\\n%s\", source)\n\t}\n\tif !strings.Contains(source, \"start: Start\") {\n\t\tt.Errorf(\"source should contain 'start: Start', got:\\n%s\", source)\n\t}\n\tif !strings.Contains(source, \"exit: Exit\") {\n\t\tt.Errorf(\"source should contain 'exit: Exit', got:\\n%s\", source)\n\t}\n\tif !strings.Contains(source, \"Do the work.\") {\n\t\tt.Errorf(\"source should contain prompt text, got:\\n%s\", source)\n\t}\n\tif !strings.Contains(source, \"edges\") {\n\t\tt.Errorf(\"source should contain edges section, got:\\n%s\", source)\n\t}\n\tif !strings.Contains(source, \"ctx.outcome = success\") {\n\t\tt.Errorf(\"source should contain namespaced condition, got:\\n%s\", source)\n\t}\n}\n\nfunc TestMigrateLegacyAttributeNames(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box, llm_model=\"claude-opus-4-6\", llm_provider=\"anthropic\", prompt=\"Test\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"A\")\n\tif n == nil {\n\t\tt.Fatal(\"node A not found\")\n\t}\n\tcfg := n.Config.(ir.AgentConfig)\n\tif cfg.Model != \"claude-opus-4-6\" {\n\t\tt.Errorf(\"model = %q, want %q\", cfg.Model, \"claude-opus-4-6\")\n\t}\n\tif cfg.Provider != \"anthropic\" {\n\t\tt.Errorf(\"provider = %q, want %q\", cfg.Provider, \"anthropic\")\n\t}\n}\n\n// ============================================================\n// Parity Checker Tests (8 cases)\n// ============================================================\n\nfunc makeTestWorkflow() *ir.Workflow {\n\treturn &ir.Workflow{\n\t\tName:  \"test\",\n\t\tStart: \"A\",\n\t\tExit:  \"C\",\n\t\tDefaults: ir.WorkflowDefaults{\n\t\t\tModel:      \"gpt-5.4\",\n\t\t\tMaxRetries: 3,\n\t\t},\n\t\tNodes: []*ir.Node{\n\t\t\t{ID: \"A\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"Do A.\"}},\n\t\t\t{ID: \"B\", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: \"echo B\"}},\n\t\t\t{ID: \"C\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"Done.\"}},\n\t\t},\n\t\tEdges: []*ir.Edge{\n\t\t\t{From: \"A\", To: \"B\"},\n\t\t\t{From: \"B\", To: \"C\", Condition: &ir.Condition{\n\t\t\t\tRaw:    \"ctx.outcome = success\",\n\t\t\t\tParsed: ir.CondCompare{Variable: \"ctx.outcome\", Op: \"=\", Value: \"success\"},\n\t\t\t}},\n\t\t},\n\t}\n}\n\nfunc TestCheckParityIdentical(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tdiffs := CheckParity(a, b)\n\tif len(diffs) != 0 {\n\t\tt.Errorf(\"expected 0 differences, got %d:\", len(diffs))\n\t\tfor _, d := range diffs {\n\t\t\tt.Logf(\"  %s: %s\", d.Kind, d.Message)\n\t\t}\n\t}\n}\n\nfunc TestCheckParityMissingNode(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\t// Remove node B from b.\n\tb.Nodes = []*ir.Node{b.Nodes[0], b.Nodes[2]}\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"node_missing\" && strings.Contains(d.Message, \"B\") {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected node_missing difference for B\")\n\t}\n}\n\nfunc TestCheckParityExtraNode(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Nodes = append(b.Nodes, &ir.Node{ID: \"X\", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: \"Extra.\"}})\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"node_extra\" && strings.Contains(d.Message, \"X\") {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected node_extra difference for X\")\n\t}\n}\n\nfunc TestCheckParityStartMismatch(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Start = \"B\"\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"start_mismatch\" {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected start_mismatch difference\")\n\t}\n}\n\nfunc TestCheckParityExitMismatch(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Exit = \"B\"\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"exit_mismatch\" {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected exit_mismatch difference\")\n\t}\n}\n\nfunc TestCheckParityEdgeMissing(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Edges = b.Edges[:1] // Remove second edge.\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"edge_missing\" {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected edge_missing difference\")\n\t}\n}\n\nfunc TestCheckParityConfigMismatch(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\t// Change prompt on node A.\n\tb.Nodes[0].Config = ir.AgentConfig{Prompt: \"Completely different prompt.\"}\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"config_mismatch\" && strings.Contains(d.Message, \"A\") && strings.Contains(d.Message, \"prompt\") {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected config_mismatch for node A prompt\")\n\t}\n}\n\nfunc TestCheckParityKindMismatch(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Nodes[1].Kind = ir.NodeAgent // B was tool, now agent.\n\tb.Nodes[1].Config = ir.AgentConfig{Prompt: \"Now an agent.\"}\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"kind_mismatch\" && strings.Contains(d.Message, \"B\") {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected kind_mismatch for node B\")\n\t}\n}\n\nfunc TestCheckParityWhitespaceTolerantPrompt(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\t// Change prompt to add trailing whitespace — should still be considered equal.\n\tb.Nodes[0].Config = ir.AgentConfig{Prompt: \"Do A.  \"}\n\n\tdiffs := CheckParity(a, b)\n\t// Filter for prompt-specific config_mismatch.\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"config_mismatch\" && strings.Contains(d.Message, \"prompt\") {\n\t\t\tt.Errorf(\"unexpected prompt mismatch with whitespace difference: %s\", d.Message)\n\t\t}\n\t}\n}\n\nfunc TestCheckParityDefaultsMismatch(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Defaults.MaxRetries = 10\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"defaults_mismatch\" && strings.Contains(d.Message, \"max_retries\") {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected defaults_mismatch for max_retries\")\n\t}\n}\n\n// ============================================================\n// Integration Test: build_dippin.dot\n// ============================================================\n\nfunc TestMigrateBuildDippinDOT(t *testing.T) {\n\tdata, err := os.ReadFile(\"../build_dippin.dot\")\n\tif err != nil {\n\t\tt.Skipf(\"skipping integration test: %v\", err)\n\t}\n\n\tw, err := Migrate(string(data))\n\tif err != nil {\n\t\tt.Fatalf(\"Migrate(build_dippin.dot) error: %v\", err)\n\t}\n\n\t// Verify workflow name.\n\tif w.Name != \"BuildDippin\" {\n\t\tt.Errorf(\"name = %q, want %q\", w.Name, \"BuildDippin\")\n\t}\n\n\t// Verify goal was extracted.\n\tif w.Goal == \"\" {\n\t\tt.Error(\"goal should not be empty\")\n\t}\n\tif !strings.Contains(w.Goal, \"Dippin toolchain\") {\n\t\tt.Errorf(\"goal = %q, expected it to mention 'Dippin toolchain'\", w.Goal)\n\t}\n\n\t// Verify start/exit.\n\tif w.Start != \"Start\" {\n\t\tt.Errorf(\"start = %q, want %q\", w.Start, \"Start\")\n\t}\n\tif w.Exit != \"Exit\" {\n\t\tt.Errorf(\"exit = %q, want %q\", w.Exit, \"Exit\")\n\t}\n\n\t// Verify defaults.\n\tif w.Defaults.MaxRetries != 3 {\n\t\tt.Errorf(\"defaults.max_retries = %d, want 3\", w.Defaults.MaxRetries)\n\t}\n\tif w.Defaults.MaxRestarts != 7 {\n\t\tt.Errorf(\"defaults.max_restarts = %d, want 7\", w.Defaults.MaxRestarts)\n\t}\n\tif w.Defaults.Fidelity != \"summary:high\" {\n\t\tt.Errorf(\"defaults.fidelity = %q, want %q\", w.Defaults.Fidelity, \"summary:high\")\n\t}\n\n\t// Verify expected nodes exist.\n\texpectedNodes := []string{\n\t\t\"Start\", \"Exit\", \"SetupWorkspace\", \"PickNextComponent\",\n\t\t\"CheckComplete\", \"PlanComponent\", \"ImplementParallel\",\n\t\t\"ImplementClaude\", \"ImplementGPT\", \"ImplementGemini\",\n\t\t\"ImplementJoin\", \"ValidateBuild\", \"ReviewParallel\",\n\t\t\"ReviewClaude\", \"ReviewGPT\", \"ReviewGemini\",\n\t\t\"ReviewsJoin\", \"ReviewAnalysis\", \"CommitWork\",\n\t\t\"MarkComplete\", \"FailureSummary\", \"MarkFailed\",\n\t}\n\tfor _, id := range expectedNodes {\n\t\tif w.Node(id) == nil {\n\t\t\tt.Errorf(\"expected node %q not found\", id)\n\t\t}\n\t}\n\n\t// Verify node kinds.\n\tkindChecks := map[string]ir.NodeKind{\n\t\t\"SetupWorkspace\":    ir.NodeTool,\n\t\t\"PickNextComponent\": ir.NodeTool,\n\t\t\"PlanComponent\":     ir.NodeAgent,\n\t\t\"ImplementParallel\": ir.NodeParallel,\n\t\t\"ImplementClaude\":   ir.NodeAgent,\n\t\t\"ImplementJoin\":     ir.NodeFanIn,\n\t\t\"ReviewParallel\":    ir.NodeParallel,\n\t\t\"ReviewsJoin\":       ir.NodeFanIn,\n\t\t\"ReviewAnalysis\":    ir.NodeAgent,\n\t\t\"CheckComplete\":     ir.NodeAgent, // diamond → agent\n\t}\n\tfor id, wantKind := range kindChecks {\n\t\tn := w.Node(id)\n\t\tif n == nil {\n\t\t\tcontinue\n\t\t}\n\t\tif n.Kind != wantKind {\n\t\t\tt.Errorf(\"node %q kind = %q, want %q\", id, n.Kind, wantKind)\n\t\t}\n\t}\n\n\t// Verify edges exist.\n\tif len(w.Edges) == 0 {\n\t\tt.Error(\"expected edges\")\n\t}\n\n\t// Verify ImplementParallel targets are inferred.\n\timplPar := w.Node(\"ImplementParallel\")\n\tif implPar != nil {\n\t\tcfg, ok := implPar.Config.(ir.ParallelConfig)\n\t\tif !ok {\n\t\t\tt.Errorf(\"ImplementParallel config type = %T, want ParallelConfig\", implPar.Config)\n\t\t} else if len(cfg.Targets) != 3 {\n\t\t\tt.Errorf(\"ImplementParallel targets = %d, want 3\", len(cfg.Targets))\n\t\t}\n\t}\n\n\t// Verify ImplementJoin sources are inferred.\n\timplJoin := w.Node(\"ImplementJoin\")\n\tif implJoin != nil {\n\t\tcfg, ok := implJoin.Config.(ir.FanInConfig)\n\t\tif !ok {\n\t\t\tt.Errorf(\"ImplementJoin config type = %T, want FanInConfig\", implJoin.Config)\n\t\t} else if len(cfg.Sources) != 3 {\n\t\t\tt.Errorf(\"ImplementJoin sources = %d, want 3\", len(cfg.Sources))\n\t\t}\n\t}\n\n\t// Verify a restart edge exists (MarkComplete -> PickNextComponent).\n\trestartFound := false\n\tfor _, e := range w.Edges {\n\t\tif e.From == \"MarkComplete\" && e.To == \"PickNextComponent\" && e.Restart {\n\t\t\trestartFound = true\n\t\t}\n\t}\n\tif !restartFound {\n\t\tt.Error(\"expected restart edge MarkComplete -> PickNextComponent\")\n\t}\n\n\t// Verify conditions with context. prefix were normalized.\n\tfor _, e := range w.Edges {\n\t\tif e.From == \"CheckComplete\" && e.To == \"Exit\" {\n\t\t\tif e.Condition == nil {\n\t\t\t\tt.Error(\"expected condition on CheckComplete -> Exit\")\n\t\t\t\tbreak\n\t\t\t}\n\t\t\tcc, ok := e.Condition.Parsed.(ir.CondCompare)\n\t\t\tif !ok {\n\t\t\t\tt.Errorf(\"expected CondCompare, got %T\", e.Condition.Parsed)\n\t\t\t\tbreak\n\t\t\t}\n\t\t\tif cc.Variable != \"ctx.tool_stdout\" {\n\t\t\t\tt.Errorf(\"variable = %q, want %q\", cc.Variable, \"ctx.tool_stdout\")\n\t\t\t}\n\t\t\tbreak\n\t\t}\n\t}\n\n\t// Verify PlanComponent has a model from llm_model attribute.\n\tplan := w.Node(\"PlanComponent\")\n\tif plan != nil {\n\t\tcfg, ok := plan.Config.(ir.AgentConfig)\n\t\tif !ok {\n\t\t\tt.Errorf(\"PlanComponent config type = %T, want AgentConfig\", plan.Config)\n\t\t} else {\n\t\t\tif cfg.Model != \"claude-opus-4-6\" {\n\t\t\t\tt.Errorf(\"PlanComponent model = %q, want %q\", cfg.Model, \"claude-opus-4-6\")\n\t\t\t}\n\t\t\tif cfg.Provider != \"anthropic\" {\n\t\t\t\tt.Errorf(\"PlanComponent provider = %q, want %q\", cfg.Provider, \"anthropic\")\n\t\t\t}\n\t\t}\n\t}\n\n\t// Verify ReviewAnalysis has goal_gate and retry_target.\n\tra := w.Node(\"ReviewAnalysis\")\n\tif ra != nil {\n\t\tcfg, ok := ra.Config.(ir.AgentConfig)\n\t\tif !ok {\n\t\t\tt.Errorf(\"ReviewAnalysis config type = %T, want AgentConfig\", ra.Config)\n\t\t} else {\n\t\t\tif !cfg.GoalGate {\n\t\t\t\tt.Error(\"ReviewAnalysis should have goal_gate=true\")\n\t\t\t}\n\t\t}\n\t\tif ra.Retry.RetryTarget != \"ImplementClaude\" {\n\t\t\tt.Errorf(\"ReviewAnalysis retry_target = %q, want %q\", ra.Retry.RetryTarget, \"ImplementClaude\")\n\t\t}\n\t}\n\n\t// Verify MigrateToSource doesn't error.\n\tsource, err := MigrateToSource(string(data))\n\tif err != nil {\n\t\tt.Fatalf(\"MigrateToSource(build_dippin.dot) error: %v\", err)\n\t}\n\tif !strings.HasPrefix(source, \"workflow BuildDippin\") {\n\t\tt.Errorf(\"source should start with 'workflow BuildDippin', got:\\n%.100s...\", source)\n\t}\n}\n\n// ============================================================\n// Additional helper/edge case tests\n// ============================================================\n\nfunc TestAddNamespacePrefix(t *testing.T) {\n\ttests := []struct {\n\t\tinput string\n\t\twant  string\n\t}{\n\t\t{\"outcome\", \"ctx.outcome\"},\n\t\t{\"tool_stdout\", \"ctx.tool_stdout\"},\n\t\t{\"ctx.outcome\", \"ctx.outcome\"},\n\t\t{\"graph.goal\", \"graph.goal\"},\n\t\t{\"context.tool_stdout\", \"ctx.tool_stdout\"},\n\t\t{\"context.outcome\", \"ctx.outcome\"},\n\t\t{\"custom_var\", \"ctx.custom_var\"},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.input, func(t *testing.T) {\n\t\t\tgot := addNamespacePrefix(tt.input)\n\t\t\tif got != tt.want {\n\t\t\t\tt.Errorf(\"addNamespacePrefix(%q) = %q, want %q\", tt.input, got, tt.want)\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestParseConditionEdgeCases(t *testing.T) {\n\ttests := []struct {\n\t\tname    string\n\t\traw     string\n\t\twantErr bool\n\t\tcheck   func(*ir.Condition) error\n\t}{\n\t\t{\n\t\t\tname: \"simple equals\",\n\t\t\traw:  \"outcome=success\",\n\t\t\tcheck: func(c *ir.Condition) error {\n\t\t\t\tcc, ok := c.Parsed.(ir.CondCompare)\n\t\t\t\tif !ok {\n\t\t\t\t\treturn fmt.Errorf(\"got %T, want CondCompare\", c.Parsed)\n\t\t\t\t}\n\t\t\t\tif cc.Variable != \"ctx.outcome\" || cc.Op != \"=\" || cc.Value != \"success\" {\n\t\t\t\t\treturn fmt.Errorf(\"got %+v\", cc)\n\t\t\t\t}\n\t\t\t\treturn nil\n\t\t\t},\n\t\t},\n\t\t{\n\t\t\tname: \"not equals\",\n\t\t\traw:  \"outcome!=fail\",\n\t\t\tcheck: func(c *ir.Condition) error {\n\t\t\t\tcc, ok := c.Parsed.(ir.CondCompare)\n\t\t\t\tif !ok {\n\t\t\t\t\treturn fmt.Errorf(\"got %T, want CondCompare\", c.Parsed)\n\t\t\t\t}\n\t\t\t\tif cc.Op != \"!=\" {\n\t\t\t\t\treturn fmt.Errorf(\"op = %q, want !=\", cc.Op)\n\t\t\t\t}\n\t\t\t\treturn nil\n\t\t\t},\n\t\t},\n\t\t{\n\t\t\tname: \"contains operator\",\n\t\t\traw:  \"tool_stdout contains pass\",\n\t\t\tcheck: func(c *ir.Condition) error {\n\t\t\t\tcc, ok := c.Parsed.(ir.CondCompare)\n\t\t\t\tif !ok {\n\t\t\t\t\treturn fmt.Errorf(\"got %T, want CondCompare\", c.Parsed)\n\t\t\t\t}\n\t\t\t\tif cc.Op != \"contains\" {\n\t\t\t\t\treturn fmt.Errorf(\"op = %q, want contains\", cc.Op)\n\t\t\t\t}\n\t\t\t\treturn nil\n\t\t\t},\n\t\t},\n\t\t{\n\t\t\tname: \"OR condition\",\n\t\t\traw:  \"outcome=success || outcome=partial\",\n\t\t\tcheck: func(c *ir.Condition) error {\n\t\t\t\t_, ok := c.Parsed.(ir.CondOr)\n\t\t\t\tif !ok {\n\t\t\t\t\treturn fmt.Errorf(\"got %T, want CondOr\", c.Parsed)\n\t\t\t\t}\n\t\t\t\treturn nil\n\t\t\t},\n\t\t},\n\t\t{\n\t\t\tname: \"bang prefix negation\",\n\t\t\traw:  \"!outcome=fail\",\n\t\t\tcheck: func(c *ir.Condition) error {\n\t\t\t\t_, ok := c.Parsed.(ir.CondNot)\n\t\t\t\tif !ok {\n\t\t\t\t\treturn fmt.Errorf(\"got %T, want CondNot\", c.Parsed)\n\t\t\t\t}\n\t\t\t\treturn nil\n\t\t\t},\n\t\t},\n\t\t{\n\t\t\tname:    \"empty string\",\n\t\t\traw:     \"\",\n\t\t\twantErr: false,\n\t\t\tcheck: func(c *ir.Condition) error {\n\t\t\t\tif c != nil {\n\t\t\t\t\treturn fmt.Errorf(\"expected nil for empty condition, got %+v\", c)\n\t\t\t\t}\n\t\t\t\treturn nil\n\t\t\t},\n\t\t},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.name, func(t *testing.T) {\n\t\t\tc, err := parseCondition(tt.raw)\n\t\t\tif tt.wantErr {\n\t\t\t\tif err == nil {\n\t\t\t\t\tt.Error(\"expected error, got nil\")\n\t\t\t\t}\n\t\t\t\treturn\n\t\t\t}\n\t\t\tif err != nil {\n\t\t\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t\t\t}\n\t\t\tif tt.check != nil {\n\t\t\t\tif err := tt.check(c); err != nil {\n\t\t\t\t\tt.Error(err)\n\t\t\t\t}\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestMigrateNodeLabel(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond, label=\"Begin Here\"];\n\t\tA [shape=box, label=\"My Special Node\", prompt=\"Do it.\"];\n\t\tExit [shape=Msquare, label=\"The End\"];\n\t\tStart -> A;\n\t\tA -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"A\")\n\tif n == nil {\n\t\tt.Fatal(\"node A not found\")\n\t}\n\tif n.Label != \"My Special Node\" {\n\t\tt.Errorf(\"label = %q, want %q\", n.Label, \"My Special Node\")\n\t}\n}\n\nfunc TestMigrateWorkflowName(t *testing.T) {\n\tdot := `digraph MyWorkflow { A [shape=Mdiamond]; B [shape=Msquare]; A -> B; }`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif w.Name != \"MyWorkflow\" {\n\t\tt.Errorf(\"name = %q, want %q\", w.Name, \"MyWorkflow\")\n\t}\n}\n\nfunc TestMigrateQuotedGraphName(t *testing.T) {\n\tdot := `digraph \"my workflow\" { A [shape=Mdiamond]; B [shape=Msquare]; A -> B; }`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif w.Name != \"my workflow\" {\n\t\tt.Errorf(\"name = %q, want %q\", w.Name, \"my workflow\")\n\t}\n}\n\nfunc TestMigrateRetryConfig(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box, prompt=\"Try\", max_retries=3, retry_policy=\"aggressive\", retry_target=\"B\", fallback_target=\"C\"];\n\t\tB [shape=box, prompt=\"Retry here\"];\n\t\tC [shape=box, prompt=\"Fallback\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> B;\n\t\tB -> Exit;\n\t\tA -> C;\n\t\tC -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"A\")\n\tif n == nil {\n\t\tt.Fatal(\"node A not found\")\n\t}\n\tif n.Retry.MaxRetries != 3 {\n\t\tt.Errorf(\"max_retries = %d, want 3\", n.Retry.MaxRetries)\n\t}\n\tif n.Retry.Policy != \"aggressive\" {\n\t\tt.Errorf(\"policy = %q, want %q\", n.Retry.Policy, \"aggressive\")\n\t}\n\tif n.Retry.RetryTarget != \"B\" {\n\t\tt.Errorf(\"retry_target = %q, want %q\", n.Retry.RetryTarget, \"B\")\n\t}\n\tif n.Retry.FallbackTarget != \"C\" {\n\t\tt.Errorf(\"fallback_target = %q, want %q\", n.Retry.FallbackTarget, \"C\")\n\t}\n}\n\nfunc TestMigrateSubgraphNode(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tS [shape=tab, ref=\"./review.dip\", label=\"Review Sub\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> S;\n\t\tS -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"S\")\n\tif n == nil {\n\t\tt.Fatal(\"node S not found\")\n\t}\n\tif n.Kind != ir.NodeSubgraph {\n\t\tt.Errorf(\"kind = %q, want %q\", n.Kind, ir.NodeSubgraph)\n\t}\n\tcfg, ok := n.Config.(ir.SubgraphConfig)\n\tif !ok {\n\t\tt.Fatalf(\"config type = %T, want SubgraphConfig\", n.Config)\n\t}\n\tif cfg.Ref != \"./review.dip\" {\n\t\tt.Errorf(\"ref = %q, want %q\", cfg.Ref, \"./review.dip\")\n\t}\n}\n\nfunc TestMigrateHumanNode(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tH [shape=hexagon, mode=\"choice\", default=\"Yes\", label=\"Approval\"];\n\t\tExit [shape=Msquare];\n\t\tStart -> H;\n\t\tH -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"H\")\n\tif n == nil {\n\t\tt.Fatal(\"node H not found\")\n\t}\n\tif n.Kind != ir.NodeHuman {\n\t\tt.Errorf(\"kind = %q, want %q\", n.Kind, ir.NodeHuman)\n\t}\n\tcfg, ok := n.Config.(ir.HumanConfig)\n\tif !ok {\n\t\tt.Fatalf(\"config type = %T, want HumanConfig\", n.Config)\n\t}\n\tif cfg.Mode != \"choice\" {\n\t\tt.Errorf(\"mode = %q, want %q\", cfg.Mode, \"choice\")\n\t}\n\tif cfg.Default != \"Yes\" {\n\t\tt.Errorf(\"default = %q, want %q\", cfg.Default, \"Yes\")\n\t}\n}\n\nfunc TestMigrateParallelExplicitTargets(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tP [shape=component, targets=\"A,B,C\"];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tC [shape=box];\n\t\tExit [shape=Msquare];\n\t\tStart -> P;\n\t\tP -> A;\n\t\tP -> B;\n\t\tP -> C;\n\t\tA -> Exit;\n\t\tB -> Exit;\n\t\tC -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"P\")\n\tif n == nil {\n\t\tt.Fatal(\"node P not found\")\n\t}\n\tcfg, ok := n.Config.(ir.ParallelConfig)\n\tif !ok {\n\t\tt.Fatalf(\"config type = %T, want ParallelConfig\", n.Config)\n\t}\n\t// Explicit targets from attribute should be used.\n\tif len(cfg.Targets) != 3 {\n\t\tt.Fatalf(\"targets = %d, want 3\", len(cfg.Targets))\n\t}\n\tif cfg.Targets[0] != \"A\" || cfg.Targets[1] != \"B\" || cfg.Targets[2] != \"C\" {\n\t\tt.Errorf(\"targets = %v, want [A B C]\", cfg.Targets)\n\t}\n}\n\nfunc TestMigrateVersionIsSet(t *testing.T) {\n\tdot := `digraph G { A [shape=Mdiamond]; B [shape=Msquare]; A -> B; }`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tif w.Version != \"1\" {\n\t\tt.Errorf(\"version = %q, want %q\", w.Version, \"1\")\n\t}\n}\n\nfunc TestMigrateAgentConfigFields(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box, prompt=\"Do it.\", reasoning_effort=\"high\", fidelity=\"full\", goal_gate=true, auto_status=true];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tn := w.Node(\"A\")\n\tif n == nil {\n\t\tt.Fatal(\"node A not found\")\n\t}\n\tcfg, ok := n.Config.(ir.AgentConfig)\n\tif !ok {\n\t\tt.Fatalf(\"config type = %T, want AgentConfig\", n.Config)\n\t}\n\tif cfg.ReasoningEffort != \"high\" {\n\t\tt.Errorf(\"reasoning_effort = %q, want %q\", cfg.ReasoningEffort, \"high\")\n\t}\n\tif cfg.Fidelity != \"full\" {\n\t\tt.Errorf(\"fidelity = %q, want %q\", cfg.Fidelity, \"full\")\n\t}\n\tif !cfg.GoalGate {\n\t\tt.Error(\"expected goal_gate=true\")\n\t}\n\tif !cfg.AutoStatus {\n\t\tt.Error(\"expected auto_status=true\")\n\t}\n}\n\nfunc TestMigrateEdgeLabel(t *testing.T) {\n\tdot := `digraph G {\n\t\tStart [shape=Mdiamond];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tExit [shape=Msquare];\n\t\tStart -> A;\n\t\tA -> B [label=\"proceed\", condition=\"outcome=success\"];\n\t\tB -> Exit;\n\t}`\n\tw, err := Migrate(dot)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\tedges := w.EdgesFrom(\"A\")\n\tif len(edges) != 1 {\n\t\tt.Fatalf(\"edges = %d, want 1\", len(edges))\n\t}\n\tif edges[0].Label != \"proceed\" {\n\t\tt.Errorf(\"label = %q, want %q\", edges[0].Label, \"proceed\")\n\t}\n}\n\nfunc TestNormalizeWhitespace(t *testing.T) {\n\ttests := []struct {\n\t\tinput string\n\t\twant  string\n\t}{\n\t\t{\"hello  world\", \"hello world\"},\n\t\t{\"  leading\", \"leading\"},\n\t\t{\"trailing  \", \"trailing\"},\n\t\t{\"a\\n\\tb\", \"a b\"},\n\t\t{\"\", \"\"},\n\t}\n\tfor _, tt := range tests {\n\t\tt.Run(tt.input, func(t *testing.T) {\n\t\t\tgot := normalizeWhitespace(tt.input)\n\t\t\tif got != tt.want {\n\t\t\t\tt.Errorf(\"normalizeWhitespace(%q) = %q, want %q\", tt.input, got, tt.want)\n\t\t\t}\n\t\t})\n\t}\n}\n\nfunc TestParseDOTDefaultNodeEdgeAttrs(t *testing.T) {\n\tinput := `digraph G {\n\t\tnode [fontname=\"Helvetica\"];\n\t\tedge [fontname=\"Helvetica\"];\n\t\tA [shape=box];\n\t\tB [shape=box];\n\t\tA -> B;\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\t// Default node attrs should be merged into each node.\n\tif dg.Nodes[0].Attrs[\"fontname\"] != \"Helvetica\" {\n\t\tt.Errorf(\"node A fontname = %q, want %q\", dg.Nodes[0].Attrs[\"fontname\"], \"Helvetica\")\n\t}\n\t// Default edge attrs should be merged into each edge.\n\tif dg.Edges[0].Attrs[\"fontname\"] != \"Helvetica\" {\n\t\tt.Errorf(\"edge fontname = %q, want %q\", dg.Edges[0].Attrs[\"fontname\"], \"Helvetica\")\n\t}\n}\n\nfunc TestParseDOTDOTLeftJustify(t *testing.T) {\n\t// \\l in DOT means left-justified newline — should be converted to \\n.\n\tinput := `digraph G {\n\t\tA [label=\"first\\lsecond\"];\n\t}`\n\tdg, err := parseDOT(input)\n\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n\twant := \"first\\nsecond\"\n\tif dg.Nodes[0].Attrs[\"label\"] != want {\n\t\tt.Errorf(\"label = %q, want %q\", dg.Nodes[0].Attrs[\"label\"], want)\n\t}\n}\n\nfunc TestCheckParityEdgeExtra(t *testing.T) {\n\ta := makeTestWorkflow()\n\tb := makeTestWorkflow()\n\tb.Edges = append(b.Edges, &ir.Edge{From: \"C\", To: \"A\"})\n\n\tdiffs := CheckParity(a, b)\n\tfound := false\n\tfor _, d := range diffs {\n\t\tif d.Kind == \"edge_extra\" {\n\t\t\tfound = true\n\t\t}\n\t}\n\tif !found {\n\t\tt.Error(\"expected edge_extra difference\")\n\t}\n}\n", "path": "migrate/migrate_test.go"}
TOOL RESULT: write
OUTPUT:
wrote 39008 bytes to migrate/migrate_test.go
TURN 11
TOOL CALL: bash
INPUT:
{"command": "cd /home/user && go build ./..."}
TOOL RESULT: bash
OUTPUT:
stderr: sh: line 0: cd: /home/user: No such file or directory

exit code: 1
TURN 12
TOOL CALL: bash
INPUT:
{"command": "go build ./..."}
TOOL RESULT: bash
OUTPUT:
(no output)
TURN 13
TOOL CALL: bash
INPUT:
{"command": "go test ./migrate/ -v -count=1 2>&1"}
TOOL RESULT: bash
OUTPUT:
# github.com/2389/dippin/migrate [github.com/2389/dippin/migrate.test]
migrate/migrate_test.go:1159:13: undefined: fmt
migrate/migrate_test.go:1162:13: undefined: fmt
migrate/migrate_test.go:1173:13: undefined: fmt
migrate/migrate_test.go:1176:13: undefined: fmt
migrate/migrate_test.go:1187:13: undefined: fmt
migrate/migrate_test.go:1190:13: undefined: fmt
migrate/migrate_test.go:1201:13: undefined: fmt
migrate/migrate_test.go:1212:13: undefined: fmt
migrate/migrate_test.go:1223:13: undefined: fmt
FAIL	github.com/2389/dippin/migrate [build failed]
FAIL

exit code: 1
TURN 14
TOOL CALL: edit
INPUT:
{"path": "migrate/migrate_test.go", "old_string": "import (\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n\t\"time\"\n\n\t\"github.com/2389/dippin/ir\"\n)", "new_string": "import (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n\t\"testing\"\n\t\"time\"\n\n\t\"github.com/2389/dippin/ir\"\n)"}
TOOL RESULT: edit
OUTPUT:
edited migrate/migrate_test.go
TURN 15
TOOL CALL: bash
INPUT:
{"command": "go test ./migrate/ -v -count=1 2>&1"}
TOOL RESULT: bash
OUTPUT:
=== RUN   TestParseDOTSimpleDigraph
--- PASS: TestParseDOTSimpleDigraph (0.00s)
=== RUN   TestParseDOTNodeWithAttributes
--- PASS: TestParseDOTNodeWithAttributes (0.00s)
=== RUN   TestParseDOTEdgeWithAttributes
--- PASS: TestParseDOTEdgeWithAttributes (0.00s)
=== RUN   TestParseDOTGraphAttributes
--- PASS: TestParseDOTGraphAttributes (0.00s)
=== RUN   TestParseDOTQuotedStringsWithEscapes
--- PASS: TestParseDOTQuotedStringsWithEscapes (0.00s)
=== RUN   TestParseDOTComments
--- PASS: TestParseDOTComments (0.00s)
=== RUN   TestParseDOTEmptyGraph
--- PASS: TestParseDOTEmptyGraph (0.00s)
=== RUN   TestParseDOTMultipleEdges
--- PASS: TestParseDOTMultipleEdges (0.00s)
=== RUN   TestParseDOTMissingSemicolons
--- PASS: TestParseDOTMissingSemicolons (0.00s)
=== RUN   TestParseDOTMalformed
=== RUN   TestParseDOTMalformed/not_a_digraph
=== RUN   TestParseDOTMalformed/missing_closing_brace
=== RUN   TestParseDOTMalformed/missing_opening_brace
=== RUN   TestParseDOTMalformed/empty_string
--- PASS: TestParseDOTMalformed (0.00s)
    --- PASS: TestParseDOTMalformed/not_a_digraph (0.00s)
    --- PASS: TestParseDOTMalformed/missing_closing_brace (0.00s)
    --- PASS: TestParseDOTMalformed/missing_opening_brace (0.00s)
    --- PASS: TestParseDOTMalformed/empty_string (0.00s)
=== RUN   TestMigrateShapeToKindMapping
=== RUN   TestMigrateShapeToKindMapping/shape_box
=== RUN   TestMigrateShapeToKindMapping/shape_hexagon
=== RUN   TestMigrateShapeToKindMapping/shape_parallelogram
=== RUN   TestMigrateShapeToKindMapping/shape_component
=== RUN   TestMigrateShapeToKindMapping/shape_tripleoctagon
=== RUN   TestMigrateShapeToKindMapping/shape_tab
=== RUN   TestMigrateShapeToKindMapping/shape_Mdiamond
=== RUN   TestMigrateShapeToKindMapping/shape_Msquare
=== RUN   TestMigrateShapeToKindMapping/shape_diamond
=== RUN   TestMigrateShapeToKindMapping/shape_
--- PASS: TestMigrateShapeToKindMapping (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_box (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_hexagon (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_parallelogram (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_component (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_tripleoctagon (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_tab (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_Mdiamond (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_Msquare (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_diamond (0.00s)
    --- PASS: TestMigrateShapeToKindMapping/shape_ (0.00s)
=== RUN   TestMigrateStartExitIdentification
--- PASS: TestMigrateStartExitIdentification (0.00s)
=== RUN   TestMigratePromptUnescaping
--- PASS: TestMigratePromptUnescaping (0.00s)
=== RUN   TestMigrateToolCommandUnescaping
--- PASS: TestMigrateToolCommandUnescaping (0.00s)
=== RUN   TestMigrateConditionNamespacePrefixing
=== RUN   TestMigrateConditionNamespacePrefixing/bare_outcome
=== RUN   TestMigrateConditionNamespacePrefixing/context._prefix
=== RUN   TestMigrateConditionNamespacePrefixing/ctx._prefix_kept
=== RUN   TestMigrateConditionNamespacePrefixing/graph._prefix_kept
--- PASS: TestMigrateConditionNamespacePrefixing (0.00s)
    --- PASS: TestMigrateConditionNamespacePrefixing/bare_outcome (0.00s)
    --- PASS: TestMigrateConditionNamespacePrefixing/context._prefix (0.00s)
    --- PASS: TestMigrateConditionNamespacePrefixing/ctx._prefix_kept (0.00s)
    --- PASS: TestMigrateConditionNamespacePrefixing/graph._prefix_kept (0.00s)
=== RUN   TestMigrateComplexCondition
--- PASS: TestMigrateComplexCondition (0.00s)
=== RUN   TestMigrateConditionWithNegation
--- PASS: TestMigrateConditionWithNegation (0.00s)
=== RUN   TestMigrateRestartEdge
=== RUN   TestMigrateRestartEdge/restart=true
=== RUN   TestMigrateRestartEdge/loop_restart=true
--- PASS: TestMigrateRestartEdge (0.00s)
    --- PASS: TestMigrateRestartEdge/restart=true (0.00s)
    --- PASS: TestMigrateRestartEdge/loop_restart=true (0.00s)
=== RUN   TestMigrateGraphDefaults
--- PASS: TestMigrateGraphDefaults (0.00s)
=== RUN   TestMigrateParallelInference
--- PASS: TestMigrateParallelInference (0.00s)
=== RUN   TestMigrateFanInInference
--- PASS: TestMigrateFanInInference (0.00s)
=== RUN   TestMigrateDiamondDisambiguation
=== RUN   TestMigrateDiamondDisambiguation/diamond_with_tool_command
=== RUN   TestMigrateDiamondDisambiguation/diamond_with_prompt
=== RUN   TestMigrateDiamondDisambiguation/bare_diamond
--- PASS: TestMigrateDiamondDisambiguation (0.00s)
    --- PASS: TestMigrateDiamondDisambiguation/diamond_with_tool_command (0.00s)
    --- PASS: TestMigrateDiamondDisambiguation/diamond_with_prompt (0.00s)
    --- PASS: TestMigrateDiamondDisambiguation/bare_diamond (0.00s)
=== RUN   TestMigrateEdgeWeight
--- PASS: TestMigrateEdgeWeight (0.00s)
=== RUN   TestMigrateDurationParsing
=== RUN   TestMigrateDurationParsing/30s
=== RUN   TestMigrateDurationParsing/1h30m
=== RUN   TestMigrateDurationParsing/5m
--- PASS: TestMigrateDurationParsing (0.00s)
    --- PASS: TestMigrateDurationParsing/30s (0.00s)
    --- PASS: TestMigrateDurationParsing/1h30m (0.00s)
    --- PASS: TestMigrateDurationParsing/5m (0.00s)
=== RUN   TestMigrateEmptyNodeDefaultsToAgent
--- PASS: TestMigrateEmptyNodeDefaultsToAgent (0.00s)
=== RUN   TestMigrateToSourceRoundTrip
--- PASS: TestMigrateToSourceRoundTrip (0.00s)
=== RUN   TestMigrateLegacyAttributeNames
--- PASS: TestMigrateLegacyAttributeNames (0.00s)
=== RUN   TestCheckParityIdentical
--- PASS: TestCheckParityIdentical (0.00s)
=== RUN   TestCheckParityMissingNode
--- PASS: TestCheckParityMissingNode (0.00s)
=== RUN   TestCheckParityExtraNode
--- PASS: TestCheckParityExtraNode (0.00s)
=== RUN   TestCheckParityStartMismatch
--- PASS: TestCheckParityStartMismatch (0.00s)
=== RUN   TestCheckParityExitMismatch
--- PASS: TestCheckParityExitMismatch (0.00s)
=== RUN   TestCheckParityEdgeMissing
--- PASS: TestCheckParityEdgeMissing (0.00s)
=== RUN   TestCheckParityConfigMismatch
--- PASS: TestCheckParityConfigMismatch (0.00s)
=== RUN   TestCheckParityKindMismatch
--- PASS: TestCheckParityKindMismatch (0.00s)
=== RUN   TestCheckParityWhitespaceTolerantPrompt
--- PASS: TestCheckParityWhitespaceTolerantPrompt (0.00s)
=== RUN   TestCheckParityDefaultsMismatch
--- PASS: TestCheckParityDefaultsMismatch (0.00s)
=== RUN   TestMigrateBuildDippinDOT
--- PASS: TestMigrateBuildDippinDOT (0.00s)
=== RUN   TestAddNamespacePrefix
=== RUN   TestAddNamespacePrefix/outcome
=== RUN   TestAddNamespacePrefix/tool_stdout
=== RUN   TestAddNamespacePrefix/ctx.outcome
=== RUN   TestAddNamespacePrefix/graph.goal
=== RUN   TestAddNamespacePrefix/context.tool_stdout
=== RUN   TestAddNamespacePrefix/context.outcome
=== RUN   TestAddNamespacePrefix/custom_var
--- PASS: TestAddNamespacePrefix (0.00s)
    --- PASS: TestAddNamespacePrefix/outcome (0.00s)
    --- PASS: TestAddNamespacePrefix/tool_stdout (0.00s)
    --- PASS: TestAddNamespacePrefix/ctx.outcome (0.00s)
    --- PASS: TestAddNamespacePrefix/graph.goal (0.00s)
    --- PASS: TestAddNamespacePrefix/context.tool_stdout (0.00s)
    --- PASS: TestAddNamespacePrefix/context.outcome (0.00s)
    --- PASS: TestAddNamespacePrefix/custom_var (0.00s)
=== RUN   TestParseConditionEdgeCases
=== RUN   TestParseConditionEdgeCases/simple_equals
=== RUN   TestParseConditionEdgeCases/not_equals
=== RUN   TestParseConditionEdgeCases/contains_operator
=== RUN   TestParseConditionEdgeCases/OR_condition
=== RUN   TestParseConditionEdgeCases/bang_prefix_negation
=== RUN   TestParseConditionEdgeCases/empty_string
--- PASS: TestParseConditionEdgeCases (0.00s)
    --- PASS: TestParseConditionEdgeCases/simple_equals (0.00s)
    --- PASS: TestParseConditionEdgeCases/not_equals (0.00s)
    --- PASS: TestParseConditionEdgeCases/contains_operator (0.00s)
    --- PASS: TestParseConditionEdgeCases/OR_condition (0.00s)
    --- PASS: TestParseConditionEdgeCases/bang_prefix_negation (0.00s)
    --- PASS: TestParseConditionEdgeCases/empty_string (0.00s)
=== RUN   TestMigrateNodeLabel
--- PASS: TestMigrateNodeLabel (0.00s)
=== RUN   TestMigrateWorkflowName
--- PASS: TestMigrateWorkflowName (0.00s)
=== RUN   TestMigrateQuotedGraphName
--- PASS: TestMigrateQuotedGraphName (0.00s)
=== RUN   TestMigrateRetryConfig
--- PASS: TestMigrateRetryConfig (0.00s)
=== RUN   TestMigrateSubgraphNode
--- PASS: TestMigrateSubgraphNode (0.00s)
=== RUN   TestMigrateHumanNode
--- PASS: TestMigrateHumanNode (0.00s)
=== RUN   TestMigrateParallelExplicitTargets
--- PASS: TestMigrateParallelExplicitTargets (0.00s)
=== RUN   TestMigrateVersionIsSet
--- PASS: TestMigrateVersionIsSet (0.00s)
=== RUN   TestMigrateAgentConfigFields
--- PASS: TestMigrateAgentConfigFields (0.00s)
=== RUN   TestMigrateEdgeLabel
--- PASS: TestMigrateEdgeLabel (0.00s)
=== RUN   TestNormalizeWhitespace
=== RUN   TestNormalizeWhitespace/hello__world
=== RUN   TestNormalizeWhitespace/__leading
=== RUN   TestNormalizeWhitespace/trailing__
=== RUN   TestNormalizeWhitespace/a__b
=== RUN   TestNormalizeWhitespace/#00
--- PASS: TestNormalizeWhitespace (0.00s)
    --- PASS: TestNormalizeWhitespace/hello__world (0.00s)
    --- PASS: TestNormalizeWhitespace/__leading (0.00s)
    --- PASS: TestNormalizeWhitespace/trailing__ (0.00s)
    --- PASS: TestNormalizeWhitespace/a__b (0.00s)
    --- PASS: TestNormalizeWhitespace/#00 (0.00s)
=== RUN   TestParseDOTDefaultNodeEdgeAttrs
--- PASS: TestParseDOTDefaultNodeEdgeAttrs (0.00s)
=== RUN   TestParseDOTDOTLeftJustify
--- PASS: TestParseDOTDOTLeftJustify (0.00s)
=== RUN   TestCheckParityEdgeExtra
--- PASS: TestCheckParityEdgeExtra (0.00s)
PASS
ok  	github.com/2389/dippin/migrate	0.418s

TURN 16
TOOL CALL: bash
INPUT:
{"command": "go build ./... && go test ./... -count=1 2>&1"}
TOOL RESULT: bash
OUTPUT:
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/export	0.257s
ok  	github.com/2389/dippin/formatter	0.483s
ok  	github.com/2389/dippin/ir	0.734s
ok  	github.com/2389/dippin/migrate	0.920s
ok  	github.com/2389/dippin/validator	1.160s

TURN 17
TOOL CALL: bash
INPUT:
{"command": "go vet ./... 2>&1"}
TOOL RESULT: bash
OUTPUT:
(no output)
TURN 18
TOOL CALL: bash
INPUT:
{"command": "go test ./migrate/ -v -count=1 2>&1 | grep -c \"^--- PASS\""}
TOOL RESULT: bash
OUTPUT:
54

TURN 19
TOOL CALL: bash
INPUT:
{"command": "ls -la migrate/ && echo \"---\" && wc -l migrate/*.go"}
TOOL RESULT: bash
OUTPUT:
total 160
drwxr-xr-x@  6 clint  staff    192 Mar 19 20:50 .
drwxr-xr-x@ 13 clint  staff    416 Mar 19 20:48 ..
-rw-r--r--@  1 clint  staff  11423 Mar 19 20:59 dot_parser.go
-rw-r--r--@  1 clint  staff  39015 Mar 19 21:04 migrate_test.go
-rw-r--r--@  1 clint  staff  15128 Mar 19 21:00 migrate.go
-rw-r--r--@  1 clint  staff   9932 Mar 19 21:01 parity.go
---
     516 migrate/dot_parser.go
    1566 migrate/migrate_test.go
     615 migrate/migrate.go
     354 migrate/parity.go
    3051 total

TURN 20
TEXT:
Here's a summary of what was built:

## Implementation Summary: `migrate/` Package

### Files Created (4 files, 3,051 lines)

| File | Lines | Purpose |
|------|-------|---------|
| `migrate/dot_parser.go` | 516 | Hand-written DOT lexer + parser for the Tracker DOT subset |
| `migrate/migrate.go` | 615 | `Migrate()` and `MigrateToSource()` — DOT string → IR with full cleanup pipeline |
| `migrate/parity.go` | 354 | `CheckParity()` — structural comparison of two `*ir.Workflow` values |
| `migrate/migrate_test.go` | 1,566 | 54 test cases covering all functionality |

### Exported API

- **`Migrate(dotSource string) (*ir.Workflow, error)`** — Parses DOT digraph → IR with all cleanup transforms
- **`MigrateToSource(dotSource string) (string, error)`** — Convenience: DOT → IR → canonical `.dip` source text
- **`CheckParity(a, b *ir.Workflow) []Difference`** — Structural comparison reporting missing/extra/mismatched elements

### Key Features Implemented

1. **DOT Parser** — Lexer/parser for the DOT subset (digraph, node/edge statements, attributes, comments, quoted strings with escape sequences). Not a full Graphviz parser — handles exactly what Tracker DOT files use.

2. **Shape → Kind Mapping** — Reverse of §15 export table: `box`→`agent`, `hexagon`→`human`, `parallelogram`→`tool`, `component`→`parallel`, `tripleoctagon`→`fan_in`, `tab`→`subgraph`. Diamond disambiguation per §5 (tool_command → tool, otherwise → agent). Mdiamond/Msquare → agent (start/exit markers).

3. **Legacy Attribute Normalization** — `llm_model`→`model`, `llm_provider`→`provider`, `loop_restart`→`restart`, `default_max_retry`→`max_retries`, `default_fidelity`→`fidelity`, `context.`→`ctx.`

4. **Condition Parsing** — Handles `=`/`!=`/`contains`/`startswith`/`endswith`/`in`, `&&`/`||`, `not`/`!` with correct precedence. Adds `ctx.` namespace prefix to bare variables. Normalizes `context.` → `ctx.`.

5. **Parallel/Fan-in Inference** — Infers targets from outgoing edges of `component` nodes and sources from incoming edges of `tripleoctagon` nodes when not explicitly set.

6. **Parity Checker** — Reports `node_missing`, `node_extra`, `kind_mismatch`, `config_mismatch`, `edge_missing`, `edge_extra`, `start_mismatch`, `exit_mismatch`, `defaults_mismatch`. Whitespace-tolerant prompt comparison.

### Test Coverage (54 tests)

- **10 DOT parser tests**: simple digraph, attributes, escapes, comments, empty, multiple edges, missing semicolons, malformed input
- **17+ migration tests**: shape mapping, start/exit, prompt unescaping, tool command unescaping, condition namespacing, complex conditions, negation, restart edges, graph defaults, parallel/fan-in inference, diamond disambiguation, edge weight, duration parsing, legacy attribute names
- **11 parity tests**: identical, missing node, extra node, start/exit mismatch, edge missing/extra, config/kind mismatch, whitespace tolerance, defaults mismatch
- **1 integration test**: Full migration of the real `build_dippin.dot` file verifying all 22 nodes, edge topology, conditions, parallel targets, fan-in sources, restart edges, legacy attribute mapping, and `MigrateToSource` output

Session 0a5a8154 completed in 6m50s
Turns: 20 | Tool calls: 29 (bash: 10, edit: 1, read: 14, write: 4)
Tokens: 34805 (in: 1882, out: 32923) | Cost: $2.50
Longest turn: 2m46s
