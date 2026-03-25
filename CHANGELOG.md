# Changelog

All notable changes to dippin-lang are documented here. Versions follow [semver](https://semver.org/).

## [v0.5.0] — 2026-03-25

### Added
- **`dippin explain <DIPxxx>`** — rich explanations for all 32 diagnostic codes. Shows trigger conditions, fix guidance, and example snippets. Supports text and JSON output.
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

[v0.4.3]: https://github.com/2389-research/dippin-lang/compare/v0.4.2...v0.4.3
[v0.4.2]: https://github.com/2389-research/dippin-lang/compare/v0.4.1...v0.4.2
[v0.4.1]: https://github.com/2389-research/dippin-lang/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/2389-research/dippin-lang/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/2389-research/dippin-lang/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/2389-research/dippin-lang/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/2389-research/dippin-lang/releases/tag/v0.1.0
