TURN 1
TOOL CALL: bash
INPUT:
{"command": "cd /home/user && find . -type f -name '*.go' | head -40"}
TOOL RESULT: bash
OUTPUT:
stderr: sh: line 0: cd: /home/user: No such file or directory

exit code: 1
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
TURN 2
TOOL CALL: bash
INPUT:
{"command": "find . -type f -name '*.go' | sort"}
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

TURN 3
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
{"path": "validator/lint.go"}
TOOL RESULT: read
OUTPUT:
package validator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/2389/dippin/ir"
)

// Lint runs all semantic quality checks (DIP101–DIP112) on the workflow
// and returns all diagnostics found. These are warnings, not errors —
// the workflow can still execute, but the findings indicate likely bugs
// or quality issues.
//
// Lint is independent of Validate. Callers should run both:
//
//	structureResult := validator.Validate(w)
//	lintResult := validator.Lint(w)
func Lint(w *ir.Workflow) Result {
	var diags []Diagnostic

	diags = append(diags, lintConditionalReachability(w)...)
	diags = append(diags, lintDefaultEdge(w)...)
	diags = append(diags, lintOverlappingConditions(w)...)
	diags = append(diags, lintUnboundedRetry(w)...)
	diags = append(diags, lintSuccessPath(w)...)
	diags = append(diags, lintUndefinedVariables(w)...)
	diags = append(diags, lintUnusedWrites(w)...)
	diags = append(diags, lintModelProvider(w)...)
	diags = append(diags, lintNamespaceCollisions(w)...)
	diags = append(diags, lintEmptyPrompts(w)...)
	diags = append(diags, lintToolTimeout(w)...)
	diags = append(diags, lintReadsWithoutUpstreamWrites(w)...)

	return Result{Diagnostics: diags}
}

// lintConditionalReachability checks DIP101: nodes that are only reachable
// through conditional edges may be unreachable at runtime if conditions are
// not satisfied. A node is flagged if ALL of its incoming edges are conditional
// (have a non-nil Condition), meaning there is no guaranteed path to it.
func lintConditionalReachability(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	// Build a map of incoming edges per node.
	incoming := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		incoming[e.To] = append(incoming[e.To], e)
	}

	for _, n := range w.Nodes {
		// Start node is always reachable by definition.
		if n.ID == w.Start {
			continue
		}
		edges := incoming[n.ID]
		if len(edges) == 0 {
			// No incoming edges at all — DIP004 handles this.
			continue
		}
		allConditional := true
		for _, e := range edges {
			if e.Condition == nil {
				allConditional = false
				break
			}
		}
		if allConditional {
			diags = append(diags, Diagnostic{
				Code:     DIP101,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q is only reachable through conditional edges and may be skipped at runtime", n.ID),
				Location: n.Source,
				Help:     "add an unconditional edge to this node, or verify all conditions are exhaustive",
			})
		}
	}
	return diags
}

// lintDefaultEdge checks DIP102: nodes that have outgoing conditional edges
// but no unconditional (default/fallback) edge. Without a default edge,
// execution may get stuck at this node if no condition matches.
func lintDefaultEdge(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	for _, n := range w.Nodes {
		outgoing := w.EdgesFrom(n.ID)
		if len(outgoing) == 0 {
			continue
		}

		hasConditional := false
		hasUnconditional := false
		for _, e := range outgoing {
			if e.Condition != nil {
				hasConditional = true
			} else {
				hasUnconditional = true
			}
		}

		// Only flag if there are conditional edges but no unconditional fallback.
		if hasConditional && !hasUnconditional {
			diags = append(diags, Diagnostic{
				Code:     DIP102,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has conditional outgoing edges but no unconditional default edge", n.ID),
				Location: n.Source,
				Help:     "add an unconditional edge as a fallback, or ensure conditions are exhaustive",
			})
		}
	}
	return diags
}

// lintOverlappingConditions checks DIP103: multiple edges from the same node
// with conditions that compare the same variable to the same value using "=".
// This indicates contradictory or duplicated routing logic.
func lintOverlappingConditions(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	// Group edges by source node.
	edgesBySource := make(map[string][]*ir.Edge)
	for _, e := range w.Edges {
		if e.Condition != nil {
			edgesBySource[e.From] = append(edgesBySource[e.From], e)
		}
	}

	for from, edges := range edgesBySource {
		// Extract top-level equality comparisons from each edge condition.
		type condKey struct {
			variable string
			op       string
			value    string
		}

		seen := make(map[condKey]*ir.Edge)
		for _, e := range edges {
			comparisons := extractComparisons(e.Condition.Parsed)
			for _, cmp := range comparisons {
				key := condKey{variable: cmp.Variable, op: cmp.Op, value: cmp.Value}
				if first, ok := seen[key]; ok {
					diags = append(diags, Diagnostic{
						Code:     DIP103,
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("node %q has overlapping conditions: edges to %q and %q both check %s %s %s", from, first.To, e.To, cmp.Variable, cmp.Op, cmp.Value),
						Location: e.Source,
						Help:     "review the conditions to ensure they route to different targets for different states",
					})
				} else {
					seen[key] = e
				}
			}
		}
	}
	return diags
}

