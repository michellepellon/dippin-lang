TURN 1
TOOL CALL: bash
INPUT:
{"command": "cat .tracker/current_context.md"}
TOOL RESULT: bash
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

TOOL CALL: bash
INPUT:
{"command": "cat .tracker/ledger.md 2>/dev/null || echo \"No ledger found\""}
TOOL RESULT: bash
OUTPUT:
No ledger found

TURN 2
TOOL CALL: bash
INPUT:
{"command": "cat ../DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
stderr: cat: ../DIPPIN_DESIGN_PLAN.md: No such file or directory

exit code: 1
TURN 3
TOOL CALL: bash
INPUT:
{"command": "find / -name \"DIPPIN_DESIGN_PLAN.md\" -maxdepth 4 2>/dev/null | head -5"}
TOOL RESULT: bash
OUTPUT:
(no output)
TOOL CALL: bash
INPUT:
{"command": "find . -name \"*.md\" | head -20"}
TOOL RESULT: bash
OUTPUT:
./.tracker/current_plan.md
./.tracker/current_context.md
./.tracker/runs/794b94d438fb/PlanComponent/prompt.md
./.tracker/runs/794b94d438fb/PlanComponent/response.md
./.tracker/runs/51dba511ab85/PlanComponent/prompt.md
./.tracker/runs/51dba511ab85/PlanComponent/response.md
./.tracker/runs/e9f45ce02855/PlanComponent/prompt.md
./.tracker/runs/e9f45ce02855/PlanComponent/response.md
./.tracker/runs/e9f45ce02855/ReviewClaude/prompt.md
./.tracker/runs/e9f45ce02855/ReviewClaude/response.md
./.tracker/runs/e9f45ce02855/ImplementGemini/prompt.md
./.tracker/runs/e9f45ce02855/ImplementGemini/response.md
./.tracker/runs/e9f45ce02855/ReviewGPT/prompt.md
./.tracker/runs/e9f45ce02855/ReviewGPT/response.md
./.tracker/runs/e9f45ce02855/ReviewAnalysis/prompt.md
./.tracker/runs/e9f45ce02855/ReviewAnalysis/response.md
./.tracker/runs/e9f45ce02855/ImplementClaude/prompt.md
./.tracker/runs/e9f45ce02855/ImplementClaude/response.md
./.tracker/runs/e9f45ce02855/ReviewGemini/prompt.md
./.tracker/runs/e9f45ce02855/ReviewGemini/response.md

TURN 4
TOOL CALL: bash
INPUT:
{"command": "find / -name \"DIPPIN*\" -maxdepth 5 2>/dev/null | head -10"}
TOOL RESULT: bash
OUTPUT:
Tool error (bash): command timed out after 10s
ERROR:
true
TOOL CALL: bash
INPUT:
{"command": "ls -la ../ 2>/dev/null; ls -la . 2>/dev/null"}
TOOL RESULT: bash
OUTPUT:
total 16
drwxr-xr-x  14 clint  staff   448 Mar 19 19:24 .
drwxr-xr-x  50 clint  staff  1600 Mar 18 20:03 ..
-rw-r--r--@  1 clint  staff  6148 Mar  4  2025 .DS_Store
drwxr-xr-x  31 clint  staff   992 Feb  5  2025 botkeeper
drwxr-xr-x@ 34 clint  staff  1088 Mar 16 09:44 chatalpha
drwxr-xr-x@  7 clint  staff   224 Mar 19 20:00 dippin
drwxr-xr-x@ 12 clint  staff   384 Mar 19 20:33 dippin-lang
drwxr-xr-x  15 clint  staff   480 Jul  9  2024 findbuybot
drwxr-xr-x@ 38 clint  staff  1216 Dec 22 21:31 jeff
drwxr-xr-x@ 31 clint  staff   992 Mar 28  2025 llm-pragma-agent
drwxr-xr-x  34 clint  staff  1088 Apr  9  2025 mbus
drwxr-xr-x  37 clint  staff  1184 Feb 22 17:33 pragma_agent
drwxr-xr-x  37 clint  staff  1184 Oct 24  2024 vision-buy
drwxr-xr-x  21 clint  staff   672 Oct 30  2024 vision-buy-web
total 168
drwxr-xr-x@ 12 clint  staff    384 Mar 19 20:33 .
drwxr-xr-x  14 clint  staff    448 Mar 19 19:24 ..
drwxr-xr-x@ 12 clint  staff    384 Mar 19 20:44 .git
drwxr-xr-x@  6 clint  staff    192 Mar 19 20:44 .tracker
-rw-r--r--@  1 clint  staff  15534 Mar 19 20:02 build_dippin.dot
drwxr-xr-x@  3 clint  staff     96 Mar 19 19:24 cmd
-rw-r--r--@  1 clint  staff  65276 Mar 19 20:02 DIPPIN_DESIGN_PLAN.md
drwxr-xr-x@  4 clint  staff    128 Mar 19 20:35 export
drwxr-xr-x@  4 clint  staff    128 Mar 19 20:27 formatter
-rw-r--r--@  1 clint  staff     41 Mar 19 19:24 go.mod
drwxr-xr-x@  7 clint  staff    224 Mar 19 19:26 ir
drwxr-xr-x@  9 clint  staff    288 Mar 19 20:17 validator

TURN 5
TOOL CALL: bash
INPUT:
{"command": "cat DIPPIN_DESIGN_PLAN.md"}
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
{"command": "grep -n \"migrat\\|Migrat\\|DOT.*IR\\|DOT.*dip\\|\\.dot.*\\.dip\\|Phase 5\\|parity\" DIPPIN_DESIGN_PLAN.md | head -60"}
TOOL RESULT: bash
OUTPUT:
19:This plan proposes a concrete syntax (two candidates evaluated), a canonical IR, a composition model, tooling strategy, and phased migration path. Every recommendation is grounded in the actual Tracker codebase and the real dotpowers pipelines.
270:**Key property**: The existing engine (`pipeline.Engine`) continues to operate on the IR. The parser is a new frontend. The DOT exporter is a new backend. The engine itself needs minimal changes — mainly accepting IR structs instead of (or in addition to) DOT-parsed structs.
272:### Migration bridge
274:During migration, both paths exist:
276:- `.dot` → DOT Parser (existing) → IR adapter → Engine
472:The engine can then be incrementally migrated to accept `ir.Workflow` directly, or a reverse adapter `ir.Workflow → pipeline.Graph` can be used during transition.
781:| **Migration cost** | Moderate — new syntax to learn | Lower — YAML-adjacent feels familiar | B |
798:| Migration cost | 3 | 4 |
1137:## 16. Migration Strategy
1141:Build `dippin migrate <input.dot> [output.dip]`:
1156:- Prompts with DOT escape artifacts (`\n` → real newlines, `\"` → `"`) — the migration tool should handle most of this automatically
1162:### Phase 2: Behavioral parity validation (weeks 2-3)
1164:Build `dippin validate-migration <old.dot> <new.dip>`:
1174:1. Engine accepts both `.dot` and `.dip` files (detect by extension)
1175:2. CI validates that migrated `.dip` files produce identical IR to original `.dot`
1177:4. Existing pipelines migrated file-by-file with parity checks
1180:### Migration order (by risk)
1188:7. `dotpowers/dotpowers-simple.dot` — first large migration
1216:**Pipeline: `migration_fixture.dot`** — Generate migration test fixtures
1274:- [ ] Test against existing DOT files (parse DOT → IR → export DOT → parse DOT → compare topology)
1277:### Phase 5: Migration Tool (1 week)
1279:- [ ] Implement `dippin migrate <file.dot>` using existing parser + IR + formatter
1280:- [ ] Post-migration cleanup: un-escape prompts, reformat tool commands, add namespace prefixes to conditions
1281:- [ ] `dippin validate-migration <old.dot> <new.dip>` parity checker
1282:- [ ] Migrate all example files; commit both versions during transition
1288:- [ ] Incrementally migrate engine to accept IR directly
1308:1. **Parse all existing patterns**: Every DOT pipeline in the repo can be expressed in Dippin and parsed to equivalent IR
1315:8. **DOT export works**: `dippin export-dot` produces valid, renderable DOT
1316:9. **Migration tool works**: `dippin migrate` converts all example files with no manual edits needed for correct execution
1319:12. **CLI is functional**: `dippin parse`, `dippin validate`, `dippin fmt`, `dippin export-dot`, `dippin migrate`
1381:**Rationale**: In the current engine, condition nodes do nothing — the engine evaluates edge conditions during edge selection regardless of node kind. Making routing a surface-syntax convenience (`route X -> A when ... -> B when ...`) is cleaner than requiring authors to declare a node kind that has no configuration fields. The IR can emit a `NodeCondition` internally for migration compatibility.
1400:4. **Should `reads:`/`writes:` become mandatory in a future version?** Probably yes, once all existing pipelines are migrated and annotated. But that's a v2 decision.
1425:12. **Implement DOT exporter** in `dippin/export/dot.go`
1426:13. **Implement migration tool** in `dippin/migrate/migrate.go`
1427:14. **Migrate first 3 example pipelines** and verify parity
1432:16. **Migrate remaining examples**
1434:18. **Begin adding `reads:`/`writes:` annotations** to migrated pipelines
1467:├── migrate/
1468:│   ├── migrate.go             # DOT → Dippin conversion
1469:│   ├── parity.go              # Behavioral parity checker
1470:│   └── migrate_test.go
1473:│       └── main.go            # CLI: parse, validate, fmt, export-dot, migrate
1476:    ├── ask_and_execute.dip    # Migrated from DOT
1477:    └── consensus_task.dip     # Migrated from DOT

TOOL CALL: bash
INPUT:
{"command": "cat build_dippin.dot"}
TOOL RESULT: bash
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

TURN 7
TOOL CALL: bash
INPUT:
{"command": "sed -n '/^## 16\\. Migration Strategy/,/^## 17/p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
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

TOOL CALL: bash
INPUT:
{"command": "cat ir/ir.go"}
TOOL RESULT: bash
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

TOOL CALL: bash
INPUT:
{"command": "cat ir/edge.go"}
TOOL RESULT: bash
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

TURN 8
TOOL CALL: bash
INPUT:
{"command": "cat ir/lookup.go"}
TOOL RESULT: bash
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

TOOL CALL: bash
INPUT:
{"command": "cat ir/source.go"}
TOOL RESULT: bash
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

TOOL CALL: bash
INPUT:
{"command": "cat formatter/format.go"}
TOOL RESULT: bash
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

TURN 9
TOOL CALL: bash
INPUT:
{"command": "cat export/dot.go"}
TOOL RESULT: bash
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

TURN 10
TOOL CALL: bash
INPUT:
{"command": "sed -n '/^## 15/,/^## 16/p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
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

TOOL CALL: bash
INPUT:
{"command": "sed -n '/^## 6/,/^## 7/p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
## 6. Recommended Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│  .dip file   │────▶│   Parser     │────▶│  Canonical IR    │
│  (Dippin)    │     │  (Go pkg)    │     │  (Go structs)    │
└──────────────┘     └──────────────┘     └────────┬─────────┘
                                                   │
                          ┌────────────────────────┼────────────────┐
                          │                        │                │
                          ▼                        ▼                ▼
                   ┌──────────────┐     ┌──────────────┐   ┌──────────────┐
                   │  Validator   │     │   Engine      │   │ DOT Exporter │
                   │  Linter      │     │  (existing)   │   │              │
                   └──────────────┘     └──────────────┘   └──────────────┘
```

**Key property**: The existing engine (`pipeline.Engine`) continues to operate on the IR. The parser is a new frontend. The DOT exporter is a new backend. The engine itself needs minimal changes — mainly accepting IR structs instead of (or in addition to) DOT-parsed structs.

### Migration bridge

During migration, both paths exist:
- `.dip` → Dippin Parser → IR → Engine
- `.dot` → DOT Parser (existing) → IR adapter → Engine

The IR adapter is a thin function that converts `pipeline.Graph` to the new IR types.

---

## 7. Canonical IR Proposal

TURN 11
TOOL CALL: bash
INPUT:
{"command": "sed -n '/^## 4/,/^## 5/p' DIPPIN_DESIGN_PLAN.md | head -200"}
TOOL RESULT: bash
OUTPUT:
## 4. Semantic Model That Must Survive

These are the **true semantics** extracted from the codebase — things Dippin must represent:

### 4.1 Node kinds (handler semantics)

| Kind | What it does | Must survive |
|------|-------------|-------------|
| `agent` (codergen) | LLM call with tools, agentic loop | Yes — core value |
| `human` (wait.human) | Pause for human input (choice or freeform) | Yes — core value |
| `tool` | Execute shell command | Yes — core value |
| `conditional` | No-op decision point; engine evaluates outgoing edges | No — deferred from v1. Routing handled by conditional edges directly. See §8.3. |
| `parallel` | Fan-out to concurrent branches | Yes |
| `fan_in` | Join parallel branches | Yes |
| `start` / `exit` | Entry/exit terminals | Yes, but explicit declaration, not shape-based |
| `subgraph` | Embed sub-pipeline | Yes — needs improvement |
| `manager_loop` | Placeholder (currently no-op) | No — remove until real semantics exist |

### 4.2 Edge semantics

- **Unconditional**: default flow
- **Conditional**: boolean expression evaluated against context
- **Labeled**: human-readable name, used for human gate choices and edge selection
- **Weighted**: priority hint for edge selection among unconditional edges
- **Restart**: back-edge that triggers downstream clearing and re-execution (see §4.3)

### 4.3 Execution semantics

- **Sequential by default**: one node at a time, walk the graph
- **Parallel via explicit fan-out/fan-in**: component → branches → tripleoctagon
- **Retry with policy**: named policies (standard, aggressive, patient, linear, none), per-node or graph-level max, backoff functions, retry targets, fallback targets
- **Loop restart**: when an edge targets an already-completed node, clear downstream and restart (bounded by `max_restarts`)
- **Goal gates**: certain nodes are "goal gates" — if they fail, the pipeline fails even at exit
- **Checkpoint/resume**: serialize full state after each node, restore with fidelity degradation

#### Loop/restart runtime semantics (precise specification)

This is one of the most implementation-critical behaviors and must be brutally explicit:

1. **Back-edge detection**: When `selectEdge()` resolves to a target node that is already in the checkpoint's `completedSet`, the engine treats this as a loop restart.

2. **Restart counter**: The engine increments `cp.RestartCount` (global, not per-edge). If `RestartCount >= max_restarts` (graph attr, default 5), the pipeline fails with `EventPipelineFailed`.

3. **Downstream clearing**: The engine calls `clearDownstream(targetNode)` which:
   - BFS from target node through all outgoing edges
   - Removes each reachable node from `cp.CompletedNodes`
   - Clears retry counts for each removed node (fresh budgets)
   - Does NOT clear context — all context key-values survive

4. **Context preservation**: Context is fully preserved across restarts. The restarting node sees all context from the previous iteration. This is intentional — it enables iterative refinement (e.g., review feedback feeds back into implementation).

5. **Restart target override**: If `graph.Attrs["restart_target"]` is set, the engine jumps there instead of the edge's target.

6. **Checkpoint behavior**: After clearing downstream, the engine saves a checkpoint at the new current position. If the process crashes during restart, it resumes from the restart point, not the original edge source.

7. **Subgraph interaction (current Tracker behavior)**: In the current engine, subgraph engines have their own `RestartCount`, independent from the parent. **In Dippin v1**, subgraphs are expanded inline at compile time (see §12), so there are no runtime subgraph boundaries. Restart counters are global within the expanded workflow. If runtime subgraph isolation becomes a real need post-v1, it can be added as an execution mode later.

8. **Stats**: Node-local `SessionStats` (turns, tool calls, etc.) are NOT preserved across restarts. Each re-execution of a node produces fresh stats. The trace accumulates entries for every execution (including re-executions).

### 4.4 Context model

- `map[string]string` shared state
- Handlers write via `ContextUpdates`
- Reserved keys for outcome, responses, tool output
- Graph-level attrs auto-injected with `graph.` prefix
- Prompt variable expansion (`$goal`)
- Context injection (append prior outputs to prompts)
- Fidelity-based compaction for long-running pipelines

### 4.5 LLM configuration

- Per-node model and provider override
- Graph-level defaults
- CSS-like stylesheet with selectors (*, shape, .class, #id)
- Reasoning effort levels
- System prompts
- Max turns, command timeout, cache, compaction settings

---

## 5. Legacy Hacks That Should Die

TOOL CALL: bash
INPUT:
{"command": "cat .tracker/current_plan.md 2>/dev/null | head -20"}
TOOL RESULT: bash
OUTPUT:
# Plan: Formatter (`formatter/`)

## Component

**Canonical Formatter**: `ir.Workflow` → Dippin source text (`.dip` format)

This is the inverse of the parser. Given a fully populated IR, emit canonical Dippin syntax that is deterministic and idempotent.

## Design Spec References

- **§8 "Dippin Syntax Proposal A"** — full syntax example and all 15 syntax rules
- **§8.1 "Multiline content"** — prompt/command indented block rules
- **§8.2 "Context variable namespaces"** — `${ns.var}` in prompts (formatter must preserve these as-is)
- **§14 "Formatter"** (under Tooling) — canonical formatting rules:
  - 2-space indentation
  - One blank line between nodes
  - Section comments preserved (NOTE: we don't have comment IR, so this is N/A for v1)
  - Trailing whitespace removed
  - Single trailing newline
  - Canonical field ordering per node kind

TURN 12
TOOL CALL: bash
INPUT:
{"command": "sed -n '/^## 8/,/^## 9/p' DIPPIN_DESIGN_PLAN.md | head -250"}
TOOL RESULT: bash
OUTPUT:
## 8. Dippin Syntax Proposal A: "Indented Graph"

Design principles: graph-native, indentation-structured, minimal punctuation, multiline blocks with heredoc-style delimiters.

### Example: ask-and-execute pipeline

```dippin
workflow ask_and_execute
  goal: "Ask user for a task, implement it, review, ship"
  start: AskUser
  exit: Done

  defaults
    model: claude-opus-4-6
    provider: anthropic
    retry_policy: standard
    fidelity: summary:high

  # ── Phase 1: Gather ──────────────────────────────

  human AskUser
    label: "What would you like to build?"
    mode: freeform

  agent Interpret
    label: "Interpret the request"
    reads: human_response
    writes: plan
    prompt:
      You are a senior software architect.

      Read the user's request below and produce a clear,
      actionable implementation plan.

      ## User Request
      ${ctx.human_response}

  # ── Phase 2: Implement (parallel) ────────────────

  parallel ImplementFanOut -> ImplementClaude, ImplementCodex, ImplementGemini

  agent ImplementClaude
    label: "Implement (Claude)"
    model: gpt-5.4
    provider: openai
    reads: last_response
    prompt:
      Implement the plan from the previous step.
      ${ctx.last_response}

  agent ImplementCodex
    label: "Implement (Codex)"
    model: gpt-5.3-codex
    provider: openai
    reads: last_response
    prompt:
      Implement the plan from the previous step.
      ${ctx.last_response}

  agent ImplementGemini
    label: "Implement (Gemini)"
    model: gemini-3.5-flash
    provider: gemini
    reads: last_response
    prompt:
      Implement the plan from the previous step.
      ${ctx.last_response}

  fan_in ImplementJoin <- ImplementClaude, ImplementCodex, ImplementGemini

  # ── Phase 3: Review ──────────────────────────────

  agent Validate
    label: "Validate implementation"
    goal_gate: true
    auto_status: true
    max_retries: 2
    reads: last_response
    prompt:
      Review the implementations. Run tests.
      Respond with STATUS: success or STATUS: fail.

  human Approve
    label: "Ship it?"
    default: "Yes"

  # ── Edges ─────────────────────────────────────────

  edges
    AskUser -> Interpret
    Interpret -> ImplementFanOut
    ImplementJoin -> Validate
    Validate -> Approve          when ctx.outcome = success
    Validate -> Interpret        when ctx.outcome = fail     label: "retry"  restart: true
    Approve -> Done
```

### Syntax rules

1. **Workflow declaration**: `workflow <name>` at top level
2. **Explicit entry/exit**: `start: <NodeID>` and `exit: <NodeID>` are required fields on the workflow. No inference from declaration order.
3. **Blocks**: `defaults`, `edges` are top-level sections
4. **Nodes**: `<kind> <ID>` starts a node block; indented lines are fields
5. **Fields**: `key: value` — simple values on one line
6. **Multiline values**: `key:` followed by indented block (no quotes needed)
7. **Edges**: `A -> B` with optional `when <condition>`, `label: "text"`, and `restart: true`
8. **Parallel**: `parallel <ID> -> target1, target2, ...`
9. **Fan-in**: `fan_in <ID> <- source1, source2, ...`
10. **Comments**: `#` line comments
11. **Section headers**: `# ── text ──` for visual grouping (ignored by parser)
12. **Variables**: `${namespace.key}` in prompts for context interpolation (see §8.2)
13. **Conditions**: `when <expr>` on edges using namespaced variables
14. **I/O declarations**: `reads:` and `writes:` for advisory context contracts
15. **Route sugar**: Deferred from v1. See §8.3.

### 8.1 Multiline content

Prompts are just indented text blocks after `prompt:`. No escaping needed:

```dippin
  agent MyNode
    prompt:
      You are a code reviewer.

      ## Rules
      - Check for bugs
      - Check for security issues
      - Run `pytest` to validate

      ## Context
      ${ctx.last_response}
```

The indentation of the first content line sets the baseline. All content is dedented by that amount. Empty lines are preserved. No quotes. No escaping. Diffable line-by-line.

Tool command blocks work identically:

```dippin
  tool CheckTests
    label: "Run test suite"
    timeout: 60s
    command:
      #!/bin/sh
      set -eu
      if pytest --tb=short 2>&1; then
        printf 'pass'
      else
        printf 'fail'
        exit 1
      fi
```

### 8.2 Context variable namespaces

All variable references use explicit namespaces, even though the engine still uses a flat `map[string]string` underneath. This makes authoring clearer and diagnostics much better.

| Namespace | What it contains | Examples |
|-----------|-----------------|----------|
| `ctx.` | Runtime context (handler outputs, reserved keys) | `ctx.outcome`, `ctx.last_response`, `ctx.tool_stdout` |
| `graph.` | Workflow-level attributes | `graph.goal` |
| `params.` | Module parameters (composition) | `params.model`, `params.strict` |

**Lowering**: At IR → engine boundary, namespaces are stripped to flat keys. `ctx.outcome` → `outcome`, `graph.goal` → `graph.goal` (already prefixed in current engine), `params.strict` → substituted at expansion time.

**Validation tiers**:
- **Always-known variables** (`ctx.outcome`, `ctx.last_response`, `ctx.human_response`, `ctx.tool_stdout`, `ctx.tool_stderr`, `graph.goal`): validated at parse time, error if misspelled.
- **Declared outputs** (from `writes:` on upstream nodes): validated as warnings if referenced but not declared.
- **Dynamic variables** (everything else): warning-only. Never an error unless it matches no known pattern at all.

### 8.3 Route syntax sugar (deferred from v1)

Route sugar is **deferred from v1**. In v1, all conditional routing is expressed as conditional edges in the `edges` block:

```dippin
  edges
    Validate -> Approve     when ctx.outcome = success
    Validate -> Interpret   when ctx.outcome = fail     restart: true
    Validate -> HumanHelp   when ctx.outcome = retry
```

This avoids ambiguity about whether `route Validate` attaches routing to an existing node, declares a new node, or replaces edge declarations. If route sugar is added post-v1, it should use unambiguous syntax like `route after <NodeID>` to make clear it decorates an existing node's outgoing edges. The IR does not include a `NodeCondition` kind — there is no intermediate condition node, just edges with conditions.

---

## 9. Dippin Syntax Proposal B: "Declarative YAML-Adjacent"

TURN 13
TOOL CALL: bash
INPUT:
{"command": "sed -n '/^## 5/,/^## 6/p' DIPPIN_DESIGN_PLAN.md"}
TOOL RESULT: bash
OUTPUT:
## 5. Legacy Hacks That Should Die

| Current behavior | Why it's a hack | What should replace it |
|-----------------|-----------------|----------------------|
| Shape → handler mapping | Couples visuals to semantics | Explicit `kind:` field |
| Diamond + tool_command → tool handler | Special-case dispatch | Explicit kind |
| Diamond + prompt → codergen + auto_status | Invisible behavior change | Explicit kind + explicit auto_status |
| `\n` encoding in prompts | DOT string limitation | Multiline block syntax |
| Shell scripts in `tool_command` attribute | Unreadable, triple-escaping | Heredoc-style blocks or file references |
| `model_stylesheet` as DOT attribute string | CSS-in-a-string-in-a-string | Top-level stylesheet block (v1.5) or explicit per-node fields (v1) |
| `manager_loop` handler | No-op placeholder | Remove; re-add when designed |
| `Mdiamond`/`Msquare` as start/exit | DOT shape convention | Explicit `start:` / `exit:` declaration |
| Attribute-bag node config | No schema, no validation | Typed fields per node kind |
| `context.tool_stdout` in conditions | Magic variable names, no namespace | Namespaced references: `ctx.tool_stdout` |
| `parallel.results` as JSON in context string | Structured data in flat string map | First-class parallel result model |
| First-node-is-start convention | Reordering file changes semantics | Explicit `start:` declaration |

---

## 6. Recommended Architecture

TOOL CALL: bash
INPUT:
{"command": "# Look at the existing DOT pipeline files referenced in the spec to understand what DOT input format looks like\ncat build_dippin.dot | head -50"}
TOOL RESULT: bash
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

TURN 14
TOOL CALL: bash
INPUT:
{"command": "wc -l build_dippin.dot"}
TOOL RESULT: bash
OUTPUT:
     214 build_dippin.dot

TOOL CALL: bash
INPUT:
{"command": "# Look at the go.mod to see what packages are available\ncat go.mod"}
TOOL RESULT: bash
OUTPUT:
module github.com/2389/dippin

go 1.25.4

TOOL CALL: bash
INPUT:
{"command": "# Let's also check the existing validator to understand diagnostic types\ncat validator/diagnostic.go"}
TOOL RESULT: bash
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

TURN 15
TOOL CALL: write
INPUT:
{"path": ".tracker/current_plan.md", "content": "# Plan: Migration Tool (`migrate/`)\n\n## Component\n\n**Migration Tool**: Parse DOT graph files → `ir.Workflow` → canonical `.dip` source text.\n\nThis is the DOT-to-Dippin converter described in §16 of the design spec. It parses a DOT `digraph` into an IR, applies post-migration cleanup (un-escaping prompts, reformatting commands, adding namespace prefixes to conditions), and emits canonical `.dip` output via the existing `formatter.Format()`.\n\nA secondary function provides structural parity checking: given two `*ir.Workflow` values (one from DOT, one from `.dip`), report topology/config differences.\n\n## Design Spec References\n\n- **§16 \"Migration Strategy\"** — Phase 1 (automated conversion) and Phase 2 (behavioral parity validation)\n- **§15 \"DOT Export Strategy\"** — The lossless/lossy tables define the reverse mapping (DOT → IR)\n- **§5 \"Legacy Hacks That Should Die\"** — Shape→handler mapping, `\\n` encoding, escape conventions\n- **§4.1 \"Node kinds\"** — shape-to-kind mapping: `box`→`agent`, `hexagon`→`human`, `parallelogram`→`tool`, `component`→`parallel`, `tripleoctagon`→`fan_in`, `tab`→`subgraph`, `diamond`→varies (see below)\n- **§4.2 \"Edge semantics\"** — conditions, labels, weights, restart\n- **§8.2 \"Context variable namespaces\"** — migration must add `ctx.` prefix to bare condition variables\n- **ADR 1** — `restart=true` edge attribute maps to `Edge.Restart`\n- **Appendix A** — `migrate/migrate.go`, `migrate/parity.go`, `migrate/migrate_test.go`\n\n## Files to Create\n\n| File | Purpose |\n|------|---------|\n| `migrate/dot_parser.go` | DOT lexer + parser (DOT language subset) |\n| `migrate/migrate.go` | `Migrate(dotSource string) (*ir.Workflow, error)` — DOT string → IR with cleanup |\n| `migrate/parity.go` | `CheckParity(a, b *ir.Workflow) []Difference` — structural comparison |\n| `migrate/migrate_test.go` | Comprehensive tests |\n\n## Dependencies\n\n- `ir/` — IR types (`Workflow`, `Node`, `Edge`, `Condition`, all config types)\n- `formatter/` — `Format(*ir.Workflow) string` for producing final `.dip` output\n- `validator/` — Optional; can validate the produced IR, but NOT a hard dependency for the migrate package itself\n- **No external dependencies** — DOT parser is hand-written (the DOT language subset we need is small)\n\n## Architecture\n\n### DOT Parser (`dot_parser.go`)\n\nWe need a minimal DOT parser that handles the subset of DOT used by Tracker pipelines. This is NOT a full Graphviz DOT parser — it handles:\n\n- `digraph <name> { ... }` wrapper\n- Graph-level attributes: `graph [ key=value, ... ];`\n- Node statements: `NodeID [ key=value, ... ];`\n- Edge statements: `NodeID -> NodeID [ key=value, ... ];`\n- C-style comments (`// ...`) and block comments (`/* ... */`)\n- Double-quoted strings with escape sequences (`\\\"`, `\\\\`, `\\n`)\n- Semicolons (optional in real DOT, but our files use them)\n- Attributes: `key=value` or `key=\"value\"` \n\n**NOT supported** (not used in our DOT files):\n- `subgraph` blocks (these are expanded inline in our files)\n- Port syntax (`:port`)\n- HTML labels (`<...>`)\n- Multiple edge targets in one statement (`A -> B -> C`)\n\nTypes:\n```go\n// dotGraph holds the parsed DOT structure before IR conversion.\ntype dotGraph struct {\n    Name       string\n    GraphAttrs map[string]string\n    NodeAttrs  map[string]string   // default node attrs\n    EdgeAttrs  map[string]string   // default edge attrs  \n    Nodes      []dotNode\n    Edges      []dotEdge\n}\n\ntype dotNode struct {\n    ID    string\n    Attrs map[string]string\n}\n\ntype dotEdge struct {\n    From  string\n    To    string\n    Attrs map[string]string\n}\n\nfunc parseDOT(input string) (*dotGraph, error)\n```\n\n### Migrator (`migrate.go`)\n\nThe core conversion pipeline:\n\n```go\n// Migrate parses a DOT digraph string and produces a Dippin IR workflow.\n// It applies all post-migration cleanup automatically:\n// - Shape → node kind mapping\n// - \\n and \\\" un-escaping in prompts and commands\n// - Namespace prefixing for condition variables (bare \"outcome\" → \"ctx.outcome\")\n// - Start/exit identification from Mdiamond/Msquare shapes\n// - Graph-level attribute extraction to WorkflowDefaults\nfunc Migrate(dotSource string) (*ir.Workflow, error)\n\n// MigrateToSource parses DOT and returns canonical .dip source text.\n// Convenience function equivalent to: formatter.Format(Migrate(dotSource))\nfunc MigrateToSource(dotSource string) (string, error)\n```\n\n#### Shape → Kind Mapping (reverse of §15 table)\n\n| DOT shape | IR NodeKind | Notes |\n|-----------|-------------|-------|\n| `box` | `agent` | Default if no shape |\n| `hexagon` | `human` | |\n| `parallelogram` | `tool` | |\n| `component` | `parallel` | |\n| `tripleoctagon` | `fan_in` | |\n| `tab` | `subgraph` | |\n| `Mdiamond` | — | Identifies `start` node (not a real kind) |\n| `Msquare` | — | Identifies `exit` node (not a real kind) |\n| `diamond` | Special handling | Per §5: if has `tool_command` → `tool`; if has `prompt` + `auto_status` → `agent`; otherwise ignore (deferred condition node) |\n| (missing) | `agent` | Default when shape not specified |\n\n#### Post-Migration Cleanup Steps\n\n1. **Un-escape DOT strings**: `\\n` → real newline, `\\\"` → `\"`, `\\\\` → `\\`\n2. **Condition namespace prefixing**: Parse condition strings and prefix bare variable names:\n   - Known context vars (`outcome`, `last_response`, `human_response`, `tool_stdout`, `tool_stderr`) → `ctx.` prefix\n   - `graph.` prefixed vars → keep as-is\n   - Other bare vars → `ctx.` prefix (best guess for migration)\n3. **Graph attribute mapping**:\n   - `goal` → `Workflow.Goal`\n   - `rankdir` → ignored (presentation-only)\n   - `default_max_retry` / `max_retries` → `Defaults.MaxRetries`\n   - `max_restarts` → `Defaults.MaxRestarts`\n   - `default_fidelity` / `fidelity` → `Defaults.Fidelity`\n   - `model` → `Defaults.Model`\n   - `provider` → `Defaults.Provider`\n4. **Node attribute mapping**:\n   - `label` → `Node.Label`\n   - `prompt` → `AgentConfig.Prompt` (with un-escaping)\n   - `tool_command` → `ToolConfig.Command` (with un-escaping)\n   - `model` → `AgentConfig.Model`\n   - `provider` → `AgentConfig.Provider`\n   - `max_retries` → `RetryConfig.MaxRetries`\n   - `retry_policy` → `RetryConfig.Policy`\n   - `retry_target` → `RetryConfig.RetryTarget`\n   - `fallback_target` → `RetryConfig.FallbackTarget`\n   - `goal_gate` (true) → `AgentConfig.GoalGate`\n   - `auto_status` (true) → `AgentConfig.AutoStatus`\n   - `reasoning_effort` → `AgentConfig.ReasoningEffort`\n   - `fidelity` → `AgentConfig.Fidelity`\n   - `timeout` → `ToolConfig.Timeout` (parse duration string)\n   - `mode` → `HumanConfig.Mode`\n   - `default` → `HumanConfig.Default`\n   - `targets` → `ParallelConfig.Targets` (comma-separated)\n   - `sources` → `FanInConfig.Sources` (comma-separated)\n   - `ref` → `SubgraphConfig.Ref`\n   - `max_turns` → `AgentConfig.MaxTurns`\n   - `cmd_timeout` → `AgentConfig.CmdTimeout`\n   - `cache_tools` → `AgentConfig.CacheTools`\n   - `compaction` → `AgentConfig.Compaction`\n   - `system_prompt` → `AgentConfig.SystemPrompt`\n5. **Edge attribute mapping**:\n   - `label` → `Edge.Label`\n   - `condition` → `Edge.Condition` (parse + add namespaces)\n   - `weight` → `Edge.Weight`\n   - `restart` (true) → `Edge.Restart`\n6. **Parallel/fan_in inference**: If a node has shape `component`, detect targets from outgoing edges. If a node has shape `tripleoctagon`, detect sources from incoming edges.\n\n#### Condition Parsing for Migration\n\nConditions in DOT are raw strings like `outcome=success`, `tool_stdout contains pass`, `outcome=success && tool_stdout contains done`. We need a minimal parser that:\n\n1. Splits on `&&` and `||` (with proper precedence)\n2. Handles `not` / `!` prefix\n3. Handles comparison operators: `=`, `!=`, `contains`, `startswith`, `endswith`, `in`\n4. Adds namespace prefixes to bare variable names\n5. Produces `ir.Condition` with both `Raw` and `Parsed` fields\n\n```go\nfunc parseCondition(raw string) (*ir.Condition, error)\nfunc addNamespacePrefix(variable string) string\n```\n\n### Parity Checker (`parity.go`)\n\n```go\n// Difference describes a structural difference between two workflows.\ntype Difference struct {\n    Kind    string // \"node_missing\", \"edge_missing\", \"config_mismatch\", \"topology_diff\", etc.\n    Message string // Human-readable description\n    PathA   string // Location in workflow A (e.g., \"node:Validate\")\n    PathB   string // Location in workflow B (may be empty)\n}\n\n// CheckParity compares two workflows for structural equivalence.\n// It checks:\n// - Same node IDs and kinds\n// - Same edges (from/to/conditions)\n// - Same start/exit\n// - Compatible node configurations (prompt content modulo whitespace)\n// - Same graph-level defaults\nfunc CheckParity(a, b *ir.Workflow) []Difference\n```\n\n## Test Cases (22+ cases)\n\n### DOT Parser Tests\n\n1. **Simple digraph** — `digraph G { A -> B; }` → 2 nodes, 1 edge\n2. **Node with attributes** — `A [shape=box, label=\"My Agent\"];` → correct attrs\n3. **Edge with attributes** — `A -> B [label=\"yes\", condition=\"outcome=success\"];` → correct attrs\n4. **Graph attributes** — `graph [goal=\"test\", rankdir=LR];` → correct extraction\n5. **Quoted strings with escapes** — `label=\"line1\\nline2\\\"quoted\\\"\"` → proper un-escaping\n6. **Comments** — `// comment` and `/* block */` correctly skipped\n7. **Empty graph** — `digraph G {}` → no nodes, no edges\n8. **Multiple edges** — `A -> B; A -> C; B -> C;` → 3 edges\n9. **Missing semicolons** — `A -> B` without `;` should still parse (DOT allows it)\n10. **Malformed DOT** — returns descriptive error (unclosed quote, missing brace, etc.)\n\n### Migration Tests\n\n11. **Full pipeline migration** — `build_dippin.dot`-style input → correct IR with all node kinds, edges, conditions\n12. **Shape to kind mapping** — each DOT shape maps to the correct `ir.NodeKind`\n13. **Start/exit identification** — `Mdiamond` → `Workflow.Start`, `Msquare` → `Workflow.Exit`\n14. **Prompt un-escaping** — `\"line1\\nline2\\n\\\"code\\\"\"` → `\"line1\\nline2\\n\\\"code\\\"\"` (real newlines, unquoted)\n15. **Tool command un-escaping** — `tool_command=\"set -eu\\necho hello\"` → multiline command\n16. **Condition namespace prefixing** — `outcome=success` → `ctx.outcome = success`; `graph.goal` stays\n17. **Complex condition** — `outcome=success && tool_stdout contains pass` → `CondAnd{CondCompare{ctx.outcome, =, success}, CondCompare{ctx.tool_stdout, contains, pass}}`\n18. **Condition with negation** — `not outcome=fail` → `CondNot{CondCompare{ctx.outcome, =, fail}}`\n19. **Restart edge** — `A -> B [restart=true]` → `Edge.Restart = true`\n20. **Graph defaults extraction** — `graph [model=\"claude-opus-4-6\", max_restarts=7]` → `WorkflowDefaults`\n21. **Parallel node inference** — `component` shape node with outgoing edges → `ParallelConfig.Targets`\n22. **Fan-in node inference** — `tripleoctagon` shape node with incoming edges → `FanInConfig.Sources`\n23. **Diamond disambiguation** — diamond with `tool_command` → tool; diamond with `prompt` → agent\n24. **Weight on edge** — `A -> B [weight=10]` → `Edge.Weight = 10`\n25. **Duration parsing** — `timeout=\"30s\"` → `30 * time.Second`, `timeout=\"1h30m\"` → `90 * time.Minute`\n26. **Empty/nil handling** — Node with no attributes → agent with empty config (default kind)\n27. **MigrateToSource round-trip** — DOT → IR → `.dip` source; verify output is valid Dippin format\n\n### Parity Checker Tests\n\n28. **Identical workflows** — returns empty differences\n29. **Missing node** — workflow B missing a node → reports `node_missing`\n30. **Extra node** — workflow B has an extra node → reports `node_extra`\n31. **Different start/exit** — reports `start_mismatch` / `exit_mismatch`\n32. **Edge missing** — reports `edge_missing`\n33. **Config mismatch** — different prompt text → reports `config_mismatch`\n34. **Kind mismatch** — same node ID, different kind → reports `kind_mismatch`\n35. **Whitespace-tolerant prompt comparison** — prompts that differ only in whitespace → no difference\n\n### Integration Test\n\n36. **build_dippin.dot migration** — Parse the actual `build_dippin.dot` file in the repo, migrate it, verify the IR has all expected nodes/edges/attributes. This is the real-world integration test.\n\n## Implementation Notes\n\n### DOT String Un-escaping\n\nDOT uses `\\n` for newline, `\\\"` for quote, `\\\\` for backslash. During migration:\n- `\\n` → literal newline character (`\\n`)\n- `\\\"` → literal quote (`\"`)\n- `\\\\` → literal backslash (`\\`)\n- `\\l` → literal newline (DOT left-justified line break)\n- `\\r` → ignore (DOT carriage return, not meaningful)\n\n### Workflow Name Extraction\n\nThe DOT `digraph <name>` becomes `Workflow.Name`. If the name is quoted, strip quotes.\n\n### Node Ordering\n\nPreserve the declaration order from the DOT file for `Workflow.Nodes`. This ensures deterministic output and maintains the author's intended reading order.\n\n### Edge Ordering\n\nPreserve edge declaration order from the DOT file for `Workflow.Edges`.\n\n### Start/Exit Node Handling\n\nNodes with `shape=Mdiamond` or `shape=Msquare` are terminal markers:\n- They are NOT added as real nodes in `Workflow.Nodes` (they are syntactic, not semantic)\n- Instead, if `Mdiamond` node has edges pointing TO a real node, that real node becomes `Workflow.Start`\n- If real nodes have edges pointing TO the `Msquare` node, the `Msquare` determines `Workflow.Exit`\n- If `Mdiamond` has a label other than \"Start\", use it for the workflow name if graph name is generic\n- **Alternative approach (simpler)**: Include Start/Exit as agent nodes in the IR with the ID from DOT. Set `Workflow.Start` and `Workflow.Exit` to those IDs. The formatter and other consumers already handle start/exit by ID. This approach is simpler and matches how `build_dippin.dot` works (Start and Exit are real nodes with edges).\n\n**Decision**: Use the simpler approach — keep Start/Exit as nodes in the IR. Their kind will be `agent` (the default when shape doesn't map to another kind, since Mdiamond/Msquare don't have a kind mapping). The formatter knows to emit them because `Workflow.Start`/`.Exit` point to them. The validator requires them to exist.\n\nActually, re-examining: Start/Exit with `Mdiamond`/`Msquare` are terminal markers with no handler logic. They should be `agent` nodes with empty configs. The important thing is `Workflow.Start` and `Workflow.Exit` point to them.\n\n### Parallel/Fan-in Target/Source Inference\n\nFor `component` (parallel) nodes:\n- If the node has a `targets` attribute, use it directly (comma-split)\n- Otherwise, infer targets from outgoing edges of that node\n\nFor `tripleoctagon` (fan_in) nodes:\n- If the node has a `sources` attribute, use it directly (comma-split)\n- Otherwise, infer sources from incoming edges to that node\n\n### Known Context Variables for Namespace Prefixing\n\nPer §8.2, these bare names get `ctx.` prefix:\n- `outcome`\n- `last_response`\n- `human_response`\n- `tool_stdout`\n- `tool_stderr`\n\nVariables already containing a `.` (like `graph.goal`) are left as-is.\n\nThe prefix `context.` in DOT conditions (e.g., `context.tool_stdout`) should be normalized to `ctx.` (per §8.2 namespace table).\n\n### Condition Operator Mapping\n\nDOT conditions use `=` for equality. Dippin IR uses `=` as well. Operators:\n- `=` → `=`\n- `!=` → `!=`\n- `contains` → `contains`\n- `startswith` → `startswith`\n- `endswith` → `endswith`\n- `in` → `in`\n- `&&` → `CondAnd`\n- `||` → `CondOr`\n- `not` / `!` → `CondNot`\n"}
TOOL RESULT: write
OUTPUT:
wrote 15546 bytes to .tracker/current_plan.md
TURN 16
TOOL CALL: bash
INPUT:
{"command": "wc -l .tracker/current_plan.md"}
TOOL RESULT: bash
OUTPUT:
     322 .tracker/current_plan.md

TOOL CALL: bash
INPUT:
{"command": "# Also let's check if there's a parser package we should know about (listed as complete in ledger but no files visible)\nfind . -name '*.go' -path '*/parser/*' 2>/dev/null"}
TOOL RESULT: bash
OUTPUT:
(no output)
TOOL CALL: bash
INPUT:
{"command": "# Verify the existing test suite still passes\ncd /Users/clint/dippin-lang && go test ./... 2>&1"}
TOOL RESULT: bash
OUTPUT:
stderr: sh: line 1: cd: /Users/clint/dippin-lang: No such file or directory

exit code: 1
TURN 17
TOOL CALL: bash
INPUT:
{"command": "go test ./... 2>&1"}
TOOL RESULT: bash
OUTPUT:
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/export	(cached)
ok  	github.com/2389/dippin/formatter	(cached)
ok  	github.com/2389/dippin/ir	(cached)
ok  	github.com/2389/dippin/validator	(cached)

TURN 18
TOOL CALL: bash
INPUT:
{"command": "# Show the variety of node/edge patterns in the real DOT file\ngrep -E '^\\s*(Start|Exit|\\w+)\\s*\\[' build_dippin.dot | head -20"}
TOOL RESULT: bash
OUTPUT:
  graph [
  Start [shape=Mdiamond, label="Start"];
  Exit [shape=Msquare, label="Exit"];
  SetupWorkspace [
  PickNextComponent [
  CheckComplete [
  PlanComponent [
  ImplementParallel [shape=component, label="Implement (Parallel)"];
  ImplementClaude [
  ImplementGPT [
  ImplementGemini [
  ImplementJoin [shape=tripleoctagon, label="Implement Join"];
  ValidateBuild [
  ReviewParallel [shape=component, label="Reviews (Parallel)"];
  ReviewClaude [
  ReviewGPT [
  ReviewGemini [
  ReviewsJoin [shape=tripleoctagon, label="Reviews Join"];
  ReviewAnalysis [
  CommitWork [

TOOL CALL: bash
INPUT:
{"command": "# Show edge patterns\ngrep -E '^\\s*\\w+\\s*->' build_dippin.dot | head -20"}
TOOL RESULT: bash
OUTPUT:
  Start -> SetupWorkspace;
  SetupWorkspace -> PickNextComponent;
  PickNextComponent -> CheckComplete;
  CheckComplete -> Exit [condition="context.tool_stdout=all_complete", label="all done"];
  CheckComplete -> PlanComponent [condition="context.tool_stdout=has_next", label="has work"];
  PlanComponent -> ImplementParallel;
  ImplementParallel -> ImplementClaude;
  ImplementParallel -> ImplementGPT;
  ImplementParallel -> ImplementGemini;
  ImplementClaude -> ImplementJoin;
  ImplementGPT -> ImplementJoin;
  ImplementGemini -> ImplementJoin;
  ImplementJoin -> ValidateBuild;
  ValidateBuild -> ReviewParallel [condition="outcome=success", label="validated"];
  ValidateBuild -> ImplementClaude [condition="outcome=fail", label="fix_build"];
  ReviewParallel -> ReviewClaude;
  ReviewParallel -> ReviewGPT;
  ReviewParallel -> ReviewGemini;
  ReviewClaude -> ReviewsJoin;
  ReviewGPT -> ReviewsJoin;

TURN 19
TOOL CALL: bash
INPUT:
{"command": "grep -E '^\\s*\\w+\\s*->' build_dippin.dot | tail -10"}
TOOL RESULT: bash
OUTPUT:
  ReviewGPT -> ReviewsJoin;
  ReviewGemini -> ReviewsJoin;
  ReviewsJoin -> ReviewAnalysis;
  ReviewAnalysis -> CommitWork [condition="outcome=success", label="pass"];
  ReviewAnalysis -> ImplementClaude [condition="outcome=retry", label="rework"];
  ReviewAnalysis -> FailureSummary [condition="outcome=fail", label="escalate"];
  CommitWork -> MarkComplete;
  MarkComplete -> PickNextComponent [label="next component", loop_restart=true];
  FailureSummary -> MarkFailed;
  MarkFailed -> Exit;

TOOL CALL: bash
INPUT:
{"command": "# Check for multiline attributes (prompt, tool_command) and their escaping patterns\ngrep -c 'tool_command=' build_dippin.dot\ngrep -c 'prompt=' build_dippin.dot\ngrep -c 'loop_restart=' build_dippin.dot\ngrep -c 'restart=' build_dippin.dot"}
TOOL RESULT: bash
OUTPUT:
5
10
1
1

TURN 20
TOOL CALL: bash
INPUT:
{"command": "# Check all attribute names used in edges\ngrep -oP '\\[\\K[^\\]]+' build_dippin.dot | grep -oP '\\w+=' | sort -u"}
TOOL RESULT: bash
OUTPUT:
stderr: grep: invalid option -- P
usage: grep [-abcdDEFGHhIiJLlMmnOopqRSsUVvwXxZz] [-A num] [-B num] [-C[num]]
	[-e pattern] [-f file] [--binary-files=value] [--color=when]
	[--context[=num]] [--directories=action] [--label] [--line-buffered]
	[--null] [pattern] [file ...]
grep: invalid option -- P
usage: grep [-abcdDEFGHhIiJLlMmnOopqRSsUVvwXxZz] [-A num] [-B num] [-C[num]]
	[-e pattern] [-f file] [--binary-files=value] [--color=when]
	[--context[=num]] [--directories=action] [--label] [--line-buffered]
	[--null] [pattern] [file ...]

TOOL CALL: bash
INPUT:
{"command": "# Check all attribute names used in nodes\ngrep -E '^\\s*\\w+\\s*\\[' build_dippin.dot | grep -oP '\\w+=' | sort -u"}
TOOL RESULT: bash
OUTPUT:
stderr: grep: invalid option -- P
usage: grep [-abcdDEFGHhIiJLlMmnOopqRSsUVvwXxZz] [-A num] [-B num] [-C[num]]
	[-e pattern] [-f file] [--binary-files=value] [--color=when]
	[--context[=num]] [--directories=action] [--label] [--line-buffered]
	[--null] [pattern] [file ...]

TURN 21
TOOL CALL: bash
INPUT:
{"command": "# macOS grep doesn't have -P, use awk/sed instead\ngrep '\\[' build_dippin.dot | sed 's/.*\\[//; s/\\].*//; s/,/\\n/g' | sed 's/^ *//; s/=.*//' | sort -u | grep -v '^$'"}
TOOL RESULT: bash
OUTPUT:
-z \"$NEXT\" 
! -f .tracker/ledger.tsv 
condition
label
loop_restart
shape

TURN 22
TOOL CALL: bash
INPUT:
{"command": "# Let's see all the node attributes more precisely\n# Check full node declarations\ngrep -E 'shape=' build_dippin.dot | head -20"}
TOOL RESULT: bash
OUTPUT:
  Start [shape=Mdiamond, label="Start"];
  Exit [shape=Msquare, label="Exit"];
    shape=parallelogram,
    shape=parallelogram,
    shape=diamond,
    shape=box,
  ImplementParallel [shape=component, label="Implement (Parallel)"];
    shape=box,
    shape=box,
    shape=box,
  ImplementJoin [shape=tripleoctagon, label="Implement Join"];
    shape=parallelogram,
  ReviewParallel [shape=component, label="Reviews (Parallel)"];
    shape=box,
    shape=box,
    shape=box,
  ReviewsJoin [shape=tripleoctagon, label="Reviews Join"];
    shape=box,
    shape=box,
    shape=parallelogram,

TOOL CALL: bash
INPUT:
{"command": "# What other attrs do nodes have? Look at multi-line node declarations\nawk '/^\\s*\\w+\\s*\\[/{found=1} found{print} /\\];/{found=0}' build_dippin.dot | grep -E '^\\s+\\w+=' | sed 's/=.*//' | sed 's/^ *//' | sort -u"}
TOOL RESULT: bash
OUTPUT:
(no output)
TURN 23
TOOL CALL: bash
INPUT:
{"command": "# Get all attribute keys from multi-line node blocks\nawk '/^\\s*\\w+\\s*\\[/{found=1; next} /\\];/{found=0} found' build_dippin.dot | grep -E '^\\s+\\w+' | sed 's/^\\s*//' | sed 's/=.*//' | sort -u"}
TOOL RESULT: bash
OUTPUT:
(no output)
TURN 24
TOOL CALL: bash
INPUT:
{"command": "# Let me look at the raw format of multiline node attributes\nsed -n '/ImplementClaude \\[/,/\\];/p' build_dippin.dot"}
TOOL RESULT: bash
OUTPUT:
  ImplementClaude [
    shape=box,
    label="Claude Implementation",
    llm_provider="anthropic",
    llm_model="claude-opus-4-6",
    reasoning_effort="high",
    fidelity="full",
    prompt="Implement the planned component for the Dippin toolchain. Read .tracker/current_plan.md for exactly what to build and .tracker/current_context.md for the current codebase state.\n\nRead the design spec at ../DIPPIN_DESIGN_PLAN.md for exact specifications.\nRead existing code in ir/ to match types precisely.\n\nRules:\n- Write idiomatic Go. Standard library only (no external deps beyond what go.mod already has).\n- Every exported function gets a test. Test files go next to source.\n- Match the IR types in ir/ exactly. Do not modify ir/ unless the plan says to.\n- Table-driven tests with edge cases from the plan.\n- One responsibility per package.\n- Use diagnostic codes from the spec (DIP001-DIP112) where applicable.\n- Write real, complete code — no stubs, no TODOs, no placeholders.\n- Run 'go build ./...' and 'go test ./...' to verify before finishing."
  ];
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

TURN 25
TOOL CALL: bash
INPUT:
{"command": "# Get all unique attribute names in the DOT file\nsed -n '/\\[/,/\\]/p' build_dippin.dot | grep -oE '[a-z_]+=' | sort -u"}
TOOL RESULT: bash
OUTPUT:
comp=
condition=
count=
default_fidelity=
default_max_retry=
fidelity=
goal_gate=
goal=
label=
llm_model=
llm_provider=
loop_restart=
max_restarts=
outcome=
prompt=
rankdir=
reasoning_effort=
retry_target=
shape=
tool_command=
tool_stdout=

TURN 26
TOOL CALL: edit
INPUT:
{"path": ".tracker/current_plan.md", "old_string": "5. **Edge attribute mapping**:\n   - `label` → `Edge.Label`\n   - `condition` → `Edge.Condition` (parse + add namespaces)\n   - `weight` → `Edge.Weight`\n   - `restart` (true) → `Edge.Restart`", "new_string": "5. **Edge attribute mapping**:\n   - `label` → `Edge.Label`\n   - `condition` → `Edge.Condition` (parse + add namespaces)\n   - `weight` → `Edge.Weight`\n   - `restart` (true) → `Edge.Restart`\n   - `loop_restart` (true) → `Edge.Restart` (legacy alias used in real DOT files)"}
TOOL RESULT: edit
OUTPUT:
edited .tracker/current_plan.md
TURN 27
TOOL CALL: edit
INPUT:
{"new_string": "   - `model` / `llm_model` → `AgentConfig.Model` (both forms accepted; `llm_model` is the legacy DOT convention)\n   - `provider` / `llm_provider` → `AgentConfig.Provider` (both forms accepted; `llm_provider` is the legacy DOT convention)", "old_string": "   - `model` → `AgentConfig.Model`\n   - `provider` → `AgentConfig.Provider`", "path": ".tracker/current_plan.md"}
TOOL RESULT: edit
OUTPUT:
edited .tracker/current_plan.md
TURN 28
TOOL CALL: edit
INPUT:
{"new_string": "The prefix `context.` in DOT conditions (e.g., `context.tool_stdout`) should be normalized to `ctx.` (per §8.2 namespace table).\n\nThe prefix `graph.` is kept as-is.\n\n**Real-world example from `build_dippin.dot`**:\n- `condition=\"context.tool_stdout=all_complete\"` → `ctx.tool_stdout = all_complete`\n- `condition=\"outcome=success\"` → `ctx.outcome = success`\n- `condition=\"outcome=fail\"` → `ctx.outcome = fail`\n- `condition=\"outcome=retry\"` → `ctx.outcome = retry`\n\nNote: DOT conditions use `=` with no spaces around the operator. The condition parser should handle both `outcome=success` and `outcome = success`.\n\n### Real-World DOT Attribute Names\n\nFrom examining `build_dippin.dot`, these are the actual attribute names used:\n\n**Node attributes**: `shape`, `label`, `llm_provider`, `llm_model`, `reasoning_effort`, `fidelity`, `prompt`, `tool_command`, `goal_gate`, `retry_target`\n\n**Edge attributes**: `condition`, `label`, `loop_restart`\n\n**Graph attributes**: `goal`, `rankdir`, `default_max_retry`, `default_fidelity`, `max_restarts`\n\nThe migration tool MUST handle these legacy names:\n- `llm_model` → maps to `model` in IR\n- `llm_provider` → maps to `provider` in IR\n- `loop_restart` → maps to `restart` in IR\n- `default_max_retry` → maps to `max_retries` in IR defaults\n- `default_fidelity` → maps to `fidelity` in IR defaults\n- `context.` prefix in conditions → maps to `ctx.` prefix in IR", "old_string": "The prefix `context.` in DOT conditions (e.g., `context.tool_stdout`) should be normalized to `ctx.` (per §8.2 namespace table).", "path": ".tracker/current_plan.md"}
TOOL RESULT: edit
OUTPUT:
edited .tracker/current_plan.md
TURN 29
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
| `diamond` | Special handling | Per §5: if has `tool_command` → `tool`; if has `prompt` + `auto_status` → `agent`; otherwise ignore (deferred condition node) |
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

TURN 30
TOOL CALL: bash
INPUT:
{"command": "# Check if the CheckComplete/diamond node in build_dippin.dot has special handling needs\nsed -n '/CheckComplete/,/\\];/p' build_dippin.dot"}
TOOL RESULT: bash
OUTPUT:
  CheckComplete [
    shape=diamond,
    label="All done?"
  ];
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

TURN 31
TOOL CALL: edit
INPUT:
{"new_string": "| `diamond` | Special handling | Per §5: if has `tool_command` → `tool`; if has `prompt` + `auto_status` → `agent`; if has `prompt` only → `agent`; otherwise → `agent` with no config (routing-only decision node — the engine evaluates outgoing edge conditions, the node itself is a no-op). In v1 Dippin there's no `conditional` kind, so diamond nodes become `agent` nodes. |", "old_string": "| `diamond` | Special handling | Per §5: if has `tool_command` → `tool`; if has `prompt` + `auto_status` → `agent`; otherwise ignore (deferred condition node) |", "path": ".tracker/current_plan.md"}
TOOL RESULT: edit
OUTPUT:
edited .tracker/current_plan.md
TURN 32
TOOL CALL: bash
INPUT:
{"command": "wc -l .tracker/current_plan.md"}
TOOL RESULT: bash
OUTPUT:
     351 .tracker/current_plan.md

TURN 33
TEXT:
The plan is written to `.tracker/current_plan.md` (351 lines). Here's a summary of what it covers:

## Plan Summary: Migration Tool (`migrate/`)

### Component
DOT → IR → `.dip` conversion pipeline (§16 of design spec)

### Files to Create (4 files)
| File | Purpose |
|------|---------|
| `migrate/dot_parser.go` | Minimal DOT lexer + parser for the subset used by Tracker |
| `migrate/migrate.go` | `Migrate()` and `MigrateToSource()` — DOT string → IR with all cleanup |
| `migrate/parity.go` | `CheckParity()` — structural comparison of two workflows |
| `migrate/migrate_test.go` | 36+ test cases |

### Key Design Decisions
1. **Hand-written DOT parser** — only parses the subset we actually use (no subgraphs, no ports, no HTML labels)
2. **Shape → kind mapping** — reverse of the §15 export table, with special handling for `diamond` and `Mdiamond`/`Msquare`
3. **Legacy attribute names** — `llm_model`→`model`, `llm_provider`→`provider`, `loop_restart`→`restart`, `default_max_retry`→`max_retries`, `context.`→`ctx.`
4. **Condition parsing** — handles `=`/`!=`/`contains`/`&&`/`||`/`not` with namespace prefixing
5. **Start/Exit as agent nodes** — simpler approach, consistent with how the rest of the toolchain works

### Test Coverage (36 cases)
- 10 DOT parser tests (parsing, escapes, comments, errors)
- 17 migration tests (shape mapping, un-escaping, conditions, parallel inference, round-trip)
- 8 parity checker tests (identical, missing, extra, mismatch)
- 1 integration test against real `build_dippin.dot`

Session 455945f7 completed in 4m16s
Turns: 33 | Tool calls: 51 (bash: 45, edit: 4, read: 1, write: 1)
Tokens: 14611 (in: 4111, out: 10500) | Cost: $0.85
Longest turn: 1m30s
