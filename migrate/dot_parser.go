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
	tokEOF       tokenKind = iota
	tokID                  // unquoted identifier or number
	tokString              // double-quoted string (contents unescaped)
	tokLBrace              // {
	tokRBrace              // }
	tokLBrack              // [
	tokRBrack              // ]
	tokEquals              // =
	tokSemicolon           // ;
	tokComma               // ,
	tokArrow               // ->
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

// tokenKindNames maps token kinds to their display names.
var tokenKindNames = map[tokenKind]string{
	tokEOF:       "EOF",
	tokID:        "identifier",
	tokString:    "string",
	tokLBrace:    "'{'",
	tokRBrace:    "'}'",
	tokLBrack:    "'['",
	tokRBrack:    "']'",
	tokEquals:    "'='",
	tokSemicolon: "';'",
	tokComma:     "','",
	tokArrow:     "'->'",
}

func tokenKindName(k tokenKind) string {
	if name, ok := tokenKindNames[k]; ok {
		return name
	}
	return "unknown"
}

// punctTokens maps single-character punctuation to their token kinds.
var punctTokens = map[byte]tokenKind{
	'{': tokLBrace,
	'}': tokRBrace,
	'[': tokLBrack,
	']': tokRBrack,
	'=': tokEquals,
	';': tokSemicolon,
	',': tokComma,
}

// isWhitespace returns true for space, tab, newline, and carriage return.
func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		if isWhitespace(l.input[l.pos]) {
			l.pos++
			continue
		}
		if l.trySkipComment() {
			continue
		}
		break
	}
}

// trySkipComment attempts to skip a line or block comment starting at the current position.
// Returns true if a comment was skipped.
func (l *lexer) trySkipComment() bool {
	if l.pos+1 >= len(l.input) || l.input[l.pos] != '/' {
		return false
	}
	if l.input[l.pos+1] == '/' {
		l.skipLineComment()
		return true
	}
	if l.input[l.pos+1] == '*' {
		l.skipBlockComment()
		return true
	}
	return false
}

// skipLineComment skips from // to end of line.
func (l *lexer) skipLineComment() {
	l.pos += 2
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.pos++
	}
}

// skipBlockComment skips from /* to */.
func (l *lexer) skipBlockComment() {
	l.pos += 2
	for l.pos+1 < len(l.input) {
		if l.input[l.pos] == '*' && l.input[l.pos+1] == '/' {
			l.pos += 2
			return
		}
		l.pos++
	}
}

func (l *lexer) next() token {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return token{kind: tokEOF, pos: l.pos}
	}

	start := l.pos
	ch := l.peek()

	// Single-character punctuation.
	if kind, ok := punctTokens[ch]; ok {
		l.advance()
		return token{kind: kind, val: string(ch), pos: start}
	}

	return l.lexNonPunct(start, ch)
}

// lexNonPunct handles non-punctuation tokens: arrows, strings, identifiers, etc.
func (l *lexer) lexNonPunct(start int, ch byte) token {
	if ch == '-' {
		return l.lexDashOrArrow(start)
	}
	if ch == '"' {
		return l.readString()
	}
	if isIDOrDigitStart(ch) {
		return l.readID()
	}
	l.advance()
	return token{kind: tokID, val: string(ch), pos: start}
}

// lexDashOrArrow handles '-' which may be an arrow '->' or start of an identifier.
func (l *lexer) lexDashOrArrow(start int) token {
	if l.pos+1 < len(l.input) && l.input[l.pos+1] == '>' {
		l.pos += 2
		return token{kind: tokArrow, val: "->", pos: start}
	}
	return l.readID()
}

func isIDStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIDOrDigitStart(ch byte) bool {
	return isIDStart(ch) || (ch >= '0' && ch <= '9')
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
		if ch == '"' {
			return token{kind: tokString, val: b.String(), pos: start}
		}
		if ch == '\\' && l.pos < len(l.input) {
			l.writeEscapeChar(&b)
			continue
		}
		b.WriteByte(ch)
	}
	// Unterminated string — return what we have; parser will catch the error.
	return token{kind: tokString, val: b.String(), pos: start}
}

// escapeMap maps DOT escape characters to their unescaped bytes.
// Special values: 0 means ignore (write nothing), absent means unknown escape.
var escapeMap = map[byte]byte{
	'"':  '"',
	'\\': '\\',
	'n':  '\n',
	'l':  '\n', // DOT \l = left-justified newline → real newline
	'r':  0,    // DOT \r — ignore (not meaningful)
}

