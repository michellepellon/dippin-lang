package parser

import (
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestParseSimpleWorkflow(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: B

  agent A
    prompt: "Do A."

  agent B
    prompt: "Do B."

  edges
    A -> B
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if w.Name != "Test" {
		t.Errorf("name = %q, want %q", w.Name, "Test")
	}
	if len(w.Nodes) != 2 {
		t.Errorf("nodes = %d, want 2", len(w.Nodes))
	}
	if len(w.Edges) != 1 {
		t.Errorf("edges = %d, want 1", len(w.Edges))
	}
	if w.Start != "A" || w.Exit != "B" {
		t.Errorf("start/exit = %s/%s, want A/B", w.Start, w.Exit)
	}
}

func TestParseComplexWorkflow(t *testing.T) {
	input := `workflow Complex
  goal: "Complex"
  start: P
  exit: J

  parallel P -> A, B

  agent A
    prompt:
      Do A.
      More text.

  agent B
    prompt: Do B.

  fan_in J <- A, B

  edges
    P -> A
    P -> B
    A -> J
    B -> J
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(w.Nodes) != 4 {
		t.Errorf("nodes = %d, want 4", len(w.Nodes))
	}

	// Check parallel node
	var parallelNode *ir.Node
	for _, n := range w.Nodes {
		if n.Kind == ir.NodeParallel {
			parallelNode = n
			break
		}
	}
	if parallelNode == nil {
		t.Fatal("parallel node not found")
	}
	if len(parallelNode.Config.(ir.ParallelConfig).Targets) != 2 {
		t.Errorf("parallel targets = %d, want 2", len(parallelNode.Config.(ir.ParallelConfig).Targets))
	}

	// Check multiline prompt
	var agentANode *ir.Node
	for _, n := range w.Nodes {
		if n.ID == "A" {
			agentANode = n
			break
		}
	}
	if agentANode == nil {
		t.Fatal("agent A not found")
	}
	prompt := agentANode.Config.(ir.AgentConfig).Prompt
	if !strings.Contains(prompt, "Do A.") || !strings.Contains(prompt, "More text.") {
		t.Errorf("prompt = %q, want it to contain 'Do A.' and 'More text.'", prompt)
	}
}

func TestParseShellScriptCommand(t *testing.T) {
	input := `workflow Build
  goal: "Build"
  start: Setup
  exit: Done

  tool Setup
    command:
      #!/bin/sh
      set -eu
      if [ -f go.mod ]; then
        go build ./... 2>&1 || exit 1
        go test ./... 2>&1
      fi
      if [ -f package.json ]; then
        npm test 2>&1 || { echo "failed"; exit 1; }
      fi
      printf 'done'

  agent Done
    prompt: "Finish."

  edges
    Setup -> Done
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(w.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(w.Nodes))
	}

	var toolNode *ir.Node
	for _, n := range w.Nodes {
		if n.Kind == ir.NodeTool {
			toolNode = n
			break
		}
	}
	if toolNode == nil {
		t.Fatal("tool node not found")
	}

	cmd := toolNode.Config.(ir.ToolConfig).Command
	// Must preserve shell syntax verbatim
	for _, want := range []string{
		"#!/bin/sh",
		"set -eu",
		"if [ -f go.mod ]; then",
		"  go build ./... 2>&1 || exit 1",
		"fi",
		`{ echo "failed"; exit 1; }`,
		"printf 'done'",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command missing %q\ngot:\n%s", want, cmd)
		}
	}
}

func TestParseColonInValue(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: A

  defaults
    fidelity: summary:medium
    max_retries: 3

  agent A
    fidelity: summary:high
    prompt: "Do it."

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if w.Defaults.Fidelity != "summary:medium" {
		t.Errorf("defaults fidelity = %q, want %q", w.Defaults.Fidelity, "summary:medium")
	}

	if len(w.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(w.Nodes))
	}
	cfg := w.Nodes[0].Config.(ir.AgentConfig)
	if cfg.Fidelity != "summary:high" {
		t.Errorf("agent fidelity = %q, want %q", cfg.Fidelity, "summary:high")
	}
}

