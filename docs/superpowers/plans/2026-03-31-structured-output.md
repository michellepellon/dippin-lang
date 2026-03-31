# Structured Output Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `response_format`, `response_schema`, and generic `Params` map to `AgentConfig` so `.dip` files can express structured JSON output requirements and pass-through runtime params.

**Architecture:** Three new fields on `AgentConfig` in the IR, parser dispatch for each, formatter emission in canonical order, four new lint rules (DIP130-DIP133) in a dedicated `lint_response.go` file, and explanations. The generic `Params` reuses the existing `parseParamsBlock()` and `writeSubgraphParams()` infrastructure.

**Tech Stack:** Go, existing dippin-lang packages (ir, parser, formatter, validator)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `ir/ir.go` | Modify | Add 3 fields to `AgentConfig` |
| `parser/parse_nodes.go` | Modify | Parse `response_format`, `response_schema`, `params` for agents |
| `parser/testdata/response_format.dip` | Create | Test fixture for structured output fields |
| `parser/parser_test.go` | Modify | Add test for parsing structured output fields |
| `formatter/format.go` | Modify | Emit new fields in canonical order |
| `formatter/format_test.go` | Modify | Add formatter tests |
| `validator/lint_codes.go` | Modify | Add DIP130-DIP133 constants and descriptions |
| `validator/lint_response.go` | Create | Lint rules for response_format/response_schema/params |
| `validator/lint.go` | Modify | Register new lint functions |
| `validator/explanations.go` | Modify | Add DIP130-DIP133 explanations |
| `validator/lint_test.go` | Modify | Add lint rule tests |

---

### Task 1: Add IR fields to AgentConfig

**Files:**
- Modify: `ir/ir.go:81-96`

- [ ] **Step 1: Add three new fields to AgentConfig**

In `ir/ir.go`, add the three new fields after `GoalGate`:

```go
// AgentConfig holds configuration for LLM agent nodes.
type AgentConfig struct {
	Prompt              string
	SystemPrompt        string
	Model               string // Per-node override
	Provider            string
	MaxTurns            int
	CmdTimeout          time.Duration
	CacheTools          bool
	Compaction          string
	CompactionThreshold float64
	ReasoningEffort     string
	Fidelity            string
	AutoStatus          bool              // Parse STATUS: from response
	GoalGate            bool              // Pipeline fails if this node fails
	ResponseFormat      string            // "json_object" or "json_schema"
	ResponseSchema      string            // JSON schema (when ResponseFormat is "json_schema")
	Params              map[string]string  // Generic key-value pairs passed through to runtime
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: builds cleanly (no code references these fields yet)

- [ ] **Step 3: Commit**

```bash
git add ir/ir.go
git commit -m "feat(ir): add ResponseFormat, ResponseSchema, Params to AgentConfig"
```

---

### Task 2: Parser support for new fields

**Files:**
- Modify: `parser/parse_nodes.go:165-228`
- Create: `parser/testdata/response_format.dip`
- Modify: `parser/parser_test.go`

- [ ] **Step 1: Create the test fixture**

Create `parser/testdata/response_format.dip`:

```dip
workflow ResponseFormatTest
  goal: "Test structured output parsing"
  start: Start
  exit: Exit

  agent Start
    label: Start

  agent Exit
    label: Exit

  agent JsonAgent
    label: "JSON Output"
    model: claude-sonnet-4-6
    provider: anthropic
    response_format: json_object
    prompt:
      Output a JSON object with results.

  agent SchemaAgent
    label: "Schema Output"
    model: gpt-5.4
    provider: openai
    response_format: json_schema
    response_schema:
      {
        "type": "object",
        "properties": {
          "questions": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "text": {"type": "string"}
              },
              "required": ["text"]
            }
          }
        },
        "required": ["questions"]
      }
    prompt:
      Generate interview questions.

  agent ParamsAgent
    label: "Agent with Params"
    model: claude-sonnet-4-6
    provider: anthropic
    params:
      backend: claude-code
      permission_mode: auto
    prompt:
      Do something.

  edges
    Start -> JsonAgent
    JsonAgent -> SchemaAgent
    SchemaAgent -> ParamsAgent
    ParamsAgent -> Exit