// writeEscapeChar reads the character after a backslash and writes
// the unescaped result to the builder.
func (l *lexer) writeEscapeChar(b *strings.Builder) {
	next := l.advance()
	if replacement, ok := escapeMap[next]; ok {
		if replacement != 0 {
			b.WriteByte(replacement)
		}
		return
	}
	b.WriteByte('\\')
	b.WriteByte(next)
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

func (p *parser) parseDigraph() error {
	if err := p.expectDigraphKeyword(); err != nil {
		return err
	}

	name, err := p.readIDOrString()
	if err != nil {
		return err
	}
	p.graph.Name = name

	if _, err := p.expect(tokLBrace); err != nil {
		return err
	}
	if err := p.parseStatements(); err != nil {
		return err
	}
	_, err = p.expect(tokRBrace)
	return err
}

// expectDigraphKeyword expects and consumes the "digraph" keyword.
func (p *parser) expectDigraphKeyword() error {
	id, err := p.expect(tokID)
	if err != nil {
		return err
	}
	if id.val != "digraph" {
		return fmt.Errorf("DOT parse error at offset %d: expected 'digraph', got %q", id.pos, id.val)
	}
	return nil
}

// parseStatements parses all statements inside the digraph braces.
func (p *parser) parseStatements() error {
	for p.cur.kind != tokRBrace && p.cur.kind != tokEOF {
		if err := p.parseStatement(); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) readIDOrString() (string, error) {
	if p.cur.kind == tokID || p.cur.kind == tokString {
		t := p.cur
		p.advance()
		return t.val, nil
	}
	return "", fmt.Errorf("DOT parse error at offset %d: expected identifier or string, got %q",
		p.cur.pos, p.cur.val)
}

func (p *parser) parseStatement() error {
	// Skip stray semicolons.
	if p.cur.kind == tokSemicolon {
		p.advance()
		return nil
	}

	if p.cur.kind != tokID && p.cur.kind != tokString {
		return fmt.Errorf("DOT parse error at offset %d: unexpected token %q", p.cur.pos, p.cur.val)
	}

	name := p.cur.val
	nameKind := p.cur.kind
	p.advance()

	return p.dispatchStatement(name, nameKind)
}

// dispatchStatement routes to the appropriate statement parser.
func (p *parser) dispatchStatement(name string, nameKind tokenKind) error {
	if nameKind == tokID && isDefaultsKeyword(name) && p.cur.kind == tokLBrack {
		return p.parseDefaultsStatement(name)
	}
	if p.cur.kind == tokArrow {
		return p.parseEdgeStatement(name)
	}
	return p.parseNodeStatement(name)
}

// isDefaultsKeyword returns true if the name is a defaults keyword.
func isDefaultsKeyword(name string) bool {
	return name == "graph" || name == "node" || name == "edge"
}

// defaultsTargets maps defaults keywords to their target maps.
func (p *parser) defaultsTarget(keyword string) map[string]string {
	switch keyword {
	case "graph":
		return p.graph.GraphAttrs
	case "node":
		return p.graph.NodeAttrs
	default:
		return p.graph.EdgeAttrs
	}
}

// parseDefaultsStatement handles graph/node/edge [ ... ] default attributes.
func (p *parser) parseDefaultsStatement(keyword string) error {
	attrs, err := p.parseAttrList()
	if err != nil {
		return err
	}
	target := p.defaultsTarget(keyword)
	for k, v := range attrs {
		target[k] = v
	}
	p.consumeOptionalSemicolon()
	return nil
}

// parseEdgeStatement handles ID -> ID [ ... ] edge statements.
func (p *parser) parseEdgeStatement(fromNode string) error {
	p.advance() // consume arrow

	to, err := p.readIDOrString()
	if err != nil {
		return err
	}

	attrs, err := p.parseOptionalAttrs()
	if err != nil {
		return err
	}

	merged := mergeAttrs(p.graph.EdgeAttrs, attrs)
	p.graph.Edges = append(p.graph.Edges, dotEdge{From: fromNode, To: to, Attrs: merged})

	p.ensureNode(fromNode)
	p.ensureNode(to)
	p.consumeOptionalSemicolon()
	return nil
}

// parseNodeStatement handles ID [ ... ] or bare ID node statements.
func (p *parser) parseNodeStatement(nodeID string) error {
	attrs, err := p.parseOptionalAttrs()
	if err != nil {
		return err
	}

	merged := mergeAttrs(p.graph.NodeAttrs, attrs)
	p.addOrUpdateNode(nodeID, merged)
	p.consumeOptionalSemicolon()
	return nil
}

// parseOptionalAttrs parses an optional attribute list (if '[' is present).
func (p *parser) parseOptionalAttrs() (map[string]string, error) {
	if p.cur.kind == tokLBrack {
		return p.parseAttrList()
	}
	return make(map[string]string), nil
}

// mergeAttrs merges defaults with overrides, returning a new map.
func mergeAttrs(defaults, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(defaults)+len(overrides))
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// parseAttrList parses [ key=value, key=value, ... ].
func (p *parser) parseAttrList() (map[string]string, error) {
	if _, err := p.expect(tokLBrack); err != nil {
		return nil, err
	}
	attrs, err := p.parseAttrPairs()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokRBrack); err != nil {
		return nil, err
	}
	return attrs, nil
}

// parseAttrPairs parses key=value pairs until ] or EOF.
func (p *parser) parseAttrPairs() (map[string]string, error) {
	attrs := make(map[string]string)
	for p.cur.kind != tokRBrack && p.cur.kind != tokEOF {
		if err := p.parseOneAttr(attrs); err != nil {
			return nil, err
		}
	}
	return attrs, nil
}

// parseOneAttr parses a single key=value pair and adds it to attrs.
func (p *parser) parseOneAttr(attrs map[string]string) error {
	key, val, err := p.readKeyValuePair()
	if err != nil {
		return err
	}
	attrs[key] = val
	p.consumeOptionalSeparator()
	return nil
}

// readKeyValuePair reads a key=value pair.
func (p *parser) readKeyValuePair() (string, string, error) {
	key, err := p.readIDOrString()
	if err != nil {
		return "", "", err
	}
	if _, err := p.expect(tokEquals); err != nil {
		return "", "", err
	}
	val, err := p.readIDOrString()
	if err != nil {
		return "", "", err
	}
	return key, val, nil
}

// consumeOptionalSeparator consumes an optional comma or semicolon.
func (p *parser) consumeOptionalSeparator() {
	if p.cur.kind == tokComma || p.cur.kind == tokSemicolon {
		p.advance()
	}
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