// extractComparisons recursively extracts all CondCompare nodes from a
// condition expression tree. This flattens AND/OR/NOT to find the leaf comparisons.
func extractComparisons(expr ir.ConditionExpr) []ir.CondCompare {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case ir.CondCompare:
		return []ir.CondCompare{e}
	case ir.CondAnd:
		return append(extractComparisons(e.Left), extractComparisons(e.Right)...)
	case ir.CondOr:
		return append(extractComparisons(e.Left), extractComparisons(e.Right)...)
	case ir.CondNot:
		return extractComparisons(e.Inner)
	default:
		return nil
	}
}

// lintUnboundedRetry checks DIP104: nodes with retry configuration that have
// no max_retries limit and no fallback target. This could cause infinite retry
// loops at runtime.
func lintUnboundedRetry(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		r := n.Retry
		// Only flag nodes that have some retry config but no bounds.
		hasRetryConfig := r.Policy != "" || r.RetryTarget != ""
		if hasRetryConfig && r.MaxRetries == 0 && r.FallbackTarget == "" {
			diags = append(diags, Diagnostic{
				Code:     DIP104,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has retry configuration but no max_retries or fallback_target", n.ID),
				Location: n.Source,
				Help:     "set max_retries to limit retries, or add a fallback_target for graceful degradation",
			})
		}
	}
	return diags
}

// lintSuccessPath checks DIP105: there must be at least one path from the
// start node to the exit node using only non-restart edges. If no such path
// exists, the workflow can never complete normally.
func lintSuccessPath(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Exit == "" {
		return nil
	}
	if w.Node(w.Start) == nil || w.Node(w.Exit) == nil {
		return nil
	}

	// BFS from start, following only non-restart edges.
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if !e.Restart {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}

	visited := make(map[string]bool)
	queue := []string{w.Start}
	visited[w.Start] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if curr == w.Exit {
			return nil // Found a path.
		}
		for _, next := range adj[curr] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	return []Diagnostic{{
		Code:     DIP105,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("no forward path from start node %q to exit node %q (excluding restart edges)", w.Start, w.Exit),
		Help:     "ensure there is at least one non-restart path from start to exit",
	}}
}

// varRefPattern matches ${...} variable references in prompt text.
var varRefPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// knownNamespaces lists the valid namespace prefixes for variable references.
var knownNamespaces = map[string]bool{
	"ctx":   true,
	"graph": true,
	"env":   true,
}

// lintUndefinedVariables checks DIP106: ${variable} references in prompts
// must use known namespace prefixes (ctx., graph., env.). References without
// a recognized namespace are flagged.
func lintUndefinedVariables(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		prompt := nodePrompt(n)
		if prompt == "" {
			continue
		}
		matches := varRefPattern.FindAllStringSubmatch(prompt, -1)
		for _, m := range matches {
			varRef := m[1] // The captured group inside ${...}
			parts := strings.SplitN(varRef, ".", 2)
			if len(parts) < 2 || !knownNamespaces[parts[0]] {
				diags = append(diags, Diagnostic{
					Code:     DIP106,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("node %q references undefined variable ${%s}", n.ID, varRef),
					Location: n.Source,
					Help:     fmt.Sprintf("use a namespaced variable like ${ctx.%s}, ${graph.%s}, or ${env.%s}", varRef, varRef, varRef),
				})
			}
		}
	}
	return diags
}

// lintUnusedWrites checks DIP107: context keys declared in a node's writes:
// that are not referenced in any other node's reads:. These are dead outputs
// that may indicate unused work.
func lintUnusedWrites(w *ir.Workflow) []Diagnostic {
	// Collect all reads across all nodes.
	allReads := make(map[string]bool)
	for _, n := range w.Nodes {
		for _, key := range n.IO.Reads {
			allReads[key] = true
		}
	}

	var diags []Diagnostic
	for _, n := range w.Nodes {
		for _, key := range n.IO.Writes {
			if !allReads[key] {
				diags = append(diags, Diagnostic{
					Code:     DIP107,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("node %q writes context key %q but no node declares it in reads", n.ID, key),
					Location: n.Source,
					Help:     fmt.Sprintf("add reads: %s to a downstream node, or remove this write if unused", key),
				})
			}
		}
	}
	return diags
}

// knownModelProviders lists known valid model/provider combinations.
// This is a best-effort catalog — unknown combinations produce a warning,
// not an error, since new models may be added at any time.
var knownModelProviders = map[string]map[string]bool{
	"anthropic": {
		"claude-opus-4-6":        true,
		"claude-sonnet-4-6":      true,
		"claude-haiku-3-5":       true,
		"claude-opus-4-20250116": true,
	},
	"openai": {
		"gpt-5.4":        true,
		"gpt-5.3-codex":  true,
		"gpt-4o":         true,
		"gpt-4o-mini":    true,
		"o3":             true,
		"o4-mini":        true,
	},
}

