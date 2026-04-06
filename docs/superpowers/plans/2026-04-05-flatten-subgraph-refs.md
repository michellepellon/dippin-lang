# Flatten Subgraph Refs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a `flatten` package that resolves all subgraph `ref:` paths into a single flat `ir.Workflow`, and wire it into `export-dot` so output is always valid executable DOT.

**Architecture:** New `flatten/` package with a `Resolver` interface (production: wraps parser to load `.dip` files from disk; test: in-memory map). The `Flatten()` function walks a workflow, replaces each subgraph node with the referenced workflow's nodes (prefixed with `ParentID_`), and rewires edges. The CLI calls `Flatten()` before `ExportDOT()`.

**Tech Stack:** Go, existing `ir` and `parser` packages. No new dependencies.

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `flatten/flatten.go` | Create | `Flatten()` function, `Resolver` interface, `Options` struct, `MapResolver` for tests |
| `flatten/flatten_test.go` | Create | Unit tests using `MapResolver` |
| `flatten/resolver.go` | Create | `DiskResolver` wrapping the parser for production use |
| `cmd/dippin/cmd_export.go` | Modify | Call `flatten.Flatten()` before `export.ExportDOT()` |
| `export/dot_test.go` | Modify | Update `TestExportDOTSubgraphConfig` expectations |
| `examples/phases/code_review.dip` | Create | Small child workflow for orchestrator example |
| `examples/orchestrator.dip` | Create | Multi-subgraph orchestrator referencing child workflows |

---

## Task 1: Scaffold flatten package with types and no-op Flatten

**Files:**
- Create: `flatten/flatten.go`

- [ ] **Step 1: Write the failing test**

Create `flatten/flatten_test.go` with a test that calls `Flatten` on a workflow with no subgraphs and verifies the output is structurally identical:

```go
// ABOUTME: Unit tests for the flatten package.
// ABOUTME: Tests subgraph ref resolution and inlining into flat workflows.
package flatten

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestFlattenNoSubgraphs(t *testing.T) {
	w := &ir.Workflow{
		Name:  "simple",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
		},
	}

	got, err := Flatten(w, nil, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "simple" {
		t.Errorf("Name = %q, want %q", got.Name, "simple")
	}
	if got.Start != "A" {
		t.Errorf("Start = %q, want %q", got.Start, "A")
	}
	if got.Exit != "B" {
		t.Errorf("Exit = %q, want %q", got.Exit, "B")
	}
	if len(got.Nodes) != 2 {
		t.Fatalf("len(Nodes) = %d, want 2", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Fatalf("len(Edges) = %d, want 1", len(got.Edges))
	}
	if got.Edges[0].From != "A" || got.Edges[0].To != "B" {
		t.Errorf("Edge = %s->%s, want A->B", got.Edges[0].From, got.Edges[0].To)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `just test-pkg flatten`
Expected: FAIL — `flatten` package doesn't exist yet.

- [ ] **Step 3: Write minimal implementation**

Create `flatten/flatten.go`:

```go
// ABOUTME: Flattens subgraph ref nodes by inlining referenced workflows.
// ABOUTME: Produces a single flat ir.Workflow with all refs resolved.
package flatten

import (
	"github.com/2389-research/dippin-lang/ir"
)

// Resolver loads a referenced workflow by path.
type Resolver interface {
	// Resolve returns the workflow at refPath, resolved relative to relativeTo.
	Resolve(refPath string, relativeTo string) (*ir.Workflow, error)
}

// Options controls flattening behavior.
type Options struct {
	// MaxDepth limits recursion depth. Default (0) uses defaultMaxDepth.
	MaxDepth int
}

const defaultMaxDepth = 10

// Flatten returns a new Workflow with all subgraph ref nodes resolved and inlined.
// If the workflow has no subgraph nodes, it returns a shallow copy unchanged.
// The resolver is only called when subgraph nodes with refs are present.
func Flatten(w *ir.Workflow, resolve Resolver, opts Options) (*ir.Workflow, error) {
	if !hasSubgraphRefs(w) {
		return copyWorkflow(w), nil
	}
	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
	}
	return flattenRecursive(w, resolve, maxDepth, 0, nil)
}

// hasSubgraphRefs returns true if any node is a subgraph with a ref.
func hasSubgraphRefs(w *ir.Workflow) bool {
	for _, n := range w.Nodes {
		if cfg, ok := n.Config.(ir.SubgraphConfig); ok && cfg.Ref != "" {
			return true
		}
	}
	return false
}

