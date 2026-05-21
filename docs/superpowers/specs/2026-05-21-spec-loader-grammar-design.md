# Spec Loader Grammar Design

> Status: **Design**. Implementation plan: [`docs/superpowers/plans/2026-05-21-spec-loader-grammar-implementation.md`](../plans/2026-05-21-spec-loader-grammar-implementation.md).

## Motivation

Dippin workflows currently treat their input spec — `SPEC.md`, `feature.yaml`, a Jira ticket, whatever — as opaque text routed through an `agent` node's prompt. The Tracker built-in `build_product` workflow even hard-codes "read `SPEC.md`" as a tool node. This works but leaves three things on the floor:

1. **Stable cross-artifact references.** Modern spec formats (acai's `feature.yaml`, Gherkin, EARS) carry stable IDs — acai calls them ACIDs (`my-feature.AUTH.1`) — that exist precisely so code, tests, dashboards, and reviewers can all point at the same requirement. Dippin has no syntax for a node to declare which requirements it satisfies, so the link is reconstructed in prose every time.
2. **Spec-shaped iteration.** `build_product`'s iteration unit is the *milestone* (a file/PR concept). The natural unit for spec-first development is the *requirement* (a spec concept). Without language-level support for requirement IDs, workflows can't decompose, verify, or report at requirement granularity without doing it all in agent prompts.
3. **Bidirectional status.** When an acai-style spec server exists, status flows both ways (pull at start to skip done work, push as you go to update the dashboard). The runtime can't do this without knowing which requirements a node satisfies.

The acai team's recent [Specsmaxxing post](https://acai.sh/blog/specsmaxxing) argues — convincingly — that as AI codegen gets cheaper, the spec becomes the primary artifact, and the toolchain should treat it that way. This change is dippin's grammar-level half of that move.

## Non-goals

- **Loading or interpreting spec files.** Dippin stays format-agnostic. It validates *syntax* (the ACID shape, the header structure) but never opens a `feature.yaml` or knows what acai is. The actual loading is a tracker runtime concern (the SpecLoader plugin interface tracked separately).
- **Rendering specs to markdown.** Explicitly *not* done. The motivation is to avoid the lossy MD intermediate, not to standardize it. Workflows that want to render still can — via a tool node — but the language doesn't bless that path.
- **Engine semantics.** What it means for a node to "satisfy" a requirement at runtime — prompt injection, verification, status reporting — is up to the runtime. The grammar only carries the declaration.
- **Backwards-incompatible IR changes.** Both new fields are optional. Workflows that don't use them parse and behave exactly as before.

## What changes

### Grammar

Two additions, both optional.

**Workflow header — `spec:` line:**

```
workflow MyPipeline
  goal: "Implement the cognitoforms-py client"
  spec: acai features/cognitoforms-py/features.yaml
  start: Begin
  exit: Done
```

The value is a single line: a loader name (identifier) followed by whitespace and a path (any non-whitespace run). Dippin doesn't validate the loader name against a registry — that's the runtime's job, since loaders are pluggable. Dippin validates only that there's a name *and* a path.

**Node body — `satisfies:` line:**

```
agent ImplementAuth
  satisfies: cognitoforms-py.AUTH.1, cognitoforms-py.AUTH.2, cognitoforms-py.AUTH.3
  prompt: |
    Implement the auth requirements listed in the spec.
```

The value is a comma-separated list of ACID references. Three accepted forms:

- **Bare ACID**: `cognitoforms-py.AUTH.1` or `cognitoforms-py.AUTH.1-1` (sub-requirement).
- **Range**: `cognitoforms-py.AUTH.[1-3]` — expands at runtime to ACIDs 1, 2, 3 in component AUTH.
- **Wildcard**: `cognitoforms-py.AUTH.*` — every requirement in component AUTH.

Range and wildcard expansion is a runtime concern (it needs the loaded spec); dippin only validates the syntactic shape.

`satisfies:` is a common field — any node kind may carry it (agent, tool, human, parallel, fan_in, subgraph, conditional, manager_loop). A `fan_in` declaring `satisfies:` on the join makes sense when the join is what "completes" the work; the engine semantics there are deferred to the runtime.

### IR

```go
// New types in ir/spec.go
type SpecRef struct {
    Loader string // e.g. "acai"
    Path   string // path resolved relative to the .dip file's directory
}

// Workflow gains an optional spec reference.
type Workflow struct {
    // ...existing fields...
    Spec *SpecRef // nil = no spec attached
}

// Node gains an optional satisfies list.
type Node struct {
    // ...existing fields...
    Satisfies []string // raw ACID/range/wildcard refs; nil = none
}
```

`Satisfies` stores the raw strings verbatim (no expansion), matching how `Condition.Raw` stores condition text without parsing — the runtime expander is one concern, the parser is another.

### Lint codes

Four new codes in the **DIP139–DIP142** range (DIP138 is the current high-water mark):

- **DIP139** — malformed ACID. The reference doesn't match the syntactic shape `name(.COMPONENT)+\.(\d+(-\d+)?|[*]|\[\d+-\d+\])`. Fires on each malformed entry in a `satisfies:` list. Error severity.
- **DIP140** — `satisfies:` declared but workflow has no `spec:`. Without a spec there's nothing for the ACID to reference; this is almost always a typo or copy-paste from another workflow. Warning severity (the engine still works — the ACID just won't resolve at runtime).
- **DIP141** — workflow declares `spec:` but no node references any requirement. The spec is unused; either the workflow is incomplete or the `spec:` line is stale. Warning severity.
- **DIP142** — duplicate ACID across `satisfies:` lists. Two different nodes declare ownership of the same requirement. Almost always a mistake (only one node should be the source of truth for "did this pass"). Warning severity.