// lintModelProvider checks DIP108: model/provider combinations should be
// in the known catalog. Unknown combinations may indicate typos.
func lintModelProvider(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}

		model := cfg.Model
		provider := cfg.Provider

		// Use workflow defaults as fallback.
		if model == "" {
			model = w.Defaults.Model
		}
		if provider == "" {
			provider = w.Defaults.Provider
		}

		// Only check if both are specified.
		if model == "" || provider == "" {
			continue
		}

		providerModels, providerKnown := knownModelProviders[provider]
		if !providerKnown {
			diags = append(diags, Diagnostic{
				Code:     DIP108,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q uses unknown provider %q", n.ID, provider),
				Location: n.Source,
				Help:     fmt.Sprintf("known providers: %s", knownProviderList()),
			})
			continue
		}
		if !providerModels[model] {
			diags = append(diags, Diagnostic{
				Code:     DIP108,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q uses unknown model %q for provider %q", n.ID, model, provider),
				Location: n.Source,
				Help:     fmt.Sprintf("known models for %s: %s", provider, knownModelList(provider)),
			})
		}
	}
	return diags
}

// knownProviderList returns a sorted comma-separated list of known providers.
func knownProviderList() string {
	providers := make([]string, 0, len(knownModelProviders))
	for p := range knownModelProviders {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return strings.Join(providers, ", ")
}

// knownModelList returns a sorted comma-separated list of known models for a provider.
func knownModelList(provider string) string {
	models := knownModelProviders[provider]
	list := make([]string, 0, len(models))
	for m := range models {
		list = append(list, m)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}

// lintNamespaceCollisions checks DIP109: multiple subgraph nodes referencing
// the same file without different parameters could cause namespace collisions
// when expanded.
func lintNamespaceCollisions(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic

	type subgraphRef struct {
		node *ir.Node
		ref  string
	}

	seen := make(map[string]*ir.Node)
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.SubgraphConfig)
		if !ok {
			continue
		}
		if first, exists := seen[cfg.Ref]; exists {
			diags = append(diags, Diagnostic{
				Code:     DIP109,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("nodes %q and %q both reference subgraph %q, which may cause namespace collisions", first.ID, n.ID, cfg.Ref),
				Location: n.Source,
				Help:     "use distinct node IDs and ensure imported names do not collide after expansion",
			})
		} else {
			seen[cfg.Ref] = n
		}
	}
	return diags
}

// lintEmptyPrompts checks DIP110: agent nodes should have a non-empty prompt.
// An agent without a prompt has nothing to send to the LLM.
func lintEmptyPrompts(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if strings.TrimSpace(cfg.Prompt) == "" {
			diags = append(diags, Diagnostic{
				Code:     DIP110,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("agent node %q has an empty prompt", n.ID),
				Location: n.Source,
				Help:     "add a prompt: field with instructions for the LLM",
			})
		}
	}
	return diags
}

// lintToolTimeout checks DIP111: tool nodes should have a timeout configured.
// Without a timeout, a hanging tool command could block the entire pipeline.
func lintToolTimeout(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.ToolConfig)
		if !ok {
			continue
		}
		if cfg.Timeout == 0 {
			diags = append(diags, Diagnostic{
				Code:     DIP111,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("tool node %q has no timeout configured", n.ID),
				Location: n.Source,
				Help:     "add a timeout (e.g., timeout: 60s) to prevent hanging commands",
			})
		}
	}
	return diags
}

// lintReadsWithoutUpstreamWrites checks DIP112: keys in a node's reads: list
// should appear in the writes: list of at least one upstream node (reachable
// via forward edges from start). This uses a simple flow analysis — for each
// node, compute the set of keys written by upstream nodes, then check reads.
func lintReadsWithoutUpstreamWrites(w *ir.Workflow) []Diagnostic {
	if w.Start == "" || w.Node(w.Start) == nil {
		return nil
	}

	// Build forward adjacency (non-restart edges).
	adj := make(map[string][]string)
	for _, e := range w.Edges {
		if !e.Restart {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}

	// Topological order via BFS (Kahn's algorithm).
	inDegree := make(map[string]int)
	for _, n := range w.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range w.Edges {
		if !e.Restart {
			inDegree[e.To]++
		}
	}

	queue := []string{}
	for _, n := range w.Nodes {
		if inDegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}

	// For each node, compute the set of keys available from upstream writes.
	available := make(map[string]map[string]bool)
	for _, n := range w.Nodes {
		available[n.ID] = make(map[string]bool)
	}

	var order []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)

		// Add this node's writes to what's available for downstream.
		n := w.Node(curr)
		if n != nil {
			for _, key := range n.IO.Writes {
				available[curr][key] = true
			}
		}

		for _, next := range adj[curr] {
			// Merge current node's available keys into the next node's available set.
			for key := range available[curr] {
				available[next][key] = true
			}
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	var diags []Diagnostic
	for _, n := range w.Nodes {
		for _, key := range n.IO.Reads {
			if !available[n.ID][key] {
				diags = append(diags, Diagnostic{
					Code:     DIP112,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("node %q reads context key %q but no upstream node declares it in writes", n.ID, key),
					Location: n.Source,
					Help:     fmt.Sprintf("add writes: %s to an upstream node, or the key may be auto-injected at runtime", key),
				})
			}
		}
	}
	return diags
}

// nodePrompt extracts the prompt text from a node if it has one.
func nodePrompt(n *ir.Node) string {
	switch cfg := n.Config.(type) {
	case ir.AgentConfig:
		return cfg.Prompt
	default:
		return ""
	}
}

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
{"path": "validator/lint_codes.go"}
TOOL RESULT: read
OUTPUT:
package validator

// Diagnostic codes for semantic quality warnings (DIP101–DIP112).
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
}

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

