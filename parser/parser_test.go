package parser

import (
	"strings"
	"testing"

	"github.com/2389/dippin/ir"
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