Resolvability of ACIDs against the actual loaded spec (does `cognitoforms-py.AUTH.1` exist in the file?) is not a dippin-level lint — that requires loading the spec, which lives in tracker. Tracker's `tracker validate` is the right home for that check.

### CLI surface

- **`dippin parse` / `dippin check` JSON output** — IR JSON gains `spec: {loader, path}` on the workflow and `satisfies: [...]` on nodes when populated.
- **`dippin export-dot`** — `satisfies:` rendered as a sublabel beneath the node name when present, similar to how `class:` is currently surfaced. Always-on (no flag): the information is structural, not editorial, and the rendered count is bounded by ACIDs-per-node which is small.
- **`dippin fmt`** — canonical placement: `spec:` immediately after `goal:` in the workflow header block; `satisfies:` immediately after `label:` in node bodies (or first if no `label:`).
- **`dippin doctor`** — new "Spec" probe: reports whether the workflow declares a `spec:` and lists how many nodes carry `satisfies:`. Doesn't try to resolve the loader (that's tracker's job).
- **`dippin lint`** — runs DIP139–DIP142 as part of the standard pass.
- **`dippin validate`** — runs DIP139 only (the others are warnings, not errors).

### `.dipx` round-trip

Both fields round-trip through the dipx bundle format unchanged. The bundle's manifest gains no new top-level field; `spec:` and `satisfies:` are inside the workflow IR, which dipx already serializes verbatim.

## Acceptance criteria

A user can:

1. Write a `.dip` file with `spec: acai path/to/features.yaml` in the header; it parses, validates, and round-trips through `dippin fmt`.
2. Add `satisfies: foo.BAR.1` to any node; it parses, validates, and round-trips through `dippin fmt`.
3. See `cognitoforms-py.AUTH.[1-3]` and `cognitoforms-py.AUTH.*` parse as valid ACID syntax.
4. See `DIP139` fire on `satisfies: not.a-valid.acid` and on `satisfies: foo.bar.baz` (lowercase component).
5. See `DIP140` fire when a node has `satisfies:` but the workflow has no `spec:`.
6. See `DIP141` fire when the workflow has `spec:` but no node has `satisfies:`.
7. See `DIP142` fire when two nodes declare the same ACID.
8. See `spec` and `satisfies` in `dippin parse --format json` output.
9. See `satisfies:` rendered as a sublabel in `dippin export-dot` output.
10. See `dippin doctor` report the spec presence and satisfies coverage.
11. Pack a workflow with both fields into a `.dipx` bundle and unpack it with identical IR.

## Compatibility

- **Backwards compatibility:** Both fields are optional. Existing `.dip` files parse identically.
- **Forwards compatibility:** Tracker versions older than the SpecLoader integration will ignore unknown IR fields (Go zero-value semantics). The dipx bundle format is unchanged.
- **CHANGELOG:** New entry under `## [Unreleased]` flagged as v0.32.0+ (v0.31.0 is already in flight for unrelated work, #45 + #49).

## Open questions

- **Should `satisfies:` accept multi-line block syntax** for long lists (`satisfies: |` followed by indented entries)? Single-line comma-separated is enough for the cognitoforms-py spec (max ~12 ACIDs per component) and matches `requires:`. Defer until a real workflow hits the limit.
- **Should DIP141 be configurable to off?** A workflow could legitimately declare `spec:` purely for the tracker reporter to push status, with no node-level `satisfies:` (the runtime auto-attributes work). Defer until that pattern actually appears.
- **Tree-sitter grammar coverage** — adding `spec` and `satisfies` to `editors/tree-sitter-dippin/grammar.js` is in scope for this work (the tests would otherwise drift). Confirmed via grep of grammar.js that workflow header keywords are explicitly enumerated.

## What this isn't

This is the *grammar* half of acai integration. The other half — `pkg/spec/` interface, `pkg/spec/acai/` loader, bidirectional reporter, `verify_acid:` primitive, `ship_acai_spec.dip` built-in workflow — all lives in tracker and is tracked separately. Dippin's contract here is: "if you give me a workflow with `spec:` and `satisfies:`, I'll preserve and surface that information; what it *means* is up to you."
