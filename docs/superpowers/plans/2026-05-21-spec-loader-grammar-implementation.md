# Spec Loader Grammar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add optional `spec:` workflow header and `satisfies:` node attribute to the dippin grammar + IR, plus four new lint codes (DIP139–DIP142), CLI surfacing in parse/export-dot/fmt/doctor, and tree-sitter coverage. Backwards-compatible, dippin-only — no runtime semantics.

**Architecture:** Five layers, in dependency order:

1. **IR** — new `ir/spec.go` with `SpecRef`; new `Workflow.Spec` and `Node.Satisfies` fields.
2. **Parser** — `spec:` as a workflow header field (mirrors `requires:` parser at `parser/parser.go:155`); `satisfies:` as a common node field (extends `applyCommonPlainField` at `parser/parse_nodes.go:187`).
3. **Validator** — DIP139 (malformed ACID), DIP140 (`satisfies:` without `spec:`), DIP141 (`spec:` without `satisfies:`), DIP142 (duplicate ACID).
4. **CLI surface** — `dippin parse --format json` includes both fields; `dippin export-dot` renders satisfies sublabel; `dippin fmt` canonical placement; `dippin doctor` "Spec" probe.
5. **Tree-sitter grammar** — extend `editors/tree-sitter-dippin/grammar.js` with `spec` and `satisfies` keywords + at least one corpus test.

**Tech Stack:** Go 1.25+. Test runner via `just test-pkg <pkg>` for single packages; `just check` for the full pre-commit suite. All `.dip` parsing through `github.com/2389-research/dippin-lang/parser.NewParser(src, "filename").Parse()` per CLAUDE.md.

**Reference:** `docs/superpowers/specs/2026-05-21-spec-loader-grammar-design.md`.

---

## File Structure

**Created:**
- `ir/spec.go` — `SpecRef` type
- `ir/spec_test.go` — round-trip tests
- `validator/lint_satisfies.go` — DIP139–DIP142 implementation
- `validator/lint_satisfies_test.go` — DIP139–DIP142 unit tests
- `examples/spec_loader.dip` — example workflow exercising both fields
- `editors/tree-sitter-dippin/test/corpus/spec_loader.txt` — tree-sitter corpus test

**Modified:**
- `ir/ir.go` — `Workflow.Spec *SpecRef`, `Node.Satisfies []string`
- `parser/parser.go` — `dispatchWorkflowTailField` adds `"spec"` case
- `parser/parse_nodes.go` — `applyCommonPlainField` adds `"satisfies"` case
- `parser/parser_test.go` — parse tests for both fields
- `parser/roundtrip_test.go` — round-trip both fields through parse → render → parse
- `validator/codes.go` — extend `CodeDescription` for DIP139–DIP142
- `validator/lint_codes.go` — DIP139–DIP142 constants and descriptions
- `validator/lint.go` — register `lintSatisfies` in the lint pipeline
- `validator/explanations.go` — explanations for DIP139–DIP142
- `validator/explanations_test.go` — cover the new codes
- `cmd/dippin/cmd_parse.go` — surfaces are already JSON-encoded reflection of IR; verify
- `export/export.go` — render `satisfies:` sublabel in DOT output
- `formatter/formatter.go` — canonical placement of `spec:` and `satisfies:`
- `doctor/doctor.go` — "Spec" probe section
- `docs/GRAMMAR.ebnf` — add `spec` and `satisfies` to workflow_field and common_field productions
- `editors/tree-sitter-dippin/grammar.js` — add `spec` and `satisfies` keywords to header/node field productions
- `editors/vscode/syntaxes/dippin.tmLanguage.json` — highlight `spec` and `satisfies` (if keywords are enumerated there)
- `editors/zed-dippin/languages/dippin/highlights.scm` — same
- `CHANGELOG.md` — `## [Unreleased]` entry
- `README.md` — short blurb under the language reference, pointing to design doc

---

## Task 1: IR — failing tests first

**Files:**
- Create: `ir/spec.go` (after Step 2)
- Create: `ir/spec_test.go`
- Modify: `ir/ir.go`

- [ ] **Step 1: Write failing test for `SpecRef` and `Workflow.Spec` field**

Create `ir/spec_test.go`:

```go
package ir_test

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestWorkflow_Spec_Nil_ByDefault(t *testing.T) {
	w := &ir.Workflow{}
	if w.Spec != nil {
		t.Fatalf("expected Workflow.Spec nil by default, got %#v", w.Spec)
	}
}

func TestWorkflow_Spec_RoundTrip(t *testing.T) {
	w := &ir.Workflow{Spec: &ir.SpecRef{Loader: "acai", Path: "features.yaml"}}
	if w.Spec.Loader != "acai" || w.Spec.Path != "features.yaml" {
		t.Fatalf("Spec did not round-trip: %#v", w.Spec)
	}
}

func TestNode_Satisfies_Nil_ByDefault(t *testing.T) {
	n := &ir.Node{}
	if n.Satisfies != nil {
		t.Fatalf("expected Node.Satisfies nil by default, got %#v", n.Satisfies)
	}
}

func TestNode_Satisfies_RoundTrip(t *testing.T) {
	n := &ir.Node{Satisfies: []string{"foo.BAR.1", "foo.BAR.2"}}
	if len(n.Satisfies) != 2 || n.Satisfies[0] != "foo.BAR.1" {
		t.Fatalf("Satisfies did not round-trip: %#v", n.Satisfies)
	}
}
```

Run: `just test-pkg ir`

Expected: **compile failure** — `SpecRef`, `Workflow.Spec`, and `Node.Satisfies` don't exist yet.

- [ ] **Step 2: Create `ir/spec.go`**

```go
package ir

// SpecRef references an external spec document the workflow's nodes can
// declare alignment with via Node.Satisfies. The loader name is a key into
// a runtime-side registry (e.g. "acai"); the path is resolved relative to
// the .dip file's directory. Dippin does not load or interpret the spec —
// it only carries the reference through the IR.
type SpecRef struct {
	Loader string
	Path   string
}
```

- [ ] **Step 3: Add fields to `ir.Workflow` and `ir.Node`**

In `ir/ir.go`:

```go
// Workflow is the top-level IR structure representing a complete pipeline.
type Workflow struct {
	// ...existing fields...
	Spec       *SpecRef          // Optional external-spec reference; nil = no spec attached
}
```

```go
// Node represents a single step in the workflow.
type Node struct {
	// ...existing fields...
	Satisfies []string // Optional list of spec requirement refs this node satisfies; nil = none
}
```

- [ ] **Step 4: Re-run tests, confirm pass**

Run: `just test-pkg ir`

Expected: all four new tests pass; existing tests unchanged.

---

## Task 2: Parser — `spec:` workflow header

**Files:**
- Modify: `parser/parser.go` (`dispatchWorkflowTailField`, add `parseWorkflowSpecField`)
- Modify: `parser/parser_test.go`

- [ ] **Step 1: Failing test**

In `parser/parser_test.go`, add:

```go
func TestParser_WorkflowSpec_Basic(t *testing.T) {
	src := `workflow X
  goal: "test"
  spec: acai path/to/features.yaml
  start: A
  exit: A

  agent A
    label: A
`
	w, err := NewParser(src, "test.dip").Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if w.Spec == nil {
		t.Fatalf("expected Spec to be set")
	}
	if w.Spec.Loader != "acai" {
		t.Errorf("loader = %q, want acai", w.Spec.Loader)
	}
	if w.Spec.Path != "path/to/features.yaml" {
		t.Errorf("path = %q, want path/to/features.yaml", w.Spec.Path)
	}
}

func TestParser_WorkflowSpec_MissingPath(t *testing.T) {
	src := `workflow X
  goal: "test"
  spec: acai
  start: A
  exit: A

  agent A
    label: A
`
	_, err := NewParser(src, "test.dip").Parse()
	if err == nil {
		t.Fatalf("expected parse error for spec without path")
	}
}

func TestParser_WorkflowSpec_Absent(t *testing.T) {
	src := `workflow X
  goal: "test"
  start: A
  exit: A

  agent A
    label: A
`
	w, err := NewParser(src, "test.dip").Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if w.Spec != nil {
		t.Fatalf("expected Spec nil when absent, got %#v", w.Spec)
	}
}
```

Run: `just test-pkg parser -run TestParser_WorkflowSpec`

Expected: FAIL — `"spec"` is not a recognized workflow field.

- [ ] **Step 2: Add `spec` to dispatcher**

