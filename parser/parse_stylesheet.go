package parser

import (
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// parseStylesheet parses the stylesheet: raw block and converts it to rules.
func (p *Parser) parseStylesheet() {
	p.lexer.NextToken() // "stylesheet"
	p.expect(TokenColon)
	val := p.readFieldValue(p.lexer.PeekToken().Location.Line)
	p.workflow.Stylesheet = parseStylesheetRaw(val)
}

// parseStylesheetRaw parses a raw block of stylesheet rules.
func parseStylesheetRaw(raw string) []ir.StylesheetRule {
	lines := strings.Split(raw, "\n")
	return collectRules(lines)
}

// collectRules iterates over lines to build stylesheet rules.
func collectRules(lines []string) []ir.StylesheetRule {
	var rules []ir.StylesheetRule
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			i++
			continue
		}
		indent := lineIndent(line)
		if indent == 0 {
			rule, newI := collectOneRule(trimmed, lines, i+1)
			rules = append(rules, rule)
			i = newI
		} else {
			i++
		}
	}
	return rules
}

// collectOneRule collects a selector and its indented properties.
func collectOneRule(selector string, lines []string, start int) (ir.StylesheetRule, int) {
	rule := ir.StylesheetRule{
		Selector:   parseSelector(selector),
		Properties: make(map[string]string),
	}
	i := start
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			i++
			continue
		}
		if lineIndent(line) == 0 {
			break
		}
		k, v := splitKeyValue(trimmed)
		if k != "" {
			rule.Properties[k] = v
		}
		i++
	}
	return rule, i
}

// parseSelector converts a selector string to a StyleSelector.
func parseSelector(s string) ir.StyleSelector {
	if s == "*" {
		return ir.StyleSelector{Kind: "universal", Value: "*"}
	}
	if strings.HasPrefix(s, ".") {
		return ir.StyleSelector{Kind: "class", Value: s[1:]}
	}
	if strings.HasPrefix(s, "#") {
		return ir.StyleSelector{Kind: "id", Value: s[1:]}
	}
	return ir.StyleSelector{Kind: "kind", Value: s}
}
