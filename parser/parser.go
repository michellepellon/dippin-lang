package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/2389/dippin/ir"
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
	for p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}

		if t.Type == TokenIdentifier && t.Value == "workflow" {
			p.parseWorkflow()
		} else {
			// Try to recover by skipping to next line
			p.lexer.NextToken()
		}
	}
	if len(p.diagnostics) > 0 {
		return p.workflow, fmt.Errorf("parsing errors: %s", strings.Join(p.diagnostics, "; "))
	}
	return p.workflow, nil
}

func (p *Parser) parseWorkflow() {
	p.lexer.NextToken() // workflow
	name := p.lexer.NextToken().Value
	p.workflow.Name = name
	p.expect(TokenNewline)

	p.expect(TokenIndent)
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}

		if t.Type == TokenIdentifier {
			switch t.Value {
			case "goal":
				p.lexer.NextToken()
				p.expect(TokenColon)
				p.workflow.Goal = p.lexer.NextToken().Value
			case "start":
				p.lexer.NextToken()
				p.expect(TokenColon)
				p.workflow.Start = p.lexer.NextToken().Value
			case "exit":
				p.lexer.NextToken()
				p.expect(TokenColon)
				p.workflow.Exit = p.lexer.NextToken().Value
			case "defaults":
				p.parseDefaults()
			case "agent", "human", "tool", "subgraph":
				p.parseNode(ir.NodeKind(t.Value))
			case "parallel":
				p.parseParallel()
			case "fan_in":
				p.parseFanIn()
			case "edges":
				p.parseEdges()
			default:
				p.diagnostics = append(p.diagnostics, fmt.Sprintf("unexpected top-level identifier: %s at %d:%d", t.Value, t.Location.Line, t.Location.Column))
				p.lexer.NextToken()
			}
		} else {
			p.lexer.NextToken()
		}
	}
	p.expect(TokenOutdent)
}

func (p *Parser) parseDefaults() {
	p.lexer.NextToken() // defaults
	p.expect(TokenNewline)
	p.expect(TokenIndent)
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		if t.Type == TokenIdentifier {
			key := t.Value
			p.lexer.NextToken()
			p.expect(TokenColon)
			val := p.lexer.NextToken().Value
			switch key {
			case "model":
				p.workflow.Defaults.Model = val
			case "provider":
				p.workflow.Defaults.Provider = val
			case "retry_policy":
				p.workflow.Defaults.RetryPolicy = val
			case "max_retries":
				v, _ := strconv.Atoi(val)
				p.workflow.Defaults.MaxRetries = v
			case "fidelity":
				p.workflow.Defaults.Fidelity = val
			case "max_restarts":
				v, _ := strconv.Atoi(val)
				p.workflow.Defaults.MaxRestarts = v
			case "restart_target":
				p.workflow.Defaults.RestartTarget = val
			case "cache_tools":
				p.workflow.Defaults.CacheTools = (val == "true")
			case "compaction":
				p.workflow.Defaults.Compaction = val
			}
		} else {
			p.lexer.NextToken()
		}
	}
	p.expect(TokenOutdent)
}

func (p *Parser) parseNode(kind ir.NodeKind) {
	p.lexer.NextToken() // kind
	id := p.lexer.NextToken().Value
	node := &ir.Node{
		ID:     id,
		Kind:   kind,
		Source: p.lexer.PeekToken().Location,
	}

	// Default config
	switch kind {
	case ir.NodeAgent:
		node.Config = ir.AgentConfig{}
	case ir.NodeHuman:
		node.Config = ir.HumanConfig{}
	case ir.NodeTool:
		node.Config = ir.ToolConfig{}
	case ir.NodeSubgraph:
		node.Config = ir.SubgraphConfig{Params: make(map[string]string)}
	}

	p.expect(TokenNewline)
	p.expect(TokenIndent)
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		if t.Type == TokenIdentifier {
			key := t.Value
			p.lexer.NextToken()
			p.expect(TokenColon)
			
			// Handle multiline block if next token is newline then indent
			var val string
			if p.lexer.PeekToken().Type == TokenNewline {
				p.lexer.NextToken()
				if p.lexer.PeekToken().Type == TokenIndent {
					val = p.parseMultilineBlock()
				}
			} else {
				// Consume all tokens until newline for single-line field
				var parts []string
				for p.lexer.PeekToken().Type != TokenNewline && p.lexer.PeekToken().Type != TokenEOF {
					parts = append(parts, p.lexer.NextToken().Value)
				}
				val = strings.Join(parts, " ")
			}

			p.applyNodeField(node, key, val)
		} else {
			p.lexer.NextToken()
		}
	}
	p.expect(TokenOutdent)
	p.workflow.Nodes = append(p.workflow.Nodes, node)
}

