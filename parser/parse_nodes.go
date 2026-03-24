package parser

import (
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// defaultNodeConfig returns the zero config for a given node kind.
func defaultNodeConfig(kind ir.NodeKind) ir.NodeConfig {
	switch kind {
	case ir.NodeAgent:
		return ir.AgentConfig{}
	case ir.NodeHuman:
		return ir.HumanConfig{}
	case ir.NodeTool:
		return ir.ToolConfig{}
	case ir.NodeSubgraph:
		return ir.SubgraphConfig{Params: make(map[string]string)}
	default:
		return ir.AgentConfig{}
	}
}

func (p *Parser) parseNode(kind ir.NodeKind) {
	p.lexer.NextToken() // kind
	id := p.lexer.NextToken().Value
	node := &ir.Node{
		ID:     id,
		Kind:   kind,
		Source: p.lexer.PeekToken().Location,
		Config: defaultNodeConfig(kind),
	}

	p.expect(TokenNewline)
	p.expect(TokenIndent)
	p.parseNodeBody(node)
	p.expect(TokenOutdent)
	p.workflow.Nodes = append(p.workflow.Nodes, node)
}

// parseNodeBody parses the indented fields within a node declaration.
func (p *Parser) parseNodeBody(node *ir.Node) {
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
			val := p.readFieldValue(t.Location.Line)
			p.applyNodeField(node, key, val, t.Location)
		} else {
			p.lexer.NextToken()
		}
	}
}

func (p *Parser) applyNodeField(n *ir.Node, key, val string, loc ir.SourceLocation) {
	if p.tryApplyCommonField(n, key, val, loc) {
		return
	}
	p.applyConfigField(n, key, val, loc)
}

// applyConfigField dispatches to config-specific field handlers.
func (p *Parser) applyConfigField(n *ir.Node, key, val string, loc ir.SourceLocation) {
	switch cfg := n.Config.(type) {
	case ir.AgentConfig:
		p.applyAgentField(&cfg, key, val, loc)
		n.Config = cfg
	case ir.HumanConfig:
		p.applyHumanField(&cfg, key, val, loc)
		n.Config = cfg
	case ir.ToolConfig:
		p.applyToolField(&cfg, key, val, loc)
		n.Config = cfg
	case ir.SubgraphConfig:
		p.applySubgraphField(&cfg, key, val, loc)
		n.Config = cfg
	}
}

// tryApplyCommonField applies fields that are common to all node types.
// Returns true if the field was handled, false otherwise.
func (p *Parser) tryApplyCommonField(n *ir.Node, key, val string, loc ir.SourceLocation) bool {
	if applyCommonStringField(n, key, val) {
		return true
	}
	return p.applyCommonComplexField(n, key, val, loc)
}

// applyCommonStringField handles simple string/slice assignments for common fields.
func applyCommonStringField(n *ir.Node, key, val string) bool {
	if applyCommonPlainField(n, key, val) {
		return true
	}
	return applyCommonRetryField(n, key, val)
}

// applyCommonPlainField handles label, class, reads, writes.
func applyCommonPlainField(n *ir.Node, key, val string) bool {
	switch key {
	case "label":
		n.Label = val
	case "class":
		n.Classes = splitComma(val)
	case "reads":
		n.IO.Reads = splitComma(val)
	case "writes":
		n.IO.Writes = splitComma(val)
	default:
		return false
	}
	return true
}

// applyCommonRetryField handles retry-related string fields.
func applyCommonRetryField(n *ir.Node, key, val string) bool {
	switch key {
	case "retry_policy":
		n.Retry.Policy = val
	case "retry_target":
		n.Retry.RetryTarget = val
	case "fallback_target":
		n.Retry.FallbackTarget = val
	default:
		return false
	}
	return true
}

// applyCommonComplexField handles fields that need parsing (int, duration).
func (p *Parser) applyCommonComplexField(n *ir.Node, key, val string, loc ir.SourceLocation) bool {
	switch key {
	case "max_retries":
		n.Retry.MaxRetries = p.parseInt(val, key, loc)
	case "base_delay":
		n.Retry.BaseDelay = p.parseDuration(val, key, loc)
	default:
		return false
	}
	return true
}

// applyAgentField applies agent-specific configuration fields.
func (p *Parser) applyAgentField(cfg *ir.AgentConfig, key, val string, loc ir.SourceLocation) {
	if applyAgentStringField(cfg, key, val) {
		return
	}
	p.applyAgentComplexField(cfg, key, val, loc)
}

// applyAgentStringField handles simple string assignments for agent config.
func applyAgentStringField(cfg *ir.AgentConfig, key, val string) bool {
	if applyAgentPromptField(cfg, key, val) {
		return true
	}
	return applyAgentModelField(cfg, key, val)
}

// applyAgentPromptField handles prompt-related agent fields.
func applyAgentPromptField(cfg *ir.AgentConfig, key, val string) bool {
	switch key {
	case "prompt":
		cfg.Prompt = val
	case "system_prompt":
		cfg.SystemPrompt = val
	case "reasoning_effort":
		cfg.ReasoningEffort = val
	default:
		return false
	}
	return true
}

// applyAgentModelField handles model-related agent fields.
func applyAgentModelField(cfg *ir.AgentConfig, key, val string) bool {
	switch key {
	case "model":
		cfg.Model = val
	case "provider":
		cfg.Provider = val
	case "fidelity":
		cfg.Fidelity = val
	default:
		return false
	}
	return true
}

// applyAgentComplexField handles fields needing parsing for agent config.
func (p *Parser) applyAgentComplexField(cfg *ir.AgentConfig, key, val string, loc ir.SourceLocation) {
	if applyAgentBoolField(cfg, key, val) {
		return
	}
	p.applyAgentParsedField(cfg, key, val, loc)
}

