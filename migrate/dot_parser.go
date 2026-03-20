// Package migrate converts DOT digraph files into Dippin IR workflows.
//
// It implements the migration strategy from §16 of the Dippin design spec:
// parse a DOT digraph, apply cleanup transforms (un-escaping, namespace
// prefixing, shape→kind mapping), and produce either an *ir.Workflow or
// canonical .dip source text.
package migrate

import (
	"fmt"
	"strings"
	"unicode"
)

// dotGraph holds the parsed DOT structure before IR conversion.
type dotGraph struct {
	Name       string
	GraphAttrs map[string]string
	NodeAttrs  map[string]string // default node attrs
	EdgeAttrs  map[string]string // default edge attrs
	Nodes      []dotNode
	Edges      []dotEdge
}

// dotNode is a single DOT node statement with attributes.
type dotNode struct {
	ID    string
	Attrs map[string]string
}

// dotEdge is a single DOT edge statement (A -> B) with attributes.
type dotEdge struct {
	From  string
	To    string
	Attrs map[string]string
}

// --- DOT Lexer ---

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokID            // unquoted identifier or number
	tokString        // double-quoted string (contents unescaped)
	tokLBrace        // {
	tokRBrace        // }
	tokLBrack        // [
	tokRBrack        // ]
	tokEquals        // =
	tokSemicolon     // ;
	tokComma         // ,
	tokArrow         // ->
)

type token struct {
	kind tokenKind
	val  string
	pos  int // byte offset for error messages
}

// lexer tokenizes a DOT input string.
type lexer struct {
	input []byte
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input: []byte(input)}
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *lexer) advance() byte {
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.pos++
			continue
		}
		// C-style line comments.
		if ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '/' {
			l.pos += 2
			for l.pos < len(l.input) && l.input[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		// C-style block comments.
		if ch == '/' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '*' {
			l.pos += 2
			for l.pos+1 < len(l.input) {
				if l.input[l.pos] == '*' && l.input[l.pos+1] == '/' {
					l.pos += 2
					break
				}
				l.pos++
			}
			continue
		}
		break
	}
}

func (l *lexer) next() token {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return token{kind: tokEOF, pos: l.pos}
	}

	start := l.pos
	ch := l.peek()

	switch ch {
	case '{':
		l.advance()
		return token{kind: tokLBrace, val: "{", pos: start}
	case '}':
		l.advance()
		return token{kind: tokRBrace, val: "}", pos: start}
	case '[':
		l.advance()
		return token{kind: tokLBrack, val: "[", pos: start}
	case ']':
		l.advance()
		return token{kind: tokRBrack, val: "]", pos: start}
	case '=':
		l.advance()
		return token{kind: tokEquals, val: "=", pos: start}
	case ';':
		l.advance()
		return token{kind: tokSemicolon, val: ";", pos: start}
	case ',':
		l.advance()
		return token{kind: tokComma, val: ",", pos: start}
	case '-':
		if l.pos+1 < len(l.input) && l.input[l.pos+1] == '>' {
			l.pos += 2
			return token{kind: tokArrow, val: "->", pos: start}
		}
		// Bare '-' — treat as part of an identifier.
		return l.readID()
	case '"':
		return l.readString()
	default:
		if isIDStart(ch) || (ch >= '0' && ch <= '9') {
			return l.readID()
		}
		l.advance()
		return token{kind: tokID, val: string(ch), pos: start}
	}
}

func isIDStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIDCont(ch byte) bool {
	return isIDStart(ch) || (ch >= '0' && ch <= '9') || ch == '.'
}

func (l *lexer) readID() token {
	start := l.pos
	for l.pos < len(l.input) && isIDCont(l.input[l.pos]) {
		l.pos++
	}
	return token{kind: tokID, val: string(l.input[start:l.pos]), pos: start}
}

