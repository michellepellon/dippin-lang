package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
	"agent": true, "human": true, "tool": true, "subgraph": true,
}

// workflowSimpleBlocks maps workflow block keywords to their parser methods.
// Populated lazily to avoid init-order issues; see dispatchWorkflowBlock.

// dispatchWorkflowField routes a workflow-level identifier to the right handler.
func (p *Parser) dispatchWorkflowField(t Token) {
	switch t.Value {
	case "goal", "start", "exit":
		p.parseWorkflowStringField(t)
	case "defaults":
		p.parseDefaults()
	case "edges":
		p.parseEdges()
	default:
		p.dispatchWorkflowBlock(t)
	}
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

func (p *Parser) parseDefaults() {
	p.lexer.NextToken() // defaults
	p.expect(TokenNewline)
	p.expect(TokenIndent)
	p.parseDefaultsBody()
	p.expect(TokenOutdent)
}

// parseDefaultsBody parses the indented body of a defaults block.
func (p *Parser) parseDefaultsBody() {
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		if t.Type == TokenIdentifier {
			p.parseDefaultField(t)
		} else {
			p.lexer.NextToken()
		}
	}
}

// parseDefaultField reads a single default field (key: value) and applies it.
func (p *Parser) parseDefaultField(t Token) {
	key := t.Value
	p.lexer.NextToken()
	p.expect(TokenColon)
	val := p.readFieldValue(t.Location.Line)
	p.applyDefaultField(key, val, t.Location)
}

// applyDefaultField applies a single default field value to the workflow defaults.
func (p *Parser) applyDefaultField(key, val string, loc ir.SourceLocation) {
	if p.applyDefaultStringField(key, val) {
		return
	}
	p.applyDefaultComplexField(key, val, loc)
}

// applyDefaultStringField handles simple string assignments for defaults.
func (p *Parser) applyDefaultStringField(key, val string) bool {
	if applyDefaultCoreField(&p.workflow.Defaults, key, val) {
		return true
	}
	return applyDefaultExtraField(&p.workflow.Defaults, key, val)
}

// applyDefaultCoreField handles model, provider, retry_policy defaults.
func applyDefaultCoreField(d *ir.WorkflowDefaults, key, val string) bool {
	switch key {
	case "model":
		d.Model = val
	case "provider":
		d.Provider = val
	case "retry_policy":
		d.RetryPolicy = val
	default:
		return false
	}
	return true
}

// applyDefaultExtraField handles fidelity, restart_target, compaction defaults.
func applyDefaultExtraField(d *ir.WorkflowDefaults, key, val string) bool {
	switch key {
	case "fidelity":
		d.Fidelity = val
	case "restart_target":
		d.RestartTarget = val
	case "compaction":
		d.Compaction = val
	default:
		return false
	}
	return true
}

