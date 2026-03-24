package coverage

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// makeEdge creates a simple unconditional edge.
func makeEdge(from, to string) *ir.Edge {
	return &ir.Edge{From: from, To: to}
}

// makeCondEdge creates an edge with a simple equality condition.
func makeCondEdge(from, to, variable, value string) *ir.Edge {
	return &ir.Edge{
		From: from,
		To:   to,
		Condition: &ir.Condition{
			Raw:    variable + " = " + value,
			Parsed: ir.CondCompare{Variable: variable, Op: "=", Value: value},
		},
	}
}

func TestCoveredToolNode(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "check", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "printf 'pass'"}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "check"),
			makeCondEdge("check", "end", "ctx.result", "pass"),
		},
	}

	r := Analyze(w)
	cov := r.Nodes["check"]
	if cov.Status != "covered" {
		t.Errorf("expected status covered, got %s", cov.Status)
	}
	if len(cov.MissingEdges) != 0 {
		t.Errorf("expected no missing edges, got %v", cov.MissingEdges)
	}
}

func TestPartialToolNode(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "check", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "printf 'pass'\nprintf 'fail'",
			}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "check"),
			makeCondEdge("check", "end", "ctx.result", "pass"),
		},
	}

	r := Analyze(w)
	cov := r.Nodes["check"]
	if cov.Status != "partial" {
		t.Errorf("expected status partial, got %s", cov.Status)
	}
	if len(cov.MissingEdges) != 1 || cov.MissingEdges[0] != "fail" {
		t.Errorf("expected missing edge [fail], got %v", cov.MissingEdges)
	}
}

func TestFallbackMakesCovered(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "check", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "printf 'pass'\nprintf 'fail'",
			}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "check"),
			makeCondEdge("check", "end", "ctx.result", "pass"),
			makeEdge("check", "end"), // fallback
		},
	}

	r := Analyze(w)
	cov := r.Nodes["check"]
	if cov.Status != "covered" {
		t.Errorf("expected status covered with fallback, got %s", cov.Status)
	}
	if !cov.HasFallback {
		t.Error("expected HasFallback to be true")
	}
}

func TestUnknownOutputs(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "check", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "run-custom-binary --flag",
			}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "check"),
			makeCondEdge("check", "end", "ctx.result", "ok"),
		},
	}

	r := Analyze(w)
	cov := r.Nodes["check"]
	if cov.Status != "unknown" {
		t.Errorf("expected status unknown, got %s", cov.Status)
	}
}

func TestNoConditionsStatus(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "check", Kind: ir.NodeTool, Config: ir.ToolConfig{
				Command: "printf 'done'",
			}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "check"),
			makeEdge("check", "end"),
		},
	}

	r := Analyze(w)
	cov := r.Nodes["check"]
	if cov.Status != "no_conditions" {
		t.Errorf("expected status no_conditions, got %s", cov.Status)
	}
}

func TestUnreachableNode(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
			{ID: "orphan", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "lost"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "end"),
		},
	}

	r := Analyze(w)
	if r.Reachability.ReachableNodes != 2 {
		t.Errorf("expected 2 reachable nodes, got %d", r.Reachability.ReachableNodes)
	}
	if len(r.Reachability.UnreachableNodes) != 1 || r.Reachability.UnreachableNodes[0] != "orphan" {
		t.Errorf("expected unreachable [orphan], got %v", r.Reachability.UnreachableNodes)
	}
}

func TestSinkNodeDetection(t *testing.T) {
	w := &ir.Workflow{
		Start: "start",
		Exit:  "end",
		Nodes: []*ir.Node{
			{ID: "start", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go"}},
			{ID: "sink", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "stuck"}},
			{ID: "end", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
		},
		Edges: []*ir.Edge{
			makeEdge("start", "sink"),
			makeEdge("start", "end"),
			// sink has no path to end
		},
	}

	r := Analyze(w)
	if r.Termination.AllPathsTerminate {
		t.Error("expected AllPathsTerminate to be false")
	}
	if len(r.Termination.SinkNodes) != 1 || r.Termination.SinkNodes[0] != "sink" {
		t.Errorf("expected sink nodes [sink], got %v", r.Termination.SinkNodes)
	}
}

func TestExtractToolOutputs(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"single_printf", "printf 'pass'", []string{"pass"}},
		{"double_quote_printf", `printf "fail"`, []string{"fail"}},
		{"echo", "echo 'done'", []string{"done"}},
		{"printf_format", "printf '%s' 'value'", []string{"value"}},
		{"multiple", "printf 'a'\nprintf 'b'", []string{"a", "b"}},
		{"no_match", "run-binary", nil},
		{"dedup", "printf 'x'\nprintf 'x'", []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolOutputs(tt.command)
			if !slicesEqual(got, tt.want) {
				t.Errorf("extractToolOutputs(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
