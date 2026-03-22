package migrate

import (
	"testing"
)

func TestDOTLexerTokens(t *testing.T) {
	input := `digraph G { A -> B [label="test"]; }`
	lex := newLexer(input)

	expected := []struct {
		kind tokenKind
		val  string
	}{
		{tokID, "digraph"},
		{tokID, "G"},
		{tokLBrace, "{"},
		{tokID, "A"},
		{tokArrow, "->"},
		{tokID, "B"},
		{tokLBrack, "["},
		{tokID, "label"},
		{tokEquals, "="},
		{tokString, "test"},
		{tokRBrack, "]"},
		{tokSemicolon, ";"},
		{tokRBrace, "}"},
		{tokEOF, ""},
	}

	for i, exp := range expected {
		tok := lex.next()
		if tok.kind != exp.kind {
			t.Errorf("token[%d] kind = %v, want %v (val=%q)", i, tok.kind, exp.kind, tok.val)
		}
		if tok.val != exp.val {
			t.Errorf("token[%d] val = %q, want %q", i, tok.val, exp.val)
		}
	}
}

func TestDOTParseSimpleDigraph(t *testing.T) {
	input := `digraph test {
		A -> B;
		B -> C;
	}`
	g, err := parseDOT(input)
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "test" {
		t.Errorf("name = %q, want test", g.Name)
	}
	if len(g.Edges) != 2 {
		t.Errorf("edges = %d, want 2", len(g.Edges))
	}
}

func TestDOTParseNodeAttributes(t *testing.T) {
	input := `digraph test {
		A [shape=box, label="Step A"];
		A -> B;
	}`
	g, err := parseDOT(input)
	if err != nil {
		t.Fatal(err)
	}

	var nodeA *dotNode
	for i := range g.Nodes {
		if g.Nodes[i].ID == "A" {
			nodeA = &g.Nodes[i]
			break
		}
	}
	if nodeA == nil {
		t.Fatal("node A not found")
	}
	if nodeA.Attrs["shape"] != "box" {
		t.Errorf("A.shape = %q, want box", nodeA.Attrs["shape"])
	}
	if nodeA.Attrs["label"] != "Step A" {
		t.Errorf("A.label = %q, want 'Step A'", nodeA.Attrs["label"])
	}
}

func TestDOTParseQuotedStringUnescaping(t *testing.T) {
	input := `digraph test {
		A [label="hello\nworld"];
	}`
	g, err := parseDOT(input)
	if err != nil {
		t.Fatal(err)
	}

	var nodeA *dotNode
	for i := range g.Nodes {
		if g.Nodes[i].ID == "A" {
			nodeA = &g.Nodes[i]
			break
		}
	}
	if nodeA == nil {
		t.Fatal("node A not found")
	}
	if nodeA.Attrs["label"] != "hello\nworld" {
		t.Errorf("A.label = %q, want 'hello\\nworld'", nodeA.Attrs["label"])
	}
}

func TestDOTParseEdgeAttributes(t *testing.T) {
	input := `digraph test {
		A -> B [label="next", style=bold];
	}`
	g, err := parseDOT(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(g.Edges))
	}
	if g.Edges[0].Attrs["label"] != "next" {
		t.Errorf("edge label = %q, want next", g.Edges[0].Attrs["label"])
	}
}

func TestDOTParseDefaultAttrs(t *testing.T) {
	input := `digraph test {
		node [shape=box];
		A;
		B;
		A -> B;
	}`
	g, err := parseDOT(input)
	if err != nil {
		t.Fatal(err)
	}

	// Default node attrs should be merged into node attrs
	for _, n := range g.Nodes {
		if n.Attrs["shape"] != "box" {
			t.Errorf("node %s.shape = %q, want box", n.ID, n.Attrs["shape"])
		}
	}
}

func TestDOTParseErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing digraph", `graph G { A; }`},
		{"missing brace", `digraph G A;`},
		{"unterminated attrs", `digraph G { A [label="x" }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDOT(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestDOTParseComments(t *testing.T) {
	input := `digraph test {
		// line comment
		A -> B;
		/* block comment */
		B -> C;
	}`
	g, err := parseDOT(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 2 {
		t.Errorf("edges = %d, want 2", len(g.Edges))
	}
}

func TestDOTNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello  world", "hello world"},
		{"  spaces  ", "spaces"},
		{"a\n\tb", "a b"},
	}

	for _, tt := range tests {
		got := normalizeWhitespace(tt.input)
		if got != tt.want {
			t.Errorf("normalizeWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