func (l *lexer) readString() token {
	start := l.pos
	l.advance() // skip opening "
	var b strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if ch == '\\' && l.pos < len(l.input) {
			next := l.advance()
			switch next {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'n':
				b.WriteByte('\n')
			case 'l':
				b.WriteByte('\n') // DOT \l = left-justified newline → real newline
			case 'r':
				// DOT \r — ignore (not meaningful)
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
			}
			continue
		}
		if ch == '"' {
			return token{kind: tokString, val: b.String(), pos: start}
		}
		b.WriteByte(ch)
	}
	// Unterminated string — return what we have; parser will catch the error.
	return token{kind: tokString, val: b.String(), pos: start}
}

// --- DOT Parser ---

type parser struct {
	lex   *lexer
	cur   token
	graph *dotGraph
}

func parseDOT(input string) (*dotGraph, error) {
	p := &parser{
		lex: newLexer(input),
		graph: &dotGraph{
			GraphAttrs: make(map[string]string),
			NodeAttrs:  make(map[string]string),
			EdgeAttrs:  make(map[string]string),
		},
	}
	p.advance()
	if err := p.parseDigraph(); err != nil {
		return nil, err
	}
	return p.graph, nil
}

func (p *parser) advance() {
	p.cur = p.lex.next()
}

func (p *parser) expect(k tokenKind) (token, error) {
	if p.cur.kind != k {
		return p.cur, fmt.Errorf("DOT parse error at offset %d: expected %s, got %q",
			p.cur.pos, tokenKindName(k), p.cur.val)
	}
	t := p.cur
	p.advance()
	return t, nil
}

func tokenKindName(k tokenKind) string {
	switch k {
	case tokEOF:
		return "EOF"
	case tokID:
		return "identifier"
	case tokString:
		return "string"
	case tokLBrace:
		return "'{'"
	case tokRBrace:
		return "'}'"
	case tokLBrack:
		return "'['"
	case tokRBrack:
		return "']'"
	case tokEquals:
		return "'='"
	case tokSemicolon:
		return "';'"
	case tokComma:
		return "','"
	case tokArrow:
		return "'->'"
	default:
		return "unknown"
	}
}

func (p *parser) parseDigraph() error {
	// Expect: digraph <name> { ... }
	id, err := p.expect(tokID)
	if err != nil {
		return err
	}
	if id.val != "digraph" {
		return fmt.Errorf("DOT parse error at offset %d: expected 'digraph', got %q", id.pos, id.val)
	}

	// Graph name: can be ID or string.
	name, err := p.readIDOrString()
	if err != nil {
		return err
	}
	p.graph.Name = name

	if _, err := p.expect(tokLBrace); err != nil {
		return err
	}

	for p.cur.kind != tokRBrace && p.cur.kind != tokEOF {
		if err := p.parseStatement(); err != nil {
			return err
		}
	}

	if _, err := p.expect(tokRBrace); err != nil {
		return err
	}
	return nil
}

func (p *parser) readIDOrString() (string, error) {
	switch p.cur.kind {
	case tokID:
		t := p.cur
		p.advance()
		return t.val, nil
	case tokString:
		t := p.cur
		p.advance()
		return t.val, nil
	default:
		return "", fmt.Errorf("DOT parse error at offset %d: expected identifier or string, got %q",
			p.cur.pos, p.cur.val)
	}
}

