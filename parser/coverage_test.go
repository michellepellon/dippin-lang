package parser

import (
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// --- tryParseBranch edge cases ---

func TestTryParseBranch_NonBranchToken(t *testing.T) {
	// A parallel block with a non-branch identifier token should be skipped.
	input := `workflow Test
  start: split
  exit: join

  agent worker_a
    prompt: "A"

  parallel split
    something: ignored
    branch: worker_a

  fan_in join <- worker_a

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
	if len(cfg.Branches) != 1 {
		t.Errorf("branches = %d, want 1", len(cfg.Branches))
	}
}

func TestTryParseBranch_NewlineOnly(t *testing.T) {
	// A parallel block that has blank lines between branches.
	input := `workflow Test
  start: split
  exit: join

  agent worker_a
    prompt: "A"

  agent worker_b
    prompt: "B"

  parallel split
    branch: worker_a

    branch: worker_b

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
	if len(cfg.Branches) != 2 {
		t.Errorf("branches = %d, want 2", len(cfg.Branches))
	}
}

// --- parseTopLevel: unknown top-level tokens ---

func TestParseTopLevel_UnknownIdentifier(t *testing.T) {
	input := `bogus_thing
workflow Test
  start: A
  exit: A
  agent A
    prompt: "Do it."
  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, _ := p.Parse()
	// Should still parse the workflow despite the unknown token.
	if w.Name != "Test" {
		t.Errorf("name = %q, want Test", w.Name)
	}
}

// --- dispatchWorkflowDefault: unknown workflow-level identifier ---

func TestDispatchWorkflowDefault_Unknown(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A
  unknown_field: value
  agent A
    prompt: "Do it."
  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	_, err := p.Parse()
	if err == nil {
		t.Fatal("expected error for unknown workflow field")
	}
	if !strings.Contains(err.Error(), "unexpected top-level identifier") {
		t.Errorf("error = %q, want 'unexpected top-level identifier'", err.Error())
	}
}

// --- expect: wrong token type ---

func TestExpect_WrongToken(t *testing.T) {
	// The "edges" block expects "A -> B" but we'll feed it garbage.
	input := `workflow Test
  start: A
  exit: A
  agent A
    prompt: "Do it."
  edges
    A : B
`
	p := NewParser(input, "test.dip")
	_, err := p.Parse()
	if err == nil {
		t.Fatal("expected error for wrong token in edge")
	}
}

// --- readBlockAfterNewline: newline not followed by raw block ---

func TestReadFieldValue_NewlineThenNoBlock(t *testing.T) {
	// prompt followed by just a newline and then a different field.
	input := `workflow Test
  start: A
  exit: A
  agent A
    prompt:
    model: gpt-4
  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	w, _ := p.Parse()
	if len(w.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(w.Nodes))
	}
	// Prompt should be empty since there's no indented block.
	cfg := w.Nodes[0].Config.(ir.AgentConfig)
	if cfg.Prompt != "" {
		t.Errorf("prompt = %q, want empty", cfg.Prompt)
	}
}

// --- parseFloat error ---

func TestParseFloat_Invalid(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A
  agent A
    compaction_threshold: notanumber
    prompt: "Do it."
  edges
    A -> A
`
	p := NewParser(input, "test.dip")
	_, err := p.Parse()
	if err == nil {
		t.Fatal("expected error for invalid float")
	}
	if !strings.Contains(err.Error(), "invalid float") {
		t.Errorf("error = %q, want 'invalid float'", err.Error())
	}
}

// --- splitKeyValue edge case ---

func TestSplitKeyValue_NoColon(t *testing.T) {
	k, v := splitKeyValue("nocolon")
	if k != "" || v != "" {
		t.Errorf("expected empty, got %q/%q", k, v)
	}
}

// --- PeekToken at EOF ---

func TestLexer_PeekAtEOF(t *testing.T) {
	l := NewLexer("", "test.dip")
	tok := l.PeekToken()
	if tok.Type != TokenEOF {
		t.Errorf("expected EOF, got %v", tok.Type)
	}
	tok2 := l.NextToken()
	if tok2.Type != TokenEOF {
		t.Errorf("expected EOF, got %v", tok2.Type)
	}
}

// --- RawValueText edge cases ---

func TestRawValueText_OutOfRange(t *testing.T) {
	l := NewLexer("hello: world", "test.dip")
	val := l.RawValueText(0) // line 0 is out of range (1-indexed)
	if val != "" {
		t.Errorf("expected empty for line 0, got %q", val)
	}
	val2 := l.RawValueText(999)
	if val2 != "" {
		t.Errorf("expected empty for line 999, got %q", val2)
	}
}

func TestRawValueText_NoColon(t *testing.T) {
	l := NewLexer("no colon here", "test.dip")
	val := l.RawValueText(1)
	if val != "" {
		t.Errorf("expected empty for no colon, got %q", val)
	}
}

// --- parseEdgesBody: empty edges block ---

func TestParseEdges_EmptyBody(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A
  agent A
    prompt: "Do it."
  edges

`
	p := NewParser(input, "test.dip")
	w, _ := p.Parse()
	if len(w.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(w.Edges))
	}
}

// --- defaultNodeConfig: missing start/exit uses different defaults ---

func TestDefaultNodeConfig_NoDefaultModel(t *testing.T) {
	input := `workflow Test
  start: A
  exit: A
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
	cfg := w.Nodes[0].Config.(ir.AgentConfig)
	// No defaults set, so model should be empty.
	if cfg.Model != "" {
		t.Errorf("model = %q, want empty", cfg.Model)
	}
}