```

- [ ] **Step 2: Write the parser test**

Add to `parser/parser_test.go`:

```go
func TestParseResponseFormat(t *testing.T) {
	w := parseFile(t, "testdata/response_format.dip")

	// json_object mode
	jsonNode := findNode(t, w, "JsonAgent")
	cfg := jsonNode.Config.(ir.AgentConfig)
	if cfg.ResponseFormat != "json_object" {
		t.Errorf("JsonAgent ResponseFormat = %q, want %q", cfg.ResponseFormat, "json_object")
	}
	if cfg.ResponseSchema != "" {
		t.Errorf("JsonAgent ResponseSchema should be empty, got %q", cfg.ResponseSchema)
	}

	// json_schema mode with schema
	schemaNode := findNode(t, w, "SchemaAgent")
	scfg := schemaNode.Config.(ir.AgentConfig)
	if scfg.ResponseFormat != "json_schema" {
		t.Errorf("SchemaAgent ResponseFormat = %q, want %q", scfg.ResponseFormat, "json_schema")
	}
	if scfg.ResponseSchema == "" {
		t.Fatal("SchemaAgent ResponseSchema should not be empty")
	}
	if !json.Valid([]byte(scfg.ResponseSchema)) {
		t.Errorf("SchemaAgent ResponseSchema is not valid JSON: %s", scfg.ResponseSchema)
	}

	// params
	paramsNode := findNode(t, w, "ParamsAgent")
	pcfg := paramsNode.Config.(ir.AgentConfig)
	if pcfg.Params == nil {
		t.Fatal("ParamsAgent Params should not be nil")
	}
	if pcfg.Params["backend"] != "claude-code" {
		t.Errorf("ParamsAgent Params[backend] = %q, want %q", pcfg.Params["backend"], "claude-code")
	}
	if pcfg.Params["permission_mode"] != "auto" {
		t.Errorf("ParamsAgent Params[permission_mode] = %q, want %q", pcfg.Params["permission_mode"], "auto")
	}
}
```

Note: You'll need `"encoding/json"` in the imports. Also check if `parseFile` and `findNode` helper functions already exist in the test file — if not, `parseFile` parses a `.dip` file and returns `*ir.Workflow`, and `findNode` finds a node by ID. Look at existing tests for the exact helper names.

- [ ] **Step 3: Run the test to verify it fails**

Run: `just test-pkg parser`
Expected: FAIL — `response_format`, `response_schema`, and `params` are not parsed yet

- [ ] **Step 4: Add parser dispatch for the three fields**

In `parser/parse_nodes.go`, make these changes:

**Add `response_format` to `applyAgentModelField`** (line ~181):

```go
func applyAgentModelField(cfg *ir.AgentConfig, key, val string) bool {
	switch key {
	case "model":
		cfg.Model = val
	case "provider":
		cfg.Provider = val
	case "fidelity":
		cfg.Fidelity = val
	case "response_format":
		cfg.ResponseFormat = val
	default:
		return false
	}
	return true
}
```

**Add `response_schema` to `applyAgentPromptField`** (line ~166):

```go
func applyAgentPromptField(cfg *ir.AgentConfig, key, val string) bool {
	switch key {
	case "prompt":
		cfg.Prompt = val
	case "system_prompt":
		cfg.SystemPrompt = val
	case "reasoning_effort":
		cfg.ReasoningEffort = val
	case "response_schema":
		cfg.ResponseSchema = val
	default:
		return false
	}
	return true
}
```

**Add `params` to `applyAgentComplexField`** (line ~196):

```go
func (p *Parser) applyAgentComplexField(cfg *ir.AgentConfig, key, val string, loc ir.SourceLocation) {
	if applyAgentBoolField(cfg, key, val) {
		return
	}
	if key == "params" {
		cfg.Params = parseParamsBlock(val)
		return
	}
	p.applyAgentParsedField(cfg, key, val, loc)
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `just test-pkg parser`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add parser/parse_nodes.go parser/testdata/response_format.dip parser/parser_test.go
git commit -m "feat(parser): parse response_format, response_schema, params on agent nodes"
```

---

### Task 3: Formatter support for new fields

**Files:**
- Modify: `formatter/format.go:252-263`
- Modify: `formatter/format_test.go`

- [ ] **Step 1: Write the formatter tests**

Add to `formatter/format_test.go`:

```go
func TestFormatResponseFormat(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "S",
		Exit:  "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Label: "Agent", Config: ir.AgentConfig{
				Model:          "claude-sonnet-4-6",
				Provider:       "anthropic",
				ResponseFormat: "json_object",
				Prompt:         "Output JSON.",
			}},
		},
	}
	output := Format(w)
	assertContains(t, output, "    response_format: json_object")
	// response_format should appear after provider, before goal_gate
	modelIdx := strings.Index(output, "provider: anthropic")
	rfIdx := strings.Index(output, "response_format: json_object")
	promptIdx := strings.Index(output, "prompt:")
	if rfIdx < modelIdx {
		t.Error("response_format should appear after provider")
	}
	if rfIdx > promptIdx {
		t.Error("response_format should appear before prompt")
	}
}

