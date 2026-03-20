package simulate

import (
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

// ParseCondition parses a raw condition string into a ConditionExpr AST.
// This is needed because the parser stores raw condition text but does not
// always populate the Parsed field.
//
// Supported grammar:
//
//	expr   = orExpr
//	orExpr = andExpr ("or" andExpr)*
//	andExpr = unary ("and" unary)*
//	unary  = "not" unary | compare
//	compare = VARIABLE OP VALUE
//	OP     = "=" | "==" | "!=" | "contains" | "startswith" | "endswith" | "in"
func ParseCondition(raw string) (ir.ConditionExpr, error) {
	p := &condParser{tokens: tokenizeCondition(raw), pos: 0}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.tokens) {
		return nil, fmt.Errorf("unexpected token %q at position %d", p.tokens[p.pos], p.pos)
	}
	return expr, nil
}

type condParser struct {
	tokens []string
	pos    int
}

func (p *condParser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

func (p *condParser) next() string {
	t := p.peek()
	p.pos++
	return t
}

func (p *condParser) parseOr() (ir.ConditionExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "or" {
		p.next() // consume "or"
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = ir.CondOr{Left: left, Right: right}
	}
	return left, nil
}

func (p *condParser) parseAnd() (ir.ConditionExpr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek() == "and" {
		p.next() // consume "and"
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = ir.CondAnd{Left: left, Right: right}
	}
	return left, nil
}

func (p *condParser) parseUnary() (ir.ConditionExpr, error) {
	if p.peek() == "not" {
		p.next() // consume "not"
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return ir.CondNot{Inner: inner}, nil
	}
	return p.parseCompare()
}

func (p *condParser) parseCompare() (ir.ConditionExpr, error) {
	variable := p.next()
	if variable == "" {
		return nil, fmt.Errorf("expected variable, got end of input")
	}

	op := p.next()
	if op == "" {
		return nil, fmt.Errorf("expected operator after %q, got end of input", variable)
	}

	// Validate operator.
	switch op {
	case "=", "==", "!=", "<", ">", "<=", ">=", "contains", "startswith", "endswith", "in":
		// valid
	default:
		return nil, fmt.Errorf("unknown operator %q", op)
	}

	value := p.next()
	if value == "" {
		return nil, fmt.Errorf("expected value after %q %s, got end of input", variable, op)
	}

	return ir.CondCompare{
		Variable: variable,
		Op:       op,
		Value:    value,
	}, nil
}

// tokenizeCondition splits a raw condition string into tokens.
// Handles quoted strings and standard whitespace splitting.
func tokenizeCondition(raw string) []string {
	var tokens []string
	raw = strings.TrimSpace(raw)
	i := 0
	for i < len(raw) {
		// Skip whitespace.
		for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t') {
			i++
		}
		if i >= len(raw) {
			break
		}

		// Quoted string.
		if raw[i] == '"' || raw[i] == '\'' {
			quote := raw[i]
			i++
			start := i
			for i < len(raw) && raw[i] != quote {
				i++
			}
			tokens = append(tokens, raw[start:i])
			if i < len(raw) {
				i++ // skip closing quote
			}
			continue
		}

		// Check for multi-char operators (before single-char).
		if i+1 < len(raw) {
			two := raw[i : i+2]
			if two == "==" || two == "!=" || two == "<=" || two == ">=" {
				tokens = append(tokens, two)
				i += 2
				continue
			}
		}
		// Single-char operators.
		if raw[i] == '=' || raw[i] == '<' || raw[i] == '>' || raw[i] == '!' {
			tokens = append(tokens, string(raw[i]))
			i++
			continue
		}

		// Regular token (identifier, keyword, value).
		start := i
		for i < len(raw) && raw[i] != ' ' && raw[i] != '\t' &&
			raw[i] != '=' && raw[i] != '!' && raw[i] != '<' && raw[i] != '>' {
			i++
		}
		if i > start {
			tokens = append(tokens, raw[start:i])
		}
	}
	return tokens
}

// EnsureConditionsParsed walks all edges in a workflow and ensures that any
// Condition with a Raw string but nil Parsed field gets parsed. This is needed
// because the .dip parser may not always populate the Parsed AST.
func EnsureConditionsParsed(w *ir.Workflow) error {
	for _, e := range w.Edges {
		if e.Condition != nil && e.Condition.Parsed == nil && e.Condition.Raw != "" {
			parsed, err := ParseCondition(e.Condition.Raw)
			if err != nil {
				return fmt.Errorf("edge %s -> %s: invalid condition %q: %w", e.From, e.To, e.Condition.Raw, err)
			}
			e.Condition.Parsed = parsed
		}
	}
	return nil
}