In `parser/parser.go`, `dispatchWorkflowTailField`:

```go
func dispatchWorkflowTailField(p *Parser, t Token) bool {
	switch t.Value {
	case "edges":
		p.parseEdges()
	case "stylesheet":
		p.parseStylesheet()
	case "requires":
		p.parseWorkflowRequiresField(t)
	case "spec":
		p.parseWorkflowSpecField(t)
	default:
		return false
	}
	return true
}
```

Add helper (keep cyclomatic ≤ 5):

```go
// parseWorkflowSpecField parses "spec: <loader> <path>" into Workflow.Spec.
// Both loader and path are required; missing path emits a diagnostic and leaves Spec nil.
func (p *Parser) parseWorkflowSpecField(t Token) {
	p.lexer.NextToken() // spec
	p.expect(TokenColon)
	val := p.readFieldValue(t.Location.Line)
	loader, path, ok := splitSpecValue(val)
	if !ok {
		p.diagnostics = append(p.diagnostics, fmt.Sprintf(
			"spec: requires both a loader name and a path at %d:%d (got %q)",
			t.Location.Line, t.Location.Column, val))
		return
	}
	p.workflow.Spec = &ir.SpecRef{Loader: loader, Path: path}
}

// splitSpecValue splits "loader path" on the first whitespace run.
// Returns ok=false if either side is empty.
func splitSpecValue(val string) (loader, path string, ok bool) {
	val = strings.TrimSpace(val)
	if val == "" {
		return "", "", false
	}
	idx := strings.IndexAny(val, " \t")
	if idx < 0 {
		return "", "", false
	}
	loader = strings.TrimSpace(val[:idx])
	path = strings.TrimSpace(val[idx+1:])
	if loader == "" || path == "" {
		return "", "", false
	}
	return loader, path, true
}
```

- [ ] **Step 3: Re-run tests, confirm pass**

Run: `just test-pkg parser -run TestParser_WorkflowSpec`

Expected: all three tests pass.

---

## Task 3: Parser — `satisfies:` common node field

**Files:**
- Modify: `parser/parse_nodes.go` (`applyCommonPlainField`)
- Modify: `parser/parser_test.go`

- [ ] **Step 1: Failing test**

```go
func TestParser_NodeSatisfies_Basic(t *testing.T) {
	src := `workflow X
  goal: "test"
  start: A
  exit: A

  agent A
    label: A
    satisfies: foo.BAR.1, foo.BAR.2-1, foo.BAR.[1-3], foo.BAR.*
`
	w, err := NewParser(src, "test.dip").Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	got := w.Nodes[0].Satisfies
	want := []string{"foo.BAR.1", "foo.BAR.2-1", "foo.BAR.[1-3]", "foo.BAR.*"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %#v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("entry %d: got %q, want %q", i, got[i], w)
		}
	}
}

func TestParser_NodeSatisfies_AllKinds(t *testing.T) {
	// Verify satisfies works on every node kind (it's a common field).
	for _, kind := range []string{"agent", "human", "tool", "subgraph", "conditional", "manager_loop"} {
		src := "workflow X\n  goal: \"t\"\n  start: N\n  exit: N\n\n  " + kind + " N\n    label: N\n    satisfies: foo.BAR.1\n"
		w, err := NewParser(src, "test.dip").Parse()
		if err != nil {
			// Some kinds may have additional required fields; skip parse errors that aren't about satisfies.
			continue
		}
		if len(w.Nodes) > 0 && (len(w.Nodes[0].Satisfies) != 1 || w.Nodes[0].Satisfies[0] != "foo.BAR.1") {
			t.Errorf("kind %s: satisfies not preserved: %#v", kind, w.Nodes[0].Satisfies)
		}
	}
}
```

Run: `just test-pkg parser -run TestParser_NodeSatisfies`

Expected: FAIL — `satisfies` falls through to `emitUnknownFieldHint`.

- [ ] **Step 2: Extend `applyCommonPlainField`**

In `parser/parse_nodes.go`:

```go
func applyCommonPlainField(n *ir.Node, key, val string) bool {
	switch key {
	case "label":
		n.Label = val
	case "class":
		n.Classes = splitComma(val)
	case "reads":
		n.IO.Reads = splitComma(val)
	case "writes":
		n.IO.Writes = splitComma(val)
	case "satisfies":
		n.Satisfies = splitCommaNoEmpty(val)
	default:
		return false
	}
	return true
}
```