func TestFormatResponseSchema(t *testing.T) {
	schema := "{\n  \"type\": \"object\",\n  \"required\": [\"name\"]\n}"
	w := &ir.Workflow{
		Name:  "test",
		Start: "S",
		Exit:  "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Label: "Agent", Config: ir.AgentConfig{
				ResponseFormat: "json_schema",
				ResponseSchema: schema,
				Prompt:         "Generate.",
			}},
		},
	}
	output := Format(w)
	assertContains(t, output, "    response_format: json_schema")
	assertContains(t, output, "    response_schema:")
	assertContains(t, output, "\"type\": \"object\"")
}

func TestFormatAgentParams(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "S",
		Exit:  "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Label: "Agent", Config: ir.AgentConfig{
				Params: map[string]string{"backend": "claude-code", "permission_mode": "auto"},
				Prompt: "Do something.",
			}},
		},
	}
	output := Format(w)
	assertContains(t, output, "    params:")
	assertContains(t, output, "      backend: claude-code")
	assertContains(t, output, "      permission_mode: auto")
	// params should appear before prompt
	paramsIdx := strings.Index(output, "params:")
	promptIdx := strings.Index(output, "prompt:")
	if paramsIdx > promptIdx {
		t.Error("params should appear before prompt")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `just test-pkg formatter`
Expected: FAIL — new fields not emitted yet

- [ ] **Step 3: Add formatter emission for new fields**

In `formatter/format.go`, add a new helper function after `writeAgentModelFields`:

```go
// writeAgentResponseFields writes response format fields for agent nodes.
func writeAgentResponseFields(wr *writer, cfg ir.AgentConfig) {
	if cfg.ResponseFormat != "" {
		wr.line("response_format: %s", quoteValue(cfg.ResponseFormat))
	}
	if cfg.ResponseSchema != "" {
		wr.multilineBlock("response_schema", cfg.ResponseSchema)
	}
}
```

Update `writeAgentFields` to call the new helper and emit params:

```go
func writeAgentFields(wr *writer, n *ir.Node, cfg ir.AgentConfig) {
	writeCommonNodeFields(wr, n)
	writeAgentModelFields(wr, cfg)
	writeAgentResponseFields(wr, cfg)
	writeAgentBehaviorFields(wr, cfg)
	writeAgentCompactionFields(wr, cfg)
	writeRetryFields(wr, n)
	writeIOFields(wr, n)
	writeSubgraphParams(wr, cfg.Params)

	if cfg.Prompt != "" {
		wr.multilineBlock("prompt", cfg.Prompt)
	}
}
```

Note: `writeSubgraphParams` already exists and handles `map[string]string` — reuse it directly. It handles nil/empty maps by returning immediately.

- [ ] **Step 4: Run tests to verify they pass**

Run: `just test-pkg formatter`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add formatter/format.go formatter/format_test.go
git commit -m "feat(formatter): emit response_format, response_schema, params for agent nodes"
```

---

### Task 4: Round-trip test (parse → format → parse)

**Files:**
- Modify: `parser/parser_test.go` (or `parser/roundtrip_test.go` if it exists)

- [ ] **Step 1: Check if a roundtrip test file exists**

Run: `ls parser/roundtrip_test.go 2>/dev/null || echo "not found"`

If it exists, add to it. If not, add to `parser/parser_test.go`.

- [ ] **Step 2: Write the round-trip test**

```go
func TestRoundtripResponseFormat(t *testing.T) {
	// Parse the fixture
	w := parseFile(t, "testdata/response_format.dip")

	// Format it
	formatted := formatter.Format(w)

	// Parse the formatted output
	p2 := parser.New(formatted)
	w2, err := p2.Parse()
	if err != nil {
		t.Fatalf("failed to parse formatted output: %v", err)
	}

	// Verify response_format survives
	jsonNode := findNode(t, w2, "JsonAgent")
	cfg := jsonNode.Config.(ir.AgentConfig)
	if cfg.ResponseFormat != "json_object" {
		t.Errorf("round-trip: JsonAgent ResponseFormat = %q, want %q", cfg.ResponseFormat, "json_object")
	}

	// Verify response_schema JSON indentation survives
	schemaNode := findNode(t, w2, "SchemaAgent")
	scfg := schemaNode.Config.(ir.AgentConfig)
	if scfg.ResponseFormat != "json_schema" {
		t.Errorf("round-trip: SchemaAgent ResponseFormat = %q, want %q", scfg.ResponseFormat, "json_schema")
	}
	if !json.Valid([]byte(scfg.ResponseSchema)) {
		t.Errorf("round-trip: SchemaAgent ResponseSchema is not valid JSON")
	}

	// Verify params survive
	paramsNode := findNode(t, w2, "ParamsAgent")
	pcfg := paramsNode.Config.(ir.AgentConfig)
	if pcfg.Params["backend"] != "claude-code" {
		t.Errorf("round-trip: ParamsAgent Params[backend] = %q, want %q", pcfg.Params["backend"], "claude-code")
	}

	// Verify idempotency: format again and compare
	formatted2 := formatter.Format(w2)
	if formatted != formatted2 {
		t.Errorf("format is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", formatted, formatted2)
	}
}
```

Note: You'll need imports for `"encoding/json"`, `"github.com/2389-research/dippin-lang/formatter"`, and `"github.com/2389-research/dippin-lang/parser"`. Check existing roundtrip tests for the exact import pattern — the parser constructor may be `parser.NewFromString()` or similar.

- [ ] **Step 3: Run to verify it passes**

Run: `just test-pkg parser`
Expected: PASS (if Tasks 2 and 3 are complete)

- [ ] **Step 4: Commit**

```bash
git add parser/parser_test.go
git commit -m "test: round-trip test for response_format, response_schema, params"
```

---

### Task 5: Lint rules DIP130-DIP133

**Files:**
- Modify: `validator/lint_codes.go`
- Create: `validator/lint_response.go`
- Modify: `validator/lint.go`
- Modify: `validator/lint_test.go`

- [ ] **Step 1: Add DIP codes and descriptions**

In `validator/lint_codes.go`, add the four new constants after DIP129:

```go
const (
	// ... existing codes DIP101-DIP129 ...
	DIP129 = "DIP129" // interview mode with conflicting choice-style edges
	DIP130 = "DIP130" // invalid response_format value or on non-agent node
	DIP131 = "DIP131" // response_schema/response_format mismatch
	DIP132 = "DIP132" // response_schema is not valid JSON
	DIP133 = "DIP133" // agent params key shadows a first-class field
)
```

In the `init()` function, add descriptions:

```go
	CodeDescription[DIP130] = "invalid response_format value or on non-agent node"
	CodeDescription[DIP131] = "response_schema and response_format mismatch"
	CodeDescription[DIP132] = "response_schema is not valid JSON"
	CodeDescription[DIP133] = "agent params key shadows a first-class field"
```

- [ ] **Step 2: Write the lint tests**

Add to `validator/lint_test.go`:

```go
func TestLintResponseFormatInvalid(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{ResponseFormat: "xml"}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP130)
	if found == nil {
		t.Error("expected DIP130 for invalid response_format")
	}
}

