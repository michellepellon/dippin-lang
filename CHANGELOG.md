# Changelog

All notable changes to dippin-lang are documented here. Versions follow [semver](https://semver.org/).

## [v0.16.0] — 2026-03-31

### Added
- **Structured output support** for agent nodes. New fields:
  - `response_format`: force LLM to produce structured JSON output (`json_object` or `json_schema`)
  - `response_schema`: inline JSON Schema definition (multiline block, like `prompt:`)
  - `params`: generic key-value pass-through for runtime features (same syntax as subgraph `params`)
- **DIP130**: lint warning for invalid `response_format` value.
- **DIP131**: lint warning when `response_schema` is set without `response_format: json_schema` (schema ignored); hint when `json_schema` is set without a schema.
- **DIP132**: lint warning when `response_schema` is not valid JSON.
- **DIP133**: lint hint when agent `params` key shadows a first-class field (e.g., `model`, `provider`).
- `cmd_timeout` field now parsed and formatted on agent nodes (previously only populated by DOT migrator).

### Fixed
- **Duplicate params keys** now emit a parse diagnostic instead of silently last-write-wins.
- **Unknown defaults fields** now emit a parse diagnostic instead of being silently discarded.
- **`AgentConfig.Params`** initialized to empty map (matching `SubgraphConfig`), preventing nil-pointer issues in downstream consumers.
- **Cyclomatic/cognitive complexity** violations resolved across 6 files (lint_response.go, lint_human.go, parse_nodes.go, format.go, interactive.go).

## [v0.15.0] — 2026-03-31

### Added
- **Interview mode** for human nodes (`mode: interview`). Runtimes extract questions from upstream agent output and present each as an individual form field with optional suggested answers. New fields: `questions_key`, `answers_key`.
- **DIP127**: lint warning for invalid human node mode values.
- **DIP128**: lint warning when interview mode has a meaningless `default` value.
- **DIP129**: lint warning when interview mode has conflicting choice-style labeled edges.
- Integration guide updated with interview mode implementation guidance and recommended answer JSON schema.
- `api_design.dip` example updated to use interview mode for Q&A collection.
- **`interview_loop.dip`** example: reusable interview subgraph with iterative Q&A. Parameterized by topic and focus areas. LLM generates questions with suggested options, human answers via interview mode, assessor loops until requirements are clear. Grade A, ~$0.92/run.
- **3 blog posts**: Multi-line Prompts Without Escaping (deep dive), Conditional Edges (tutorial), Cost Estimation (tutorial). Hub-and-spokes model with cross-links.
- **Auto-deploy**: CI now deploys `site/` to GitHub Pages on successful main builds.

### Fixed
- `--version` / `-version` flags now work (previously failed with "flag provided but not defined").
- **Formatter idempotency**: subgraph param values with quotes (e.g., `"API design"`) were double-quoted on each format pass. Parser now strips surrounding quotes from param values.

## [v0.14.0] — 2026-03-27

### Added
- **`code_health_check.dip`** example: self-contained pipeline that audits a Go repo. Gathers context with shell tools, runs vet/staticcheck/tests in parallel, three-model independent review, synthesized report with quality gate and retry loop. 5 test scenarios. Grade A, ~$1/run.

## [v0.13.2] — 2026-03-27

### Changed
- **Single-source nav**: `site/_layout/nav.html` is the one source of truth. `scripts/sync-nav.sh` propagates it to all 16 pages with correct prefixes and active states. Pre-commit hook runs it automatically. No more editing nav in 16 files.
- `scripts/gen-changelog-html.sh` emits a placeholder nav that `sync-nav.sh` fills.
- `just sync-nav` recipe added.

## [v0.13.1] — 2026-03-27

### Fixed
- `just install` and `just build` now inject commit hash and build timestamp via ldflags. `dippin version` shows `dev (commit: abc1234, built: 2026-03-27T18:45:10Z)` instead of `dev (commit: none, built: unknown)`.

## [v0.13.0] — 2026-03-27

### Changed
- **Two-tier navigation** across all site pages. Top row: Docs, Playground, Blog, GitHub. Bottom row: CLI, Language, Testing, Validation, Analysis, Architecture, Editors, Changelog. Mobile collapses to hamburger with divider-separated groups.

### Fixed
- Mobile nav menu no longer renders as unstyled text on desktop (missing `display: none`).
- Blog index only shows the 5 published posts — removed 20 dead links to unwritten articles.
- Playground content no longer overlaps the nav bar (padding adjusted for two-tier height).
- Playground now has the floating dots background matching all other pages.
- Homepage "See all 25 posts" corrected to "All posts".
- Section spacing tightened (6rem → 4.5rem padding).
- Tracker team field report response written (`.tracker/field-report-response-2026-03-27.md`).

## [v0.12.0] — 2026-03-27

### Added
- **Blog section** with 25 planned post cards and topic filtering (Guides, Tutorials, Deep Dives, Reference).
- **5 blog posts** published: Getting Started, Scenario Testing, Migrating from DOT, CI Integration, Editor Setup. Edited for voice, clarity, and inline links.
- **Homepage "From the Blog"** section featuring 3 latest posts below the fold.
- **SEO meta tags** on all 12 site pages: Open Graph, Twitter Cards, descriptions, canonical URLs. Pages render rich previews when shared.
- **Blog ideas doc** (`docs/blog-ideas.md`) with 25 post synopses, coverage plans, and approach notes.
- Blog nav link added to all site pages.

## [v0.11.2] — 2026-03-27

### Fixed
- **Playground**: syntax-highlighted editor with transparent textarea over colored `<pre>` overlay. Tab key inserts 2 spaces.
- **Playground**: parse output shows highlighted JSON (keys, strings, booleans, numbers). Format output shows highlighted Dippin. Lint errors display with severity coloring.
- **Playground**: WASM race condition — polls for function registration before auto-linting on load. Returns `[]` not `null` for zero diagnostics.
- **Site**: syntax highlighting CSS selectors changed from `pre .hl-*` to `.hl-*` so colors work in playground output div, not just `<pre>` blocks.
- **Site**: JSON blocks inside `compare-code` divs (like gate.test.json on Testing page) now get highlighted — skip logic checks for existing `<span>` tags instead of parent class.
- **Site**: highlight.js token protection via `\x00N\x00` placeholders prevents regex passes from matching inside previously generated `<span>` class attributes.
- **Site**: JSON inside terminal output blocks (e.g. `$ dippin --format json test`) gets JSON highlighting applied to the embedded body.
- **Site**: changelog auto-generated from CHANGELOG.md via `scripts/gen-changelog-html.sh`. Pre-commit hook runs it when CHANGELOG.md is staged.

## [v0.11.1] — 2026-03-27

### Fixed
- **Playground**: WASM files (`dippin.wasm`, `wasm_exec.js`) now deployed to gh-pages so the playground actually loads.
- **Playground**: auto-runs lint on WASM load instead of showing a confusing "Ready" message while the Lint button appears active.
- **Site**: syntax highlighting (`highlight.js`) for all code blocks — Dippin, shell, terminal, and diagnostic output.
- **Site**: changelog page added at `changelog.html` with full version history.

## [v0.11.0] — 2026-03-27

### Added
- **DIP126** lint rule: subgraph `ref:` file validation — warns when referenced workflow file does not exist on disk.
- **`dippin watch`** command: file watcher that re-runs lint on `.dip` changes with 200ms debounce. Uses `fsnotify`.
- **`dippin test --coverage`** flag: edge coverage summary showing which workflow edges were/weren't traversed by test scenarios.
- **Tree-sitter grammar** scaffolding in `editors/tree-sitter-dippin/` — grammar.js, external scanner for indentation, highlight queries, and test corpus. Enables proper syntax highlighting in Neovim, Helix, and Zed.
- **WASM playground** at `site/playground.html` — browser-based editor with live parse, lint, and format via WebAssembly. Build with `just wasm`.
- `gemini-3.1-pro-preview-customtools` added to model catalog and pricing tables.
- 35 diagnostic codes total (was 34).

### Fixed
- **CI failures**: golangci-lint `errcheck` on `f.Close()`, `funlen` on `buildLintExplanations` (split into 4 functions), misspell false positive in DIP118 example.
- **Migration parity**: `consensus_task_parity.dip` and `semport_thematic.dip` model names now match DOT originals (`gemini-3.1-pro-preview-customtools`).

### Changed
- `validator/lint_tool_cmd.go` split with `//go:build !wasm` / `wasm` tags — `bash -n` syntax check and `exec.LookPath` binary check are no-ops in WASM.
- `validator/lint_subgraph.go` similarly gated for WASM (no `os.Stat`).
- Site mobile CSS improvements: table overflow handling, code word-break, install-cmd sizing.
- Site nav updated with Playground link across all pages.

### Documentation
- All references updated from 34→35 codes, DIP101–DIP125→DIP101–DIP126 across README, CLAUDE.md, docs/, and site/.
- `docs/validation.md` — full entry for DIP126.
- `docs/cli.md` — `watch` command section, `test --coverage` flag.
- `docs/editor-setup.md` — tree-sitter grammar availability.
- `dippin explain DIP126` — explanation with trigger and fix guidance.
- `mode: labeled` documented as not supported in `docs/nodes.md`.

## [v0.10.0] — 2026-03-26

### Added
- **DIP123** lint rule: tool command shell syntax errors detected via `bash -n`.
- **DIP124** lint rule: `${ctx.*}` references in tool commands that expand to empty at runtime.
- **DIP125** lint rule (hint): tool command binary not found on PATH (environment-dependent).
- **Brochure site** with 8 pages: home, CLI, Language Reference, Testing, Validation, Analysis, Architecture, Editor Setup. Hosted on GitHub Pages.
- 34 diagnostic codes total (was 31).

### Documentation
- All references updated from 31→34 codes, DIP101–DIP122→DIP101–DIP125 across README, CLAUDE.md, docs/, and site/.
- `docs/validation.md` — full entries for DIP123, DIP124, DIP125.
- `dippin explain DIP123/DIP124/DIP125` — explanations with triggers and fix guidance.

## [v0.9.0] — 2026-03-25

### Fixed
- **`preferred_label` now works on human gates** — scenario key `preferred_label` (or per-node `Gate.preferred_label`) matches against edge labels (case-insensitive substring). Previously silently ignored on freeform gates.
- **`prompt:` blocks now parse on human nodes** — `HumanConfig` gained a `Prompt` field. Multiline prompt blocks work the same as on agent nodes. Formatter round-trips correctly.
- **Tool auto-defaults no longer mask fallback edges** — empty-string scenario values (`"Node.tool_stdout": ""`) now suppress the auto-seeded `success` default, allowing unconditional fallback edges to fire.

### Added
- **`immediately_after` test assertion** — assert adjacency in the execution path: `"immediately_after": {"NodeX": "NodeY"}` checks that NodeY is the very next node after NodeX.
- **`branch` field for targeted parallel testing** — `"branch": ["WorkerA"]` limits which parallel fan-out branches are simulated. Without it, all branches are walked (new default).
- **Simulator walks all parallel branches** — parallel fan-out now visits all targets, not just the first. Matches real runtime behavior.
- **Example test suites** — `.test.json` files for `vulnerability_analyzer`, `consensus_task`, `code_quality_sweep`, and `sprint_exec` (20 tests across 4 workflows).
- **Test coverage at 95.7%** — up from 85.6%. Six packages at 100%.
- `just cover` now excludes untestable files (`main.go`, `cmd_lsp.go`) from coverage reports.

### Documentation
- `docs/testing.md` — added Caveats section (`not_visited` fragility with loop-breaking), Clearing Defaults section (empty-string technique), `immediately_after` field documentation.

## [v0.8.0] — 2026-03-25

### Fixed
- **Graph truncation on pipelines with restart edges** — `buildAdjacency()` included restart (back) edges, creating cycles that prevented Kahn's algorithm from assigning layers to downstream nodes. All nodes are now rendered. Affects both full and compact modes.
- **Simulator infinite loop on tool-gated loops** — pipelines with `when ctx.tool_stdout not contains all-done` loops would spin to the 500-step limit. New `MaxNodeVisits` option forces the loop-exit edge after N visits. The test runner sets this to 3 by default.
- **Per-node scenario injection in `dippin test`** — `NodeName.key=value` scenarios now work reliably because the loop-breaking fix allows the simulation to reach the target node.
- **Testrunner accepts empty/invalid schemas silently** — `LoadTestFile` now rejects `.test.json` files with zero tests.

### Added
- **CLI integration tests** — 32 new tests covering 10 previously untested commands (cost, coverage, doctor, optimize, unused, graph, diff, feedback, explain, test). `cmd/dippin` coverage: 44.9% → 79.3%.
- **Graph tests for parallel and restart-loop fixtures**.
- **DIP121 compound condition test** — verifies `and`/`or` conditions correctly fire per-variable.
- **Unused clean-workflow test** — verifies no false positives on linear workflows.
- `just release tag msg` recipe for tagging releases.
- DIP121/DIP122 added to README warnings table; `explain`, `unused`, `graph`, `test` added to commands table.

### Fixed (cosmetic)
- **README stale numbers** — diagnostic codes 30→34, DIP120→DIP125, examples 15→17, lint rules 21→25.
- **`appendConnector` dead branch** — identical if/else branches collapsed.

## [v0.7.0] — 2026-03-25

### Added
- **`dippin test`** — scenario test runner for workflow assertions. Define `.test.json` files alongside `.dip` workflows with expected status, visited/not-visited nodes, and path ordering. Supports `--verbose` flag for path tracing and JSON output for CI integration.
- **New package:** `testrunner/` — loads `.test.json` suites, runs each case through the simulator with injected scenario values, checks assertions against results.
- **New doc:** `docs/testing.md` — documents the `.test.json` format and test runner usage.

## [v0.6.0] — 2026-03-25

### Added
- **DIP121** lint rule: condition references variable not produced by source node's `IO.Writes`. Skips when writes are empty (advisory) or variable is a reserved runtime key (`ctx.outcome`, `ctx.status`, `ctx.internal.*`, `graph.*`, `params.*`).
- **DIP122** lint rule: condition tests value not declared in source tool's `ToolConfig.Outputs`. Only fires for tool nodes with explicitly declared outputs.
- Explanations for DIP121/DIP122 in `dippin explain`.

## [v0.5.0] — 2026-03-25

### Added
- **`dippin explain <DIPxxx>`** — rich explanations for all 34 diagnostic codes. Shows trigger conditions, fix guidance, and example snippets. Supports text and JSON output.
- **`dippin unused <file>`** — detects dead-branch nodes (reachable from start but no path to exit) and estimates wasted cost per run. Reuses `coverage.Analyze()` sink detection + `cost.Analyze()` for cost enrichment.
- **`dippin graph [--compact] <file>`** — terminal-rendered ASCII DAG visualization. Full mode renders box-drawing nodes with connectors; compact mode outputs single-line `[A] → [B] → [C]` format. JSON mode outputs layer structure.
- **New packages:** `unused/`, `graph/`, `testrunner/`
- **New files:** `validator/explanations.go` with `Explanation` struct for all DIP codes.

## [v0.4.3] — 2026-03-25

### Fixed
- **DIP101 suppressed for mixed routing** — when a source node has both unconditional and conditional outgoing edges, the conditional branches are intentional routing. DIP101 no longer fires on their destinations. Covers all four reported patterns: compound inequality conditions, exhaustive set + fallback, mixed unconditional/conditional, and labeled fallback edges.

## [v0.4.2] — 2026-03-25

### Fixed
- **DIP101/DIP102 exhaustive detection** now recognizes any complete partition — if all conditional edges from a node test the same variable with equality (2+ values), the conditions are treated as exhaustive. No longer limited to hardcoded `{success, fail}` pairs. Handles `done/more_questions`, `tasks_remain/all_done`, and any custom value set.

## [v0.4.1] — 2026-03-25

### Fixed
- **EBNF grammar** audited against parser — added infix negation, tool `outputs` field, removed undocumented numeric operators (`<`, `>`, `<=`, `>=` parsed but silently returned false)
- **Docs accuracy** — removed `state.*` namespace (not implemented), removed `ctx.preferred_label` (not in codebase), added `==` as `=` alias, added `not contains` infix syntax
- **Condition parser** — removed `<`, `>`, `<=`, `>=` from valid operators (never evaluated, silent false was a trap)

### Added
- `CHANGELOG.md` with retroactive history for all versions
- `docs/CONTRIBUTING.md` — documentation accuracy protocol with persona matrix
- `CLAUDE.md` — project conventions, gotchas, versioning policy
- Integration test (`TestLintExamples`) — lints all examples through real parser
- "Last verified" dates on model catalog and pricing table
- Tool `outputs` field documented in nodes.md and README.md

## [v0.4.0] — 2026-03-25

### Added
- **New commands:** `dippin cost`, `dippin coverage`, `dippin doctor`, `dippin optimize`, `dippin diff`, `dippin feedback`, `dippin lsp`
- **New providers:** DeepSeek, xAI (Grok), Mistral, Cohere — model validation and cost estimation
- **DIP116–DIP120** lint rules: compaction threshold, on_resume, stylesheet refs, reasoning_effort, namespace prefix
- **LSP server** with diagnostics, hover, go-to-definition, autocomplete, document symbols
- **Condition parser:** infix negation syntax (`var not contains val`)
- **Complementary pair detection:** `contains X` + `not contains X` recognized as exhaustive
- **New docs:** `docs/analysis.md`, `docs/editor-setup.md`

### Fixed
- **DIP101 false positives** — conditions were never parsed into ASTs; `Lint()` now calls `EnsureConditionsParsed()` before running checks
- **DIP101 exhaustive suppression** works for all three real-world patterns: exhaustive + fallback, exhaustive + extra variables, complementary pairs
- **DIP103** no longer flags `contains X` / `not contains X` as overlapping
- **DIP110** exempts start/exit lifecycle nodes from empty prompt warnings
- **Model pricing** corrected against official docs (claude-opus-4-6 was $15/$75, actually $5/$25; o3 was $10/$40, actually $2/$8)
- **Model IDs** corrected: `gemini-3-pro` → `gemini-3.1-pro-preview`, removed nonexistent IDs
- **Gemini pricing** added (was missing entirely from cost estimates)

### Changed
- Full documentation rewrite — all docs updated for post-v0.3.0 toolchain
- Mermaid diagrams use `<br>` instead of `<br/>`

## [v0.3.0] — 2026-03-21

### Added
- `dippin cost` and `dippin coverage` commands
- DIP119 (reasoning_effort validation), DIP120 (namespace prefix)
- DIP114 extended to parallel branch fidelity

### Fixed
- Scenario-injected values protected from node default overwrite
- errcheck and staticcheck lint errors in coverage package

### Changed
- Four largest files decomposed into focused modules

## [v0.2.0] — 2026-03-20

### Added
- `dippin check`, `dippin new` commands with 5 scaffold templates
- DIP113–DIP115 lint rules (retry policy, fidelity, goal gate)
- `base_delay` field for retry override
- Subgraph params, compaction, fidelity degradation, parallel branches, stylesheets
- `dippin version` command
- Justfile for dev workflows
- GoReleaser + GitHub Actions release pipeline
- Homebrew tap

### Changed
- All functions reduced to cyclomatic ≤ 5, cognitive ≤ 7

## [v0.1.0] — 2026-03-19

### Added
- Initial release
- Parser (indentation-aware lexer + recursive descent)
- Validator (DIP001–DIP009 structural checks)
- Linter (DIP101–DIP112 semantic warnings)
- Formatter (canonical idempotent output)
- DOT exporter with shape mapping
- DOT → Dippin migration with parity checker
- Simulator with JSONL event streaming
- 15 example workflows including 5 stress tests
- VS Code extension (syntax highlighting)

[v0.8.0]: https://github.com/2389-research/dippin-lang/compare/v0.7.0...v0.8.0
[v0.7.0]: https://github.com/2389-research/dippin-lang/compare/v0.6.0...v0.7.0
[v0.6.0]: https://github.com/2389-research/dippin-lang/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/2389-research/dippin-lang/compare/v0.4.3...v0.5.0
[v0.4.3]: https://github.com/2389-research/dippin-lang/compare/v0.4.2...v0.4.3
[v0.4.2]: https://github.com/2389-research/dippin-lang/compare/v0.4.1...v0.4.2
[v0.4.1]: https://github.com/2389-research/dippin-lang/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/2389-research/dippin-lang/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/2389-research/dippin-lang/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/2389-research/dippin-lang/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/2389-research/dippin-lang/releases/tag/v0.1.0