Verify `splitCommaNoEmpty` exists (it does — used by `parseWorkflowRequiresField`). It trims whitespace and drops empty entries, which is exactly the semantics we want.

- [ ] **Step 3: Re-run tests, confirm pass**

Run: `just test-pkg parser -run TestParser_NodeSatisfies`

Expected: both tests pass.

---

## Task 4: Validator — DIP139 malformed ACID

**Files:**
- Modify: `validator/lint_codes.go`
- Modify: `validator/codes.go`
- Create: `validator/lint_satisfies.go`
- Create: `validator/lint_satisfies_test.go`
- Modify: `validator/lint.go`

- [ ] **Step 1: Failing test for DIP139**

Create `validator/lint_satisfies_test.go`:

```go
package validator

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestLint_DIP139_MalformedACID(t *testing.T) {
	cases := []struct {
		name      string
		satisfies []string
		wantFire  bool
	}{
		{"valid simple", []string{"foo.BAR.1"}, false},
		{"valid sub", []string{"foo.BAR.1-1"}, false},
		{"valid range", []string{"foo.BAR.[1-3]"}, false},
		{"valid wildcard", []string{"foo.BAR.*"}, false},
		{"valid multi-component", []string{"foo-bar.BAR.QUUX.1"}, false},
		{"missing component", []string{"foo.1"}, true},
		{"lowercase component", []string{"foo.bar.1"}, true},
		{"missing requirement number", []string{"foo.BAR"}, true},
		{"empty string", []string{""}, true},
		{"leading dot", []string{".foo.BAR.1"}, true},
		{"trailing dot", []string{"foo.BAR.1."}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := cleanMinimalWorkflow()
			w.Spec = &ir.SpecRef{Loader: "acai", Path: "features.yaml"}
			w.Nodes[0].Satisfies = tc.satisfies
			res := Lint(w)
			fired := hasCode(res, DIP139)
			if fired != tc.wantFire {
				t.Errorf("DIP139 fired=%v, want %v for %#v", fired, tc.wantFire, tc.satisfies)
			}
		})
	}
}
```

Helpers `cleanMinimalWorkflow` and `hasCode` (and `assertNoCode`) already exist in `validator/lint_test.go` — verify naming, adjust if different.

Run: `just test-pkg validator -run TestLint_DIP139`

Expected: FAIL — DIP139 doesn't exist.

- [ ] **Step 2: Add DIP139 constant + description**

In `validator/lint_codes.go`:

```go
const (
	// ... existing DIP101-DIP138 ...
	DIP139 = "DIP139" // malformed ACID in satisfies list
	DIP140 = "DIP140" // satisfies declared but workflow has no spec
	DIP141 = "DIP141" // spec declared but no node references any requirement
	DIP142 = "DIP142" // duplicate ACID across satisfies lists
)

func init() {
	// ... existing entries ...
	CodeDescription[DIP139] = "malformed ACID reference in satisfies list"
	CodeDescription[DIP140] = "satisfies declared on a node but workflow has no spec"
	CodeDescription[DIP141] = "workflow declares spec but no node has satisfies"
	CodeDescription[DIP142] = "duplicate ACID across satisfies lists"
}
```

- [ ] **Step 3: Implement `lintSatisfies` for DIP139**

Create `validator/lint_satisfies.go`:

```go
package validator

import (
	"regexp"

	"github.com/2389-research/dippin-lang/ir"
)

// acidPattern matches a single ACID reference in any of three forms:
//   bare:     name(.COMPONENT)+\.requirement
//   range:    name(.COMPONENT)+\.[N-M]
//   wildcard: name(.COMPONENT)+\.*
// where:
//   name       = [a-z][a-z0-9_-]*
//   COMPONENT  = [A-Z][A-Z0-9_]* (one or more, dot-separated)
//   requirement = digits or "digits-digits" (sub-requirement, single level)
var acidPattern = regexp.MustCompile(
	`^[a-z][a-z0-9_-]*(?:\.[A-Z][A-Z0-9_]*)+\.(?:\d+(?:-\d+)?|\*|\[\d+-\d+\])$`,
)

func lintSatisfies(w *ir.Workflow) []Diagnostic {
	var out []Diagnostic
	out = append(out, lintMalformedACIDs(w)...)
	out = append(out, lintSatisfiesWithoutSpec(w)...)
	out = append(out, lintSpecWithoutSatisfies(w)...)
	out = append(out, lintDuplicateACIDs(w)...)
	return out
}

func lintMalformedACIDs(w *ir.Workflow) []Diagnostic {
	var out []Diagnostic
	for _, n := range w.Nodes {
		for _, ref := range n.Satisfies {
			if acidPattern.MatchString(ref) {
				continue
			}
			out = append(out, Diagnostic{
				Code:     DIP139,
				Severity: SeverityError,
				Message:  "malformed ACID reference: " + ref,
				NodeID:   n.ID,
			})
		}
	}
	return out
}
```

