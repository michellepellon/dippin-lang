package parser

import (
	"strings"
	"testing"
)

func TestTokenTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []TokenType
	}{
		{
			name:  "colon",
			input: "key: val",
			want:  []TokenType{TokenIdentifier, TokenColon, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "arrow",
			input: "A -> B",
			want:  []TokenType{TokenIdentifier, TokenArrow, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "back arrow",
			input: "J <- A",
			want:  []TokenType{TokenIdentifier, TokenBackArrow, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "comma",
			input: "A, B, C",
			want:  []TokenType{TokenIdentifier, TokenComma, TokenIdentifier, TokenComma, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "operators",
			input: "a == b",
			want:  []TokenType{TokenIdentifier, TokenOperator, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "not equal",
			input: "a != b",
			want:  []TokenType{TokenIdentifier, TokenOperator, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "less equal",
			input: "a <= b",
			want:  []TokenType{TokenIdentifier, TokenOperator, TokenIdentifier, TokenNewline, TokenEOF},
		},
		{
			name:  "quoted string",
			input: `"hello world"`,
			want:  []TokenType{TokenLiteral, TokenNewline, TokenEOF},
		},
		{
			name:  "parens",
			input: "(a)",
			want:  []TokenType{TokenLParen, TokenIdentifier, TokenRParen, TokenNewline, TokenEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input, "test")
			var got []TokenType
			for {
				tok := l.NextToken()
				got = append(got, tok.Type)
				if tok.Type == TokenEOF {
					break
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("token count = %d, want %d\ngot types: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d] type = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIndentOutdent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []TokenType
	}{
		{
			name:  "single indent",
			input: "a\n  b",
			want:  []TokenType{TokenIdentifier, TokenNewline, TokenIndent, TokenIdentifier, TokenNewline, TokenOutdent, TokenEOF},
		},
		{
			name:  "indent and outdent",
			input: "a\n  b\nc",
			want: []TokenType{
				TokenIdentifier, TokenNewline,
				TokenIndent, TokenIdentifier, TokenNewline, TokenOutdent,
				TokenIdentifier, TokenNewline,
				TokenEOF,
			},
		},
		{
			name:  "multi-level indent",
			input: "a\n  b\n    c\nd",
			want: []TokenType{
				TokenIdentifier, TokenNewline,
				TokenIndent, TokenIdentifier, TokenNewline,
				TokenIndent, TokenIdentifier, TokenNewline,
				TokenOutdent, TokenOutdent,
				TokenIdentifier, TokenNewline,
				TokenEOF,
			},
		},
		{
			name:  "remaining outdents at EOF",
			input: "a\n  b\n    c",
			want: []TokenType{
				TokenIdentifier, TokenNewline,
				TokenIndent, TokenIdentifier, TokenNewline,
				TokenIndent, TokenIdentifier, TokenNewline,
				TokenOutdent, TokenOutdent,
				TokenEOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input, "test")
			var got []TokenType
			for {
				tok := l.NextToken()
				got = append(got, tok.Type)
				if tok.Type == TokenEOF {
					break
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("token count = %d, want %d\ngot: %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCommentStripping(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no comment", "hello world", "hello world"},
		{"inline comment", "hello # comment", "hello "},
		{"hash at start", "#comment line", ""},
		{"hash in quotes", `"hello # world"`, `"hello # world"`},
		{"no space before hash", "hello#notcomment", "hello#notcomment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripComment(tt.input)
			if got != tt.want {
				t.Errorf("stripComment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCommentOnlyLinesSkipped(t *testing.T) {
	input := "# comment\na\n# another comment\nb"
	l := NewLexer(input, "test")

	var idents []string
	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
		if tok.Type == TokenIdentifier {
			idents = append(idents, tok.Value)
		}
	}
	if len(idents) != 2 || idents[0] != "a" || idents[1] != "b" {
		t.Errorf("identifiers = %v, want [a b]", idents)
	}
}

func TestRawBlock(t *testing.T) {
	input := "prompt:\n  line one\n  line two\nnext"
	l := NewLexer(input, "test")

	var rawBlock string
	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
		if tok.Type == TokenRawBlock {
			rawBlock = tok.Value
		}
	}
	if rawBlock == "" {
		t.Fatal("expected TokenRawBlock, got none")
	}
	if !strings.Contains(rawBlock, "line one") {
		t.Errorf("raw block missing 'line one': %q", rawBlock)
	}
	if !strings.Contains(rawBlock, "line two") {
		t.Errorf("raw block missing 'line two': %q", rawBlock)
	}
}

func TestRawBlockWithBlankLines(t *testing.T) {
	input := "prompt:\n  para one\n\n  para two\nnext"
	l := NewLexer(input, "test")

	var rawBlock string
	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
		if tok.Type == TokenRawBlock {
			rawBlock = tok.Value
		}
	}
	if !strings.Contains(rawBlock, "para one") {
		t.Errorf("raw block missing 'para one': %q", rawBlock)
	}
	if !strings.Contains(rawBlock, "para two") {
		t.Errorf("raw block missing 'para two': %q", rawBlock)
	}
	if !strings.Contains(rawBlock, "\n\n") {
		t.Errorf("blank line not preserved in raw block: %q", rawBlock)
	}
}

func TestEmptyInput(t *testing.T) {
	l := NewLexer("", "test")
	tok := l.NextToken()
	if tok.Type != TokenEOF {
		t.Errorf("empty input should yield EOF, got %v", tok.Type)
	}
}

func TestAllCommentFile(t *testing.T) {
	input := "# just comments\n# nothing else\n"
	l := NewLexer(input, "test")
	tok := l.NextToken()
	if tok.Type != TokenEOF {
		t.Errorf("all-comment file should yield EOF, got %v", tok.Type)
	}
}

func TestRawValueText(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"simple", "  fidelity: summary:medium", "summary:medium"},
		{"quoted", `  prompt: "hello"`, `"hello"`},
		{"with comment", "  model: gpt-5.4 # fast", "gpt-5.4"},
		{"no colon", "  novalue", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.line, "test")
			got := l.RawValueText(1)
			if got != tt.want {
				t.Errorf("RawValueText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLineIndent(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"hello", 0},
		{"  hello", 2},
		{"\thello", 1},
		{"    hello", 4},
		{"", 0},
	}

	for _, tt := range tests {
		got := lineIndent(tt.line)
		if got != tt.want {
			t.Errorf("lineIndent(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestIsKeyColonLine(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"prompt:", true},
		{"prompt: value", false},
		{"prompt:value", false},
		{":bad", false},
		{"a_b:", true},
	}

	for _, tt := range tests {
		got := isKeyColonLine(tt.content)
		if got != tt.want {
			t.Errorf("isKeyColonLine(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestMixedTabsSpaces(t *testing.T) {
	// Tabs and spaces mixed — should not panic
	input := "a\n\t b\nc"
	l := NewLexer(input, "test")
	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
	}
	// If we get here without panic, the test passes
}

func TestPeekDoesNotAdvance(t *testing.T) {
	l := NewLexer("hello world", "test")
	first := l.PeekToken()
	second := l.PeekToken()
	if first.Value != second.Value {
		t.Errorf("PeekToken advanced: first=%q, second=%q", first.Value, second.Value)
	}
}

func TestQuotedStringEscapes(t *testing.T) {
	input := `"hello \"world\""`
	l := NewLexer(input, "test")
	tok := l.NextToken()
	if tok.Type != TokenLiteral {
		t.Fatalf("expected TokenLiteral, got %v", tok.Type)
	}
	if tok.Value != `hello "world"` {
		t.Errorf("value = %q, want %q", tok.Value, `hello "world"`)
	}
}

func TestFindUnquotedHash(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"no hash", -1},
		{"has # hash", 4},
		{`"# inside quotes"`, -1},
		{`"quoted" # after`, 9},
	}

	for _, tt := range tests {
		got := findUnquotedHash(tt.input)
		if got != tt.want {
			t.Errorf("findUnquotedHash(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestIsBlankOrComment(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"", true},
		{"   ", true},
		{"# comment", true},
		{"  # indented comment", true},
		{"code", false},
		{"  code", false},
	}

	for _, tt := range tests {
		got := isBlankOrComment(tt.line)
		if got != tt.want {
			t.Errorf("isBlankOrComment(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
