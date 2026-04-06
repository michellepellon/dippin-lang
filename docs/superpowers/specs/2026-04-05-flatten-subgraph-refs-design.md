# Flatten Subgraph Refs in export-dot

**Issue:** [#7](https://github.com/2389-research/dippin-lang/issues/7)
**Date:** 2026-04-05

## Problem

`dippin export-dot` emits subgraph nodes as opaque `shape="tab"` nodes with a `ref=` attribute. DOT has no native import/ref mechanism, so this output is not valid executable DOT. The `ref=` attribute is a dippin invention with no meaning to DOT consumers.

## Solution

Create a new `flatten` package that performs IR-level subgraph resolution. Given a workflow with subgraph nodes, it recursively resolves all `ref:` paths, parses the referenced `.dip` files, and inlines their nodes and edges into a single flat `ir.Workflow`. The DOT exporter calls this transform before rendering.

## Architecture

### New package: `flatten/`

```go
// Resolver loads a referenced workflow by path.
type Resolver interface {
    Resolve(refPath string, relativeTo string) (*ir.Workflow, error)
}

// Options controls flattening behavior.
type Options struct {
    MaxDepth int // default 10; catches circular refs
}

// Flatten returns a new Workflow with all subgraph refs resolved and inlined.
func Flatten(w *ir.Workflow, resolve Resolver, opts Options) (*ir.Workflow, error)
```

**Production resolver:** Wraps the parser — resolves the ref path relative to the source file's directory, then parses the `.dip` file.

**Test resolver:** `MapResolver` backed by `map[string]*ir.Workflow` for deterministic unit tests with no filesystem.

### Algorithm

For each node in the input workflow:

1. If `node.Kind != ir.NodeSubgraph` or has no `ref:` → copy to output unchanged.
2. If subgraph with `ref:`:
   a. Check cycle detection set. Error if ref already in resolution stack.
   b. Call `resolver.Resolve(ref, sourceDir)` → child `*ir.Workflow`.
   c. Validate child has declared `Start` and `Exit` nodes. Error if not.
   d. Prefix all child node IDs with `ParentNodeID_` (underscore separator).
   e. Rewire parent edges:
      - Edges TO the subgraph node → redirect to `ParentNodeID_<child.Start>`
      - Edges FROM the subgraph node → redirect from `ParentNodeID_<child.Exit>`
   f. Add all prefixed child nodes and internal edges to the output.
   g. Remove the original subgraph node from the output.
   h. If the child workflow itself contains subgraph nodes, recurse (depth + 1).

3. Copy all non-subgraph edges to the output, applying any rewiring from step 2e.

### Node prefixing

Separator: underscore. Example: subgraph node `Review` referencing a workflow with nodes `Analyze` and `Summarize` produces `Review_Analyze` and `Review_Summarize`.

Multiple references to the same file each get their own prefix (the parent subgraph node's ID), so there are no collisions.

### Cycle detection

Maintain a set of resolved file paths in the current resolution stack. If a path appears twice, return an error: `"cycle detected: a.dip → b.dip → a.dip"`.

### Depth limit

Default `MaxDepth = 10`. If exceeded, return error: `"max depth 10 exceeded resolving subgraphs"`. This catches pathological nesting even without cycles.

### Params handling

`SubgraphConfig.Params` are runtime configuration, not structural. They do not affect flattening. Params from the parent subgraph node are preserved as metadata on the prefixed child nodes (implementation detail TBD — may use node labels or a new field).

## CLI Integration

In `cmd/dippin/cmd_export.go`, the export-dot flow becomes:

1. Parse the input `.dip` file → `*ir.Workflow`
2. Call `flatten.Flatten(w, diskResolver, defaultOpts)` → flat `*ir.Workflow`
3. Pass the flat workflow to `export.ExportDOT()`

Flattening is always-on for `export-dot`. No flag needed — DOT does not support external references, so unresolved refs produce invalid output.

If a workflow has no subgraph nodes, `Flatten` returns a copy unchanged (fast path).

## Error Handling

All errors are returned, never panicked. Error messages include:

- **Missing ref file:** `"flatten: node \"Review\": cannot resolve ref \"./review.dip\": file not found"`
- **Parse failure:** `"flatten: node \"Review\": parsing \"./review.dip\": <parser error>"`
- **Missing start/exit:** `"flatten: node \"Review\": resolved workflow \"review\" has no start node"`
- **Circular ref:** `"flatten: cycle detected: orchestrator.dip → review.dip → orchestrator.dip"`
- **Depth exceeded:** `"flatten: max depth 10 exceeded at node \"DeepRef\""`

The CLI prints the error to stderr and exits with a non-zero code.

## Testing

### Unit tests (`flatten/flatten_test.go`)

All use an in-memory `MapResolver`:

| Test | Verifies |
|------|----------|
| `TestFlattenNoSubgraphs` | Workflow without subgraphs passes through unchanged |
| `TestFlattenSingleSubgraph` | Node prefixing and edge rewiring for one subgraph |
| `TestFlattenMultipleSubgraphs` | Two subgraphs, distinct prefixes, no collision |
| `TestFlattenNested` | A→B→C all resolve to flat output |
| `TestFlattenCircularRef` | A→B→A returns cycle error |
| `TestFlattenMaxDepth` | Deeply nested refs beyond limit returns depth error |
| `TestFlattenMissingRef` | Resolver error propagated cleanly |
| `TestFlattenMissingStartExit` | Ref resolves but no start/exit → error |
| `TestFlattenPreservesEdgeConditions` | Conditions on edges to/from subgraph nodes survive |
| `TestFlattenPreservesParams` | Params preserved as metadata on flattened nodes |

### Integration tests (`export/dot_test.go`)

- Update `TestExportDOTSubgraphConfig` expectations for flattened output
- New test: parse real `.dip` file pair → flatten → export-dot → verify inlined nodes

### Example files

Add `examples/orchestrator.dip` + `examples/phases/child.dip` pair to exercise the full path via `just validate-examples`.

## Files Changed

| File | Change |
|------|--------|
| `flatten/flatten.go` | New: Flatten function, Resolver interface, Options, algorithm |
| `flatten/flatten_test.go` | New: Unit tests with MapResolver |
| `flatten/resolver.go` | New: DiskResolver (wraps parser) |
| `cmd/dippin/cmd_export.go` | Call flatten.Flatten before ExportDOT |
| `export/dot_test.go` | Update subgraph test expectations |
| `examples/orchestrator.dip` | New: multi-subgraph example |
| `examples/phases/child.dip` | New: referenced child workflow |

## Out of Scope

- Making other tools (simulate, cost, validate) use flattened workflows — they can opt in later via the same `flatten.Flatten` call.
- Parameter substitution at flatten time — params are runtime config.
- A `--no-flatten` flag — if someone wants opaque refs, they shouldn't use `export-dot`.
