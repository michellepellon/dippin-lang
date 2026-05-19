# Issue #39 — Tool Routing Fields (`marker_grep`, `route_required`, `output_limit`)

**Status:** Design approved 2026-05-19.
**Issue:** [#39](https://github.com/2389-research/dippin-lang/issues/39).
**Target release:** v0.28.0.
**Deferred follow-ups:** #42 (lint suppression + DIP138), #43 (parseBoolAttr normalization), #44 (outputs DOT export gap).

## Problem

Tracker's runtime supports three tool-node primitives for stdout-driven routing:

- `marker_grep: '<regex>'` — typed routing channel; populates `ctx.tool_marker`.
- `route_required: true` — paired with `_TRACKER_ROUTE=<value>` sentinel lines; populates `ctx.tool_route`.
- `output_limit: <bytes>` — per-node stdout tail-window override.

Tracker's `TRK101` lint recommends these as the canonical fix for stdout truncation. Dippin hard-fails: `unrecognized tool field "marker_grep"`. Authors who follow TRK101 advice can't.

Strings pass through verbatim. Dippin does not interpret routing semantics; tracker does.

## IR — `ir/ir.go`

Extend `ToolConfig`:

```go
type ToolConfig struct {
    Command       string
    Timeout       time.Duration
    Outputs       []string
    MarkerGrep    string // regex matched line-by-line against captured stdout; populates ctx.tool_marker
    RouteRequired bool   // true → node fails if no _TRACKER_ROUTE= sentinel is emitted
    OutputLimit   int    // bytes; > 0 = override engine default
}
```

## Parser — `parser/parse_nodes.go`

Three new fields in `applyToolField`:
- `marker_grep` — string, verbatim
- `route_required` — strict `val == "true"` (matches existing bool convention; #43 will normalize)
- `output_limit` — `parseInt`; diagnostic on negative

`applyToolField` is at the cyclomatic-5 budget; adding cases requires the same string/bool/parsed helper split the agent and human handlers already use. The plan covers shape.

## Formatter — `formatter/format.go`

`writeToolFields` emits new fields when non-default, clustering routing/output-shape fields:

```
outputs → marker_grep → route_required → output_limit → timeout → reads → writes → command
```

Zero/empty/false values are not emitted (matches existing convention for `goal_gate`, `cmd_timeout`, etc.). Documented in `docs/nodes.md` so authors know writing `output_limit: 0` round-trips to absent.

## DOT export — `export/dot.go`

`applyToolSemanticAttrs` emits `marker_grep` (when non-empty), `route_required` (as `"true"` when set), `output_limit` (decimal int when > 0). Same emission contract as the formatter.

## Migrate — `migrate/migrate.go`, `migrate/parity.go`

`buildToolConfig` reads the three attrs back. `compareToolConfigs` extends equality to all `ToolConfig` fields, backfilling pre-existing `Timeout` / `Outputs` gaps while in the area.

## LSP — `lsp/completion.go`, `lsp/hover.go`

Three new entries in `fieldCompletions`. Tool hover surfaces the new fields when set.

## Docs

- `docs/nodes.md` — three rows in the tool fields table; expand "Markers and Verbose Output" to mention `marker_grep:` as the typed alternative.
- `docs/llm-reference.md` — tool row + a row in the common-mistakes table referencing TRK101.
- `docs/context.md`, `docs/edges.md`, `docs/validation.md` — add `ctx.tool_marker` and `ctx.tool_route` to the reserved `ctx.*` variable lists.

## Website

- `site/static/skill.md` — mirror docs changes; add `ctx.tool_marker` / `ctx.tool_route` to the Context Variables table.
- `site/content/language.md` — `### tool` section gets the new fields.
- `site/content/blog/whats-new-v028.md` — short release post in the `whats-new-v027.md` style. Use Hugo `publishDate` set to the v0.28.0 tag date so it doesn't go live before the binary publishes (single-PR approach; `draft: true` flip is the alternative).

## Tooling

- `editors/vscode/syntaxes/dippin.tmLanguage.json` — append three field-name keywords to the existing alternation.
- Tree-sitter / Zed — no change (generic `field_name` rule).

## Example

New `examples/marker_routing.dip` — lint-clean, exercises all three fields, picked up by `TestLintExamples` for parser regression coverage.

## Tests

Per-layer happy path + round-trip via `parser/testdata/all_features.dip` extension. Migrate parity gets explicit mismatch tests for each new field (plus the backfilled `Timeout` / `Outputs`). The plan enumerates specific cases.

## CHANGELOG & release

- CHANGELOG entry under `## [v0.28.0]` in existing v0.27 style (Added: three fields + two ctx vars; Changed: parity backfill).
- Tag `v0.28.0`; GoReleaser publishes binaries + Homebrew tap.
- Tracker bumps `go.mod` to `dippin-lang@v0.28.0` in a follow-up PR; that same PR updates `extractToolAttrs` to forward the three attrs. Unkeyed `ir.ToolConfig{...}` literals (if any) surface and get fixed inline there.

## Out of scope (tracked separately)

- **#42** — Coverage / DIP101 / DIP102 suppression when `marker_grep` is declared. Reserves DIP138.
- **#43** — `parseBoolAttr` helper to normalize the four bool fields.
- **#44** — DOT round-trip for `ToolConfig.Outputs`.
- SI/IEC suffix parsing for `output_limit` (raw bytes for v1).
