package parser

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/2389-research/dippin-lang/ir"
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
	TokenRawBlock // Raw text block (multiline prompt/command content)
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
	lines       []string // original lines for raw extraction
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
		lines:       strings.Split(input, "\n"),
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

// lineIndent returns the number of leading whitespace bytes in a line.
// Tabs and spaces each count as one byte. This is used for indentation
// tracking where the returned value is also used as a byte offset
// into the string (e.g., line[lineIndent(line):] to get content).
func lineIndent(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// isBlankOrComment returns true if a line is empty, whitespace-only, or a comment-only line.
// A comment-only line has only optional whitespace followed by #.
func isBlankOrComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) == 0 || trimmed[0] == '#'
}

// stripComment removes a trailing comment from a line, but only if # is preceded by
// whitespace (not inside a value). Does NOT strip # that starts the line content.
func stripComment(line string) string {
	trimmed := strings.TrimRight(line, " \t\r")
	// Find # that's preceded by whitespace and not inside quotes
	inQuote := false
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == '"' {
			inQuote = !inQuote
		}
		if !inQuote && trimmed[i] == '#' {
			// Only strip if preceded by whitespace or at start of content
			indent := lineIndent(trimmed)
			if i == indent {
				// # is the first non-space char — this is a comment-only line
				return trimmed[:i]
			}
			if i > 0 && (trimmed[i-1] == ' ' || trimmed[i-1] == '\t') {
				return trimmed[:i]
			}
		}
	}
	return trimmed
}

func (l *Lexer) lex(filename string) {
	i := 0
	for i < len(l.lines) {
		line := l.lines[i]
		l.line = i + 1
		l.col = 1
		trimmed := strings.TrimRight(line, " \t\r")

		// Skip blank lines entirely
		if isBlankOrComment(trimmed) {
			i++
			continue
		}

		// Strip inline comments (but not # inside content)
		trimmed = stripComment(trimmed)
		if len(strings.TrimSpace(trimmed)) == 0 {
			i++
			continue
		}

		// Calculate indentation
		indent := lineIndent(trimmed)
		content := trimmed[indent:]

		// Emit indent/outdent tokens
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

		// Check if this line is a "key:" pattern that starts a multiline block.
		// Pattern: identifier followed by colon, nothing else on the line.
		// If the next non-blank line is indented deeper, emit a raw block token.
		if isKeyColonLine(content) {
			keyEnd := strings.Index(content, ":")
			key := content[:keyEnd]
			loc := ir.SourceLocation{File: filename, Line: l.line, Column: indent + 1}
			l.tokens = append(l.tokens, Token{Type: TokenIdentifier, Value: key, Location: loc})
			l.tokens = append(l.tokens, Token{Type: TokenColon, Value: ":", Location: ir.SourceLocation{File: filename, Line: l.line, Column: indent + keyEnd + 1}})

			// Look ahead for indented block
			blockStart := i + 1
			nextContentLine := blockStart
			for nextContentLine < len(l.lines) {
				nl := strings.TrimRight(l.lines[nextContentLine], " \t\r")
				if len(strings.TrimSpace(nl)) == 0 {
					nextContentLine++
					continue
				}
				break
			}

			if nextContentLine < len(l.lines) {
				nextIndent := lineIndent(l.lines[nextContentLine])
				if nextIndent > indent {
					// This is a multiline block. Collect all lines until
					// indentation drops to <= the key's indentation level.
					blockEnd := nextContentLine
					for blockEnd < len(l.lines) {
						bl := l.lines[blockEnd]
						blTrimmed := strings.TrimRight(bl, " \t\r")
						// Include blank lines within the block
						if len(strings.TrimSpace(blTrimmed)) == 0 {
							blockEnd++
							continue
						}
						blIndent := lineIndent(blTrimmed)
						if blIndent <= indent {
							break
						}
						blockEnd++
					}

					// Extract raw text, stripping the block's indentation prefix
					rawText := l.extractRawBlock(nextContentLine, blockEnd, nextIndent)
					l.tokens = append(l.tokens, Token{Type: TokenNewline, Location: ir.SourceLocation{File: filename, Line: l.line, Column: len(line) + 1}})
					l.tokens = append(l.tokens, Token{Type: TokenRawBlock, Value: rawText, Location: ir.SourceLocation{File: filename, Line: nextContentLine + 1, Column: nextIndent + 1}})
					l.tokens = append(l.tokens, Token{Type: TokenNewline, Location: ir.SourceLocation{File: filename, Line: blockEnd + 1, Column: 1}})

					i = blockEnd
					continue
				}
			}

			// No multiline block follows — just a key: with empty value
			l.tokens = append(l.tokens, Token{Type: TokenNewline, Location: ir.SourceLocation{File: filename, Line: l.line, Column: len(line) + 1}})
			i++
			continue
		}

		l.lexLine(content, filename)
		l.tokens = append(l.tokens, Token{Type: TokenNewline, Location: ir.SourceLocation{File: filename, Line: l.line, Column: len(line) + 1}})
		i++
	}

	// Outdent remaining
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		l.tokens = append(l.tokens, Token{Type: TokenOutdent, Location: ir.SourceLocation{File: filename, Line: l.line, Column: 1}})
	}
	l.tokens = append(l.tokens, Token{Type: TokenEOF, Location: ir.SourceLocation{File: filename, Line: l.line, Column: 1}})
}

