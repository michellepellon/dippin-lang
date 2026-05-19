package parser

import (
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

type Parser struct {
	lexer       *Lexer
	filename    string
	diagnostics []string // Simple for now
	workflow    *ir.Workflow
}

func NewParser(input string, filename string) *Parser {
	return &Parser{
		lexer:    NewLexer(input, filename),
		filename: filename,
		workflow: &ir.Workflow{
			SourceMap: &ir.SourceMap{},
		},
	}
}

func (p *Parser) Parse() (*ir.Workflow, error) {
	p.parseTopLevel()
	if len(p.diagnostics) > 0 {
		return p.workflow, fmt.Errorf("parsing errors: %s", strings.Join(p.diagnostics, "; "))
	}
	return p.workflow, nil
}

// Diagnostics returns the accumulated diagnostic messages.
func (p *Parser) Diagnostics() []string {
	return p.diagnostics
}

// parseTopLevel consumes top-level tokens looking for workflow declarations.
func (p *Parser) parseTopLevel() {
	for p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		if t.Type == TokenIdentifier && t.Value == "workflow" {
			p.parseWorkflow()
		} else {
			p.lexer.NextToken()
		}
	}
}

func (p *Parser) parseWorkflow() {
	p.lexer.NextToken() // workflow
	name := p.lexer.NextToken().Value
	p.workflow.Name = name
	p.expect(TokenNewline)

	p.expect(TokenIndent)
	p.parseWorkflowBody()
	p.expect(TokenOutdent)
}

// parseWorkflowBody parses the indented body of a workflow declaration.
func (p *Parser) parseWorkflowBody() {
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		if t.Type == TokenIdentifier {
			p.dispatchWorkflowField(t)
		} else {
			p.lexer.NextToken()
		}
	}
}

// workflowNodeKinds maps identifiers to their node kinds for dispatch.
var workflowNodeKinds = map[string]bool{
	"agent": true, "human": true, "tool": true,
	"subgraph": true, "conditional": true, "manager_loop": true,
}

// workflowSimpleBlocks maps workflow block keywords to their parser methods.
// Populated lazily to avoid init-order issues; see dispatchWorkflowBlock.

// dispatchWorkflowField routes a workflow-level identifier to the right handler.
func (p *Parser) dispatchWorkflowField(t Token) {
	if dispatchWorkflowSimpleField(p, t) {
		return
	}
	p.dispatchWorkflowBlock(t)
}

// dispatchWorkflowSimpleField handles header fields and config blocks (defaults, vars). Returns true if handled.
func dispatchWorkflowSimpleField(p *Parser, t Token) bool {
	switch t.Value {
	case "goal", "start", "exit":
		p.parseWorkflowStringField(t)
	case "defaults":
		p.parseDefaults()
	case "vars":
		p.parseVars()
	default:
		return dispatchWorkflowTailField(p, t)
	}
	return true
}

// dispatchWorkflowTailField handles edges, stylesheet, and requires. Returns true if handled.
func dispatchWorkflowTailField(p *Parser, t Token) bool {
	switch t.Value {
	case "edges":
		p.parseEdges()
	case "stylesheet":
		p.parseStylesheet()
	case "requires":
		p.parseWorkflowRequiresField(t)
	default:
		return false
	}
	return true
}

// dispatchWorkflowBlock handles parallel, fan_in, node kinds, and unknown identifiers.
func (p *Parser) dispatchWorkflowBlock(t Token) {
	switch t.Value {
	case "parallel":
		p.parseParallel()
	case "fan_in":
		p.parseFanIn()
	default:
		p.dispatchWorkflowDefault(t)
	}
}

// dispatchWorkflowDefault handles node kinds and unknown identifiers.
func (p *Parser) dispatchWorkflowDefault(t Token) {
	if workflowNodeKinds[t.Value] {
		p.parseNode(ir.NodeKind(t.Value))
		return
	}
	p.diagnostics = append(p.diagnostics, fmt.Sprintf("unexpected top-level identifier: %s at %d:%d", t.Value, t.Location.Line, t.Location.Column))
	p.lexer.NextToken()
}

