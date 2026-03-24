package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/2389-research/dippin-lang/ir"
)

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		res = append(res, strings.TrimSpace(p))
	}
	return res
}

// splitKeyValue splits "key: value" into (key, value).
func splitKeyValue(line string) (string, string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", ""
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
}

func (p *Parser) parseInt(val string, key string, loc ir.SourceLocation) int {
	v, err := strconv.Atoi(val)
	if err != nil {
		p.diagnostics = append(p.diagnostics, fmt.Sprintf("invalid integer %q for %s at %d:%d", val, key, loc.Line, loc.Column))
	}
	return v
}

func (p *Parser) parseFloat(val string, key string, loc ir.SourceLocation) float64 {
	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		p.diagnostics = append(p.diagnostics, fmt.Sprintf("invalid float %q for %s at %d:%d", val, key, loc.Line, loc.Column))
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