// isKeyColonLine checks if the line content (after indent) is just "identifier:"
// with nothing after the colon (or only whitespace).
func isKeyColonLine(content string) bool {
	colonIdx := strings.Index(content, ":")
	if colonIdx <= 0 {
		return false
	}
	// Everything before colon must be an identifier
	key := content[:colonIdx]
	for _, ch := range key {
		if !isAlphaNumRune(ch) && ch != '_' {
			return false
		}
	}
	// Everything after colon must be whitespace
	after := strings.TrimSpace(content[colonIdx+1:])
	return len(after) == 0
}

// extractRawBlock extracts raw text from original lines, stripping indent prefix.
// startIdx and endIdx are 0-based line indices.
func (l *Lexer) extractRawBlock(startIdx, endIdx, indent int) string {
	var result []string
	for i := startIdx; i < endIdx && i < len(l.lines); i++ {
		line := l.lines[i]
		line = strings.TrimRight(line, "\r")
		// Strip the indent prefix (byte count)
		j := 0
		for j < len(line) && j < indent && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		result = append(result, line[j:])
	}
	// Trim trailing empty lines
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\n")
}

func (l *Lexer) lexLine(line string, filename string) {
	i := 0
	colOffset := l.col + (l.indentStack[len(l.indentStack)-1])

	for i < len(line) {
		// Skip whitespace
		i = skipWhitespace(line, i)
		if i >= len(line) {
			break
		}

		loc := ir.SourceLocation{File: filename, Line: l.line, Column: colOffset + i}

		// Try each token type in order
		if newI, ok := l.tryLexPunctuation(line, i, loc); ok {
			i = newI
			continue
		}
		if newI, ok := l.tryLexArrow(line, i, loc); ok {
			i = newI
			continue
		}
		if newI, ok := l.tryLexOperator(line, i, loc); ok {
			i = newI
			continue
		}
		if newI, ok := l.tryLexQuotedString(line, i, loc); ok {
			i = newI
			continue
		}
		if newI, ok := l.tryLexIdentifier(line, i, loc); ok {
			i = newI
			continue
		}

		// Unknown character, skip it
		i++
	}
}

// skipWhitespace advances the index past any whitespace characters.
func skipWhitespace(line string, i int) int {
	for i < len(line) && unicode.IsSpace(rune(line[i])) {
		i++
	}
	return i
}

// tryLexPunctuation handles single-character punctuation: : , ( )
func (l *Lexer) tryLexPunctuation(line string, i int, loc ir.SourceLocation) (int, bool) {
	ch := line[i]

	var tokType TokenType
	switch ch {
	case ':':
		tokType = TokenColon
	case ',':
		tokType = TokenComma
	case '(':
		tokType = TokenLParen
	case ')':
		tokType = TokenRParen
	default:
		return i, false
	}

	l.tokens = append(l.tokens, Token{
		Type:     tokType,
		Value:    string(ch),
		Location: loc,
	})
	return i + 1, true
}