// applyDefaultComplexField handles fields needing parsing for defaults.
func (p *Parser) applyDefaultComplexField(key, val string, loc ir.SourceLocation) {
	switch key {
	case "max_retries":
		p.workflow.Defaults.MaxRetries = p.parseInt(val, key, loc)
	case "max_restarts":
		p.workflow.Defaults.MaxRestarts = p.parseInt(val, key, loc)
	case "cache_tools":
		p.workflow.Defaults.CacheTools = (val == "true")
	}
}

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
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return raw
	}
	unquoted := raw[1 : len(raw)-1]
	unquoted = strings.ReplaceAll(unquoted, `\"`, `"`)
	unquoted = strings.ReplaceAll(unquoted, `\\`, `\`)
	return unquoted
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
	switch key {
	case "max_turns":
		cfg.MaxTurns = p.parseInt(val, key, loc)
	case "goal_gate":
		cfg.GoalGate = (val == "true")
	case "auto_status":
		cfg.AutoStatus = (val == "true")
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
	}
}

// applySubgraphField applies subgraph-specific configuration fields.
func (p *Parser) applySubgraphField(cfg *ir.SubgraphConfig, key, val string, loc ir.SourceLocation) {
	switch key {
	case "ref":
		cfg.Ref = val
	case "params":
		// Params is a block, but my parser is simple.
		// Let's assume params are handled elsewhere or I'll fix this later.
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
	p.parseEdgesBody()
	p.expect(TokenOutdent)
}

// parseEdgesBody parses the indented body of an edges block.
func (p *Parser) parseEdgesBody() {
	for p.lexer.PeekToken().Type != TokenOutdent && p.lexer.PeekToken().Type != TokenEOF {
		t := p.lexer.PeekToken()
		if t.Type == TokenNewline {
			p.lexer.NextToken()
			continue
		}
		p.parseSingleEdge()
	}
}

// parseSingleEdge parses a single edge declaration: "from -> to [attributes...]"
func (p *Parser) parseSingleEdge() {
	from := p.lexer.NextToken().Value
	p.expect(TokenArrow)
	to := p.lexer.NextToken().Value
	edge := &ir.Edge{From: from, To: to}
	p.parseEdgeAttributes(edge)
	p.workflow.Edges = append(p.workflow.Edges, edge)
	p.expect(TokenNewline)
}

// parseEdgeAttributes parses optional attributes (when, label, weight, restart) on an edge.
func (p *Parser) parseEdgeAttributes(edge *ir.Edge) {
	for p.lexer.PeekToken().Type != TokenNewline && p.lexer.PeekToken().Type != TokenEOF {
		attr := p.lexer.NextToken()
		p.applyEdgeAttribute(edge, attr.Value)
	}
}

// edgeAttrKeywords contains the set of edge attribute keywords that terminate condition parsing.
var edgeAttrKeywords = map[string]bool{
	"label": true, "weight": true, "restart": true,
}

// applyEdgeAttribute applies a single edge attribute.
func (p *Parser) applyEdgeAttribute(edge *ir.Edge, attrName string) {
	switch attrName {
	case "when":
		edge.Condition = &ir.Condition{Raw: p.readConditionRaw()}
	case "label":
		p.expect(TokenColon)
		edge.Label = p.lexer.NextToken().Value
	case "weight":
		p.expect(TokenColon)
		wt := p.lexer.NextToken()
		edge.Weight = p.parseInt(wt.Value, "weight", wt.Location)
	case "restart":
		p.expect(TokenColon)
		edge.Restart = (p.lexer.NextToken().Value == "true")
	}
}

// readConditionRaw reads tokens until a newline/EOF or a known edge attribute keyword.
func (p *Parser) readConditionRaw() string {
	var parts []string
	for p.lexer.PeekToken().Type != TokenNewline && p.lexer.PeekToken().Type != TokenEOF {
		pk := p.lexer.PeekToken()
		if edgeAttrKeywords[pk.Value] {
			break
		}
		t := p.lexer.NextToken()
		parts = append(parts, formatConditionToken(t))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// formatConditionToken formats a single token for raw condition text.
func formatConditionToken(t Token) string {
	if t.Type == TokenLiteral {
		return "\"" + t.Value + "\""
	}
	return t.Value
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

func (p *Parser) parseInt(val string, key string, loc ir.SourceLocation) int {
	v, err := strconv.Atoi(val)
	if err != nil {
		p.diagnostics = append(p.diagnostics, fmt.Sprintf("invalid integer %q for %s at %d:%d", val, key, loc.Line, loc.Column))
	}
	return v
}

func (p *Parser) parseDuration(val string, key string, loc ir.SourceLocation) time.Duration {
	d, err := time.ParseDuration(val)
	if err != nil {
		p.diagnostics = append(p.diagnostics, fmt.Sprintf("invalid duration %q for %s at %d:%d (use e.g. 30s, 5m, 1h)", val, key, loc.Line, loc.Column))
	}
	return d
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		res = append(res, strings.TrimSpace(p))
	}
	return res
}