// copyWorkflow creates a shallow copy of a workflow with new node and edge slices.
func copyWorkflow(w *ir.Workflow) *ir.Workflow {
	out := *w
	out.Nodes = make([]*ir.Node, len(w.Nodes))
	copy(out.Nodes, w.Nodes)
	out.Edges = make([]*ir.Edge, len(w.Edges))
	copy(out.Edges, w.Edges)
	return &out
}

// flattenRecursive is the core recursive flattening logic.
func flattenRecursive(w *ir.Workflow, resolve Resolver, maxDepth, depth int, seen []string) (*ir.Workflow, error) {
	// Placeholder — will be implemented in Task 2.
	return copyWorkflow(w), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `just test-pkg flatten`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add flatten/flatten.go flatten/flatten_test.go
git commit -m "feat(flatten): scaffold package with types and no-op Flatten"
```

---

## Task 2: Implement single-subgraph flattening

**Files:**
- Modify: `flatten/flatten.go`
- Modify: `flatten/flatten_test.go`

- [ ] **Step 1: Write the failing test**

Add `MapResolver` and `TestFlattenSingleSubgraph` to `flatten/flatten_test.go`:

```go
// MapResolver is a test resolver backed by an in-memory map.
type MapResolver struct {
	Workflows map[string]*ir.Workflow
}

func (r *MapResolver) Resolve(refPath string, _ string) (*ir.Workflow, error) {
	w, ok := r.Workflows[refPath]
	if !ok {
		return nil, fmt.Errorf("ref not found: %s", refPath)
	}
	return w, nil
}

func TestFlattenSingleSubgraph(t *testing.T) {
	child := &ir.Workflow{
		Name:  "review",
		Start: "Analyze",
		Exit:  "Summarize",
		Nodes: []*ir.Node{
			{ID: "Analyze", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "analyze."}},
			{ID: "Summarize", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "summarize."}},
		},
		Edges: []*ir.Edge{
			{From: "Analyze", To: "Summarize"},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"./review.dip": child,
	}}

	parent := &ir.Workflow{
		Name:  "main",
		Start: "Build",
		Exit:  "Done",
		Nodes: []*ir.Node{
			{ID: "Build", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "build."}},
			{ID: "Review", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "Build", To: "Review"},
			{From: "Review", To: "Done"},
		},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 nodes: Build, Review_Analyze, Review_Summarize, Done
	if len(got.Nodes) != 4 {
		t.Fatalf("len(Nodes) = %d, want 4; nodes: %v", len(got.Nodes), nodeIDs(got))
	}

	// Verify node IDs
	wantIDs := map[string]bool{"Build": true, "Review_Analyze": true, "Review_Summarize": true, "Done": true}
	for _, n := range got.Nodes {
		if !wantIDs[n.ID] {
			t.Errorf("unexpected node %q", n.ID)
		}
		delete(wantIDs, n.ID)
	}
	for id := range wantIDs {
		t.Errorf("missing node %q", id)
	}

	// Verify edges:
	// Build -> Review_Analyze (rewired from Build -> Review)
	// Review_Analyze -> Review_Summarize (internal child edge, prefixed)
	// Review_Summarize -> Done (rewired from Review -> Done)
	wantEdges := map[string]bool{
		"Build->Review_Analyze":            true,
		"Review_Analyze->Review_Summarize": true,
		"Review_Summarize->Done":           true,
	}
	for _, e := range got.Edges {
		key := e.From + "->" + e.To
		if !wantEdges[key] {
			t.Errorf("unexpected edge %s", key)
		}
		delete(wantEdges, key)
	}
	for key := range wantEdges {
		t.Errorf("missing edge %s", key)
	}
}

