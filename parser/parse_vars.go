package parser

import (
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

func (p *Parser) parseVars() {
	p.lexer.NextToken() // vars
	p.expect(TokenNewline)
	p.expect(TokenIndent)
	p.parseVarsBody()
	p.expect(TokenOutdent)
}

func (p *Parser) parseVarsBody() {
	p.ensureVarsMap()
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		if t.Type == TokenIdentifier {
			p.parseVarField(t)
		} else {
			p.lexer.NextToken()
		}
	}
}

func (p *Parser) ensureVarsMap() {
	if p.workflow.Vars == nil {
		p.workflow.Vars = make(map[string]string)
	}
}

func (p *Parser) parseVarField(t Token) {
	key := t.Value
	p.lexer.NextToken()
	p.expect(TokenColon)
	val := p.readFieldValue(t.Location.Line)
	p.applyVarField(key, val, t.Location)
}

func (p *Parser) applyVarField(key, val string, loc ir.SourceLocation) {
	if _, exists := p.workflow.Vars[key]; exists {
		p.diagnostics = append(p.diagnostics,
			fmt.Sprintf("duplicate vars key %q at %d:%d", key, loc.Line, loc.Column))
	}
	p.workflow.Vars[key] = val
}