TURN 4
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

TOOL CALL: read
INPUT:
{"path": "validator/lint_test.go"}
TOOL RESULT: read
OUTPUT:
package validator

import (
	"strings"
	"testing"
	"time"

	"github.com/2389/dippin/ir"
)

// --- Test fixtures ---

// cleanMinimalWorkflow returns a minimal valid workflow with no lint warnings.
func cleanMinimalWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "clean",
		Start: "Begin",
		Exit:  "End",
		Nodes: []*ir.Node{
			{ID: "Begin", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Hello."}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Begin", To: "End"},
		},
	}
}

// cleanComplexWorkflow returns a complex valid workflow with no lint warnings.
func cleanComplexWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "clean_complex",
		Start: "Ask",
		Exit:  "Done",
		Defaults: ir.WorkflowDefaults{
			Model:    "claude-opus-4-6",
			Provider: "anthropic",
		},
		Nodes: []*ir.Node{
			{ID: "Ask", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"},
				IO: ir.NodeIO{Writes: []string{"human_response"}}},
			{ID: "Plan", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				Prompt: "Plan the work based on ${ctx.human_response}.",
			}, IO: ir.NodeIO{Reads: []string{"human_response"}, Writes: []string{"plan"}}},
			{ID: "Execute", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				Prompt: "Execute the plan.",
			}, IO: ir.NodeIO{Reads: []string{"plan"}}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Ship it."}},
		},
		Edges: []*ir.Edge{
			{From: "Ask", To: "Plan"},
			{From: "Plan", To: "Execute"},
			{From: "Execute", To: "Done"},
		},
	}
}

// --- Table-driven tests ---