// tryLexArrow handles two-character arrows: -> and <-
func (l *Lexer) tryLexArrow(line string, i int, loc ir.SourceLocation) (int, bool) {
	if strings.HasPrefix(line[i:], "->") {
		l.tokens = append(l.tokens, Token{
			Type:     TokenArrow,
			Value:    "->",
			Location: loc,
		})
		return i + 2, true
	}

	if strings.HasPrefix(line[i:], "<-") {
		l.tokens = append(l.tokens, Token{
			Type:     TokenBackArrow,
			Value:    "<-",
			Location: loc,
		})
		return i + 2, true
	}

	return i, false
}

// tryLexOperator handles comparison operators: ==, !=, <=, >=, =, <, >, !
func (l *Lexer) tryLexOperator(line string, i int, loc ir.SourceLocation) (int, bool) {
	ch := line[i]

	// Not an operator character
	if ch != '=' && ch != '!' && ch != '<' && ch != '>' {
		return i, false
	}

	// Try two-character operators first
	if i+1 < len(line) {
		twoChar := line[i : i+2]
		if twoChar == "==" || twoChar == "!=" || twoChar == "<=" || twoChar == ">=" {
			l.tokens = append(l.tokens, Token{
				Type:     TokenOperator,
				Value:    twoChar,
				Location: loc,
			})
			return i + 2, true
		}
	}

	// Single-character operator
	l.tokens = append(l.tokens, Token{
		Type:     TokenOperator,
		Value:    string(ch),
		Location: loc,
	})
	return i + 1, true
}

// tryLexQuotedString handles double-quoted string literals with escape sequences.
func (l *Lexer) tryLexQuotedString(line string, i int, loc ir.SourceLocation) (int, bool) {
	if line[i] != '"' {
		return i, false
	}

	i++ // skip opening quote
	var content strings.Builder

	for i < len(line) && line[i] != '"' {
		if line[i] == '\\' && i+1 < len(line) {
			// Escape sequence
			i++
			content.WriteByte(line[i])
		} else {
			content.WriteByte(line[i])
		}
		i++
	}

	if i < len(line) && line[i] == '"' {
		i++ // skip closing quote
	}

	l.tokens = append(l.tokens, Token{
		Type:     TokenLiteral,
		Value:    content.String(),
		Location: loc,
	})
	return i, true
}

// tryLexIdentifier handles alphanumeric identifiers including _, -, ., /
func (l *Lexer) tryLexIdentifier(line string, i int, loc ir.SourceLocation) (int, bool) {
	if !isAlphaNum(line[i]) {
		return i, false
	}

	start := i
	for i < len(line) && (isAlphaNum(line[i]) || line[i] == '_' || line[i] == '-' || line[i] == '.' || line[i] == '/') {
		i++
	}

	l.tokens = append(l.tokens, Token{
		Type:     TokenIdentifier,
		Value:    line[start:i],
		Location: loc,
	})
	return i, true
}

// RawValueText extracts the raw value text from a line, starting after the colon
// following the field name. Used for single-line values like "fidelity: summary:medium".
// Strips inline comments (# preceded by whitespace) from the extracted value.
func (l *Lexer) RawValueText(lineNum int) string {
	if lineNum < 1 || lineNum > len(l.lines) {
		return ""
	}
	line := l.lines[lineNum-1]
	line = strings.TrimRight(line, " \t\r")
	// Find the first colon after the field name
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return ""
	}
	val := strings.TrimSpace(line[colonIdx+1:])
	// Strip inline comments, but only if the value doesn't start with #
	// (e.g., a quoted value like `"#ff0000"` should not be stripped).
	if len(val) > 0 && val[0] != '#' && val[0] != '"' {
		val = stripComment(val)
		val = strings.TrimSpace(val)
	}
	return val
}

func isAlphaNum(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func isAlphaNumRune(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}
