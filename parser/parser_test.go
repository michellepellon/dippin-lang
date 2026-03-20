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