func TestLint(t *testing.T) {
	tests := []struct {
		name       string
		workflow   *ir.Workflow
		wantCodes  []string // Expected diagnostic codes (empty = no diagnostics)
		wantNoDiag bool     // If true, expect zero diagnostics
	}{
		// --- Happy path ---
		{
			name:       "clean minimal workflow",
			workflow:   cleanMinimalWorkflow(),
			wantNoDiag: true,
		},
		{
			name:       "clean complex workflow with IO contracts",
			workflow:   cleanComplexWorkflow(),
			wantNoDiag: true,
		},
		{
			name: "workflow with known model/provider is clean",
			workflow: &ir.Workflow{
				Name:  "known_model",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt:   "Hello.",
						Model:    "gpt-5.4",
						Provider: "openai",
					}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "tool with timeout is clean",
			workflow: &ir.Workflow{
				Name:  "tool_timeout",
				Start: "T",
				Exit:  "T",
				Nodes: []*ir.Node{
					{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
						Command: "echo test",
						Timeout: 30 * time.Second,
					}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP101: Unreachable nodes after conditional branches ---
		{
			name: "DIP101: node only reachable via conditional edges",
			workflow: &ir.Workflow{
				Name:  "cond_only",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.x = 1",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					}},
					{From: "A", To: "C"},
					{From: "B", To: "C"},
				},
			},
			wantCodes: []string{DIP101},
		},
		{
			name: "DIP101: node with unconditional incoming edge is fine",
			workflow: &ir.Workflow{
				Name:  "uncond_ok",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP102: Routing node without default edge ---
		{
			name: "DIP102: conditional outgoing but no default",
			workflow: &ir.Workflow{
				Name:  "no_default",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.x = 1",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					}},
					{From: "A", To: "C", Condition: &ir.Condition{
						Raw:    "ctx.x = 2",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
					}},
					{From: "B", To: "C"},
				},
			},
			wantCodes: []string{DIP102},
		},
		{
			name: "DIP102: mixed conditional + unconditional is fine",
			workflow: &ir.Workflow{
				Name:  "with_default",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.x = 1",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					}},
					{From: "A", To: "C"}, // unconditional default
					{From: "B", To: "C"},
				},
			},
			// B is only reachable via conditional edge (DIP101), but no DIP102.
			wantCodes: []string{DIP101},
		},

		// --- DIP103: Overlapping conditions ---
		{
			name: "DIP103: two edges with same condition",
			workflow: &ir.Workflow{
				Name:  "overlap",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.outcome = success",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					}},
					{From: "A", To: "C", Condition: &ir.Condition{
						Raw:    "ctx.outcome = success",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					}},
				},
			},
			wantCodes: []string{DIP103},
		},
		{
			name: "DIP103: different conditions from same node is fine for overlap",
			workflow: &ir.Workflow{
				Name:  "no_overlap",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.outcome = success",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					}},
					{From: "A", To: "C", Condition: &ir.Condition{
						Raw:    "ctx.outcome = fail",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
					}},
					{From: "B", To: "C"},
				},
			},
			// No DIP103, but DIP101 (B, C only via conditional) and DIP102 (A has no default).
			wantCodes: []string{DIP101, DIP102},
		},

		// --- DIP104: Unbounded retry ---
		{
			name: "DIP104: retry config but no max_retries or fallback",
			workflow: &ir.Workflow{
				Name:  "unbounded_retry",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						Retry: ir.RetryConfig{Policy: "standard"}},
				},
			},
			wantCodes: []string{DIP104},
		},
		{
			name: "DIP104: retry with max_retries is fine",
			workflow: &ir.Workflow{
				Name:  "bounded_retry",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						Retry: ir.RetryConfig{Policy: "standard", MaxRetries: 3}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "DIP104: retry with fallback is fine",
			workflow: &ir.Workflow{
				Name:  "retry_fallback",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						Retry: ir.RetryConfig{Policy: "standard", FallbackTarget: "B"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP105: No success path to exit ---
		{
			name: "DIP105: no forward path to exit",
			workflow: &ir.Workflow{
				Name:  "no_path",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "A", Restart: true}, // only restart edges back
				},
			},
			wantCodes: []string{DIP105},
		},
		{
			name: "DIP105: forward path exists even with restart edges",
			workflow: &ir.Workflow{
				Name:  "has_path",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
					{From: "B", To: "A", Restart: true},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP106: Undefined variables in prompts ---
		{
			name: "DIP106: unnamespaced variable reference",
			workflow: &ir.Workflow{
				Name:  "bad_var",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "Process ${unknown_var} now.",
					}},
				},
			},
			wantCodes: []string{DIP106},
		},
		{
			name: "DIP106: known namespace is fine",
			workflow: &ir.Workflow{
				Name:  "good_var",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "Process ${ctx.data} and ${graph.goal} and ${env.API_KEY}.",
					}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP107: Unused writes ---
		{
			name: "DIP107: writes key that nobody reads",
			workflow: &ir.Workflow{
				Name:  "unused_write",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"orphan_key"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantCodes: []string{DIP107},
		},
		{
			name: "DIP107: writes key that is read downstream is fine",
			workflow: &ir.Workflow{
				Name:  "used_write",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"data"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"data"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP108: Unknown model/provider ---
		{
			name: "DIP108: unknown provider",
			workflow: &ir.Workflow{
				Name:  "bad_provider",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt:   "a",
						Model:    "some-model",
						Provider: "unknown-provider",
					}},
				},
			},
			wantCodes: []string{DIP108},
		},
		{
			name: "DIP108: unknown model for known provider",
			workflow: &ir.Workflow{
				Name:  "bad_model",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt:   "a",
						Model:    "gpt-99",
						Provider: "openai",
					}},
				},
			},
			wantCodes: []string{DIP108},
		},
		{
			name: "DIP108: inherits from defaults",
			workflow: &ir.Workflow{
				Name:  "defaults_model",
				Start: "A",
				Exit:  "A",
				Defaults: ir.WorkflowDefaults{
					Model:    "gpt-99",
					Provider: "openai",
				},
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
				},
			},
			wantCodes: []string{DIP108},
		},

		// --- DIP109: Namespace collisions in imports ---
		{
			name: "DIP109: two subgraphs referencing same file",
			workflow: &ir.Workflow{
				Name:  "collision",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
					{ID: "B", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantCodes: []string{DIP109},
		},
		{
			name: "DIP109: different subgraph refs is fine",
			workflow: &ir.Workflow{
				Name:  "no_collision",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
					{ID: "B", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./test.dip"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP110: Empty prompts ---
		{
			name: "DIP110: agent with empty prompt",
			workflow: &ir.Workflow{
				Name:  "empty_prompt",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: ""}},
				},
			},
			wantCodes: []string{DIP110},
		},
		{
			name: "DIP110: agent with whitespace-only prompt",
			workflow: &ir.Workflow{
				Name:  "ws_prompt",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "   \n  "}},
				},
			},
			wantCodes: []string{DIP110},
		},
		{
			name: "DIP110: non-agent node types do not trigger",
			workflow: &ir.Workflow{
				Name:  "no_prompt_needed",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP111: Tool without timeout ---
		{
			name: "DIP111: tool with no timeout",
			workflow: &ir.Workflow{
				Name:  "no_timeout",
				Start: "T",
				Exit:  "T",
				Nodes: []*ir.Node{
					{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo hello"}},
				},
			},
			wantCodes: []string{DIP111},
		},
		{
			name: "DIP111: tool with timeout is clean",
			workflow: &ir.Workflow{
				Name:  "has_timeout",
				Start: "T",
				Exit:  "T",
				Nodes: []*ir.Node{
					{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
						Command: "echo hello",
						Timeout: 10 * time.Second,
					}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP112: Reads without upstream writes ---
		{
			name: "DIP112: reads key with no upstream writer",
			workflow: &ir.Workflow{
				Name:  "no_writer",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"missing_key"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantCodes: []string{DIP112},
		},
		{
			name: "DIP112: reads key with upstream writer is fine",
			workflow: &ir.Workflow{
				Name:  "has_writer",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"data"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"data"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- Edge cases ---
		{
			name: "empty workflow only reports DIP105 if start/exit are missing",
			workflow: &ir.Workflow{
				Name: "empty",
			},
			// DIP105 skips if start/exit are empty, DIP110/111/etc have no nodes to check
			wantNoDiag: true,
		},
		{
			name: "multiple lint warnings at once",
			workflow: &ir.Workflow{
				Name:  "multi_warn",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "", // DIP110
					}, Retry: ir.RetryConfig{Policy: "aggressive"}}, // DIP104
				},
			},
			wantCodes: []string{DIP110, DIP104},
		},
		{
			name: "DIP106: multiple undefined vars in one prompt",
			workflow: &ir.Workflow{
				Name:  "multi_var",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "Use ${foo} and ${bar} now.",
					}},
				},
			},
			wantCodes: []string{DIP106, DIP106},
		},
		{
			name: "DIP104: no retry config at all does not trigger",
			workflow: &ir.Workflow{
				Name:  "no_retry",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "DIP112: transitive writes propagation",
			workflow: &ir.Workflow{
				Name:  "transitive",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"key1"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"key1"}, Writes: []string{"key2"}}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"},
						IO: ir.NodeIO{Reads: []string{"key1", "key2"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantNoDiag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Lint(tt.workflow)

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
				// Also check total count matches expectations for single-diagnostic tests.
				if len(tt.wantCodes) == 1 {
					codeCount := 0
					for _, d := range result.Diagnostics {
						if d.Code == tt.wantCodes[0] {
							codeCount++
						}
					}
					if codeCount < 1 {
						t.Errorf("expected at least 1 %s diagnostic, got 0. All: %v", tt.wantCodes[0], gotCodes)
					}
				}
			}
		})
	}
}

func TestLintDiagnosticSeverity(t *testing.T) {
	// All lint diagnostics should be warnings, not errors.
	w := &ir.Workflow{
		Name:  "severity_check",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: ""},
				Retry: ir.RetryConfig{Policy: "standard"}},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Severity != SeverityWarning {
			t.Errorf("lint diagnostic %s has severity %s, want warning", d.Code, d.Severity)
		}
	}
}

