# Tool-routing follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship issues #42 (DIP101/DIP102 suppression on marker_grep + DIP138 reserved), #43 (parseBoolAttr widens accepted bool forms), #44 (outputs round-trips through DOT export/migrate) as v0.29.0.

**Architecture:** Three independent code paths, sequenced as five tasks. Task 1 sets up the worktree. Task 2 implements #43 (parser-only, no downstream callers). Task 3 implements #42 (validator + reserved lint code). Task 4 implements #44 (export + migrate). Task 5 writes the CHANGELOG, docs, opens the PR, merges, tags v0.29.0.

**Tech Stack:** Go 1.x, `just` for build automation, golangci-lint + gocyclo (≤5) + gocognit (≤7) enforced via pre-commit hook.

---

## Conventions

- **Always use `just`** — `just check`, `just test-pkg <pkg>`, `just fmt`, etc. Never raw `go test`. (See CLAUDE.md.)
- **Tests must parse real `.dip` text** through the parser. No hand-built IR. (See CLAUDE.md.)
- **Complexity gates:** cyclomatic ≤ 5, cognitive ≤ 7 per function (excludes `_test.go`). Extract helpers if any function grows too big — never add `//nolint`.
- **Dippin parser only recognizes double-quoted strings** as a delimiter. Single quotes are not a delimiter (only valid inside shell command bodies).
- **All work happens in the worktree at the path created in Task 1.** Subagents must cd to that path and verify `git log --oneline -3` before committing.

---

## Task 1: Worktree setup + branch

**Files:**
- N/A (environment setup)

- [ ] **Step 1: Create worktree via EnterWorktree**

Use the EnterWorktree tool with `name: "feat-issue-42-43-44-followups"`. This branches from `origin/main` (default `worktree.baseRef = fresh`) and switches the session CWD into the worktree. Note the path returned — every subsequent subagent prompt must include it explicitly.

- [ ] **Step 2: Verify worktree state**

Run: `git log --oneline -3 && git status`
Expected: tip is the latest origin/main commit (`4ab75b8 chore(site): boost a14y agent-readability score (#46)`), working tree clean, branch is `feat-issue-42-43-44-followups`.

- [ ] **Step 3: Establish baseline**

Run: `just check`
Expected: all checks pass. If anything fails on a clean origin/main, stop and investigate before proceeding.

---

## Task 2: #43 — parseBoolAttr helper + migrate four bool fields

**Files:**
- Modify: `parser/parse_helpers.go` (add `parseBoolAttr` at the end)
- Modify: `parser/parse_nodes.go` (widen signatures of `applyAgentBoolField` + `applyToolBoolField`; swap 4 `val == "true"` lines)
- Create: `parser/parse_bool_test.go`

### Step 2.1: Write the failing test

- [ ] Create `parser/parse_bool_test.go`:

```go
package parser

import (
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// TestParseBoolAttrFields verifies that all four bool fields
// (goal_gate, auto_status, cache_tools, route_required) accept
// the canonical truthy/falsy forms case-insensitively, and that
// any other value produces a parse diagnostic.
func TestParseBoolAttrFields(t *testing.T) {
	cases := []struct {
		name        string
		field       string // "goal_gate" | "auto_status" | "cache_tools" | "route_required"
		val         string
		wantBool    bool
		wantDiagSub string // substring expected in diagnostics; "" means no diag
	}{
		// agent fields — goal_gate
		{"goal_gate=true", "goal_gate", "true", true, ""},
		{"goal_gate=FALSE", "goal_gate", "FALSE", false, ""},
		{"goal_gate=yes", "goal_gate", "yes", true, ""},
		{"goal_gate=no", "goal_gate", "no", false, ""},
		{"goal_gate=1", "goal_gate", "1", true, ""},
		{"goal_gate=0", "goal_gate", "0", false, ""},
		{"goal_gate=on", "goal_gate", "on", true, ""},
		{"goal_gate=Off", "goal_gate", "Off", false, ""},
		{"goal_gate=maybe", "goal_gate", "maybe", false, "invalid boolean"},
		{"goal_gate=2", "goal_gate", "2", false, "invalid boolean"},

		// agent fields — auto_status
		{"auto_status=yes", "auto_status", "yes", true, ""},
		{"auto_status=garbage", "auto_status", "garbage", false, "invalid boolean"},

		// agent fields — cache_tools
		{"cache_tools=on", "cache_tools", "on", true, ""},
		{"cache_tools=nope", "cache_tools", "nope", false, "invalid boolean"},

		// tool field — route_required
		{"route_required=YES", "route_required", "YES", true, ""},
		{"route_required=true", "route_required", "true", true, ""},
		{"route_required=False", "route_required", "False", false, ""},
		{"route_required=maybe", "route_required", "maybe", false, "invalid boolean"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := buildBoolFieldDip(tc.field, tc.val)
			w, diags := ParseString(src)
			got := readBoolField(t, w, tc.field)
			if got != tc.wantBool {
				t.Errorf("field %s = %q: got %v, want %v", tc.field, tc.val, got, tc.wantBool)
			}
			joined := strings.Join(diags, "\n")
			if tc.wantDiagSub == "" {
				if joined != "" {
					t.Errorf("expected no diagnostics, got: %s", joined)
				}
				return
			}
			if !strings.Contains(joined, tc.wantDiagSub) {
				t.Errorf("expected diagnostic containing %q, got: %s", tc.wantDiagSub, joined)
			}
		})
	}
}

// buildBoolFieldDip produces a minimal valid .dip workflow that exercises
// one of the four bool fields. The agent / tool node is the start node so
// the workflow always has at least one node.
func buildBoolFieldDip(field, val string) string {
	switch field {
	case "goal_gate", "auto_status", "cache_tools":
		return "workflow X\n" +
			"  start: A\n" +
			"  exit: A\n" +
			"  agent A\n" +
			"    " + field + ": " + val + "\n"
	case "route_required":
		return "workflow X\n" +
			"  start: T\n" +
			"  exit: T\n" +
			"  tool T\n" +
			"    command: echo hi\n" +
			"    " + field + ": " + val + "\n"
	}
	panic("unknown field: " + field)
}

// readBoolField extracts the value of the bool field under test from the
// parsed workflow. Fails the test if the node or config kind is wrong.
func readBoolField(t *testing.T, w *ir.Workflow, field string) bool {
	t.Helper()
	if len(w.Nodes) == 0 {
		t.Fatal("workflow has no nodes")
	}
	n := w.Nodes[0]
	switch field {
	case "goal_gate":
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			t.Fatalf("expected AgentConfig, got %T", n.Config)
		}
		return cfg.GoalGate
	case "auto_status":
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			t.Fatalf("expected AgentConfig, got %T", n.Config)
		}
		return cfg.AutoStatus
	case "cache_tools":
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			t.Fatalf("expected AgentConfig, got %T", n.Config)
		}
		return cfg.CacheTools
	case "route_required":
		cfg, ok := n.Config.(ir.ToolConfig)
		if !ok {
			t.Fatalf("expected ToolConfig, got %T", n.Config)
		}
		return cfg.RouteRequired
	}
	t.Fatalf("unknown field: %s", field)
	return false
}
```

**Note:** Confirm `ParseString` is the package's public parse entry point. If the actual name differs (e.g., `Parse(src string)`), adjust the call site. Use `grep -n "^func Parse" parser/*.go` to find it.

### Step 2.2: Run the test, verify it fails

- [ ] Run: `just test-pkg parser`
Expected: `TestParseBoolAttrFields` fails. Specifically the `yes`/`on`/`1`/etc. truthy cases will return `false` (strict equality only matches `true`), and the `maybe`/`2` cases will not produce diagnostics (the strict equality silently coerces).

### Step 2.3: Add parseBoolAttr to parse_helpers.go

- [ ] Append to `parser/parse_helpers.go`:

```go
// parseBoolAttr normalizes boolean field parsing across node configs.
// Accepts (case-insensitive, surrounding whitespace tolerated):
//   - truthy: true, 1, yes, on
//   - falsy:  false, 0, no, off
// Any other value produces a parse diagnostic and returns false.
func (p *Parser) parseBoolAttr(val, key string, loc ir.SourceLocation) bool {
	s := strings.ToLower(strings.TrimSpace(val))
	switch s {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	p.diagnostics = append(p.diagnostics, fmt.Sprintf(
		"invalid boolean %q for %s at %d:%d (use true/false, 1/0, yes/no, on/off)",
		val, key, loc.Line, loc.Column))
	return false
}
```

### Step 2.4: Widen applyAgentBoolField signature

- [ ] In `parser/parse_nodes.go`, replace `applyAgentBoolField`:

```go
// applyAgentBoolField handles boolean and string agent fields.
func (p *Parser) applyAgentBoolField(cfg *ir.AgentConfig, key, val string, loc ir.SourceLocation) bool {
	switch key {
	case "goal_gate":
		cfg.GoalGate = p.parseBoolAttr(val, key, loc)
	case "auto_status":
		cfg.AutoStatus = p.parseBoolAttr(val, key, loc)
	case "cache_tools":
		cfg.CacheTools = p.parseBoolAttr(val, key, loc)
	case "compaction":
		cfg.Compaction = val
	default:
		return false
	}
	return true
}
```

