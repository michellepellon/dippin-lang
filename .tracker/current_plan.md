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