func TestLintDIP101MessageContent(t *testing.T) {
	w := &ir.Workflow{
		Name:  "msg_check",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{
				Raw:    "ctx.x = 1",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			}},
			{From: "A", To: "C"},
			{From: "B", To: "C"},
		},
	}

	result := Lint(w)
	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == DIP101 {
			found = true
			if !strings.Contains(d.Message, `"B"`) {
				t.Errorf("DIP101 message should mention node B, got: %s", d.Message)
			}
			if !strings.Contains(d.Message, "conditional") {
				t.Errorf("DIP101 message should mention 'conditional', got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Error("expected DIP101 diagnostic")
	}
}

func TestLintDIP102MessageContent(t *testing.T) {
	w := &ir.Workflow{
		Name:  "msg_check",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{
				Raw:    "ctx.x = 1",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			}},
			{From: "A", To: "C", Condition: &ir.Condition{
				Raw:    "ctx.x = 2",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
			}},
			{From: "B", To: "C"},
		},
	}

	result := Lint(w)
	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == DIP102 {
			found = true
			if !strings.Contains(d.Message, `"A"`) {
				t.Errorf("DIP102 message should mention node A, got: %s", d.Message)
			}
			if !strings.Contains(d.Message, "unconditional") || !strings.Contains(d.Message, "default") {
				t.Errorf("DIP102 message should mention 'unconditional default', got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Error("expected DIP102 diagnostic")
	}
}

func TestLintDIP103OverlappingANDConditions(t *testing.T) {
	// Overlapping condition buried in an AND expression.
	w := &ir.Workflow{
		Name:  "and_overlap",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{
				Raw: "ctx.x = 1 and ctx.y = 2",
				Parsed: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
			}},
			{From: "A", To: "C", Condition: &ir.Condition{
				Raw:    "ctx.x = 1",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			}},
		},
	}

	result := Lint(w)
	found := false
	for _, d := range result.Diagnostics {
		if d.Code == DIP103 {
			found = true
		}
	}
	if !found {
		t.Error("expected DIP103 for overlapping condition in AND expression")
	}
}

func TestLintDIP105StartEqualsExit(t *testing.T) {
	// When start == exit, trivially reachable.
	w := &ir.Workflow{
		Name:  "trivial",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Code == DIP105 {
			t.Errorf("DIP105 should not trigger when start == exit, got: %s", d.Message)
		}
	}
}