// applyAgentBoolField handles boolean and string agent fields.
func applyAgentBoolField(cfg *ir.AgentConfig, key, val string) bool {
	switch key {
	case "goal_gate":
		cfg.GoalGate = (val == "true")
	case "auto_status":
		cfg.AutoStatus = (val == "true")
	case "cache_tools":
		cfg.CacheTools = (val == "true")
	case "compaction":
		cfg.Compaction = val
	default:
		return false
	}
	return true
}

// applyAgentParsedField handles agent fields that require parsing.
func (p *Parser) applyAgentParsedField(cfg *ir.AgentConfig, key, val string, loc ir.SourceLocation) {
	switch key {
	case "max_turns":
		cfg.MaxTurns = p.parseInt(val, key, loc)
	case "compaction_threshold":
		cfg.CompactionThreshold = p.parseFloat(val, key, loc)
	}
}

// applyHumanField applies human-specific configuration fields.
func (p *Parser) applyHumanField(cfg *ir.HumanConfig, key, val string, loc ir.SourceLocation) {
	switch key {
	case "mode":
		cfg.Mode = val
	case "default":
		cfg.Default = val
	}
}

// applyToolField applies tool-specific configuration fields.
func (p *Parser) applyToolField(cfg *ir.ToolConfig, key, val string, loc ir.SourceLocation) {
	switch key {
	case "command":
		cfg.Command = val
	case "timeout":
		cfg.Timeout = p.parseDuration(val, key, loc)
	case "outputs":
		cfg.Outputs = splitComma(val)
	}
}

// applySubgraphField applies subgraph-specific configuration fields.
func (p *Parser) applySubgraphField(cfg *ir.SubgraphConfig, key, val string, loc ir.SourceLocation) {
	switch key {
	case "ref":
		cfg.Ref = val
	case "params":
		cfg.Params = parseParamsBlock(val)
	}
}

// parseParamsBlock parses a raw block of key: value lines into a map.
func parseParamsBlock(raw string) map[string]string {
	params := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v := splitKeyValue(line)
		if k != "" {
			params[k] = v
		}
	}
	return params
}

func (p *Parser) parseParallel() {
	p.lexer.NextToken() // parallel
	id := p.lexer.NextToken().Value

	if p.lexer.PeekToken().Type == TokenArrow {
		p.parseParallelInline(id)
		return
	}
	p.parseParallelBlock(id)
}

// parseParallelInline handles: parallel ID -> target, target
func (p *Parser) parseParallelInline(id string) {
	p.expect(TokenArrow)
	targets := p.parseCommaList()
	p.workflow.Nodes = append(p.workflow.Nodes, &ir.Node{
		ID:     id,
		Kind:   ir.NodeParallel,
		Config: ir.ParallelConfig{Targets: targets},
	})
	p.expect(TokenNewline)
}

// parseParallelBlock handles block form with per-branch config.
func (p *Parser) parseParallelBlock(id string) {
	p.expect(TokenNewline)
	p.expect(TokenIndent)
	branches := p.parseParallelBranches()
	p.expect(TokenOutdent)

	targets := branchTargets(branches)
	p.workflow.Nodes = append(p.workflow.Nodes, &ir.Node{
		ID:   id,
		Kind: ir.NodeParallel,
		Config: ir.ParallelConfig{
			Targets:  targets,
			Branches: branches,
		},
	})
}

// parseParallelBranches parses branch declarations inside a parallel block.
func (p *Parser) parseParallelBranches() []ir.BranchConfig {
	var branches []ir.BranchConfig
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if b, ok := p.tryParseBranch(t); ok {
			branches = append(branches, b)
		}
	}
	return branches
}

// tryParseBranch tries to parse a branch or skip a non-branch token.
func (p *Parser) tryParseBranch(t Token) (ir.BranchConfig, bool) {
	if t.Type == TokenNewline {
		p.lexer.NextToken()
		return ir.BranchConfig{}, false
	}
	if t.Type == TokenIdentifier && t.Value == "branch" {
		return p.parseOneBranch(), true
	}
	p.lexer.NextToken()
	return ir.BranchConfig{}, false
}

// parseOneBranch parses: branch: target\n  model: ...\n  provider: ...
func (p *Parser) parseOneBranch() ir.BranchConfig {
	p.lexer.NextToken() // "branch"
	p.expect(TokenColon)
	target := p.lexer.NextToken().Value
	bc := ir.BranchConfig{Target: target}
	p.consumeUntilNewline()

	if p.lexer.PeekToken().Type == TokenNewline {
		p.lexer.NextToken()
	}
	if p.lexer.PeekToken().Type != TokenIndent {
		return bc
	}
	p.expect(TokenIndent)
	p.parseBranchFields(&bc)
	p.expect(TokenOutdent)
	return bc
}

// parseBranchFields parses fields within a branch block.
func (p *Parser) parseBranchFields(bc *ir.BranchConfig) {
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
			val := p.readFieldValue(t.Location.Line)
			applyBranchField(bc, key, val)
		} else {
			p.lexer.NextToken()
		}
	}
}

// applyBranchField sets a field on a BranchConfig.
func applyBranchField(bc *ir.BranchConfig, key, val string) {
	switch key {
	case "model":
		bc.Model = val
	case "provider":
		bc.Provider = val
	case "fidelity":
		bc.Fidelity = val
	}
}

// branchTargets extracts target IDs from branch configs.
func branchTargets(branches []ir.BranchConfig) []string {
	targets := make([]string, len(branches))
	for i, b := range branches {
		targets[i] = b.Target
	}
	return targets
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
