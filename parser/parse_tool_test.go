package parser

import (
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// parseToolFixture parses a minimal workflow with a single tool node and returns
// the parsed ToolConfig plus any diagnostics. Tests must parse real .dip text —
// hand-building ir.ToolConfig bypasses the parser and masks bugs (per CLAUDE.md).
func parseToolFixture(t *testing.T, body string) (ir.ToolConfig, []string) {
	t.Helper()
	src := "workflow W\n  start: T\n  exit: T\n\n  tool T\n" + body + "\n\n  edges\n    T -> T\n"
	p := NewParser(src, "test.dip")
	wf, _ := p.Parse()
	if len(wf.Nodes) == 0 {
		t.Fatalf("no nodes parsed; diagnostics: %v", p.Diagnostics())
	}
	cfg, ok := wf.Nodes[0].Config.(ir.ToolConfig)
	if !ok {
		t.Fatalf("node 0 is not a tool: %T", wf.Nodes[0].Config)
	}
	return cfg, p.Diagnostics()
}

func TestParseToolMarkerGrep(t *testing.T) {
	cfg, diags := parseToolFixture(t, `    marker_grep: "^(green|red)$"`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if cfg.MarkerGrep != "^(green|red)$" {
		t.Errorf("MarkerGrep = %q, want %q", cfg.MarkerGrep, "^(green|red)$")
	}
}

func TestParseToolMarkerGrepUnquoted(t *testing.T) {
	cfg, diags := parseToolFixture(t, "    marker_grep: tests_pass")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if cfg.MarkerGrep != "tests_pass" {
		t.Errorf("MarkerGrep = %q, want %q", cfg.MarkerGrep, "tests_pass")
	}
}

func TestParseToolMarkerGrepRegexMetachars(t *testing.T) {
	// Stored verbatim — dippin does not validate the regex (tracker does).
	cfg, diags := parseToolFixture(t, `    marker_grep: "^\[(PASS|FAIL)\]\s+\d+$"`)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !strings.Contains(cfg.MarkerGrep, `\[`) {
		t.Errorf("MarkerGrep lost metachars: %q", cfg.MarkerGrep)
	}
}