func TestLintDIP106NoPromptNodes(t *testing.T) {
	// Human and tool nodes should not trigger DIP106.
	w := &ir.Workflow{
		Name:  "no_prompt",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{ID: "B", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo ${not_a_prompt}", Timeout: 10 * time.Second}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Code == DIP106 {
			t.Errorf("DIP106 should not trigger for non-agent nodes, got: %s", d.Message)
		}
	}
}

func TestLintDIP108NoModelOrProvider(t *testing.T) {
	// If model or provider is not specified (and no defaults), don't check.
	w := &ir.Workflow{
		Name:  "no_model",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Code == DIP108 {
			t.Errorf("DIP108 should not trigger when model/provider are unset, got: %s", d.Message)
		}
	}
}

func TestLintDIP112CycleDoesNotPanic(t *testing.T) {
	// A workflow with a cycle (via non-restart edges) should not cause
	// the topological sort in DIP112 to hang or panic.
	w := &ir.Workflow{
		Name:  "cycle_safe",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
				IO: ir.NodeIO{Writes: []string{"data"}}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
				IO: ir.NodeIO{Reads: []string{"data"}}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "A"}, // cycle
			{From: "B", To: "C"},
		},
	}

	// Should not panic.
	result := Lint(w)
	_ = result
}

func TestLintCodeDescriptionCoverage(t *testing.T) {
	codes := []string{DIP101, DIP102, DIP103, DIP104, DIP105, DIP106, DIP107, DIP108, DIP109, DIP110, DIP111, DIP112}
	for _, c := range codes {
		if desc, ok := CodeDescription[c]; !ok || desc == "" {
			t.Errorf("CodeDescription[%q] is missing or empty", c)
		}
	}
}