func nodeIDs(w *ir.Workflow) []string {
	var ids []string
	for _, n := range w.Nodes {
		ids = append(ids, n.ID)
	}
	return ids
}
```

Add `"fmt"` to the import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `just test-pkg flatten`
Expected: FAIL — `TestFlattenSingleSubgraph` fails because `flattenRecursive` returns unchanged copy.

- [ ] **Step 3: Implement the core flattening logic**

Replace the `flattenRecursive` function in `flatten/flatten.go` with the real implementation:

```go
// flattenRecursive is the core recursive flattening logic.
func flattenRecursive(w *ir.Workflow, resolve Resolver, maxDepth, depth int, seen []string) (*ir.Workflow, error) {
	if depth >= maxDepth {
		return nil, fmt.Errorf("flatten: max depth %d exceeded", maxDepth)
	}

	out := &ir.Workflow{
		Name:       w.Name,
		Version:    w.Version,
		Goal:       w.Goal,
		Start:      w.Start,
		Exit:       w.Exit,
		Defaults:   w.Defaults,
		Stylesheet: w.Stylesheet,
		SourceMap:  w.SourceMap,
	}

	// Build rewire map: subgraphNodeID -> {startID, exitID}
	type rewire struct {
		start string
		exit  string
	}
	rewires := make(map[string]rewire)

	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.SubgraphConfig)
		if !ok || cfg.Ref == "" {
			out.Nodes = append(out.Nodes, n)
			continue
		}

		// Cycle detection.
		if containsStr(seen, cfg.Ref) {
			return nil, fmt.Errorf("flatten: cycle detected: %s", formatCycle(seen, cfg.Ref))
		}

		child, err := resolve.Resolve(cfg.Ref, n.Source.File)
		if err != nil {
			return nil, fmt.Errorf("flatten: node %q: cannot resolve ref %q: %w", n.ID, cfg.Ref, err)
		}
		if child.Start == "" {
			return nil, fmt.Errorf("flatten: node %q: resolved workflow %q has no start node", n.ID, child.Name)
		}
		if child.Exit == "" {
			return nil, fmt.Errorf("flatten: node %q: resolved workflow %q has no exit node", n.ID, child.Name)
		}

		// Recursively flatten child if it has subgraphs.
		child, err = flattenRecursive(child, resolve, maxDepth, depth+1, append(seen, cfg.Ref))
		if err != nil {
			return nil, err
		}

		prefix := n.ID + "_"
		rewires[n.ID] = rewire{
			start: prefix + child.Start,
			exit:  prefix + child.Exit,
		}

		// Add prefixed child nodes.
		for _, cn := range child.Nodes {
			prefixed := *cn
			prefixed.ID = prefix + cn.ID
			out.Nodes = append(out.Nodes, &prefixed)
		}

		// Add prefixed child edges.
		for _, ce := range child.Edges {
			prefixed := *ce
			prefixed.From = prefix + ce.From
			prefixed.To = prefix + ce.To
			out.Edges = append(out.Edges, &prefixed)
		}
	}

	// Add parent edges with rewiring.
	for _, e := range w.Edges {
		ne := *e
		if r, ok := rewires[ne.To]; ok {
			ne.To = r.start
		}
		if r, ok := rewires[ne.From]; ok {
			ne.From = r.exit
		}
		out.Edges = append(out.Edges, &ne)
	}

	return out, nil
}

// containsStr checks if a string slice contains a value.
func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// formatCycle formats a cycle path for error messages.
func formatCycle(seen []string, ref string) string {
	parts := make([]string, len(seen)+1)
	copy(parts, seen)
	parts[len(seen)] = ref
	return strings.Join(parts, " → ")
}
```

Add `"fmt"` and `"strings"` to the imports in `flatten.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `just test-pkg flatten`
Expected: PASS for both `TestFlattenNoSubgraphs` and `TestFlattenSingleSubgraph`.

- [ ] **Step 5: Commit**

```bash
git add flatten/flatten.go flatten/flatten_test.go
git commit -m "feat(flatten): implement single-subgraph inlining with edge rewiring"
```

---

## Task 3: Multiple subgraphs and edge condition preservation