Stub the other three helpers to return nil for now — implemented in Tasks 5, 6, 7.

- [ ] **Step 4: Register `lintSatisfies` in the lint pipeline**

In `validator/lint.go`, append `lintSatisfies(w)...` to the diagnostics aggregation.

- [ ] **Step 5: Run tests, confirm DIP139 passes**

Run: `just test-pkg validator -run TestLint_DIP139`

Expected: all subtests pass.

---

## Task 5: Validator — DIP140 satisfies without spec

- [ ] **Step 1: Failing test**

```go
func TestLint_DIP140_SatisfiesWithoutSpec(t *testing.T) {
	w := cleanMinimalWorkflow()
	// Spec deliberately NOT set
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	res := Lint(w)
	if !hasCode(res, DIP140) {
		t.Errorf("expected DIP140 to fire when satisfies declared with no spec")
	}
}

func TestLint_DIP140_QuietWithSpec(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	res := Lint(w)
	assertNoCode(t, res, DIP140)
}
```

- [ ] **Step 2: Implement**

```go
func lintSatisfiesWithoutSpec(w *ir.Workflow) []Diagnostic {
	if w.Spec != nil {
		return nil
	}
	var out []Diagnostic
	for _, n := range w.Nodes {
		if len(n.Satisfies) == 0 {
			continue
		}
		out = append(out, Diagnostic{
			Code:     DIP140,
			Severity: SeverityWarning,
			Message:  "node declares satisfies but workflow has no spec",
			NodeID:   n.ID,
		})
	}
	return out
}
```

- [ ] **Step 3: Re-run tests, confirm pass**

---

## Task 6: Validator — DIP141 spec without satisfies

- [ ] **Step 1: Failing test**

```go
func TestLint_DIP141_SpecWithoutSatisfies(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	// No node has satisfies
	res := Lint(w)
	if !hasCode(res, DIP141) {
		t.Errorf("expected DIP141 to fire when spec declared but no satisfies")
	}
}

func TestLint_DIP141_QuietWhenSatisfiesPresent(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	res := Lint(w)
	assertNoCode(t, res, DIP141)
}
```

- [ ] **Step 2: Implement**

```go
func lintSpecWithoutSatisfies(w *ir.Workflow) []Diagnostic {
	if w.Spec == nil {
		return nil
	}
	for _, n := range w.Nodes {
		if len(n.Satisfies) > 0 {
			return nil
		}
	}
	return []Diagnostic{{
		Code:     DIP141,
		Severity: SeverityWarning,
		Message:  "workflow declares spec but no node has satisfies",
	}}
}
```

- [ ] **Step 3: Re-run tests, confirm pass**

---

## Task 7: Validator — DIP142 duplicate ACID

- [ ] **Step 1: Failing test**

```go
func TestLint_DIP142_DuplicateACID(t *testing.T) {
	w := cleanMinimalWorkflowWithTwoNodes() // helper to add — see Step 2
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1", "foo.BAR.2"}
	w.Nodes[1].Satisfies = []string{"foo.BAR.2"} // duplicate
	res := Lint(w)
	if !hasCode(res, DIP142) {
		t.Errorf("expected DIP142 to fire on duplicate ACID across nodes")
	}
}

func TestLint_DIP142_QuietOnRangeOverlap(t *testing.T) {
	// Ranges and wildcards are not expanded at lint time, so [1-3] and 2 don't
	// trigger DIP142 — only exact literal duplicates do.
	w := cleanMinimalWorkflowWithTwoNodes()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.[1-3]"}
	w.Nodes[1].Satisfies = []string{"foo.BAR.2"}
	res := Lint(w)
	assertNoCode(t, res, DIP142)
}
```