func TestExtractComparisons(t *testing.T) {
	tests := []struct {
		name string
		expr ir.ConditionExpr
		want int // expected number of comparisons
	}{
		{
			name: "nil",
			expr: nil,
			want: 0,
		},
		{
			name: "single compare",
			expr: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			want: 1,
		},
		{
			name: "AND of two compares",
			expr: ir.CondAnd{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
			},
			want: 2,
		},
		{
			name: "OR of two compares",
			expr: ir.CondOr{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
			},
			want: 2,
		},
		{
			name: "NOT of compare",
			expr: ir.CondNot{Inner: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"}},
			want: 1,
		},
		{
			name: "nested AND/OR/NOT",
			expr: ir.CondAnd{
				Left: ir.CondOr{
					Left:  ir.CondCompare{Variable: "ctx.a", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.b", Op: "=", Value: "2"},
				},
				Right: ir.CondNot{
					Inner: ir.CondCompare{Variable: "ctx.c", Op: "!=", Value: "3"},
				},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractComparisons(tt.expr)
			if len(got) != tt.want {
				t.Errorf("extractComparisons() returned %d comparisons, want %d", len(got), tt.want)
			}
		})
	}
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
TOOL CALL: bash
INPUT:
{"command": "cd /home/user/repos/dippin && go vet ./... 2>&1 && go test -v -count=1 ./... 2>&1"}
TOOL RESULT: bash
OUTPUT:
stderr: sh: line 0: cd: /home/user/repos/dippin: No such file or directory

exit code: 1
TURN 6
TOOL CALL: bash
INPUT:
{"command": "pwd && ls"}
TOOL RESULT: bash
OUTPUT:
/Users/clint/code/2389/dippin-lang
build_dippin.dot
cmd
DIPPIN_DESIGN_PLAN.md
go.mod
ir
validator

TURN 7
TOOL CALL: bash
INPUT:
{"command": "go vet ./... 2>&1 && go test -v -count=1 ./... 2>&1"}
TOOL RESULT: bash
OUTPUT:
?   	github.com/2389/dippin/cmd/dippin	[no test files]
=== RUN   TestWorkflowNodeLookup
--- PASS: TestWorkflowNodeLookup (0.00s)
=== RUN   TestWorkflowEdgesFrom
--- PASS: TestWorkflowEdgesFrom (0.00s)
=== RUN   TestWorkflowEdgesTo
--- PASS: TestWorkflowEdgesTo (0.00s)
=== RUN   TestRestartEdge
--- PASS: TestRestartEdge (0.00s)
=== RUN   TestNodeKinds
--- PASS: TestNodeKinds (0.00s)
=== RUN   TestNodeConfigTypes
--- PASS: TestNodeConfigTypes (0.00s)
=== RUN   TestToolConfig
--- PASS: TestToolConfig (0.00s)
=== RUN   TestSubgraphConfig
--- PASS: TestSubgraphConfig (0.00s)
=== RUN   TestNodeIO
--- PASS: TestNodeIO (0.00s)
=== RUN   TestConditionAST
--- PASS: TestConditionAST (0.00s)
=== RUN   TestNodeIDs
--- PASS: TestNodeIDs (0.00s)
PASS
ok  	github.com/2389/dippin/ir	0.610s
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
ok  	github.com/2389/dippin/validator	0.402s

TURN 8
TOOL CALL: bash
INPUT:
{"command": "go test -cover ./validator/ 2>&1 && go test -cover ./ir/ 2>&1"}
TOOL RESULT: bash
OUTPUT:
ok  	github.com/2389/dippin/validator	0.274s	coverage: 98.9% of statements
ok  	github.com/2389/dippin/ir	0.285s	coverage: 100.0% of statements

TURN 9
TOOL CALL: bash
INPUT:
{"command": "go test -run TestDIP006 -v ./validator/ 2>&1"}
TOOL RESULT: bash
OUTPUT:
=== RUN   TestDIP006MultipleOutgoing
--- PASS: TestDIP006MultipleOutgoing (0.00s)
PASS
ok  	github.com/2389/dippin/validator	0.224s

TURN 10
TEXT:
I've completed a thorough review of the implementation against the Dippin design spec. Here is my consolidated assessment:

---

## Review: Validator & Linter Implementation vs. Dippin Design Spec

### 1. Spec Fidelity ✅

**IR Types** — All types from the spec are correctly implemented:
- `Workflow` with explicit `Start`/`Exit` (spec §"Explicit start/exit")
- `Node` with `NodeKind` enum and sealed `NodeConfig` interface (6 config types)
- `Edge` with `Restart bool` per ADR 1
- `Condition` AST with `CondAnd`, `CondOr`, `CondNot`, `CondCompare` per ADR 2
- `NodeIO` with `Reads`/`Writes`, `SourceLocation` with full range info
- `WorkflowDefaults` covering model, provider, retry, fidelity, etc.

**Validator** — All 9 structural rules (DIP001–DIP009) implemented correctly per spec Layer 3:
- BFS-based reachability (DIP004) correctly includes restart edges
- DFS cycle detection (DIP005) correctly excludes restart edges per ADR 1
- Levenshtein fuzzy matching for DIP003 "did you mean?" suggestions
- Order-insensitive set comparison for parallel/fan-in pairing (DIP007)
- Edge dedup key includes `Condition.Raw` so conditional branches aren't false positives (DIP009)

**Linter** — All 12 semantic warnings (DIP101–DIP112) implemented correctly per spec Layer 4:
- DIP112 uses Kahn's topological sort for upstream flow analysis — exceeds spec requirements

**Diagnostic format** matches the spec's §"Example diagnostic output" exactly.

### 2. Test Comprehensiveness ✅

- **Validator tests**: 22 table-driven + 11 standalone = 33 tests. Happy paths (4), error cases per rule (12), edge cases (6), auxiliary (11).
- **Lint tests**: 36 table-driven + 10 standalone = 46 tests. Happy paths (4), each rule covered with trigger + non-trigger cases, edge cases for cycles, empty workflows, multiple warnings.
- **IR tests**: 11 tests covering lookups, edge queries, all config types, condition AST, NodeIDs.
- **Coverage**: 98.9% validator, 100% ir.

### 3. Integration with ir/ Package ✅

- Correct value-type assertions (`n.Config.(ir.AgentConfig)` not pointer)
- Correct use of helper methods (`w.Node()`, `w.EdgesFrom()`, `w.EdgesTo()`)
- Proper use of `e.Restart`, `e.Condition.Raw`, `e.Condition.Parsed`, `n.Source`, `e.Source`

### 4. Idiomatic Go ✅

- Table-driven tests with `t.Run`
- Sealed interface pattern
- No external dependencies (stdlib only)
- Clean separation: `Validate()` for errors, `Lint()` for warnings
- Comprehensive doc comments on all exported types/functions
- Graceful nil/empty handling throughout

### 5. Minor Deviations (Acceptable)

1. **Package `validator/` vs spec's `validate/`** — More idiomatic Go (packages are nouns). Fine.
2. **Linter in same package** — Pragmatic; shares `Diagnostic`/`Result`/`Severity` types. Avoids a shared types package.
3. **`extractComparisons` flattens through NOT** — May flag complementary conditions as "overlapping." Spec says "overlapping **or contradictory**" so flagging for review is defensible.
4. **DIP112 uses topological flow analysis** — Exceeds spec; catches non-upstream writes that a global check would miss.

### All Tests Pass, All Checks Clean

```
go vet ./...         → clean
go test ./...        → PASS
validator coverage   → 98.9%
ir coverage          → 100%
```

---

**STATUS: success**

The implementation is solid. All 9 validator rules and 12 lint rules match the spec. Tests are comprehensive (79 total test functions, 98.9%+ coverage). Code is idiomatic Go, integrates correctly with `ir/`, and handles edge cases gracefully. The minor deviations are justified and well-reasoned.

Session 23165558 completed in 1m12s
Turns: 10 | Tool calls: 20 (bash: 7, read: 13)
Tokens: 2259 (in: 12, out: 2247) | Cost: $0.17
Longest turn: 29s