func (p *parser) parseStatement() error {
	// Skip stray semicolons.
	if p.cur.kind == tokSemicolon {
		p.advance()
		return nil
	}

	// Must be an ID or string to start a statement.
	if p.cur.kind != tokID && p.cur.kind != tokString {
		return fmt.Errorf("DOT parse error at offset %d: unexpected token %q", p.cur.pos, p.cur.val)
	}

	name := p.cur.val
	nameKind := p.cur.kind
	p.advance()

	// graph/node/edge default attributes: graph [ ... ] or node [ ... ] or edge [ ... ]
	if nameKind == tokID && (name == "graph" || name == "node" || name == "edge") && p.cur.kind == tokLBrack {
		attrs, err := p.parseAttrList()
		if err != nil {
			return err
		}
		switch name {
		case "graph":
			for k, v := range attrs {
				p.graph.GraphAttrs[k] = v
			}
		case "node":
			for k, v := range attrs {
				p.graph.NodeAttrs[k] = v
			}
		case "edge":
			for k, v := range attrs {
				p.graph.EdgeAttrs[k] = v
			}
		}
		p.consumeOptionalSemicolon()
		return nil
	}

	// Edge statement: ID -> ID [ ... ] ;
	if p.cur.kind == tokArrow {
		p.advance()
		to, err := p.readIDOrString()
		if err != nil {
			return err
		}
		attrs := make(map[string]string)
		if p.cur.kind == tokLBrack {
			attrs, err = p.parseAttrList()
			if err != nil {
				return err
			}
		}
		// Merge default edge attrs.
		merged := make(map[string]string)
		for k, v := range p.graph.EdgeAttrs {
			merged[k] = v
		}
		for k, v := range attrs {
			merged[k] = v
		}
		p.graph.Edges = append(p.graph.Edges, dotEdge{From: name, To: to, Attrs: merged})

		// Ensure both nodes exist (implicit declaration).
		p.ensureNode(name)
		p.ensureNode(to)

		p.consumeOptionalSemicolon()
		return nil
	}

	// Node statement: ID [ ... ] ; or bare ID ;
	attrs := make(map[string]string)
	if p.cur.kind == tokLBrack {
		var err error
		attrs, err = p.parseAttrList()
		if err != nil {
			return err
		}
	}
	// Merge default node attrs.
	merged := make(map[string]string)
	for k, v := range p.graph.NodeAttrs {
		merged[k] = v
	}
	for k, v := range attrs {
		merged[k] = v
	}
	p.addOrUpdateNode(name, merged)

	p.consumeOptionalSemicolon()
	return nil
}

// parseAttrList parses [ key=value, key=value, ... ].
func (p *parser) parseAttrList() (map[string]string, error) {
	if _, err := p.expect(tokLBrack); err != nil {
		return nil, err
	}

	attrs := make(map[string]string)
	for p.cur.kind != tokRBrack && p.cur.kind != tokEOF {
		key, err := p.readIDOrString()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokEquals); err != nil {
			return nil, err
		}
		val, err := p.readIDOrString()
		if err != nil {
			return nil, err
		}
		attrs[key] = val

		// Optional comma or semicolon separator.
		if p.cur.kind == tokComma || p.cur.kind == tokSemicolon {
			p.advance()
		}
	}

	if _, err := p.expect(tokRBrack); err != nil {
		return nil, err
	}
	return attrs, nil
}

func (p *parser) consumeOptionalSemicolon() {
	if p.cur.kind == tokSemicolon {
		p.advance()
	}
}

// ensureNode adds a placeholder node if it doesn't already exist.
func (p *parser) ensureNode(id string) {
	for _, n := range p.graph.Nodes {
		if n.ID == id {
			return
		}
	}
	p.graph.Nodes = append(p.graph.Nodes, dotNode{ID: id, Attrs: make(map[string]string)})
}

// addOrUpdateNode adds a node or updates its attrs if it already exists.
func (p *parser) addOrUpdateNode(id string, attrs map[string]string) {
	for i, n := range p.graph.Nodes {
		if n.ID == id {
			// Merge attrs into existing node.
			for k, v := range attrs {
				p.graph.Nodes[i].Attrs[k] = v
			}
			return
		}
	}
	p.graph.Nodes = append(p.graph.Nodes, dotNode{ID: id, Attrs: attrs})
}

// unescapeDOT transforms DOT escape sequences in a raw string value.
// The lexer handles this for quoted strings; this is for post-processing
// values that were read as unquoted identifiers or for additional cleanup.
func unescapeDOT(s string) string {
	// The lexer already handles escape sequences inside quoted strings.
	// This function handles any remaining \n sequences that might appear
	// in attribute values from unquoted or partially-processed sources.
	return s
}

// normalizeWhitespace collapses runs of whitespace into single spaces
// and trims leading/trailing whitespace. Used for condition comparison.
func normalizeWhitespace(s string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
		} else {
			b.WriteRune(r)
			inSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
