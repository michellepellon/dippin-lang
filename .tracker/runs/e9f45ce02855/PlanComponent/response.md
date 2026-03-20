TURN 1
TOOL CALL: read
INPUT:
{"path": ".tracker/current_context.md"}
TOOL RESULT: read
OUTPUT:
=== Next component: validator ===
component: validator
package: validator/
description: Graph structure validation (DIP001-DIP009)

=== Current codebase ===
./cmd/dippin/main.go
./ir/edge.go
./ir/ir_test.go
./ir/ir.go
./ir/lookup.go
./ir/source.go

=== Test status ===
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/ir	(cached)

=== Ledger ===
component	package	status	description
ir	ir/	complete	Canonical IR types (Workflow, Node, Edge, Condition)
parser-lexer	parser/	in_progress	Line-based indentation-aware lexer
parser-core	parser/	in_progress	Parser: .dip source to IR with error recovery
parser-conditions	parser/	in_progress	Condition expression parser (and/or/not/compare)
validator	validator/	in_progress	Graph structure validation (DIP001-DIP009)
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
TOOL CALL: glob
INPUT:
{"pattern": "*DESIGN*"}
TOOL RESULT: glob
OUTPUT:
DIPPIN_DESIGN_PLAN.md
TOOL CALL: glob
INPUT:
{"pattern": "**/*DESIGN*"}
TOOL RESULT: glob
OUTPUT:
no matches for pattern "**/*DESIGN*"
TOOL CALL: glob
INPUT:
{"pattern": "**/*.md"}
TOOL RESULT: glob
OUTPUT:
.tracker/current_context.md
TURN 3
TOOL CALL: read
INPUT:
{"path": "DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: read
OUTPUT:
[... truncated 40276 characters ...]
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

| Dippin concept | DOT handling |
|---------------|-------------|
| Multiline prompts | Serialized with `\n` escapes in `prompt` attribute |
| Multiline tool commands | Serialized with `\n` escapes in `tool_command` attribute |
| Import/module structure | Expanded inline; module boundaries lost |
| Source locations | Not representable in DOT |
| Parsed condition AST | Serialized back to string expression |
| Parameter defaults | Not representable; resolved values exported |
| Comments/sections | Lost (DOT has no comment attachment) |
| `reads:`/`writes:` contracts | Not represented |
| `route` sugar (post-v1) | Would expand to conditional edges; not in v1 |
| Restart edge annotation | Exported as custom attribute `restart=true` on the edge (e.g., `A -> B [restart=true]`). Old Tracker ignores unknown attrs, but the exported DOT preserves the semantic for round-tripping. |
| Variable namespaces | Stripped back to flat names |

### Intentionally omitted

| Dippin concept | Why omitted from DOT |
|---------------|---------------------|
| Validation diagnostics | Not a graph concept |
| Formatter state | Not a graph concept |
| Import resolution log | Build artifact, not graph data |

### Implementation

```go
// dippin/export/dot.go
func ExportDOT(w *ir.Workflow, opts ExportOptions) string
```

Options:
- `IncludePrompts bool` — include full prompts (default true; false for clean topology view)
- `RankDir string` — "LR" or "TB"
- `HighlightGoalGates bool` — color goal gate nodes

---

## 16. Migration Strategy

### Phase 1: Automated conversion tool (weeks 1-2)

Build `dippin migrate <input.dot> [output.dip]`:

1. **Parse DOT** using existing `pipeline.ParseDOT()`
2. **Convert to IR** using `GraphToIR()`
3. **Emit Dippin** using a pretty-printer from IR

What auto-converts cleanly:
- Graph structure (nodes, edges)
- Node kinds (via shape mapping)
- Simple attributes (model, provider, label, max_retries, etc.)
- Edge conditions (raw string preserved, with namespace prefixes added)
- Graph-level defaults
- Start/exit node identification

What needs manual cleanup:
- Prompts with DOT escape artifacts (`\n` → real newlines, `\"` → `"`) — the migration tool should handle most of this automatically
- Tool commands with triple-escaped shell — same
- Subgraph references (need import statements added manually)
- Section comments (lost in DOT parse — need manual re-addition)
- `reads:`/`writes:` declarations (entirely new — add incrementally)

### Phase 2: Behavioral parity validation (weeks 2-3)

Build `dippin validate-migration <old.dot> <new.dip>`:

1. Parse both files to IR
2. Compare graph topology (nodes, edges, conditions)
3. Compare node configurations (model, provider, prompt content ignoring whitespace)
4. Report differences with source locations in both files
5. Optionally dry-run both through engine and compare execution traces

### Phase 3: Gradual rollout (weeks 3-6)

1. Engine accepts both `.dot` and `.dip` files (detect by extension)
2. CI validates that migrated `.dip` files produce identical IR to original `.dot`
3. New pipelines authored in Dippin only
4. Existing pipelines migrated file-by-file with parity checks
5. DOT parser kept but deprecated; eventually removed from authoring path (kept for import)

### Migration order (by risk)

1. `tracker/pipeline/testdata/*.dot` — smallest, good for validating the tool
2. `tracker/examples/vulnerability_analyzer.dot` — smallest real pipeline (48 lines)
3. `tracker/examples/semport.dot` — small, tests tool nodes and conditions
4. `tracker/examples/consensus_task.dot` — tests parallel patterns
5. `tracker/examples/ask_and_execute.dot` — tests full lifecycle
6. `tracker/examples/megaplan.dot` — tests complex parallel + cross-critique
7. `dotpowers/dotpowers-simple.dot` — first large migration
8. `dotpowers/dotpowers.dot` — the big one (1,199 lines)

---

## 17. Bootstrap / Self-Hosting Strategy

Tracker can help build Dippin, but Tracker should not define Dippin.

### What Tracker should build

**Pipeline: `analyze_dot.dot`** — Analyze existing DOT files
- Agent reads each DOT file
- Extracts: node kinds, attributes used, condition patterns, prompt sizes
- Produces a structured report of the real semantic surface area
- Validates the inventory in this plan against actual usage

**Pipeline: `generate_dippin.dot`** — Synthesize candidate Dippin from DOT
- Agent reads a DOT file and the Dippin spec
- Produces a `.dip` file
- Validator checks the output
- Human reviews

**Pipeline: `generate_tests.dot`** — Generate parser test cases
- Agent reads the spec and produces edge-case `.dip` files
- Covers: empty workflows, multiline prompts with special chars, deeply nested conditions, import chains
- Produces expected IR output for each

**Pipeline: `migration_fixture.dot`** — Generate migration test fixtures
- For each example DOT file:
  - Parse to IR
  - Generate Dippin
  - Parse Dippin to IR
  - Assert IR equality

### What Tracker should NOT build

- The Dippin spec itself (must be human-reviewed markdown)
- The parser (must be hand-written Go for diagnostic quality)
- The formatter (must be deterministic, not LLM-generated)
- The IR types (must be designed, not generated)

### Guard rail

All Tracker-generated Dippin output goes through the same validator pipeline that human-authored Dippin does. No special paths.

---

## 18. Implementation Plan by Phases

### Phase 0: Spec & IR (1 week)

- [ ] Finalize Dippin syntax spec as a markdown document
- [ ] Define Go types for canonical IR (`dippin/ir/` package) — using typed NodeConfig union
- [ ] Define `GraphToIR()` adapter from existing `pipeline.Graph`
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
TURN 4
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

TURN 5
TOOL CALL: glob
INPUT:
{"pattern": "**/*.go"}
TOOL RESULT: glob
OUTPUT:
ir/edge.go
ir/ir.go
ir/ir_test.go
ir/lookup.go
ir/source.go
TURN 6
TOOL CALL: read
INPUT:
{"path": "cmd/dippin/main.go"}
TOOL RESULT: read
OUTPUT:
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dippin <command> [args]")
		fmt.Fprintln(os.Stderr, "commands: parse, validate, lint, fmt, export-dot, migrate")
		os.Exit(1)
	}

	switch os.Args[1] {
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

TOOL CALL: bash
INPUT:
{"command": "cat go.mod 2>/dev/null || echo \"no go.mod\""}
TOOL RESULT: bash
OUTPUT:
module github.com/2389/dippin

go 1.25.4

TURN 7
TOOL CALL: write
INPUT:
{"path": ".tracker/current_plan.md", "content": "# Implementation Plan: Validator (DIP001–DIP009)\n\n## Component Summary\n\n**Package**: `validator/`\n**Purpose**: Graph structure validation over `*ir.Workflow`. Implements checks DIP001 through DIP009 from the design spec §\"Validation layers — Layer 3: Graph structure (IR)\".\n\nThis is a **pure IR consumer** — it takes a `*ir.Workflow` and returns a list of diagnostics. It has no dependency on parsing, formatting, or any syntax-level concepts. It operates entirely on the canonical IR types defined in `ir/`.\n\n---\n\n## Design Spec References\n\n- **§ Validation layers — Layer 3: Graph structure (IR)** — the nine rules\n- **§ Diagnostic output** — diagnostic shape (file/line/column, codes, help/fix)\n- **§ ADR 1 (DAG-plus-loops)** — `restart: true` edges are excluded from cycle detection\n- **§ \"Acceptance Criteria\"** — #10 \"Diagnostics are actionable\", #11 \"Multi-diagnostic collection\"\n\n### Rules (verbatim from spec)\n\n| Code    | Rule                                                                                          |\n|---------|-----------------------------------------------------------------------------------------------|\n| DIP001  | `start:` node exists                                                                          |\n| DIP002  | `exit:` node exists                                                                           |\n| DIP003  | All edge endpoints exist                                                                      |\n| DIP004  | All nodes reachable from start                                                                |\n| DIP005  | No unconditional cycles after excluding edges marked `restart: true`                          |\n| DIP006  | Exit node has no outgoing edges                                                               |\n| DIP007  | Parallel fan-out has matching fan-in                                                          |\n| DIP008  | No duplicate node IDs                                                                         |\n| DIP009  | No duplicate edges                                                                            |\n\n---\n\n## Dependencies\n\n- **`ir/`** — all types: `Workflow`, `Node`, `Edge`, `NodeKind`, `SourceLocation`, `ParallelConfig`, `FanInConfig`\n- **No other packages** — the validator is self-contained. It imports only `ir/` and stdlib.\n\n---\n\n## Files to Create\n\n### 1. `validator/diagnostic.go`\n\nDiagnostic types shared by validator and (later) linter.\n\n**Types:**\n\n```go\n// Severity levels for diagnostics.\ntype Severity int\n\nconst (\n    SeverityError   Severity = iota // Must fix — workflow cannot execute\n    SeverityWarning                 // Should fix — likely a bug (used by linter, not this component)\n    SeverityInfo                    // Informational\n    SeverityHint                    // Suggestion\n)\n\n// Diagnostic represents a single validation finding.\ntype Diagnostic struct {\n    Code     string             // \"DIP001\", \"DIP002\", etc.\n    Severity Severity\n    Message  string             // Human-readable explanation\n    Location ir.SourceLocation  // Where in the source (may be zero-value if unavailable)\n    Help     string             // Optional \"did you mean X?\" or explanation\n    Fix      string             // Optional suggested replacement text\n}\n\n// String returns a formatted diagnostic string matching the spec example format:\n//   error[DIP003]: unknown node reference \"InterpretX\" in edge\n//     --> pipeline.dip:45:5\nfunc (d Diagnostic) String() string\n\n// Result holds the outcome of a validation pass.\ntype Result struct {\n    Diagnostics []Diagnostic\n}\n\n// Errors returns only error-severity diagnostics.\nfunc (r Result) Errors() []Diagnostic\n\n// HasErrors returns true if any error-severity diagnostics exist.\nfunc (r Result) HasErrors() bool\n```\n\n### 2. `validator/codes.go`\n\nError code registry — constants and human descriptions.\n\n```go\nconst (\n    DIP001 = \"DIP001\" // start node missing\n    DIP002 = \"DIP002\" // exit node missing\n    DIP003 = \"DIP003\" // unknown node reference in edge\n    DIP004 = \"DIP004\" // unreachable node(s) from start\n    DIP005 = \"DIP005\" // unconditional cycle detected\n    DIP006 = \"DIP006\" // exit node has outgoing edges\n    DIP007 = \"DIP007\" // parallel/fan_in mismatch\n    DIP008 = \"DIP008\" // duplicate node ID\n    DIP009 = \"DIP009\" // duplicate edge\n)\n\n// CodeDescription maps each code to a short human-readable description.\nvar CodeDescription = map[string]string{\n    DIP001: \"start node does not exist\",\n    DIP002: \"exit node does not exist\",\n    DIP003: \"unknown node reference in edge\",\n    DIP004: \"node unreachable from start\",\n    DIP005: \"unconditional cycle detected\",\n    DIP006: \"exit node has outgoing edges\",\n    DIP007: \"parallel fan-out/fan-in mismatch\",\n    DIP008: \"duplicate node ID\",\n    DIP009: \"duplicate edge\",\n}\n```\n\n### 3. `validator/validate.go`\n\nThe main validation entry point and all nine check functions.\n\n**Public API:**\n\n```go\n// Validate runs all graph-structure checks (DIP001–DIP009) on the workflow\n// and returns all diagnostics found. It always runs all checks — never\n// short-circuits — so that a single pass reports everything.\nfunc Validate(w *ir.Workflow) Result\n```\n\n**Internal check functions** (each appends to a shared `[]Diagnostic`):\n\n```go\nfunc checkStartExists(w *ir.Workflow) []Diagnostic        // DIP001\nfunc checkExitExists(w *ir.Workflow) []Diagnostic          // DIP002\nfunc checkEdgeEndpoints(w *ir.Workflow) []Diagnostic       // DIP003\nfunc checkReachability(w *ir.Workflow) []Diagnostic         // DIP004\nfunc checkNoCycles(w *ir.Workflow) []Diagnostic             // DIP005\nfunc checkExitNoOutgoing(w *ir.Workflow) []Diagnostic       // DIP006\nfunc checkParallelFanIn(w *ir.Workflow) []Diagnostic        // DIP007\nfunc checkNoDuplicateNodes(w *ir.Workflow) []Diagnostic     // DIP008\nfunc checkNoDuplicateEdges(w *ir.Workflow) []Diagnostic     // DIP009\n```\n\n**Algorithm notes:**\n\n- **DIP001**: Check `w.Start != \"\"` AND `w.Node(w.Start) != nil`.\n- **DIP002**: Check `w.Exit != \"\"` AND `w.Node(w.Exit) != nil`.\n- **DIP003**: For each edge, check `w.Node(e.From) != nil` and `w.Node(e.To) != nil`. Include the dangling reference name in the message. If a close match exists in node IDs (Levenshtein ≤ 2), add it as `Help: \"did you mean X?\"`.\n- **DIP004**: BFS/DFS from `w.Start` along all edges (including restart edges). Any node not visited is unreachable. Report each unreachable node individually with its source location.\n- **DIP005**: Build the subgraph of edges where `e.Restart == false`. Run a standard cycle detection (DFS with gray/black coloring). If a cycle is found, report the cycle path (list of node IDs forming the cycle).\n- **DIP006**: Check `len(w.EdgesFrom(w.Exit)) == 0`. Report each offending edge.\n- **DIP007**: For each `NodeParallel` node, verify there exists exactly one `NodeFanIn` node whose `FanInConfig.Sources` matches the `ParallelConfig.Targets`. For each `NodeFanIn`, verify a corresponding `NodeParallel` exists. Report orphaned parallel or fan_in nodes.\n- **DIP008**: Build `map[string]int` counting node IDs. Any ID with count > 1 is reported, with source locations of both declarations.\n- **DIP009**: Build `map[[2]string]int` keyed on `(From, To)` pairs. Edges with identical `(From, To)` are duplicates. Note: two edges with the same endpoints but *different conditions* are NOT duplicates — they are conditional branches. So the dedup key is `(From, To, Condition.Raw)` where `Condition.Raw == \"\"` for unconditional edges. Actually, re-reading the spec: \"No duplicate edges\" — this should mean truly identical edges (same From, To, and same condition raw text). Two edges `A -> B when X` and `A -> B when Y` are distinct. Two unconditional edges `A -> B` are duplicates.\n\n**Fuzzy matching helper** (for DIP003 help text):\n\n```go\n// closestNodeID returns the node ID most similar to the given name,\n// or \"\" if no node is within edit distance 2.\nfunc closestNodeID(w *ir.Workflow, name string) string\n```\n\nThis uses a simple Levenshtein distance function (no external deps needed — implement inline, ~20 lines).\n\n### 4. `validator/validate_test.go`\n\nComprehensive tests. See test plan below.\n\n---\n\n## Test Plan (15 cases)\n\n### Happy Path\n\n1. **Valid minimal workflow** — Two nodes (start + exit), one edge. `Validate()` returns zero diagnostics.\n2. **Valid complex workflow** — `askAndExecuteWorkflow()` fixture from `ir/ir_test.go`. Has restart edges, parallel/fan_in, conditions. Zero diagnostics.\n3. **Valid workflow with restart back-edge** — Has a cycle through a `restart: true` edge. DIP005 should NOT trigger.\n4. **Valid parallel/fan_in pair** — ParallelConfig.Targets matches FanInConfig.Sources exactly. Zero diagnostics for DIP007.\n\n### Error Cases — One Diagnostic Each\n\n5. **DIP001: Missing start node** — `w.Start = \"Nonexistent\"`. Expect exactly one DIP001 diagnostic.\n6. **DIP002: Missing exit node** — `w.Exit = \"Nonexistent\"`. Expect exactly one DIP002 diagnostic.\n7. **DIP003: Dangling edge reference** — Edge `From: \"A\", To: \"Nonexistent\"`. Expect DIP003 with the dangling name in the message.\n8. **DIP003 with fuzzy match** — Edge references \"Interpet\" (typo for \"Interpret\"). Expect DIP003 with `Help` containing \"did you mean \\\"Interpret\\\"?\".\n9. **DIP004: Unreachable node** — A node exists in `Nodes` but has no incoming edge from any reachable node. Expect DIP004.\n10. **DIP005: Unconditional cycle** — `A -> B -> C -> A` with no restart edges. Expect DIP005 listing the cycle.\n11. **DIP006: Exit has outgoing** — Exit node has an edge going somewhere. Expect DIP006.\n12. **DIP007: Orphaned parallel** — A `NodeParallel` with targets `[X, Y]` but no corresponding `NodeFanIn`. Expect DIP007.\n13. **DIP007: Orphaned fan_in** — A `NodeFanIn` with sources `[X, Y]` but no corresponding `NodeParallel`. Expect DIP007.\n14. **DIP008: Duplicate node ID** — Two nodes with same ID. Expect DIP008.\n15. **DIP009: Duplicate edge** — Two unconditional edges with same `(From, To)`. Expect DIP009.\n\n### Edge Cases\n\n16. **Multiple errors at once** — Workflow has DIP001 + DIP003 + DIP008 simultaneously. Verify all three are reported (multi-diagnostic).\n17. **Empty workflow** — Zero nodes, zero edges. Should report DIP001, DIP002 (no start/exit). Should NOT panic.\n18. **DIP005: Cycle through restart edge is OK** — `A -> B -> C -> A [restart: true]` is valid (restart edges excluded from cycle detection). Zero DIP005 diagnostics.\n19. **DIP003: Both endpoints dangling** — Edge where both From and To are nonexistent. Expect two DIP003 diagnostics (one per endpoint).\n20. **DIP009: Same endpoints, different conditions = NOT duplicate** — Two edges `A -> B when X` and `A -> B when Y`. Should NOT trigger DIP009.\n21. **DIP004: Start node unreachable from itself** — Start is declared but `w.Start` points to it; it IS reachable (it's the starting point). Other disconnected nodes ARE unreachable.\n22. **Diagnostic formatting** — Verify `Diagnostic.String()` output matches the spec format with code, message, and location.\n\n---\n\n## Implementation Order\n\n1. `validator/diagnostic.go` — types first (Diagnostic, Severity, Result)\n2. `validator/codes.go` — constants\n3. `validator/validate.go` — implement checks in order DIP008 → DIP001 → DIP002 → DIP003 → DIP006 → DIP009 → DIP004 → DIP005 → DIP007\n   - DIP008 first because duplicate detection is needed before graph traversal\n   - DIP004/DIP005 require graph traversal algorithms (BFS, DFS cycle detection)\n   - DIP007 requires matching parallel/fan_in semantics\n4. `validator/validate_test.go` — tests alongside implementation\n\n**Rationale for check order in `Validate()`**: All checks run unconditionally. However, DIP008 (duplicate nodes) logically comes first because later checks might produce confusing results on workflows with duplicate IDs. The public `Validate()` function runs all checks and concatenates results.\n\n---\n\n## Non-Goals (explicitly out of scope)\n\n- **Layer 2 (Schema validation)** — known fields, required fields, type checking. That's a separate component.\n- **Layer 4 (Semantic warnings / linter: DIP101–DIP112)** — separate `linter` component per the ledger.\n- **Parser integration** — the validator takes `*ir.Workflow`, not `.dip` source text.\n- **JSON diagnostic output** — will be added when CLI is implemented. The `Diagnostic` type should be JSON-serializable but we don't build the JSON formatter here.\n- **Autofix** — diagnostics include `Fix` text for human/tooling consumption, but no automated rewriting.\n\n---\n\n## Open Design Decisions\n\n1. **DIP007 matching semantics**: The spec says \"Parallel fan-out has matching fan-in.\" The strictest reading: for each `NodeParallel` with `Targets: [X, Y]`, there must exist exactly one `NodeFanIn` with `Sources: [X, Y]` (same set, order-insensitive). The implementation should use set comparison (sort both slices, compare).\n\n2. **DIP009 dedup key**: The spec says \"No duplicate edges.\" Two edges with the same `(From, To)` but different conditions are NOT duplicates — they represent conditional branching. The dedup key is `(From, To, conditionKey)` where `conditionKey` is `Condition.Raw` if the condition exists, or `\"\"` if unconditional.\n\n3. **DIP005 cycle reporting**: When a cycle is found, the diagnostic should include the cycle path (e.g., \"cycle: A → B → C → A\") so the user knows which edges to fix. The DFS algorithm should record the path of gray nodes when a back-edge is encountered.\n\n---\n\n## Example Usage (for implementation agent reference)\n\n```go\nimport (\n    \"fmt\"\n    \"github.com/2389/dippin/ir\"\n    \"github.com/2389/dippin/validator\"\n)\n\nfunc example() {\n    w := &ir.Workflow{\n        Name:  \"broken\",\n        Start: \"Begin\",\n        Exit:  \"End\",\n        Nodes: []*ir.Node{\n            {ID: \"Begin\", Kind: ir.NodeHuman, Config: ir.HumanConfig{}},\n            // \"End\" is missing!\n        },\n        Edges: []*ir.Edge{\n            {From: \"Begin\", To: \"End\"},\n        },\n    }\n\n    result := validator.Validate(w)\n    for _, d := range result.Diagnostics {\n        fmt.Println(d.String())\n    }\n    // Output:\n    // error[DIP002]: exit node \"End\" is declared but does not exist in the node list\n    //   --> :0:0\n    //   = help: add a node with ID \"End\" to the workflow\n    // error[DIP003]: edge references unknown node \"End\"\n    //   --> :0:0\n    //   = help: declare a node with ID \"End\" or fix the edge target\n}\n```\n"}
TOOL RESULT: write
OUTPUT:
wrote 14510 bytes to .tracker/current_plan.md
TURN 8
TOOL CALL: bash
INPUT:
{"command": "wc -l .tracker/current_plan.md && echo \"---\" && head -5 .tracker/current_plan.md"}
TOOL RESULT: bash
OUTPUT:
     278 .tracker/current_plan.md
---
# Implementation Plan: Validator (DIP001–DIP009)

## Component Summary

**Package**: `validator/`

TURN 9
TEXT:
The plan is written to `.tracker/current_plan.md`. Here's a summary of what it covers:

## Plan Summary

**Component**: `validator/` — Graph structure validation (DIP001–DIP009)

### Files to Create (4 files)
| File | Purpose |
|------|---------|
| `validator/diagnostic.go` | `Diagnostic`, `Severity`, `Result` types — shared diagnostic infrastructure |
| `validator/codes.go` | Constants `DIP001`–`DIP009` with human-readable descriptions |
| `validator/validate.go` | `Validate(w *ir.Workflow) Result` entry point + 9 check functions + Levenshtein helper |
| `validator/validate_test.go` | 22 test cases |

### The 9 Checks
| Code | Check | Algorithm |
|------|-------|-----------|
| DIP001 | Start node exists | Field + lookup |
| DIP002 | Exit node exists | Field + lookup |
| DIP003 | All edge endpoints exist | Iterate edges, lookup nodes, fuzzy-match for "did you mean?" |
| DIP004 | All nodes reachable from start | BFS from start, report unvisited |
| DIP005 | No unconditional cycles | DFS with gray/black coloring on non-restart edges |
| DIP006 | Exit has no outgoing edges | `EdgesFrom(exit)` check |
| DIP007 | Parallel/fan_in pairing | Set-compare targets↔sources |
| DIP008 | No duplicate node IDs | Count map |
| DIP009 | No duplicate edges | Keyed on `(From, To, Condition.Raw)` |

### Test Coverage: 22 cases
- 4 happy-path (valid workflows including restart edges and parallel/fan_in)
- 11 individual error cases (one per rule + fuzzy match)
- 7 edge cases (empty workflow, multiple simultaneous errors, both endpoints dangling, same-endpoint-different-conditions, diagnostic formatting)

### Key Design Decisions Documented
- Restart edges excluded from cycle detection (per ADR 1)
- DIP009 dedup key includes condition raw text (conditional branches are not duplicates)
- DIP007 uses order-insensitive set matching for targets↔sources
- All checks run unconditionally (multi-diagnostic collection)
- Levenshtein ≤ 2 for "did you mean?" suggestions in DIP003

Session 0a8d6774 completed in 2m1s
Turns: 9 | Tool calls: 16 (bash: 2, glob: 4, read: 9, write: 1)
Tokens: 11601 (in: 5850, out: 5751) | Cost: $0.52
Longest turn: 1m20s