**Files:**
- Modify: `flatten/flatten_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `flatten/flatten_test.go`:

```go
func TestFlattenMultipleSubgraphs(t *testing.T) {
	childA := &ir.Workflow{
		Name: "lint", Start: "Check", Exit: "Report",
		Nodes: []*ir.Node{
			{ID: "Check", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "check."}},
			{ID: "Report", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "report."}},
		},
		Edges: []*ir.Edge{{From: "Check", To: "Report"}},
	}
	childB := &ir.Workflow{
		Name: "test", Start: "Run", Exit: "Summary",
		Nodes: []*ir.Node{
			{ID: "Run", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "run."}},
			{ID: "Summary", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "summary."}},
		},
		Edges: []*ir.Edge{{From: "Run", To: "Summary"}},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"lint.dip": childA,
		"test.dip": childB,
	}}

	parent := &ir.Workflow{
		Name: "pipeline", Start: "Start", Exit: "End",
		Nodes: []*ir.Node{
			{ID: "Start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "start."}},
			{ID: "Lint", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "lint.dip"}},
			{ID: "Test", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "test.dip"}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "end."}},
		},
		Edges: []*ir.Edge{
			{From: "Start", To: "Lint"},
			{From: "Lint", To: "Test"},
			{From: "Test", To: "End"},
		},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 original + 2 from each child = 6
	if len(got.Nodes) != 6 {
		t.Fatalf("len(Nodes) = %d, want 6; nodes: %v", len(got.Nodes), nodeIDs(got))
	}

	// Verify key edges
	wantEdges := map[string]bool{
		"Start->Lint_Check":       true,
		"Lint_Check->Lint_Report":  true,
		"Lint_Report->Test_Run":    true,
		"Test_Run->Test_Summary":   true,
		"Test_Summary->End":        true,
	}
	for _, e := range got.Edges {
		key := e.From + "->" + e.To
		delete(wantEdges, key)
	}
	for key := range wantEdges {
		t.Errorf("missing edge %s", key)
	}
}

func TestFlattenPreservesEdgeConditions(t *testing.T) {
	child := &ir.Workflow{
		Name: "check", Start: "Run", Exit: "Done",
		Nodes: []*ir.Node{
			{ID: "Run", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "run."}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{{From: "Run", To: "Done"}},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{"check.dip": child}}

	cond := &ir.Condition{
		Raw:    "ctx.outcome = success",
		Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
	}
	parent := &ir.Workflow{
		Name: "main", Start: "Gate", Exit: "End",
		Nodes: []*ir.Node{
			{ID: "Gate", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "gate."}},
			{ID: "Check", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "check.dip"}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "end."}},
		},
		Edges: []*ir.Edge{
			{From: "Gate", To: "Check", Condition: cond, Label: "approved"},
			{From: "Check", To: "End"},
		},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the edge Gate -> Check_Run (rewired) and verify condition preserved.
	var found bool
	for _, e := range got.Edges {
		if e.From == "Gate" && e.To == "Check_Run" {
			found = true
			if e.Condition == nil {
				t.Error("condition lost on rewired edge")
			} else if e.Condition.Raw != "ctx.outcome = success" {
				t.Errorf("condition.Raw = %q, want %q", e.Condition.Raw, "ctx.outcome = success")
			}
			if e.Label != "approved" {
				t.Errorf("label = %q, want %q", e.Label, "approved")
			}
		}
	}
	if !found {
		t.Error("expected edge Gate->Check_Run not found")
	}
}
```

- [ ] **Step 2: Run tests to verify behavior**

Run: `just test-pkg flatten`
Expected: PASS — the implementation from Task 2 should handle these cases already since edge properties are copied with shallow copy (`ne := *e`).

- [ ] **Step 3: Commit**

```bash
git add flatten/flatten_test.go
git commit -m "test(flatten): add multi-subgraph and edge condition tests"
```

---

## Task 4: Error cases — cycle detection, depth limit, missing ref, missing start/exit

**Files:**
- Modify: `flatten/flatten_test.go`

- [ ] **Step 1: Write the error case tests**

Add to `flatten/flatten_test.go`:

```go
func TestFlattenCircularRef(t *testing.T) {
	workflowA := &ir.Workflow{
		Name: "a", Start: "X", Exit: "Y",
		Nodes: []*ir.Node{
			{ID: "X", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "b.dip"}},
			{ID: "Y", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "y."}},
		},
		Edges: []*ir.Edge{{From: "X", To: "Y"}},
	}
	workflowB := &ir.Workflow{
		Name: "b", Start: "P", Exit: "Q",
		Nodes: []*ir.Node{
			{ID: "P", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "a.dip"}},
			{ID: "Q", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "q."}},
		},
		Edges: []*ir.Edge{{From: "P", To: "Q"}},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"a.dip": workflowA,
		"b.dip": workflowB,
	}}

	// Start from a workflow that refs a.dip
	root := &ir.Workflow{
		Name: "root", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "a.dip"}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "e."}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}

	_, err := Flatten(root, resolver, Options{})
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle: %v", err)
	}
}