func TestParseInvalidIntegerEmitsDiagnostic(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: A

  defaults
    max_retries: three

  agent A
    max_turns: unlimited
    prompt: "Do it."

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	_, err := p.Parse()
	if err == nil {
		t.Fatal("expected error for invalid integers, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "invalid integer") {
		t.Errorf("expected 'invalid integer' diagnostic, got: %s", errStr)
	}
	if !strings.Contains(errStr, `"three"`) {
		t.Errorf("expected diagnostic to mention %q, got: %s", "three", errStr)
	}
	if !strings.Contains(errStr, `"unlimited"`) {
		t.Errorf("expected diagnostic to mention %q, got: %s", "unlimited", errStr)
	}
}

func TestParseInvalidDurationEmitsDiagnostic(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: A

  tool A
    timeout: 60
    command:
      echo done

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	_, err := p.Parse()
	if err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "invalid duration") {
		t.Errorf("expected 'invalid duration' diagnostic, got: %s", errStr)
	}
	if !strings.Contains(errStr, `"60"`) {
		t.Errorf("expected diagnostic to mention %q, got: %s", "60", errStr)
	}
}

func TestParseTabIndentation(t *testing.T) {
	// Verify that tab-indented files parse without panicking.
	// Tabs count as 1 byte of indentation (not 8).
	input := "workflow Test\n\tgoal: \"Test\"\n\tstart: A\n\texit: A\n\n\tagent A\n\t\tprompt: \"Do it.\"\n\n\tedges\n\t\tA -> A\n"
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Name != "Test" {
		t.Errorf("name = %q, want %q", w.Name, "Test")
	}
	if len(w.Nodes) != 1 {
		t.Errorf("nodes = %d, want 1", len(w.Nodes))
	}
}

func TestParseMultilineBlockWithBlankLines(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: A

  agent A
    prompt:
      Paragraph one.

      Paragraph two after blank line.

      Paragraph three.

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(w.Nodes))
	}
	prompt := w.Nodes[0].Config.(ir.AgentConfig).Prompt
	if !strings.Contains(prompt, "Paragraph one.") {
		t.Errorf("prompt missing 'Paragraph one.', got: %q", prompt)
	}
	if !strings.Contains(prompt, "Paragraph two after blank line.") {
		t.Errorf("prompt missing 'Paragraph two after blank line.', got: %q", prompt)
	}
	if !strings.Contains(prompt, "Paragraph three.") {
		t.Errorf("prompt missing 'Paragraph three.', got: %q", prompt)
	}
	// Verify blank lines are preserved
	if strings.Count(prompt, "\n\n") < 2 {
		t.Errorf("expected at least 2 blank lines preserved in prompt, got: %q", prompt)
	}
}

func TestParseEdgeWithQuotedConditionValue(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: B

  agent A
    prompt: "Do it."

  agent B
    prompt: "Done."

  edges
    A -> B when ctx.outcome == "success"
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(w.Edges))
	}
	cond := w.Edges[0].Condition
	if cond == nil {
		t.Fatal("expected condition, got nil")
	}
	raw := cond.Raw
	if !strings.Contains(raw, "ctx.outcome") {
		t.Errorf("condition raw missing 'ctx.outcome', got: %q", raw)
	}
	if !strings.Contains(raw, `"success"`) {
		t.Errorf("condition raw missing quoted value, got: %q", raw)
	}
}

func TestParseComparisonOperators(t *testing.T) {
	input := `workflow Test
  goal: "Test"
  start: A
  exit: D

  agent A
    prompt: "A"
  agent B
    prompt: "B"
  agent C
    prompt: "C"
  agent D
    prompt: "D"

  edges
    A -> B when ctx.retries <= 3
    A -> C when ctx.score >= 80
    A -> D when ctx.count < 10
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w.Edges) != 3 {
		t.Fatalf("edges = %d, want 3", len(w.Edges))
	}
	tests := []struct {
		idx  int
		want string
	}{
		{0, "<="},
		{1, ">="},
		{2, "<"},
	}
	for _, tt := range tests {
		raw := w.Edges[tt.idx].Condition.Raw
		if !strings.Contains(raw, tt.want) {
			t.Errorf("edge %d condition missing %q, got: %q", tt.idx, tt.want, raw)
		}
	}
}

func TestParseStylesheet(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A

  agent A
    class: coder
    prompt: "Do it."

  stylesheet:
    *
      temperature: 0.7
    agent
      fidelity: full
    .coder
      model: o1
      reasoning_effort: medium
    #A
      max_retries: 5

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(w.Stylesheet) != 4 {
		t.Fatalf("stylesheet rules = %d, want 4", len(w.Stylesheet))
	}

	// Check universal selector
	found := false
	for _, r := range w.Stylesheet {
		if r.Selector.Kind == "universal" {
			found = true
			if r.Properties["temperature"] != "0.7" {
				t.Errorf("universal temperature = %q, want 0.7", r.Properties["temperature"])
			}
		}
	}
	if !found {
		t.Error("universal selector not found")
	}

	// Check class selector
	for _, r := range w.Stylesheet {
		if r.Selector.Kind == "class" && r.Selector.Value == "coder" {
			if r.Properties["model"] != "o1" {
				t.Errorf(".coder model = %q, want o1", r.Properties["model"])
			}
		}
	}

	// Check ID selector
	for _, r := range w.Stylesheet {
		if r.Selector.Kind == "id" && r.Selector.Value == "A" {
			if r.Properties["max_retries"] != "5" {
				t.Errorf("#A max_retries = %q, want 5", r.Properties["max_retries"])
			}
		}
	}
}

func TestParseParallelBlockForm(t *testing.T) {
	input := `workflow Test
  start: split
  exit: join

  agent worker_a
    prompt: "A"

  agent worker_b
    prompt: "B"

  parallel split
    branch: worker_a
      model: o1
    branch: worker_b
      model: claude-3

  fan_in join <- worker_a, worker_b

  edges
    join -> join
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pNode *ir.Node
	for _, n := range w.Nodes {
		if n.Kind == ir.NodeParallel {
			pNode = n
			break
		}
	}
	if pNode == nil {
		t.Fatal("parallel node not found")
	}

	cfg := pNode.Config.(ir.ParallelConfig)
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Targets))
	}
	if len(cfg.Branches) != 2 {
		t.Fatalf("branches = %d, want 2", len(cfg.Branches))
	}
	if cfg.Branches[0].Model != "o1" {
		t.Errorf("branch[0].model = %q, want o1", cfg.Branches[0].Model)
	}
	if cfg.Branches[1].Model != "claude-3" {
		t.Errorf("branch[1].model = %q, want claude-3", cfg.Branches[1].Model)
	}
}