### Step 2.5: Update applyAgentComplexField caller

- [ ] In `parser/parse_nodes.go`, in `applyAgentComplexField`, change the call site:

From:
```go
	if applyAgentBoolField(cfg, key, val) {
		return
	}
```
To:
```go
	if p.applyAgentBoolField(cfg, key, val, loc) {
		return
	}
```

### Step 2.6: Widen applyToolBoolField signature

- [ ] In `parser/parse_nodes.go`, replace `applyToolBoolField`:

```go
// applyToolBoolField handles boolean tool fields. Returns true if handled.
func (p *Parser) applyToolBoolField(cfg *ir.ToolConfig, key, val string, loc ir.SourceLocation) bool {
	switch key {
	case "route_required":
		cfg.RouteRequired = p.parseBoolAttr(val, key, loc)
	default:
		return false
	}
	return true
}
```

### Step 2.7: Update applyToolField caller

- [ ] In `parser/parse_nodes.go`, in `applyToolField`, change the call site:

From:
```go
	if applyToolBoolField(cfg, key, val) {
		return
	}
```
To:
```go
	if p.applyToolBoolField(cfg, key, val, loc) {
		return
	}
```

### Step 2.8: Run the test, verify it passes

- [ ] Run: `just test-pkg parser`
Expected: all parser tests pass, including the new `TestParseBoolAttrFields` with all 18 sub-cases.

### Step 2.9: Run the full check

- [ ] Run: `just check`
Expected: build, vet, lint, complexity, tests, examples all pass.

If complexity rejects `applyAgentBoolField` or `applyToolBoolField`, extract `parseAgentBoolKey` / `parseToolBoolKey` helpers. Don't `//nolint`.

### Step 2.10: Commit

- [ ] Run:

```bash
git add parser/parse_helpers.go parser/parse_nodes.go parser/parse_bool_test.go
git commit -m "$(cat <<'EOF'
feat(parser): widen bool field parsing via parseBoolAttr (#43)

Adds case-insensitive acceptance of true/false, 1/0, yes/no, on/off
across goal_gate, auto_status, cache_tools, route_required. Anything
else now produces a parse diagnostic instead of silently coercing to
false.

Closes #43

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] Verify: `git log --oneline -1` shows this commit on `feat-issue-42-43-44-followups`.

---

## Task 3: #42 — DIP101/DIP102 suppression on marker_grep + DIP138 reserved

**Files:**
- Modify: `validator/lint_reachability.go` (add `toolHasMarkerRouting`; wire into `sourceIsSafe` and `lintDefaultEdge`)
- Modify: `validator/lint_codes.go` (add `DIP138` constant + description entry)
- Modify: `validator/explanations.go` (add DIP138 explanation)
- Modify: `validator/lint_test.go` (new test case)

### Step 3.1: Write the failing test

- [ ] In `validator/lint_test.go`, find the existing `TestLint` table (around line 110, where the `DIP101` cases live) and add two new cases at the end of the table, before the closing `}`:

```go
		// --- DIP138 reserve + marker_grep suppression ---
		{
			name: "DIP101/DIP102: marker_grep on tool node suppresses both",
			src: `workflow MarkerSafe
  start: T
  exit: D
  tool T
    command: echo hi
    marker_grep: "^(go|stop)$"
  agent D
    prompt: "done"
  edges
    T -> D when ctx.tool_marker = go
`,
			wantCodes: []string{}, // no DIP101, no DIP102
		},
		{
			name: "DIP101/DIP102: without marker_grep, same shape fires both",
			src: `workflow MarkerUnsafe
  start: T
  exit: D
  tool T
    command: echo hi
  agent D
    prompt: "done"
  edges
    T -> D when ctx.tool_marker = go
`,
			wantCodes: []string{DIP101, DIP102},
		},