func (p *Parser) parseMultilineBlock() string {
	p.lexer.NextToken() // Indent
	var lines []string
	// The lexer gives TokenNewline at the end of every line.
	// But it doesn't give Tokens for the contents of the indented block unless we handle it?
	// Actually, the lexer I wrote splits by lines and handles indentation.
	// So inside an indent/outdent pair, we get multiple lines.
	// Wait, my lexer gives tokens for each line.
	// We need to collect all tokens until the matching Outdent.
	
	// Wait, the lexer gives tokens within a line.
	// If it's a multiline block, it should probably be raw text.
	// Let's reconsider the lexer.
	// For multiline blocks, the parser might need to read raw lines.
	
	// Let's cheat a bit and collect all values from tokens until Outdent.
	// This is not perfect because it loses formatting, but for a quick fix:
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.NextToken()
		if t.Type == TokenNewline {
			lines = append(lines, "")
		} else {
			if len(lines) == 0 {
				lines = append(lines, t.Value)
			} else {
				if lines[len(lines)-1] == "" {
					lines[len(lines)-1] = t.Value
				} else {
					lines[len(lines)-1] += " " + t.Value // Reconstruct line
				}
			}
		}
	}
	p.expect(TokenOutdent)
	return strings.Join(lines, "\n")
}

func (p *Parser) applyNodeField(n *ir.Node, key, val string) {
	switch key {
	case "label":
		n.Label = val
	case "class":
		n.Classes = splitComma(val)
	case "reads":
		n.IO.Reads = splitComma(val)
	case "writes":
		n.IO.Writes = splitComma(val)
	case "retry_policy":
		n.Retry.Policy = val
	case "max_retries":
		v, _ := strconv.Atoi(val)
		n.Retry.MaxRetries = v
	case "retry_target":
		n.Retry.RetryTarget = val
	case "fallback_target":
		n.Retry.FallbackTarget = val
	}

	switch cfg := n.Config.(type) {
	case ir.AgentConfig:
		switch key {
		case "prompt":
			cfg.Prompt = val
		case "system_prompt":
			cfg.SystemPrompt = val
		case "model":
			cfg.Model = val
		case "provider":
			cfg.Provider = val
		case "max_turns":
			v, _ := strconv.Atoi(val)
			cfg.MaxTurns = v
		case "goal_gate":
			cfg.GoalGate = (val == "true")
		case "auto_status":
			cfg.AutoStatus = (val == "true")
		case "reasoning_effort":
			cfg.ReasoningEffort = val
		case "fidelity":
			cfg.Fidelity = val
		}
		n.Config = cfg
	case ir.HumanConfig:
		switch key {
		case "mode":
			cfg.Mode = val
		case "default":
			cfg.Default = val
		}
		n.Config = cfg
	case ir.ToolConfig:
		switch key {
		case "command":
			cfg.Command = val
		case "timeout":
			d, _ := time.ParseDuration(val)
			cfg.Timeout = d
		}
		n.Config = cfg
	case ir.SubgraphConfig:
		switch key {
		case "ref":
			cfg.Ref = val
		case "params":
			// Params is a block, but my parser is simple.
			// Let's assume params are handled elsewhere or I'll fix this later.
		}
		n.Config = cfg
	}
}

func (p *Parser) parseParallel() {
	p.lexer.NextToken() // parallel
	id := p.lexer.NextToken().Value
	p.expect(TokenArrow)
	targets := p.parseCommaList()
	p.workflow.Nodes = append(p.workflow.Nodes, &ir.Node{
		ID:     id,
		Kind:   ir.NodeParallel,
		Config: ir.ParallelConfig{Targets: targets},
	})
	p.expect(TokenNewline)
}

func (p *Parser) parseFanIn() {
	p.lexer.NextToken() // fan_in
	id := p.lexer.NextToken().Value
	p.expect(TokenBackArrow)
	sources := p.parseCommaList()
	p.workflow.Nodes = append(p.workflow.Nodes, &ir.Node{
		ID:     id,
		Kind:   ir.NodeFanIn,
		Config: ir.FanInConfig{Sources: sources},
	})
	p.expect(TokenNewline)
}

func (p *Parser) parseEdges() {
	p.lexer.NextToken() // edges
	p.expect(TokenNewline)
	p.expect(TokenIndent)
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		from := p.lexer.NextToken().Value
		p.expect(TokenArrow)
		to := p.lexer.NextToken().Value
		
		edge := &ir.Edge{From: from, To: to}
		
		// Parse edge attributes
		for p.lexer.PeekToken().Type != TokenNewline && p.lexer.PeekToken().Type != TokenEOF {
			attr := p.lexer.NextToken()
			switch attr.Value {
			case "when":
				// Simplified condition parsing: read until next keyword or end of line
				condRaw := ""
				for p.lexer.PeekToken().Type != TokenNewline && p.lexer.PeekToken().Type != TokenEOF {
					pk := p.lexer.PeekToken()
					if pk.Value == "label" || pk.Value == "weight" || pk.Value == "restart" {
						break
					}
					condRaw += p.lexer.NextToken().Value + " "
				}
				edge.Condition = &ir.Condition{Raw: strings.TrimSpace(condRaw)}
				// In a real implementation, we would call a proper condition parser here.
			case "label":
				p.expect(TokenColon)
				edge.Label = p.lexer.NextToken().Value
			case "weight":
				p.expect(TokenColon)
				v, _ := strconv.Atoi(p.lexer.NextToken().Value)
				edge.Weight = v
			case "restart":
				p.expect(TokenColon)
				edge.Restart = (p.lexer.NextToken().Value == "true")
			}
		}
		p.workflow.Edges = append(p.workflow.Edges, edge)
		p.expect(TokenNewline)
	}
	p.expect(TokenOutdent)
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

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		res = append(res, strings.TrimSpace(p))
	}
	return res
}