func TestParseParallelInlineStillWorks(t *testing.T) {
	input := `workflow Test
  start: P
  exit: J

  agent A
    prompt: "A"
  agent B
    prompt: "B"

  parallel P -> A, B
  fan_in J <- A, B

  edges
    J -> J
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pNode *ir.Node
	for _, n := range w.Nodes {
		if n.Kind == ir.NodeParallel {
			pNode = n
			break
		}
	}
	if pNode == nil {
		t.Fatal("parallel node not found")
	}
	cfg := pNode.Config.(ir.ParallelConfig)
	if len(cfg.Targets) != 2 {
		t.Errorf("targets = %d, want 2", len(cfg.Targets))
	}
	if len(cfg.Branches) != 0 {
		t.Errorf("branches = %d, want 0 (inline form)", len(cfg.Branches))
	}
}

func TestParseOnResume(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A

  defaults
    fidelity: full
    on_resume: degrade

  agent A
    prompt: "Do it."

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if w.Defaults.Fidelity != "full" {
		t.Errorf("fidelity = %q, want full", w.Defaults.Fidelity)
	}
	if w.Defaults.OnResume != "degrade" {
		t.Errorf("on_resume = %q, want degrade", w.Defaults.OnResume)
	}
}

func TestParseCompactionFields(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A

  agent A
    compaction: sliding_window
    compaction_threshold: 0.8
    cache_tools: true
    prompt: "Do it."

  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := w.Nodes[0].Config.(ir.AgentConfig)
	if cfg.Compaction != "sliding_window" {
		t.Errorf("compaction = %q, want sliding_window", cfg.Compaction)
	}
	if cfg.CompactionThreshold != 0.8 {
		t.Errorf("compaction_threshold = %f, want 0.8", cfg.CompactionThreshold)
	}
	if !cfg.CacheTools {
		t.Error("cache_tools = false, want true")
	}
}

func TestParseSubgraphParams(t *testing.T) {
	input := `workflow Test
  start: Build
  exit: Done

  agent Build
    prompt: "Build."

  subgraph Review
    ref: ./review.dip
    params:
      language: python
      strict: true

  agent Done
    prompt: "Done."

  edges
    Build -> Review
    Review -> Done
`
	p := NewParser(input, "test.dip")
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sg *ir.Node
	for _, n := range w.Nodes {
		if n.Kind == ir.NodeSubgraph {
			sg = n
			break
		}
	}
	if sg == nil {
		t.Fatal("subgraph node not found")
	}

	cfg := sg.Config.(ir.SubgraphConfig)
	if cfg.Ref != "./review.dip" {
		t.Errorf("ref = %q, want ./review.dip", cfg.Ref)
	}
	if cfg.Params["language"] != "python" {
		t.Errorf("params[language] = %q, want python", cfg.Params["language"])
	}
	if cfg.Params["strict"] != "true" {
		t.Errorf("params[strict] = %q, want true", cfg.Params["strict"])
	}
}