// parseWorkflowRequiresField parses "requires: a, b, c" into Workflow.Requires.
// Whitespace is trimmed and empty entries are dropped. A missing or empty list
// leaves Workflow.Requires nil (matches IR nil-vs-empty conventions).
func (p *Parser) parseWorkflowRequiresField(t Token) {
	p.lexer.NextToken() // requires
	p.expect(TokenColon)
	val := p.readFieldValue(t.Location.Line)
	p.workflow.Requires = splitCommaNoEmpty(val)
}

// parseWorkflowStringField parses a simple "key: value" field on the workflow.
func (p *Parser) parseWorkflowStringField(t Token) {
	p.lexer.NextToken()
	p.expect(TokenColon)
	val := p.readFieldValue(t.Location.Line)
	switch t.Value {
	case "goal":
		p.workflow.Goal = val
	case "start":
		p.workflow.Start = val
	case "exit":
		p.workflow.Exit = val
	}
}

func (p *Parser) expect(t TokenType) {
	tok := p.lexer.NextToken()
	if tok.Type != t {
		p.diagnostics = append(p.diagnostics, fmt.Sprintf("expected %v, got %v at %d:%d", t, tok.Type, tok.Location.Line, tok.Location.Column))
	}
}

func (p *Parser) parseCommaList() []string {
	var list []string
	for {
		list = append(list, p.lexer.NextToken().Value)
		if p.lexer.PeekToken().Type != TokenComma {
			break
		}
		p.lexer.NextToken() // comma
	}
	return list
}

// readFieldValue reads a field value, which may be:
// - A raw block (multiline content detected by the lexer)
// - A single-line value on the same line as the key
// - A newline followed by a raw block (key: \n <indented block>)
func (p *Parser) readFieldValue(lineNum int) string {
	if p.lexer.PeekToken().Type == TokenRawBlock {
		return p.lexer.NextToken().Value
	}
	if p.lexer.PeekToken().Type == TokenNewline {
		return p.readBlockAfterNewline()
	}
	return p.readSingleLineValue(lineNum)
}

// readBlockAfterNewline consumes a newline and checks for a raw block after it.
func (p *Parser) readBlockAfterNewline() string {
	p.lexer.NextToken() // consume newline
	if p.lexer.PeekToken().Type == TokenRawBlock {
		return p.lexer.NextToken().Value
	}
	return ""
}

// readSingleLineValue reads a single-line value using raw extraction.
func (p *Parser) readSingleLineValue(lineNum int) string {
	raw := p.lexer.RawValueText(lineNum)
	p.consumeUntilNewline()
	return unquoteRaw(raw)
}

// consumeUntilNewline consumes tokens until a newline or EOF is reached.
func (p *Parser) consumeUntilNewline() {
	for p.lexer.PeekToken().Type != TokenNewline && p.lexer.PeekToken().Type != TokenEOF {
		p.lexer.NextToken()
	}
}

// unquoteRaw unquotes a double-quoted string, handling basic escape sequences.
func unquoteRaw(raw string) string {
	if len(raw) < 2 {
		return raw
	}
	if isDoubleQuoted(raw) {
		return unquoteDouble(raw)
	}
	if isSingleQuoted(raw) {
		return raw[1 : len(raw)-1]
	}
	return raw
}

// isDoubleQuoted checks if a string is wrapped in double quotes.
func isDoubleQuoted(s string) bool {
	return len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"'
}

// isSingleQuoted checks if a string is wrapped in single quotes.
func isSingleQuoted(s string) bool {
	return len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\''
}

// unquoteDouble removes double quotes and processes escape sequences.
func unquoteDouble(raw string) string {
	unquoted := raw[1 : len(raw)-1]
	unquoted = strings.ReplaceAll(unquoted, `\"`, `"`)
	unquoted = strings.ReplaceAll(unquoted, `\\`, `\`)
	return unquoted
}