func TestLintResponseFormatValid(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{ResponseFormat: "json_object"}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP130)
	if found != nil {
		t.Error("unexpected DIP130 for valid response_format")
	}
}

func TestLintResponseFormatOnToolNode(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo hi"}},
		},
		Edges: []ir.Edge{{From: "S", To: "T"}, {From: "T", To: "E"}},
	}
	// Tool nodes don't have ResponseFormat in their config, so this tests
	// that the lint function only checks AgentConfig nodes (no crash on non-agent)
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP130)
	if found != nil {
		t.Error("unexpected DIP130 for tool node")
	}
}

func TestLintResponseSchemaWithoutJsonSchema(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				ResponseFormat: "json_object",
				ResponseSchema: `{"type": "object"}`,
			}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP131)
	if found == nil {
		t.Error("expected DIP131 warning: response_schema without json_schema")
	}
	if found != nil && found.Severity != SeverityWarning {
		t.Errorf("DIP131 for schema-without-json_schema should be warning, got %s", found.Severity)
	}
}

func TestLintJsonSchemaWithoutResponseSchema(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				ResponseFormat: "json_schema",
			}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP131)
	if found == nil {
		t.Error("expected DIP131 hint: json_schema without response_schema")
	}
	if found != nil && found.Severity != SeverityHint {
		t.Errorf("DIP131 for json_schema-without-schema should be hint, got %s", found.Severity)
	}
}