- [ ] **Step 2: Implement**

```go
func lintDuplicateACIDs(w *ir.Workflow) []Diagnostic {
	seen := make(map[string]string) // acid -> first nodeID
	var out []Diagnostic
	for _, n := range w.Nodes {
		for _, ref := range n.Satisfies {
			if prior, ok := seen[ref]; ok {
				out = append(out, Diagnostic{
					Code:     DIP142,
					Severity: SeverityWarning,
					Message:  "duplicate ACID " + ref + " also declared on node " + prior,
					NodeID:   n.ID,
				})
				continue
			}
			seen[ref] = n.ID
		}
	}
	return out
}
```

Add `cleanMinimalWorkflowWithTwoNodes` helper in `validator/lint_test.go` (or whichever test helpers file exists).

- [ ] **Step 3: Re-run tests, confirm pass**

---

## Task 8: Explanations

**Files:**
- Modify: `validator/explanations.go`
- Modify: `validator/explanations_test.go`

- [ ] **Step 1: Add Trigger / Fix for each new code**

Follow the existing pattern in `explanations.go` for each of DIP139–DIP142. Match the tone and structure of existing entries (e.g. DIP138 as the most recent reference).

- [ ] **Step 2: Run `just test-pkg validator -run TestExplanations`**

Expected: pass with new codes included in coverage.

---

## Task 9: Formatter — canonical placement

**Files:**
- Modify: `formatter/formatter.go`
- Modify: `formatter/formatter_test.go` (or equivalent)
- Modify: `parser/roundtrip_test.go`

- [ ] **Step 1: Failing round-trip test**

In `parser/roundtrip_test.go`, add a test that parses a workflow with `spec:` and a node with `satisfies:`, formats it, re-parses, and asserts equivalence.

- [ ] **Step 2: Determine canonical placement by reading existing formatter**

Open `formatter/formatter.go` and locate the workflow header emit + node body emit functions. `spec:` goes after `goal:` and before `start:`. `satisfies:` goes after `label:` and before any kind-specific fields.

- [ ] **Step 3: Implement formatter changes, ≤5 cyclomatic per function**

- [ ] **Step 4: Re-run round-trip test, confirm pass**

---

## Task 10: DOT export — satisfies sublabel

**Files:**
- Modify: `export/export.go`
- Modify: `export/export_test.go` (or equivalent)

- [ ] **Step 1: Failing test asserting satisfies is rendered in DOT label**

- [ ] **Step 2: Implement**

When a node has `Satisfies`, append `\n[satisfies: a, b, c]` to the DOT node's label. Match existing label-composition style (look at how `class:` is currently rendered if at all, or how prompts are conditionally included under `--prompts`).

- [ ] **Step 3: Re-run test, confirm pass**

---

## Task 11: Doctor — Spec probe

**Files:**
- Modify: `doctor/doctor.go`
- Modify: `doctor/doctor_test.go` (or equivalent)

- [ ] **Step 1: Failing test asserting the report includes a "Spec" section**

- [ ] **Step 2: Implement**

The probe reports:
- `Spec: <loader> <path>` if `w.Spec != nil`, else `Spec: (none)`.
- `Satisfies coverage: N of M nodes` where N = nodes with non-empty Satisfies, M = total nodes.

This is informational — it doesn't affect the grade.

- [ ] **Step 3: Re-run test, confirm pass**

---

## Task 12: Example + tree-sitter coverage

**Files:**
- Create: `examples/spec_loader.dip`
- Modify: `editors/tree-sitter-dippin/grammar.js`
- Create: `editors/tree-sitter-dippin/test/corpus/spec_loader.txt`

- [ ] **Step 1: Create `examples/spec_loader.dip`**

```dip
workflow SpecLoaderDemo
  goal: "Demonstrate spec: and satisfies: grammar"
  spec: acai features/example/features.yaml
  start: Implement
  exit: Done

  agent Implement
    label: Implement
    satisfies: example.AUTH.1, example.AUTH.2, example.AUTH.[3-5]
    prompt: |
      Implement the AUTH requirements from the spec.

  agent Done
    label: Done

  edges
    Implement -> Done
```