```

**Note:** Check the existing test rows for the exact field layout. If the existing rows use a `src string` field, match it; if they use a workflow-builder, adapt to match. The intent is two parsed-from-source rows.

### Step 3.2: Run the test, verify it fails

- [ ] Run: `just test-pkg validator`
Expected: the first new case (`marker_grep suppresses`) fails with `wantCodes` empty but `DIP101` + `DIP102` reported. The second case may pass (control case).

### Step 3.3: Add toolHasMarkerRouting helper

- [ ] In `validator/lint_reachability.go`, after `hasUnconditionalEdge`, add:

```go
// toolHasMarkerRouting returns true if the node is a tool with a non-empty
// marker_grep declaration. Such nodes route via ctx.tool_marker, a typed
// channel that the engine populates; outgoing conditional edges are
// intentional routing and DIP101/DIP102 should not fire on them.
func toolHasMarkerRouting(w *ir.Workflow, nodeID string) bool {
	n := w.Node(nodeID)
	if n == nil {
		return false
	}
	cfg, ok := n.Config.(ir.ToolConfig)
	if !ok {
		return false
	}
	return cfg.MarkerGrep != ""
}
```

### Step 3.4: Wire into sourceIsSafe (DIP101 path)

- [ ] In `validator/lint_reachability.go`, replace `sourceIsSafe`:

```go
// sourceIsSafe returns true if a source node guarantees its conditional
// destinations are intentional: via exhaustive conditions, an unconditional
// outgoing edge (mixed routing), or marker_grep-driven typed routing.
func sourceIsSafe(w *ir.Workflow, nodeID string, outgoing map[string][]*ir.Edge, exhaustive map[string]bool) bool {
	if exhaustive[nodeID] {
		return true
	}
	if hasUnconditionalEdge(outgoing[nodeID]) {
		return true
	}
	return toolHasMarkerRouting(w, nodeID)
}
```

The signature gains a `w *ir.Workflow` parameter. Update the caller `allSourcesSafe` in the same file:

```go
func allSourcesSafe(w *ir.Workflow, edges []*ir.Edge, outgoing map[string][]*ir.Edge, exhaustive map[string]bool) bool {
	for _, e := range edges {
		if !sourceIsSafe(w, e.From, outgoing, exhaustive) {
			return false
		}
	}
	return true
}
```

And update `checkConditionalReachability` to pass `w` through:

```go
func checkConditionalReachability(w *ir.Workflow, n *ir.Node, start string, incoming, outgoing map[string][]*ir.Edge, exhaustive map[string]bool) (Diagnostic, bool) {
	if n.ID == start {
		return Diagnostic{}, false
	}
	edges := incoming[n.ID]
	if len(edges) == 0 {
		return Diagnostic{}, false
	}
	if allEdgesConditional(edges) && !allSourcesSafe(w, edges, outgoing, exhaustive) {
		return Diagnostic{
			Code:     DIP101,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("node %q is only reachable through conditional edges and may be skipped at runtime", n.ID),
			Location: n.Source,
			Help:     "add an unconditional edge to this node, or verify all conditions are exhaustive",
		}, true
	}
	return Diagnostic{}, false
}
```

And update the call from `lintConditionalReachability`:

```go
func lintConditionalReachability(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	incoming := buildIncomingEdgeMap(w)
	outgoing := buildOutgoingEdgeMap(w)
	exhaustiveSources := findExhaustiveSources(w)

	for _, n := range w.Nodes {
		if d, ok := checkConditionalReachability(w, n, w.Start, incoming, outgoing, exhaustiveSources); ok {
			diags = append(diags, d)
		}
	}
	return diags
}
```

### Step 3.5: Wire into DIP102 path (lintDefaultEdge)

- [ ] In `validator/lint_reachability.go`, replace `lintDefaultEdge`:

```go
// lintDefaultEdge checks DIP102: nodes that have outgoing conditional edges
// but no unconditional (default/fallback) edge. Without a default edge,
// execution may get stuck at this node if no condition matches.
//
// Nodes whose outgoing conditions are exhaustive, or tool nodes with
// marker_grep-driven typed routing, are not flagged.
func lintDefaultEdge(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	exhaustiveSources := findExhaustiveSources(w)

	for _, n := range w.Nodes {
		outgoing := w.EdgesFrom(n.ID)
		if len(outgoing) == 0 {
			continue
		}
		if hasMissingDefault(outgoing) && !exhaustiveSources[n.ID] && !toolHasMarkerRouting(w, n.ID) {
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
```

### Step 3.6: Reserve DIP138 in lint_codes.go

- [ ] In `validator/lint_codes.go`, find the existing DIP codes block. The file is small — read it first if uncertain. Add `DIP138` to the constant block following the existing pattern:

Inspect with: `grep -n "DIP13" validator/lint_codes.go` — note the highest existing DIP13x code and insert DIP138 in numeric order.

Add:

```go
	DIP138 = "DIP138" // tool node routes on stdout but declares no marker_grep / outputs (reserved)
```

And in the `CodeDescription` init block (or wherever `CodeDescription[DIP137] = "..."` lives):

```go
	CodeDescription[DIP138] = "tool node routes on stdout but declares no marker_grep / outputs"
```

### Step 3.7: Add DIP138 to explanations.go

- [ ] In `validator/explanations.go`, find the existing DIP entry pattern (e.g., the DIP102 entry around line 113). Add a DIP138 entry:

```go
		DIP138: {
			Code:    DIP138,
			Summary: "tool node routes on stdout but declares no marker_grep / outputs",
			Details: "When a tool node has outgoing conditional edges that test ctx.tool_stdout but the tool itself does not declare a marker_grep or outputs field, the workflow is using untyped stdout-text routing. Prefer the typed marker_grep channel (ctx.tool_marker) for clearer intent and better lint coverage. (Reserved: no firing logic in v0.29.0.)",
			Fix:     "Add marker_grep: \"<regex>\" to the tool node and switch edges to test ctx.tool_marker, or declare outputs: <values> so coverage analysis can see the routing set.",
		},
```

Match the field names of the existing entries (e.g., DIP102) — if `Summary` is actually called `Title` or similar, adapt. Read `explanations.go` lines 106-120 first to be sure.

### Step 3.8: Check if a registered-codes count test needs updating

- [ ] Inspect: `grep -n "DIP10\|DIP11\|DIP12\|DIP13" validator/lint_test.go | head -20`

Look for tests like `TestExplanationsAndCodes` or test fixtures that enumerate every code. If `DIP138` must be added to such a list (e.g., the `codes := []string{...}` slice around line 1398), add it.

If the test enumerates codes as `DIP101..DIP137`, append `DIP138`.

### Step 3.9: Run tests, verify they pass

- [ ] Run: `just test-pkg validator`
Expected: all validator tests pass, including the two new cases from Step 3.1.

### Step 3.10: Run the full check

- [ ] Run: `just check`
Expected: all checks pass. If `lintDefaultEdge` exceeds cognitive complexity (it now has three guards), extract `nodeIsSafeRouter(w, exhaustive, nodeID)` helper.

### Step 3.11: Commit

- [ ] Run:

```bash
git add validator/lint_reachability.go validator/lint_codes.go validator/explanations.go validator/lint_test.go
git commit -m "$(cat <<'EOF'
feat(validator): suppress DIP101/DIP102 on marker_grep tool nodes (#42)

A tool node declaring marker_grep: routes via the typed ctx.tool_marker
channel. Outgoing conditional edges on such a node are intentional
routing, not unsafe reachability. Treat marker_grep tool nodes as
"safe sources" in both DIP101 (propagation) and DIP102 (direct) checks.

Also reserves DIP138 for a future advisory ("tool node parses stdout
for routing but declares no marker_grep / outputs"). No firing logic
in this release.

Closes #42

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] Verify: `git log --oneline -2` shows the two commits on `feat-issue-42-43-44-followups`.

---

## Task 4: #44 — outputs round-trips through DOT

**Files:**
- Modify: `export/dot.go` (`applyToolSemanticAttrs` — emit outputs)
- Modify: `migrate/migrate.go` (`applyToolStringAttrs` — read outputs)
- Modify: `migrate/roundtrip_test.go` (new round-trip test)

### Step 4.1: Write the failing round-trip test

- [ ] First inspect the existing roundtrip test file to match its style: `grep -n "^func Test" migrate/roundtrip_test.go | head -10`.

Append to `migrate/roundtrip_test.go` (adapt imports / helpers to match existing patterns):

```go
// TestRoundtripPreservesToolOutputs verifies that ToolConfig.Outputs
// survives a .dip → DOT → .dip migration. Regression test for the
// silent-drop bug in v0.28.0 where applyToolSemanticAttrs never
// emitted the outputs DOT attr.
func TestRoundtripPreservesToolOutputs(t *testing.T) {
	src := `workflow MarkerRouting
  start: T
  exit: D
  tool T
    command: echo hi
    outputs: tests_green, tests_red
    marker_grep: "^(tests_green|tests_red)$"
  agent D
    prompt: "done"
  edges
    T -> D when ctx.tool_marker = tests_green
    T -> D when ctx.tool_marker = tests_red
`
	wf, diags := parser.ParseString(src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %v", diags)
	}

	// Export to DOT
	var buf bytes.Buffer
	if err := export.WriteDOT(&buf, wf, export.Options{}); err != nil {
		t.Fatalf("WriteDOT: %v", err)
	}
	dot := buf.String()
	if !strings.Contains(dot, `outputs="tests_green,tests_red"`) {
		t.Errorf("DOT export missing outputs attr; got:\n%s", dot)
	}

	// Re-import from DOT
	wf2, err := migrate.FromDOT(buf.String())
	if err != nil {
		t.Fatalf("FromDOT: %v", err)
	}
	node := wf2.Node("T")
	if node == nil {
		t.Fatal("re-imported workflow missing node T")
	}
	cfg, ok := node.Config.(ir.ToolConfig)
	if !ok {
		t.Fatalf("expected ToolConfig, got %T", node.Config)
	}
	want := []string{"tests_green", "tests_red"}
	if !reflect.DeepEqual(cfg.Outputs, want) {
		t.Errorf("outputs: got %v, want %v", cfg.Outputs, want)
	}
}
```

**Note:** The exact function names (`export.WriteDOT`, `migrate.FromDOT`, `parser.ParseString`) and Options struct may differ. Run `grep -n "^func " export/dot.go migrate/migrate.go parser/parser.go | head -20` first and match the actual signatures. Same for imports — check the existing test file's import block and reuse it.

### Step 4.2: Run the test, verify it fails

- [ ] Run: `just test-pkg migrate`
Expected: `TestRoundtripPreservesToolOutputs` fails. The DOT export will be missing `outputs="..."`, OR the re-import will have empty `cfg.Outputs`.

### Step 4.3: Emit outputs from DOT export

- [ ] In `export/dot.go`, modify `applyToolSemanticAttrs` (current body shown for reference):

From:
```go
func applyToolSemanticAttrs(attrs map[string]string, cfg ir.ToolConfig) {
	if cfg.Timeout != 0 {
		attrs["timeout"] = formatDuration(cfg.Timeout)
	}
	if cfg.MarkerGrep != "" {
		attrs["marker_grep"] = cfg.MarkerGrep
	}
	if cfg.RouteRequired {
		attrs["route_required"] = "true"
	}
	if cfg.OutputLimit > 0 {
		attrs["output_limit"] = strconv.Itoa(cfg.OutputLimit)
	}
}
```
To:
```go
func applyToolSemanticAttrs(attrs map[string]string, cfg ir.ToolConfig) {
	if cfg.Timeout != 0 {
		attrs["timeout"] = formatDuration(cfg.Timeout)
	}
	if cfg.MarkerGrep != "" {
		attrs["marker_grep"] = cfg.MarkerGrep
	}
	if cfg.RouteRequired {
		attrs["route_required"] = "true"
	}
	if cfg.OutputLimit > 0 {
		attrs["output_limit"] = strconv.Itoa(cfg.OutputLimit)
	}
	if len(cfg.Outputs) > 0 {
		attrs["outputs"] = strings.Join(cfg.Outputs, ",")
	}
}
```

**Note:** Check whether `strings` is already imported in `export/dot.go`; if not, add it. If `strings.Join` would push complexity over the limit, extract a helper — but adding one statement to an already-flat function should be fine.

### Step 4.4: Read outputs back in migrate

- [ ] In `migrate/migrate.go`, modify `applyToolStringAttrs`. Confirm which split helper migrate uses for comma-separated values — likely `splitComma` (grep for it in the file). Use the same one for consistency.

From:
```go
func applyToolStringAttrs(cfg *ir.ToolConfig, attrs map[string]string) {
	if v, ok := attrs["tool_command"]; ok {
		cfg.Command = v
	}
	if v, ok := attrs["marker_grep"]; ok {
		cfg.MarkerGrep = v
	}
	if v, ok := attrs["route_required"]; ok {
		cfg.RouteRequired = (v == "true")
	}
}
```
To:
```go
func applyToolStringAttrs(cfg *ir.ToolConfig, attrs map[string]string) {
	if v, ok := attrs["tool_command"]; ok {
		cfg.Command = v
	}
	if v, ok := attrs["marker_grep"]; ok {
		cfg.MarkerGrep = v
	}
	if v, ok := attrs["route_required"]; ok {
		cfg.RouteRequired = (v == "true")
	}
	if v, ok := attrs["outputs"]; ok {
		cfg.Outputs = splitComma(v)
	}
}
```

If the migrate package uses a different helper name (e.g., `splitCommaTrimmed`), use that name. If migrate has no such helper, use `strings.Split` + trim each part inline, matching the export side's serialization.

### Step 4.5: Run the test, verify it passes

- [ ] Run: `just test-pkg migrate`
Expected: `TestRoundtripPreservesToolOutputs` passes. Also run `just test-pkg export` to confirm no export-side regression.

### Step 4.6: Run the full check

- [ ] Run: `just check`
Expected: all checks pass. The pre-existing `TestCompareToolConfigsDifferentOutputs` should still pass (Outputs parity already shipped in v0.28.0).

### Step 4.7: Commit

- [ ] Run:

```bash
git add export/dot.go migrate/migrate.go migrate/roundtrip_test.go
git commit -m "$(cat <<'EOF'
feat(export,migrate): round-trip ToolConfig.Outputs through DOT (#44)

DOT export now emits outputs="…" on tool nodes that declare outputs,
and the migrate path reads it back. Closes the silent-drop bug where
.dip → DOT → .dip collapsed cfg.Outputs to nil.

Closes #44

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] Verify: `git log --oneline -3` shows three commits on `feat-issue-42-43-44-followups`.

---

## Task 5: CHANGELOG + docs + PR + tag

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `docs/nodes.md`
- Modify: `docs/llm-reference.md`
- Modify: `site/static/skill.md`
- Auto-regenerates on commit: `cmd/dippin/generated-spec.md`, `docs/generated-spec.md`

### Step 5.1: Read the previous CHANGELOG entries

- [ ] Read: `CHANGELOG.md` (first 60 lines) to match the v0.27 / v0.28 entry style exactly.

### Step 5.2: Add v0.29.0 entry to CHANGELOG.md

- [ ] Add a new entry at the top of the changelog (immediately under the title / under any "Unreleased" placeholder, matching the existing pattern). Use this content and adapt headers to match the existing template:

```markdown
## [v0.29.0] — 2026-05-19

### Added
- DIP138 reserved for the future advisory "tool node routes on stdout but declares no marker_grep / outputs" — no firing logic in this release. (#42)
- `outputs:` field now survives a `.dip → DOT → .dip` round-trip. The DOT export emits `outputs="a,b,c"` on tool nodes that declare outputs, and `dippin migrate dot→.dip` reads it back. (#44)

### Changed
- DIP101 / DIP102 now suppress on tool nodes that declare `marker_grep:`. Those nodes route via the typed `ctx.tool_marker` channel — outgoing conditional edges on them are intentional routing, not unsafe reachability. (#42)
- Parser bool fields (`goal_gate`, `auto_status`, `cache_tools`, `route_required`) now accept `true/false`, `1/0`, `yes/no`, `on/off` case-insensitively via a new shared `parseBoolAttr` helper. Anything else now produces a parse diagnostic instead of silently coercing to `false`. The migrate (DOT-input) path keeps strict equality since DOT attrs are machine-emitted. (#43)

### Closed
- #42 — Lint: suppress DIP101/DIP102 on tool nodes that declare marker_grep
- #43 — Parser: introduce parseBoolAttr to normalize boolean field parsing
- #44 — DOT export / migrate: forward ToolConfig.Outputs through DOT attrs
```

### Step 5.3: Update docs/nodes.md

- [ ] Find the "Markers and Verbose Output" section: `grep -n "Markers and Verbose" docs/nodes.md`. Append at the end of that section (read 20 lines of context first so the prose flows):

```markdown

> **Lint suppression.** A tool node that declares `marker_grep:` is treated as a "safe routing source" by the validator. Outgoing conditional edges that test `ctx.tool_marker` no longer trip DIP101 (unreachable target) or DIP102 (no default edge), even if there's only a single conditional edge — the declaration is an explicit author signal that routing is typed.
```

Also find the bool-field documentation (search for `goal_gate` or `route_required`). Append a note about accepted forms:

```markdown

> **Boolean fields** (`goal_gate`, `auto_status`, `cache_tools`, `route_required`) accept `true`/`false`, `1`/`0`, `yes`/`no`, `on`/`off`, case-insensitive. Any other value emits a parse diagnostic.
```

If `docs/nodes.md` doesn't have a unified spot for bool field documentation, add it next to the most relevant field (likely `route_required`).

### Step 5.4: Update docs/llm-reference.md

- [ ] Find the "common mistakes" or equivalent table: `grep -n "common mistake\|Common Mistake" docs/llm-reference.md`. Add a row that cross-references the new suppression behavior:

```markdown
| `dippin lint` complains DIP101/DIP102 on marker-routed tool node | Add `marker_grep: "<regex>"` to the tool — `ctx.tool_marker` routing is typed and treated as safe. |
```

Match the existing column count of the table.

### Step 5.5: Update site/static/skill.md

- [ ] This file is the hosted Claude skill. Search for the same sections you edited in `docs/nodes.md` and mirror the additions verbatim.

### Step 5.6: Run the check (regenerates auto-spec)

- [ ] Run: `just check`
Expected: all checks pass. Pre-commit will regenerate `docs/generated-spec.md` and `cmd/dippin/generated-spec.md`.

### Step 5.7: Commit docs

- [ ] Run:

```bash
git add CHANGELOG.md docs/nodes.md docs/llm-reference.md site/static/skill.md
git commit -m "$(cat <<'EOF'
docs(v0.29.0): document marker_grep lint suppression, bool form normalization, outputs round-trip

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If the pre-commit hook also stages `docs/generated-spec.md` or `cmd/dippin/generated-spec.md`, let it (the hook handles this automatically).

### Step 5.8: Push branch + open PR

- [ ] Run:

```bash
git push -u origin feat-issue-42-43-44-followups
```

- [ ] Open PR with:

```bash
gh pr create --title "v0.29.0: tool-routing follow-ups (#42 #43 #44)" --body "$(cat <<'EOF'
## Summary

- **#42** — Suppress DIP101/DIP102 on tool nodes with `marker_grep:`. Reserve DIP138 for a future advisory.
- **#43** — Normalize bool parsing across `goal_gate`, `auto_status`, `cache_tools`, `route_required` via `parseBoolAttr`. Accepts `true/false`, `1/0`, `yes/no`, `on/off` case-insensitively; rejects anything else with a diagnostic.
- **#44** — Forward `ToolConfig.Outputs` through DOT export and migrate so `.dip → DOT → .dip` preserves the field.

Closes #42, closes #43, closes #44.

## Test plan
- [x] `just check` — full suite green locally
- [x] `validator/lint_test.go` — new marker_grep suppression case + control case
- [x] `parser/parse_bool_test.go` — table-driven coverage of all 4 fields × accepted/rejected forms
- [x] `migrate/roundtrip_test.go` — `.dip → DOT → .dip` preserves `outputs:`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

### Step 5.9: Monitor PR + merge

- [ ] Poll the PR until merged (do not auto-merge without checks). Use:

```bash
gh pr view --json state,statusCheckRollup,reviewDecision
```

Watch `state` (not just `reviewDecision`) — when it flips to `MERGED`, proceed.

If CodeRabbit (or any reviewer) leaves comments, triage them: address substantive ones with a follow-up commit on the same branch, dismiss style nits politely.

- [ ] Once merged, switch to main and pull:

```bash
cd /home/clint/code/2389/dippin-lang  # main repo, not the worktree
git checkout main
git pull --ff-only origin main
```

### Step 5.10: Tag v0.29.0

- [ ] Run from the main repo (not the worktree):

```bash
git tag -a v0.29.0 -m "v0.29.0 — tool-routing follow-ups (#42 #43 #44)"
git push origin v0.29.0
```

This triggers GoReleaser via GitHub Actions.

### Step 5.11: Verify release

- [ ] Wait ~2-3 minutes for GoReleaser, then:

```bash
gh release view v0.29.0
```

Expected: the release exists with cross-platform binaries published.

### Step 5.12: Clean up the worktree

- [ ] Confirm `feat-issue-42-43-44-followups` is reachable from `origin/main` (it will be, since the PR was merged). Then:

```bash
git fetch origin
git branch --contains feat-issue-42-43-44-followups | grep -q "origin/main"  # sanity check
```

- [ ] Use the ExitWorktree tool with `action: "remove"` and `discard_changes: true` (the work is committed and on origin; nothing to preserve in the worktree).

### Step 5.13: Report

- [ ] Tell the user:
  - Which issues closed (#42, #43, #44)
  - The release URL (from `gh release view v0.29.0 --json url`)
  - Any noteworthy review feedback addressed during the PR

---

## Self-review notes

- Every step has either code or an exact command.
- Type / method consistency: `parseBoolAttr` signature is identical everywhere it appears (Task 2). `toolHasMarkerRouting` signature is identical everywhere it appears (Task 3). `applyToolStringAttrs` modification matches the actual current source in Task 4.
- Each task is independently committable and produces working software (Tasks 2-4 each end with a green `just check`).
- The DOT round-trip example in Task 4.1 uses a workflow that — without #42 — would have tripped DIP101/DIP102. Order matters: Task 3 must complete before Task 4's test will pass cleanly under `just check`. (It would pass with `just test-pkg migrate` even without Task 3 since the round-trip test doesn't run the validator. But `just validate-examples` doesn't lint either, so this is fine for ordering — the round-trip test is self-contained.)
- Docs intentionally come last so the PR doc claims reflect the actually-shipped behavior.