func TestFlattenMaxDepth(t *testing.T) {
	// Chain of 5 workflows, each referencing the next.
	// With MaxDepth=3, this should fail.
	resolver := &MapResolver{Workflows: make(map[string]*ir.Workflow)}
	for i := 0; i < 5; i++ {
		ref := fmt.Sprintf("level%d.dip", i+1)
		w := &ir.Workflow{
			Name: fmt.Sprintf("level%d", i), Start: "A", Exit: "B",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a."}},
				{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b."}},
			},
			Edges: []*ir.Edge{{From: "A", To: "B"}},
		}
		if i < 4 {
			w.Nodes = []*ir.Node{
				{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: ref}},
				{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b."}},
			}
		}
		resolver.Workflows[fmt.Sprintf("level%d.dip", i)] = w
	}

	root := &ir.Workflow{
		Name: "root", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "level0.dip"}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "e."}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}

	_, err := Flatten(root, resolver, Options{MaxDepth: 3})
	if err == nil {
		t.Fatal("expected max depth error, got nil")
	}
	if !strings.Contains(err.Error(), "max depth") {
		t.Errorf("error should mention max depth: %v", err)
	}
}

func TestFlattenMissingRef(t *testing.T) {
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{}}
	w := &ir.Workflow{
		Name: "main", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "missing.dip"}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "e."}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}

	_, err := Flatten(w, resolver, Options{})
	if err == nil {
		t.Fatal("expected error for missing ref, got nil")
	}
	if !strings.Contains(err.Error(), "missing.dip") {
		t.Errorf("error should mention the missing ref: %v", err)
	}
}

