package parser

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/2389/dippin/ir"
)

type TokenType int

const (
	TokenError TokenType = iota
	TokenEOF
	TokenNewline
	TokenIndent
	TokenOutdent
	TokenKeyword
	TokenIdentifier
	TokenOperator
	TokenLiteral
	TokenColon
	TokenComma
	TokenArrow
	TokenBackArrow
	TokenLParen
	TokenRParen
)

type Token struct {
	Type     TokenType
	Value    string
	Location ir.SourceLocation
}

func (t Token) String() string {
	return fmt.Sprintf("%v(%q)@%d:%d", t.Type, t.Value, t.Location.Line, t.Location.Column)
}

type Lexer struct {
	input       string
	pos         int
	line        int
	col         int
	indentStack []int
	tokens      []Token
	tokenIdx    int
}

func NewLexer(input string, filename string) *Lexer {
	l := &Lexer{
		input:       input,
		line:        1,
		col:         1,
		indentStack: []int{0},
	}
	l.lex(filename)
	return l
}

func (l *Lexer) NextToken() Token {
	if l.tokenIdx >= len(l.tokens) {
		return Token{Type: TokenEOF, Location: ir.SourceLocation{Line: l.line, Column: l.col}}
	}
	t := l.tokens[l.tokenIdx]
	l.tokenIdx++
	return t
}

func (l *Lexer) PeekToken() Token {
	if l.tokenIdx >= len(l.tokens) {
		return Token{Type: TokenEOF, Location: ir.SourceLocation{Line: l.line, Column: l.col}}
	}
	return l.tokens[l.tokenIdx]
}

func (l *Lexer) lex(filename string) {
	lines := strings.Split(l.input, "\n")
	for i, line := range lines {
		l.line = i + 1
		l.col = 1
		trimmed := strings.TrimRight(line, " \t\r")

		// Handle comments
		if idx := strings.Index(trimmed, "#"); idx != -1 {
			trimmed = trimmed[:idx]
		}

		if len(strings.TrimSpace(trimmed)) == 0 {
			// Skip empty lines, but we might need to preserve newlines if we want to distinguish blocks?
			// Actually, Dippin is block-structured by indentation.
			continue
		}

		// Calculate indentation
		indent := 0
		for indent < len(trimmed) && (trimmed[indent] == ' ' || trimmed[indent] == '\t') {
			if trimmed[indent] == '\t' {
				indent += 8 // Arbitrary, but should be consistent
			} else {
				indent++
			}
		}

		currIndent := l.indentStack[len(l.indentStack)-1]
		if indent > currIndent {
			l.indentStack = append(l.indentStack, indent)
			l.tokens = append(l.tokens, Token{Type: TokenIndent, Location: ir.SourceLocation{File: filename, Line: l.line, Column: 1}})
		} else if indent < currIndent {
			for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
				l.indentStack = l.indentStack[:len(l.indentStack)-1]
				l.tokens = append(l.tokens, Token{Type: TokenOutdent, Location: ir.SourceLocation{File: filename, Line: l.line, Column: 1}})
			}
		}

		l.lexLine(trimmed[indent:], filename)
		l.tokens = append(l.tokens, Token{Type: TokenNewline, Location: ir.SourceLocation{File: filename, Line: l.line, Column: len(line) + 1}})
	}

	// Outdent remaining
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		l.tokens = append(l.tokens, Token{Type: TokenOutdent, Location: ir.SourceLocation{File: filename, Line: l.line, Column: 1}})
	}
	l.tokens = append(l.tokens, Token{Type: TokenEOF, Location: ir.SourceLocation{File: filename, Line: l.line, Column: 1}})
}

func (l *Lexer) lexLine(line string, filename string) {
	i := 0
	colOffset := l.col + (l.indentStack[len(l.indentStack)-1])
	for i < len(line) {
		// Skip whitespace
		for i < len(line) && unicode.IsSpace(rune(line[i])) {
			i++
		}
		if i >= len(line) {
			break
		}

		start := i
		ch := line[i]
		loc := ir.SourceLocation{File: filename, Line: l.line, Column: colOffset + i}

		switch {
		case ch == ':':
			l.tokens = append(l.tokens, Token{Type: TokenColon, Value: ":", Location: loc})
			i++
		case ch == ',':
			l.tokens = append(l.tokens, Token{Type: TokenComma, Value: ",", Location: loc})
			i++
		case ch == '(':
			l.tokens = append(l.tokens, Token{Type: TokenLParen, Value: "(", Location: loc})
			i++
		case ch == ')':
			l.tokens = append(l.tokens, Token{Type: TokenRParen, Value: ")", Location: loc})
			i++
		case strings.HasPrefix(line[i:], "->"):
			l.tokens = append(l.tokens, Token{Type: TokenArrow, Value: "->", Location: loc})
			i += 2
		case strings.HasPrefix(line[i:], "<-"):
			l.tokens = append(l.tokens, Token{Type: TokenBackArrow, Value: "<-", Location: loc})
			i += 2
		case ch == '"':
			// Quoted string
			i++
			content := ""
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					i++
					content += string(line[i])
				} else {
					content += string(line[i])
				}
				i++
			}
			if i < len(line) && line[i] == '"' {
				i++
			}
			l.tokens = append(l.tokens, Token{Type: TokenLiteral, Value: content, Location: loc})
		case isAlphaNum(ch):
			// Identifier or keyword or operator (and, or, not, contains)
			for i < len(line) && (isAlphaNum(line[i]) || line[i] == '_' || line[i] == '-' || line[i] == '.' || line[i] == '/') {
				i++
			}
			val := line[start:i]
			l.tokens = append(l.tokens, Token{Type: TokenIdentifier, Value: val, Location: loc})
		case ch == '=' || ch == '!' || ch == '<' || ch == '>':
			if strings.HasPrefix(line[i:], "!=") {
				l.tokens = append(l.tokens, Token{Type: TokenOperator, Value: "!=", Location: loc})
				i += 2
			} else if ch == '=' {
				l.tokens = append(l.tokens, Token{Type: TokenOperator, Value: "=", Location: loc})
				i++
			} else {
				// Other operators could be added here
				l.tokens = append(l.tokens, Token{Type: TokenOperator, Value: string(ch), Location: loc})
				i++
			}
		default:
			// Just treat as identifier for now
			i++
		}
	}
}

func isAlphaNum(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}
