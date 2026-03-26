package validator

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestLintToolSyntax_Valid(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "set -eu\necho hello",
			}},
		},
	}
	diags := lintToolSyntax(w)
	if len(diags) != 0 {
		t.Errorf("expected no DIP123, got %d: %v", len(diags), diags)
	}
}

func TestLintToolSyntax_Error(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "echo \"unclosed",
			}},
		},
	}
	diags := lintToolSyntax(w)
	if len(diags) != 1 {
		t.Fatalf("expected 1 DIP123, got %d", len(diags))
	}
	if diags[0].Code != DIP123 {
		t.Errorf("expected DIP123, got %s", diags[0].Code)
	}
}

func TestLintToolSyntax_EmptyCommand(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: ""}},
		},
	}
	diags := lintToolSyntax(w)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty command, got %d", len(diags))
	}
}

func TestLintToolCtxVars_Found(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "curl ${ctx.api_url}/endpoint",
			}},
		},
	}
	diags := lintToolCtxVars(w)
	if len(diags) != 1 {
		t.Fatalf("expected 1 DIP124, got %d", len(diags))
	}
	if diags[0].Code != DIP124 {
		t.Errorf("expected DIP124, got %s", diags[0].Code)
	}
}

func TestLintToolCtxVars_Multiple(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "echo ${ctx.a} ${ctx.b}",
			}},
		},
	}
	diags := lintToolCtxVars(w)
	if len(diags) != 2 {
		t.Errorf("expected 2 DIP124, got %d", len(diags))
	}
}

func TestLintToolCtxVars_None(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "echo hello $HOME",
			}},
		},
	}
	diags := lintToolCtxVars(w)
	if len(diags) != 0 {
		t.Errorf("expected no DIP124, got %d", len(diags))
	}
}

func TestLintToolBinary_Found(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "set -eu\nls -la",
			}},
		},
	}
	diags := lintToolBinary(w)
	if len(diags) != 0 {
		t.Errorf("expected no DIP125 for ls, got %d", len(diags))
	}
}

func TestLintToolBinary_NotFound(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "set -eu\nnonexistent_binary_xyz --flag",
			}},
		},
	}
	diags := lintToolBinary(w)
	if len(diags) != 1 {
		t.Fatalf("expected 1 DIP125, got %d", len(diags))
	}
	if diags[0].Code != DIP125 {
		t.Errorf("expected DIP125, got %s", diags[0].Code)
	}
}

func TestLintToolBinary_Builtin(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "echo hello",
			}},
		},
	}
	diags := lintToolBinary(w)
	if len(diags) != 0 {
		t.Errorf("expected no DIP125 for shell builtin, got %d", len(diags))
	}
}

func TestLintToolBinary_SkipsPreamble(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "T", Exit: "T",
		Nodes: []*ir.Node{
			{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "set -eu\ncd /tmp\nls -la",
			}},
		},
	}
	diags := lintToolBinary(w)
	if len(diags) != 0 {
		t.Errorf("expected no DIP125, preamble should be skipped, got %d", len(diags))
	}
}

func TestExtractBinary(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"echo hello", "echo"},
		{"set -eu\nls -la", "ls"},
		{"set -eu\ncd /tmp\ngit status", "git"},
		{"# comment\nset -eu\ncurl http://x", "curl"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBinary(tt.cmd)
		if got != tt.want {
			t.Errorf("extractBinary(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}

func TestLintToolBinary_AgentNodeIgnored(t *testing.T) {
	w := &ir.Workflow{
		Name: "test", Start: "A", Exit: "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
		},
	}
	diags := lintToolBinary(w)
	if len(diags) != 0 {
		t.Errorf("expected no DIP125 for agent node, got %d", len(diags))
	}
}