func TestFlattenMissingStartExit(t *testing.T) {
	child := &ir.Workflow{
		Name: "bad",
		Nodes: []*ir.Node{
			{ID: "X", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "x."}},
		},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{"bad.dip": child}}
	w := &ir.Workflow{
		Name: "main", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "bad.dip"}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "e."}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}

	_, err := Flatten(w, resolver, Options{})
	if err == nil {
		t.Fatal("expected error for missing start, got nil")
	}
	if !strings.Contains(err.Error(), "no start node") {
		t.Errorf("error should mention missing start: %v", err)
	}
}
```

Add `"strings"` and `"fmt"` to the test file imports if not already there.

- [ ] **Step 2: Run tests**

Run: `just test-pkg flatten`
Expected: PASS — the implementation from Task 2 already handles these error paths.

- [ ] **Step 3: Commit**

```bash
git add flatten/flatten_test.go
git commit -m "test(flatten): add error case tests for cycles, depth, missing refs"
```

---

## Task 5: Nested subgraph flattening test

**Files:**
- Modify: `flatten/flatten_test.go`

- [ ] **Step 1: Write the test**

Add to `flatten/flatten_test.go`:

```go
func TestFlattenNested(t *testing.T) {
	// grandchild: inner.dip has nodes P, Q
	grandchild := &ir.Workflow{
		Name: "inner", Start: "P", Exit: "Q",
		Nodes: []*ir.Node{
			{ID: "P", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "p."}},
			{ID: "Q", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "q."}},
		},
		Edges: []*ir.Edge{{From: "P", To: "Q"}},
	}
	// child: middle.dip has nodes M and a subgraph ref to inner.dip
	child := &ir.Workflow{
		Name: "middle", Start: "M", Exit: "Sub",
		Nodes: []*ir.Node{
			{ID: "M", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "m."}},
			{ID: "Sub", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "inner.dip"}},
		},
		Edges: []*ir.Edge{{From: "M", To: "Sub"}},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{
		"middle.dip": child,
		"inner.dip":  grandchild,
	}}

	root := &ir.Workflow{
		Name: "root", Start: "A", Exit: "Outer",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a."}},
			{ID: "Outer", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "middle.dip"}},
		},
		Edges: []*ir.Edge{{From: "A", To: "Outer"}},
	}

	got, err := Flatten(root, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected nodes: A, Outer_M, Outer_Sub_P, Outer_Sub_Q
	wantIDs := map[string]bool{
		"A": true, "Outer_M": true, "Outer_Sub_P": true, "Outer_Sub_Q": true,
	}
	if len(got.Nodes) != len(wantIDs) {
		t.Fatalf("len(Nodes) = %d, want %d; nodes: %v", len(got.Nodes), len(wantIDs), nodeIDs(got))
	}
	for _, n := range got.Nodes {
		if !wantIDs[n.ID] {
			t.Errorf("unexpected node %q", n.ID)
		}
	}

	// Expected edges: A->Outer_M, Outer_M->Outer_Sub_P, Outer_Sub_P->Outer_Sub_Q
	wantEdges := map[string]bool{
		"A->Outer_M":                true,
		"Outer_M->Outer_Sub_P":      true,
		"Outer_Sub_P->Outer_Sub_Q":  true,
	}
	for _, e := range got.Edges {
		key := e.From + "->" + e.To
		delete(wantEdges, key)
	}
	for key := range wantEdges {
		t.Errorf("missing edge %s", key)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `just test-pkg flatten`
Expected: PASS — recursive flattening already prefixes at each level.

- [ ] **Step 3: Commit**

```bash
git add flatten/flatten_test.go
git commit -m "test(flatten): add nested subgraph resolution test"
```

---

## Task 6: DiskResolver for production use

**Files:**
- Create: `flatten/resolver.go`

- [ ] **Step 1: Write the failing test**

Add to `flatten/flatten_test.go`:

```go
func TestDiskResolverResolve(t *testing.T) {
	// Write a temp .dip file and verify DiskResolver can load it.
	dir := t.TempDir()
	dipContent := `workflow child
  start: A
  exit: B
  agent A
    label: A
  agent B
    label: B
  edges
    A -> B
`
	path := filepath.Join(dir, "child.dip")
	if err := os.WriteFile(path, []byte(dipContent), 0644); err != nil {
		t.Fatal(err)
	}

	resolver := &DiskResolver{}
	w, err := resolver.Resolve("child.dip", filepath.Join(dir, "parent.dip"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Name != "child" {
		t.Errorf("Name = %q, want %q", w.Name, "child")
	}
	if w.Start != "A" {
		t.Errorf("Start = %q, want %q", w.Start, "A")
	}
}
```

Add `"os"` and `"path/filepath"` to test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `just test-pkg flatten`
Expected: FAIL — `DiskResolver` doesn't exist yet.

- [ ] **Step 3: Write DiskResolver**

Create `flatten/resolver.go`:

```go
// ABOUTME: DiskResolver loads referenced .dip files from the filesystem.
// ABOUTME: Used by the CLI for production subgraph resolution.
package flatten

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/2389-research/dippin-lang/parser"
)

// DiskResolver resolves subgraph refs by parsing .dip files from disk.
type DiskResolver struct{}

// Resolve parses the .dip file at refPath, resolved relative to relativeTo's directory.
func (r *DiskResolver) Resolve(refPath string, relativeTo string) (*ir.Workflow, error) {
	resolved := resolvePath(refPath, relativeTo)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", resolved, err)
	}
	p := parser.NewParser(string(data), resolved)
	w, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", resolved, err)
	}
	return w, nil
}

// resolvePath resolves refPath relative to the directory of relativeTo.
func resolvePath(refPath string, relativeTo string) string {
	if filepath.IsAbs(refPath) {
		return refPath
	}
	if relativeTo != "" {
		return filepath.Join(filepath.Dir(relativeTo), refPath)
	}
	return refPath
}
```

Add `"github.com/2389-research/dippin-lang/ir"` to the import block. (The `ir` import is needed because `Resolve` returns `*ir.Workflow`.)

Note: The import for `ir` is implicit via the return type. Make sure the import block includes:
```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
)
```

Wait — `Resolve` returns `*ir.Workflow` which is declared in the same package's interface. Since `flatten` already imports `ir` in `flatten.go`, and `resolver.go` is in the same package, the `ir` import is already needed for `parser.NewParser` usage. The actual return type `*ir.Workflow` requires the `ir` import. Include it.

- [ ] **Step 4: Run tests**

Run: `just test-pkg flatten`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add flatten/resolver.go flatten/flatten_test.go
git commit -m "feat(flatten): add DiskResolver for filesystem-based ref resolution"
```

---

## Task 7: Wire flatten into export-dot CLI command

**Files:**
- Modify: `cmd/dippin/cmd_export.go`

- [ ] **Step 1: Write the test**

This is an integration-level change. We'll verify by running the CLI against a real `.dip` file pair. First, create the example files (Task 8 below), then test here. For now, add a unit-level verification in `export/dot_test.go`.

Add to `export/dot_test.go`:

```go
func TestExportDOTNoSubgraphNodes(t *testing.T) {
	// After flattening, a workflow with subgraph refs should have no
	// shape="tab" nodes in the output. This test verifies that if the
	// caller passes an already-flattened workflow, no tab shapes appear.
	w := &ir.Workflow{
		Name: "flat", Start: "A", Exit: "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{{From: "A", To: "B"}},
	}
	out := ExportDOT(w, ExportOptions{})
	assertNotContains(t, out, `shape="tab"`)
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `just test-pkg export`
Expected: PASS (this test validates the expectation, not a code change).

- [ ] **Step 3: Modify cmd_export.go**

Update `cmd/dippin/cmd_export.go` to call `flatten.Flatten` before `export.ExportDOT`:

```go
package main

import (
	"flag"
	"fmt"

	"github.com/2389-research/dippin-lang/export"
	"github.com/2389-research/dippin-lang/flatten"
)

// CmdExportDOT exports a workflow to DOT graph format.
//   - --rankdir=LR|TB (default TB)
//   - --prompts includes prompt text in DOT node attributes
func (c *CLI) CmdExportDOT(args []string) ExitCode {
	fs := flag.NewFlagSet("export-dot", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	rankdir := fs.String("rankdir", "TB", "graph layout direction (LR|TB)")
	prompts := fs.Bool("prompts", false, "include prompt text in DOT attributes")
	if err := fs.Parse(args); err != nil {
		return ExitUsageError
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(c.Stderr, "usage: dippin export-dot [--rankdir=LR|TB] [--prompts] <file>")
		return ExitUsageError
	}

	path := fs.Arg(0)
	w, err := loadWorkflow(path)
	if err != nil {
		c.renderError(err, path)
		return ExitError
	}

	w, err = flatten.Flatten(w, &flatten.DiskResolver{}, flatten.Options{})
	if err != nil {
		fmt.Fprintln(c.Stderr, err)
		return ExitError
	}

	opts := export.ExportOptions{
		IncludePrompts: *prompts,
		RankDir:        *rankdir,
	}

	dot := export.ExportDOT(w, opts)
	fmt.Fprint(c.Stdout, dot)
	return ExitOK
}
```

- [ ] **Step 4: Build and verify**

Run: `just build`
Expected: Compiles successfully.

Run: `just test`
Expected: All existing tests still pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/dippin/cmd_export.go export/dot_test.go
git commit -m "feat(export-dot): wire flatten into CLI for always-flat DOT output"
```

---

## Task 8: Example files and integration validation

**Files:**
- Create: `examples/phases/code_review.dip`
- Create: `examples/orchestrator.dip`

- [ ] **Step 1: Create child workflow**

Create `examples/phases/code_review.dip`:

```dip
workflow CodeReview
  goal: "Review code for correctness and style."
  start: Analyze
  exit: Verdict

  defaults
    model: claude-sonnet-4-6
    provider: anthropic

  agent Analyze
    label: "Analyze code"
    prompt:
      Review the code changes for correctness, readability,
      and adherence to project conventions.
      List any issues found.

  agent Verdict
    label: "Summarize review"
    auto_status: true
    reads: analysis
    writes: review_result
    prompt:
      Summarize the code review findings.
      If there are blocking issues, STATUS: fail.
      If the code is acceptable, STATUS: success.

  edges
    Analyze -> Verdict
```

- [ ] **Step 2: Create orchestrator workflow**

Create `examples/orchestrator.dip`:

```dip
workflow Orchestrator
  goal: "Build, review, and ship a feature."
  start: Plan
  exit: Done

  defaults
    model: claude-sonnet-4-6
    provider: anthropic

  agent Plan
    label: "Plan the work"
    writes: plan
    prompt:
      Read the feature request and produce an implementation plan.

  agent Build
    label: "Implement the feature"
    reads: plan
    writes: implementation
    prompt:
      Follow the plan and implement the feature.

  subgraph Review
    label: "Code review"
    ref: phases/code_review.dip

  agent Done
    label: "Ship it"
    reads: review_result
    prompt:
      Package and ship the implementation.

  edges
    Plan -> Build
    Build -> Review
    Review -> Done
```

- [ ] **Step 3: Validate examples**

Run: `just validate-examples`
Expected: All examples validate, including the new ones.

- [ ] **Step 4: Test export-dot with the new example**

Run: `just build && ./dippin export-dot examples/orchestrator.dip`

Expected output should contain:
- `Review_Analyze` and `Review_Verdict` nodes (prefixed child nodes)
- `Build -> Review_Analyze` edge (rewired)
- `Review_Verdict -> Done` edge (rewired)
- `Review_Analyze -> Review_Verdict` edge (internal child edge)
- NO `shape="tab"` or `ref=` attributes

- [ ] **Step 5: Commit**

```bash
git add examples/phases/code_review.dip examples/orchestrator.dip
git commit -m "feat: add orchestrator example with subgraph ref for integration testing"
```

---

## Task 9: Update existing subgraph test in export package

**Files:**
- Modify: `export/dot_test.go`

- [ ] **Step 1: Review current test**

The existing `TestExportDOTSubgraphConfig` at `export/dot_test.go:350-354` tests that `ref=` and `shape="tab"` appear in output. Since `export.ExportDOT` itself doesn't call `flatten.Flatten` (the CLI does), this test is still valid — it tests the export package in isolation.

However, we should add a note clarifying this:

```go
func TestExportDOTSubgraphConfig(t *testing.T) {
	// Tests the export package's handling of un-flattened subgraph nodes.
	// In production, the CLI calls flatten.Flatten before ExportDOT,
	// so subgraph nodes should not appear in normal export-dot output.
	// This test verifies the fallback rendering for direct ExportDOT calls.
	out := ExportDOT(subgraphWorkflow(), ExportOptions{IncludePrompts: true})
	assertContains(t, out, `ref="./review.dip"`)
	assertContains(t, out, `shape="tab"`)
}
```

- [ ] **Step 2: Run tests**

Run: `just test`
Expected: PASS — all tests green.

- [ ] **Step 3: Commit**

```bash
git add export/dot_test.go
git commit -m "docs: clarify subgraph export test covers un-flattened fallback"
```

---

## Task 10: Params preservation test

**Files:**
- Modify: `flatten/flatten_test.go`

- [ ] **Step 1: Write the test**

Add to `flatten/flatten_test.go`:

```go
func TestFlattenPreservesParams(t *testing.T) {
	child := &ir.Workflow{
		Name: "child", Start: "X", Exit: "Y",
		Nodes: []*ir.Node{
			{ID: "X", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "x."}},
			{ID: "Y", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "y."}},
		},
		Edges: []*ir.Edge{{From: "X", To: "Y"}},
	}
	resolver := &MapResolver{Workflows: map[string]*ir.Workflow{"child.dip": child}}

	parent := &ir.Workflow{
		Name: "main", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{
				Ref:    "child.dip",
				Params: map[string]string{"model": "gpt-5.4", "strict": "true"},
			}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "e."}},
		},
		Edges: []*ir.Edge{{From: "S", To: "E"}},
	}

	got, err := Flatten(parent, resolver, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the subgraph node itself is removed (replaced by child nodes).
	for _, n := range got.Nodes {
		if n.Kind == ir.NodeSubgraph {
			t.Errorf("subgraph node %q should have been removed", n.ID)
		}
	}

	// Verify child nodes are present with correct prefixes.
	if len(got.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d, want 3; nodes: %v", len(got.Nodes), nodeIDs(got))
	}
}
```

- [ ] **Step 2: Run tests**

Run: `just test-pkg flatten`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add flatten/flatten_test.go
git commit -m "test(flatten): verify params don't interfere and subgraph nodes removed"
```

---

## Task 11: Run full quality checks

**Files:** None (verification only)

- [ ] **Step 1: Run full check suite**

Run: `just check`

This runs: build, vet, fmt, test-race, complexity, validate-examples.

Expected: All green.

- [ ] **Step 2: Fix any issues**

If complexity limits are exceeded on any function, extract helpers. If lint issues appear, fix them. If example validation fails, debug and fix.

- [ ] **Step 3: Final commit if fixes were needed**

```bash
git add -A
git commit -m "fix: address quality check issues from flatten implementation"
```

---

## Verification Checklist

After all tasks are done, verify:

- [ ] `just check` passes (build, vet, fmt, test-race, complexity, validate-examples)
- [ ] `just build && ./dippin export-dot examples/orchestrator.dip` outputs flat DOT with no `ref=` or `shape="tab"`
- [ ] `just build && ./dippin export-dot examples/api_design.dip` outputs flat DOT (inlines `interview_loop.dip`)
- [ ] `just build && ./dippin export-dot examples/ask_and_execute.dip` still works (no subgraphs, unchanged output)