- [ ] **Step 2: Update tree-sitter grammar**

In `editors/tree-sitter-dippin/grammar.js`, add `"spec"` to the workflow header keyword set and `"satisfies"` to the common-node-field keyword set. Match the existing token style.

- [ ] **Step 3: Add corpus test**

Create `editors/tree-sitter-dippin/test/corpus/spec_loader.txt` mirroring the existing tests in `test/corpus/`.

- [ ] **Step 4: Run tree-sitter tests**

```sh
cd editors/tree-sitter-dippin && npx tree-sitter generate && npx tree-sitter test
```

Expected: all corpus tests pass including the new one.

- [ ] **Step 5: Verify the example validates and lints clean**

Run: `just validate-examples` and `just lint-examples`. Expected: zero errors, zero warnings on `examples/spec_loader.dip`.

`TestLintExamples` in `validator/lint_examples_test.go` should automatically pick up the new file and pass.

---

## Task 13: Grammar doc + CHANGELOG + README blurb

**Files:**
- Modify: `docs/GRAMMAR.ebnf`
- Modify: `CHANGELOG.md`
- Modify: `README.md`
- Modify: `editors/vscode/syntaxes/dippin.tmLanguage.json` (if keywords are enumerated)
- Modify: `editors/zed-dippin/languages/dippin/highlights.scm` (if keywords are enumerated)

- [ ] **Step 1: Extend `docs/GRAMMAR.ebnf`**

Add `"spec" ":" loader_and_path` to `workflow_field`. Add `"satisfies" ":" identifier_list` to `common_field` (or wherever common fields are defined in the grammar).

- [ ] **Step 2: Verify VSCode + Zed syntax highlights**

Open each file and check whether keyword lists are enumerated. If so, add `spec` and `satisfies`. If they use a generic identifier rule, no change needed.

- [ ] **Step 3: CHANGELOG entry**

```markdown
## [Unreleased]

### Added
- New `spec:` workflow header attribute and `satisfies:` common node attribute for spec-first development (see `docs/superpowers/specs/2026-05-21-spec-loader-grammar-design.md`). Both are optional and backwards-compatible. Runtime semantics (loading, verification, status reporting) live in the consumer (tracker); dippin only carries the IR.
- DIP139 (malformed ACID), DIP140 (satisfies without spec), DIP141 (spec without satisfies), DIP142 (duplicate ACID).
```

- [ ] **Step 4: Short README blurb**

Under the Language Reference section, add a sentence pointing to the design doc.

---

## Task 14: Full `just check` + commit

- [ ] **Step 1: Run `just check`**

Expected: every step passes — build, vet, fmt, lint-go, test-race, releasecheck, complexity, validate-examples, pack-examples, tree-sitter-test.

If complexity fires, extract helpers (don't `//nolint`).

- [ ] **Step 2: Commit**

```sh
git add -A
git commit -m "feat(grammar): add spec: workflow header and satisfies: node attribute

Adds optional spec: workflow header (loader + path) and satisfies:
common node attribute (list of ACID/range/wildcard refs) for spec-first
development workflows. Backwards-compatible — both fields are optional
and existing .dip files parse and behave identically.

Includes four new lint codes (DIP139–DIP142), tree-sitter coverage,
formatter round-trip, DOT export rendering, and a doctor probe.

Runtime semantics (loading specs, verifying ACID coverage, reporting
status to a spec server) live in the consumer (tracker); dippin only
carries the IR. See docs/superpowers/specs/2026-05-21-spec-loader-grammar-design.md
for the full design.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 3: Push branch + open PR against michellepellon/dippin-lang:main**

```sh
git push -u origin spec-loader-grammar
gh pr create --repo michellepellon/dippin-lang \
  --base main \
  --title "feat(grammar): add spec: and satisfies: for spec-first workflows" \
  --body "..."
```

---

## Sequencing notes

Tasks 1–3 are strictly sequential (IR → parser-header → parser-node-field).

Tasks 4–7 are sequential within the validator (need DIP codes registered first) but the DIP140/141/142 tests can be written in parallel after Task 4 completes if running with subagents.

Tasks 8–12 are independent of each other once Tasks 1–7 are done — good candidates for parallel subagents.

Task 13 is a tidy-up pass; do last to avoid CHANGELOG churn.

Task 14 is the gate.