func TestLintResponseSchemaInvalidJSON(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				ResponseFormat: "json_schema",
				ResponseSchema: `{"type": "object"`,
			}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP132)
	if found == nil {
		t.Error("expected DIP132 for invalid JSON in response_schema")
	}
}

func TestLintResponseSchemaValidJSON(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				ResponseFormat: "json_schema",
				ResponseSchema: `{"type": "object", "required": ["name"]}`,
			}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP132)
	if found != nil {
		t.Error("unexpected DIP132 for valid JSON")
	}
}

func TestLintParamsShadowsField(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				Params: map[string]string{"model": "gpt-5.4", "backend": "claude-code"},
			}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP133)
	if found == nil {
		t.Error("expected DIP133 hint for params key 'model' shadowing first-class field")
	}
	if found != nil && found.Severity != SeverityHint {
		t.Errorf("DIP133 should be hint, got %s", found.Severity)
	}
}

func TestLintParamsNoShadow(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "S", Exit: "E",
		Nodes: []*ir.Node{
			{ID: "S", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "E", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				Params: map[string]string{"backend": "claude-code", "permission_mode": "auto"},
			}},
		},
		Edges: []ir.Edge{{From: "S", To: "A"}, {From: "A", To: "E"}},
	}
	diags := Lint(w).Diagnostics
	found := findDiag(diags, DIP133)
	if found != nil {
		t.Error("unexpected DIP133 for non-shadowing params")
	}
}
```

Note: Check if `findDiag` helper exists in the test file. It should find a diagnostic by code. If not, it looks like this:

```go
func findDiag(diags []Diagnostic, code string) *Diagnostic {
	for i := range diags {
		if diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `just test-pkg validator`
Expected: FAIL — DIP130-DIP133 not defined yet, lint functions don't exist

- [ ] **Step 4: Create `validator/lint_response.go`**

```go
package validator

import (
	"encoding/json"
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

var validResponseFormats = map[string]bool{
	"json_object": true,
	"json_schema": true,
}

// agentFirstClassFields lists AgentConfig field names that should use typed
// fields rather than the generic Params map. Used by DIP133.
var agentFirstClassFields = map[string]bool{
	"model":                  true,
	"provider":               true,
	"prompt":                 true,
	"system_prompt":          true,
	"max_turns":              true,
	"response_format":        true,
	"response_schema":        true,
	"reasoning_effort":       true,
	"fidelity":               true,
	"auto_status":            true,
	"goal_gate":              true,
	"cache_tools":            true,
	"compaction":             true,
	"compaction_threshold":   true,
}

// lintResponseFormat checks DIP130: response_format must be json_object or json_schema.
// Also catches response_format on non-agent nodes.
func lintResponseFormat(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if cfg.ResponseFormat == "" {
			continue
		}
		if !validResponseFormats[cfg.ResponseFormat] {
			diags = append(diags, Diagnostic{
				Code:     DIP130,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has response_format %q which is not recognized", n.ID, cfg.ResponseFormat),
				Location: n.Source,
				Help:     "valid values: json_object, json_schema",
			})
		}
	}
	return diags
}

// lintResponseSchemaMismatch checks DIP131:
// - response_schema without response_format: json_schema → warning (schema ignored)
// - response_format: json_schema without response_schema → hint (valid but advisory)
func lintResponseSchemaMismatch(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if cfg.ResponseSchema != "" && cfg.ResponseFormat != "json_schema" {
			diags = append(diags, Diagnostic{
				Code:     DIP131,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has response_schema but response_format is %q (schema will be ignored)", n.ID, cfg.ResponseFormat),
				Location: n.Source,
				Help:     "set response_format: json_schema to use the schema, or remove response_schema",
			})
		}
		if cfg.ResponseFormat == "json_schema" && cfg.ResponseSchema == "" {
			diags = append(diags, Diagnostic{
				Code:     DIP131,
				Severity: SeverityHint,
				Message:  fmt.Sprintf("node %q has response_format json_schema but no response_schema provided", n.ID),
				Location: n.Source,
				Help:     "add a response_schema block with a JSON schema, or use json_object if no schema is needed",
			})
		}
	}
	return diags
}

// lintResponseSchemaJSON checks DIP132: response_schema must be valid JSON.
func lintResponseSchemaJSON(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok || cfg.ResponseSchema == "" {
			continue
		}
		if !json.Valid([]byte(cfg.ResponseSchema)) {
			diags = append(diags, Diagnostic{
				Code:     DIP132,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has response_schema that is not valid JSON", n.ID),
				Location: n.Source,
				Help:     "fix the JSON syntax in the response_schema block",
			})
		}
	}
	return diags
}

// lintAgentParamsShadow checks DIP133: agent params key shadows a first-class field.
func lintAgentParamsShadow(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok || len(cfg.Params) == 0 {
			continue
		}
		for k := range cfg.Params {
			if agentFirstClassFields[k] {
				diags = append(diags, Diagnostic{
					Code:     DIP133,
					Severity: SeverityHint,
					Message:  fmt.Sprintf("node %q params key %q shadows a first-class field — use the typed field instead", n.ID, k),
					Location: n.Source,
					Help:     fmt.Sprintf("move %q from params to the dedicated field for validation and tooling support", k),
				})
			}
		}
	}
	return diags
}
```

- [ ] **Step 5: Register lint functions in `validator/lint.go`**

Add the four new calls after `lintInterviewLabeledEdges(w)...`:

```go
	diags = append(diags, lintInterviewLabeledEdges(w)...)
	diags = append(diags, lintResponseFormat(w)...)
	diags = append(diags, lintResponseSchemaMismatch(w)...)
	diags = append(diags, lintResponseSchemaJSON(w)...)
	diags = append(diags, lintAgentParamsShadow(w)...)
```

Also update the comment on line 8 from `DIP101–DIP129` to `DIP101–DIP133`.

- [ ] **Step 6: Run tests to verify they pass**

Run: `just test-pkg validator`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add validator/lint_codes.go validator/lint_response.go validator/lint.go validator/lint_test.go
git commit -m "feat(validator): DIP130-DIP133 lint rules for response_format, response_schema, params"
```

---

### Task 6: Add explanations for DIP130-DIP133

**Files:**
- Modify: `validator/explanations.go`

- [ ] **Step 1: Add explanations to `configExplanations()`**

In `validator/explanations.go`, add four new entries to the `configExplanations()` function (after DIP119):

```go
		DIP130: {
			Code:    DIP130,
			Summary: "invalid response_format value or on non-agent node",
			Trigger: "An agent node has response_format set to a value other than json_object or json_schema.",
			Fix:     "Use a valid response_format: json_object or json_schema.",
			Example: "agent A\n  response_format: xml  // invalid value",
		},
		DIP131: {
			Code:    DIP131,
			Summary: "response_schema and response_format mismatch",
			Trigger: "Either response_schema is set without response_format: json_schema (schema will be ignored), or response_format: json_schema is set without a response_schema (advisory).",
			Fix:     "Set response_format: json_schema when providing a schema, or remove the unused schema.",
			Example: "agent A\n  response_format: json_object\n  response_schema:\n    {\"type\": \"object\"}  // schema ignored without json_schema",
		},
		DIP132: {
			Code:    DIP132,
			Summary: "response_schema is not valid JSON",
			Trigger: "The response_schema block contains content that does not parse as valid JSON.",
			Fix:     "Fix the JSON syntax. Common issues: trailing commas, unquoted keys, missing brackets.",
			Example: "agent A\n  response_format: json_schema\n  response_schema:\n    {\"type\": \"object\"  // missing closing brace",
		},
		DIP133: {
			Code:    DIP133,
			Summary: "agent params key shadows a first-class field",
			Trigger: "An agent node has a params key that matches a first-class AgentConfig field name (model, provider, response_format, etc.).",
			Fix:     "Use the dedicated typed field instead of params for better validation and tooling support.",
			Example: "agent A\n  params:\n    model: gpt-5.4  // should be: model: gpt-5.4 (as a typed field)",
		},
```

- [ ] **Step 2: Run the full test suite**

Run: `just test`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add validator/explanations.go
git commit -m "docs(validator): add explanations for DIP130-DIP133"
```

---

### Task 7: Integration test with example .dip file

**Files:**
- Modify: `examples/api_design.dip` (add `response_format: json_object` to an appropriate agent)

- [ ] **Step 1: Identify the right node in api_design.dip**

Read `examples/api_design.dip` and find an agent node that produces JSON output. The Interview subgraph generates questions — an agent that feeds into it would benefit from `response_format: json_object`.

- [ ] **Step 2: Add response_format to the node**

Add `response_format: json_object` to an appropriate agent node. For example, if there's an agent that generates structured output, add the field after the model/provider fields and before behavior fields.

- [ ] **Step 3: Run validate-examples**

Run: `just validate-examples`
Expected: "All examples valid."

- [ ] **Step 4: Run lint-examples**

Run: `just lint-examples`
Expected: No DIP130-DIP132 warnings on the modified file.

- [ ] **Step 5: Verify format idempotency**

```bash
./dippin fmt examples/api_design.dip > /tmp/fmt1.dip 2>/dev/null
./dippin fmt /tmp/fmt1.dip > /tmp/fmt2.dip 2>/dev/null
diff /tmp/fmt1.dip /tmp/fmt2.dip && echo "IDEMPOTENT"
```

Expected: IDEMPOTENT

- [ ] **Step 6: Commit**

```bash
git add examples/api_design.dip
git commit -m "feat: api_design.dip uses response_format: json_object for structured output"
```

---

### Task 8: Complexity check and final verification

- [ ] **Step 1: Run the full check suite**

```bash
just check
```

Expected: build, vet, fmt, test-race, complexity, validate-examples all pass.

- [ ] **Step 2: Verify complexity thresholds**

The new `lint_response.go` functions are simple (single loop, no nesting). If any function exceeds cyclomatic 5 or cognitive 7, extract helpers.

Run: `just complexity`
Expected: PASS

- [ ] **Step 3: Verify all 4 new DIP codes are discoverable**

```bash
./dippin explain DIP130
./dippin explain DIP131
./dippin explain DIP132
./dippin explain DIP133
```

Expected: each prints its explanation with summary, trigger, fix, and example.

- [ ] **Step 4: Final commit if any cleanup was needed**

```bash
git add -A
git commit -m "chore: final cleanup for structured output support"
```

(Only if there were changes from complexity fixes or cleanup.)
