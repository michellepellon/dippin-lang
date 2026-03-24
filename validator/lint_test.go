package validator

import (
	"strings"
	"testing"
	"time"

	"github.com/2389-research/dippin-lang/ir"
)

// --- Test fixtures ---

// cleanMinimalWorkflow returns a minimal valid workflow with no lint warnings.
func cleanMinimalWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "clean",
		Start: "Begin",
		Exit:  "End",
		Nodes: []*ir.Node{
			{ID: "Begin", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Hello."}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Begin", To: "End"},
		},
	}
}

// cleanComplexWorkflow returns a complex valid workflow with no lint warnings.
func cleanComplexWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "clean_complex",
		Start: "Ask",
		Exit:  "Done",
		Defaults: ir.WorkflowDefaults{
			Model:    "claude-opus-4-6",
			Provider: "anthropic",
		},
		Nodes: []*ir.Node{
			{ID: "Ask", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"},
				IO: ir.NodeIO{Writes: []string{"human_response"}}},
			{ID: "Plan", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				Prompt: "Plan the work based on ${ctx.human_response}.",
			}, IO: ir.NodeIO{Reads: []string{"human_response"}, Writes: []string{"plan"}}},
			{ID: "Execute", Kind: ir.NodeAgent, Config: ir.AgentConfig{
				Prompt: "Execute the plan.",
			}, IO: ir.NodeIO{Reads: []string{"plan"}}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Ship it."}},
		},
		Edges: []*ir.Edge{
			{From: "Ask", To: "Plan"},
			{From: "Plan", To: "Execute"},
			{From: "Execute", To: "Done"},
		},
	}
}

// --- Table-driven tests ---

func TestLint(t *testing.T) {
	tests := []struct {
		name       string
		workflow   *ir.Workflow
		wantCodes  []string // Expected diagnostic codes (empty = no diagnostics)
		wantNoDiag bool     // If true, expect zero diagnostics
	}{
		// --- Happy path ---
		{
			name:       "clean minimal workflow",
			workflow:   cleanMinimalWorkflow(),
			wantNoDiag: true,
		},
		{
			name:       "clean complex workflow with IO contracts",
			workflow:   cleanComplexWorkflow(),
			wantNoDiag: true,
		},
		{
			name: "workflow with known model/provider is clean",
			workflow: &ir.Workflow{
				Name:  "known_model",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt:   "Hello.",
						Model:    "gpt-5.4",
						Provider: "openai",
					}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "tool with timeout is clean",
			workflow: &ir.Workflow{
				Name:  "tool_timeout",
				Start: "T",
				Exit:  "T",
				Nodes: []*ir.Node{
					{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
						Command: "echo test",
						Timeout: 30 * time.Second,
					}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP101: Unreachable nodes after conditional branches ---
		{
			name: "DIP101: node only reachable via conditional edges",
			workflow: &ir.Workflow{
				Name:  "cond_only",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.x = 1",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					}},
					{From: "A", To: "C"},
					{From: "B", To: "C"},
				},
			},
			wantCodes: []string{DIP101},
		},
		{
			name: "DIP101: node with unconditional incoming edge is fine",
			workflow: &ir.Workflow{
				Name:  "uncond_ok",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP102: Routing node without default edge ---
		{
			name: "DIP102: conditional outgoing but no default",
			workflow: &ir.Workflow{
				Name:  "no_default",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.x = 1",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					}},
					{From: "A", To: "C", Condition: &ir.Condition{
						Raw:    "ctx.x = 2",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
					}},
					{From: "B", To: "C"},
				},
			},
			wantCodes: []string{DIP102},
		},
		{
			name: "DIP102: mixed conditional + unconditional is fine",
			workflow: &ir.Workflow{
				Name:  "with_default",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.x = 1",
						Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					}},
					{From: "A", To: "C"}, // unconditional default
					{From: "B", To: "C"},
				},
			},
			// B is only reachable via conditional edge (DIP101), but no DIP102.
			wantCodes: []string{DIP101},
		},

		// --- DIP103: Overlapping conditions ---
		{
			name: "DIP103: two edges with same condition",
			workflow: &ir.Workflow{
				Name:  "overlap",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.outcome = success",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					}},
					{From: "A", To: "C", Condition: &ir.Condition{
						Raw:    "ctx.outcome = success",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					}},
				},
			},
			wantCodes: []string{DIP103},
		},
		{
			name: "DIP103: different conditions from same node is fine for overlap",
			workflow: &ir.Workflow{
				Name:  "no_overlap",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B", Condition: &ir.Condition{
						Raw:    "ctx.outcome = success",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
					}},
					{From: "A", To: "C", Condition: &ir.Condition{
						Raw:    "ctx.outcome = fail",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
					}},
					{From: "B", To: "C"},
				},
			},
			// No DIP103, but DIP101 (B, C only via conditional) and DIP102 (A has no default).
			wantCodes: []string{DIP101, DIP102},
		},

		// --- DIP104: Unbounded retry ---
		{
			name: "DIP104: retry config but no max_retries or fallback",
			workflow: &ir.Workflow{
				Name:  "unbounded_retry",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						Retry: ir.RetryConfig{Policy: "standard"}},
				},
			},
			wantCodes: []string{DIP104},
		},
		{
			name: "DIP104: retry with max_retries is fine",
			workflow: &ir.Workflow{
				Name:  "bounded_retry",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						Retry: ir.RetryConfig{Policy: "standard", MaxRetries: 3}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "DIP104: retry with fallback is fine",
			workflow: &ir.Workflow{
				Name:  "retry_fallback",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						Retry: ir.RetryConfig{Policy: "standard", FallbackTarget: "B"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP105: No success path to exit ---
		{
			name: "DIP105: no forward path to exit",
			workflow: &ir.Workflow{
				Name:  "no_path",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "A", Restart: true}, // only restart edges back
				},
			},
			wantCodes: []string{DIP105},
		},
		{
			name: "DIP105: forward path exists even with restart edges",
			workflow: &ir.Workflow{
				Name:  "has_path",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
					{From: "B", To: "A", Restart: true},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP106: Undefined variables in prompts ---
		{
			name: "DIP106: unnamespaced variable reference",
			workflow: &ir.Workflow{
				Name:  "bad_var",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "Process ${unknown_var} now.",
					}},
				},
			},
			wantCodes: []string{DIP106},
		},
		{
			name: "DIP106: known namespace is fine",
			workflow: &ir.Workflow{
				Name:  "good_var",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "Process ${ctx.data} and ${graph.goal} and ${params.strict}.",
					}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP107: Unused writes ---
		{
			name: "DIP107: writes key that nobody reads",
			workflow: &ir.Workflow{
				Name:  "unused_write",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"orphan_key"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantCodes: []string{DIP107},
		},
		{
			name: "DIP107: writes key that is read downstream is fine",
			workflow: &ir.Workflow{
				Name:  "used_write",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"data"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"data"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP108: Unknown model/provider ---
		{
			name: "DIP108: unknown provider",
			workflow: &ir.Workflow{
				Name:  "bad_provider",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt:   "a",
						Model:    "some-model",
						Provider: "unknown-provider",
					}},
				},
			},
			wantCodes: []string{DIP108},
		},
		{
			name: "DIP108: unknown model for known provider",
			workflow: &ir.Workflow{
				Name:  "bad_model",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt:   "a",
						Model:    "gpt-99",
						Provider: "openai",
					}},
				},
			},
			wantCodes: []string{DIP108},
		},
		{
			name: "DIP108: inherits from defaults",
			workflow: &ir.Workflow{
				Name:  "defaults_model",
				Start: "A",
				Exit:  "A",
				Defaults: ir.WorkflowDefaults{
					Model:    "gpt-99",
					Provider: "openai",
				},
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
				},
			},
			wantCodes: []string{DIP108},
		},

		// --- DIP109: Namespace collisions in imports ---
		{
			name: "DIP109: two subgraphs referencing same file",
			workflow: &ir.Workflow{
				Name:  "collision",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
					{ID: "B", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantCodes: []string{DIP109},
		},
		{
			name: "DIP109: different subgraph refs is fine",
			workflow: &ir.Workflow{
				Name:  "no_collision",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./review.dip"}},
					{ID: "B", Kind: ir.NodeSubgraph, Config: ir.SubgraphConfig{Ref: "./test.dip"}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done"}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP110: Empty prompts ---
		{
			name: "DIP110: agent with empty prompt",
			workflow: &ir.Workflow{
				Name:  "empty_prompt",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: ""}},
				},
			},
			wantCodes: []string{DIP110},
		},
		{
			name: "DIP110: agent with whitespace-only prompt",
			workflow: &ir.Workflow{
				Name:  "ws_prompt",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "   \n  "}},
				},
			},
			wantCodes: []string{DIP110},
		},
		{
			name: "DIP110: non-agent node types do not trigger",
			workflow: &ir.Workflow{
				Name:  "no_prompt_needed",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP111: Tool without timeout ---
		{
			name: "DIP111: tool with no timeout",
			workflow: &ir.Workflow{
				Name:  "no_timeout",
				Start: "T",
				Exit:  "T",
				Nodes: []*ir.Node{
					{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo hello"}},
				},
			},
			wantCodes: []string{DIP111},
		},
		{
			name: "DIP111: tool with timeout is clean",
			workflow: &ir.Workflow{
				Name:  "has_timeout",
				Start: "T",
				Exit:  "T",
				Nodes: []*ir.Node{
					{ID: "T", Kind: ir.NodeTool, Config: ir.ToolConfig{
						Command: "echo hello",
						Timeout: 10 * time.Second,
					}},
				},
			},
			wantNoDiag: true,
		},

		// --- DIP112: Reads without upstream writes ---
		{
			name: "DIP112: reads key with no upstream writer",
			workflow: &ir.Workflow{
				Name:  "no_writer",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"missing_key"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantCodes: []string{DIP112},
		},
		{
			name: "DIP112: reads key with upstream writer is fine",
			workflow: &ir.Workflow{
				Name:  "has_writer",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"data"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"data"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
				},
			},
			wantNoDiag: true,
		},

		// --- Edge cases ---
		{
			name: "empty workflow only reports DIP105 if start/exit are missing",
			workflow: &ir.Workflow{
				Name: "empty",
			},
			// DIP105 skips if start/exit are empty, DIP110/111/etc have no nodes to check
			wantNoDiag: true,
		},
		{
			name: "multiple lint warnings at once",
			workflow: &ir.Workflow{
				Name:  "multi_warn",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "", // DIP110
					}, Retry: ir.RetryConfig{Policy: "aggressive"}}, // DIP104
				},
			},
			wantCodes: []string{DIP110, DIP104},
		},
		{
			name: "DIP106: multiple undefined vars in one prompt",
			workflow: &ir.Workflow{
				Name:  "multi_var",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{
						Prompt: "Use ${foo} and ${bar} now.",
					}},
				},
			},
			wantCodes: []string{DIP106, DIP106},
		},
		{
			name: "DIP104: no retry config at all does not trigger",
			workflow: &ir.Workflow{
				Name:  "no_retry",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
				},
			},
			wantNoDiag: true,
		},
		{
			name: "DIP112: transitive writes propagation",
			workflow: &ir.Workflow{
				Name:  "transitive",
				Start: "A",
				Exit:  "C",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
						IO: ir.NodeIO{Writes: []string{"key1"}}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
						IO: ir.NodeIO{Reads: []string{"key1"}, Writes: []string{"key2"}}},
					{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"},
						IO: ir.NodeIO{Reads: []string{"key1", "key2"}}},
				},
				Edges: []*ir.Edge{
					{From: "A", To: "B"},
					{From: "B", To: "C"},
				},
			},
			wantNoDiag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Lint(tt.workflow)

			if tt.wantNoDiag {
				if len(result.Diagnostics) != 0 {
					t.Errorf("expected no diagnostics, got %d:", len(result.Diagnostics))
					for _, d := range result.Diagnostics {
						t.Errorf("  %s", d.String())
					}
				}
				return
			}

			if tt.wantCodes != nil {
				gotCodes := make([]string, len(result.Diagnostics))
				for i, d := range result.Diagnostics {
					gotCodes[i] = d.Code
				}

				wantCount := make(map[string]int)
				for _, c := range tt.wantCodes {
					wantCount[c]++
				}
				gotCount := make(map[string]int)
				for _, c := range gotCodes {
					gotCount[c]++
				}
				for code, want := range wantCount {
					if got := gotCount[code]; got < want {
						t.Errorf("expected at least %d %s diagnostic(s), got %d. All codes: %v", want, code, got, gotCodes)
					}
				}
				// Also check total count matches expectations for single-diagnostic tests.
				if len(tt.wantCodes) == 1 {
					codeCount := 0
					for _, d := range result.Diagnostics {
						if d.Code == tt.wantCodes[0] {
							codeCount++
						}
					}
					if codeCount < 1 {
						t.Errorf("expected at least 1 %s diagnostic, got 0. All: %v", tt.wantCodes[0], gotCodes)
					}
				}
			}
		})
	}
}

func TestLintDiagnosticSeverity(t *testing.T) {
	// All lint diagnostics should be warnings, not errors.
	w := &ir.Workflow{
		Name:  "severity_check",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: ""},
				Retry: ir.RetryConfig{Policy: "standard"}},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Severity != SeverityWarning {
			t.Errorf("lint diagnostic %s has severity %s, want warning", d.Code, d.Severity)
		}
	}
}

func TestLintDIP101MessageContent(t *testing.T) {
	w := &ir.Workflow{
		Name:  "msg_check",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{
				Raw:    "ctx.x = 1",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			}},
			{From: "A", To: "C"},
			{From: "B", To: "C"},
		},
	}

	result := Lint(w)
	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == DIP101 {
			found = true
			if !strings.Contains(d.Message, `"B"`) {
				t.Errorf("DIP101 message should mention node B, got: %s", d.Message)
			}
			if !strings.Contains(d.Message, "conditional") {
				t.Errorf("DIP101 message should mention 'conditional', got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Error("expected DIP101 diagnostic")
	}
}

func TestLintDIP102MessageContent(t *testing.T) {
	w := &ir.Workflow{
		Name:  "msg_check",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{
				Raw:    "ctx.x = 1",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			}},
			{From: "A", To: "C", Condition: &ir.Condition{
				Raw:    "ctx.x = 2",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
			}},
			{From: "B", To: "C"},
		},
	}

	result := Lint(w)
	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == DIP102 {
			found = true
			if !strings.Contains(d.Message, `"A"`) {
				t.Errorf("DIP102 message should mention node A, got: %s", d.Message)
			}
			if !strings.Contains(d.Message, "unconditional") || !strings.Contains(d.Message, "default") {
				t.Errorf("DIP102 message should mention 'unconditional default', got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Error("expected DIP102 diagnostic")
	}
}

func TestLintDIP103OverlappingANDConditions(t *testing.T) {
	// Overlapping condition buried in an AND expression.
	w := &ir.Workflow{
		Name:  "and_overlap",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{
				Raw: "ctx.x = 1 and ctx.y = 2",
				Parsed: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
			}},
			{From: "A", To: "C", Condition: &ir.Condition{
				Raw:    "ctx.x = 1",
				Parsed: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			}},
		},
	}

	result := Lint(w)
	found := false
	for _, d := range result.Diagnostics {
		if d.Code == DIP103 {
			found = true
		}
	}
	if !found {
		t.Error("expected DIP103 for overlapping condition in AND expression")
	}
}

func TestLintDIP105StartEqualsExit(t *testing.T) {
	// When start == exit, trivially reachable.
	w := &ir.Workflow{
		Name:  "trivial",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Code == DIP105 {
			t.Errorf("DIP105 should not trigger when start == exit, got: %s", d.Message)
		}
	}
}

func TestLintDIP106NoPromptNodes(t *testing.T) {
	// Human and tool nodes should not trigger DIP106.
	w := &ir.Workflow{
		Name:  "no_prompt",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{ID: "B", Kind: ir.NodeTool, Config: ir.ToolConfig{Command: "echo ${not_a_prompt}", Timeout: 10 * time.Second}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Code == DIP106 {
			t.Errorf("DIP106 should not trigger for non-agent nodes, got: %s", d.Message)
		}
	}
}

func TestLintDIP108NoModelOrProvider(t *testing.T) {
	// If model or provider is not specified (and no defaults), don't check.
	w := &ir.Workflow{
		Name:  "no_model",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"}},
		},
	}

	result := Lint(w)
	for _, d := range result.Diagnostics {
		if d.Code == DIP108 {
			t.Errorf("DIP108 should not trigger when model/provider are unset, got: %s", d.Message)
		}
	}
}

func TestLintDIP112CycleDoesNotPanic(t *testing.T) {
	// A workflow with a cycle (via non-restart edges) should not cause
	// the topological sort in DIP112 to hang or panic.
	w := &ir.Workflow{
		Name:  "cycle_safe",
		Start: "A",
		Exit:  "C",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a"},
				IO: ir.NodeIO{Writes: []string{"data"}}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b"},
				IO: ir.NodeIO{Reads: []string{"data"}}},
			{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c"}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "A"}, // cycle
			{From: "B", To: "C"},
		},
	}

	// Should not panic.
	result := Lint(w)
	_ = result
}

func TestLintCodeDescriptionCoverage(t *testing.T) {
	codes := []string{DIP101, DIP102, DIP103, DIP104, DIP105, DIP106, DIP107, DIP108, DIP109, DIP110, DIP111, DIP112}
	for _, c := range codes {
		if desc, ok := CodeDescription[c]; !ok || desc == "" {
			t.Errorf("CodeDescription[%q] is missing or empty", c)
		}
	}
}

func TestExtractComparisons(t *testing.T) {
	tests := []struct {
		name string
		expr ir.ConditionExpr
		want int // expected number of comparisons
	}{
		{
			name: "nil",
			expr: nil,
			want: 0,
		},
		{
			name: "single compare",
			expr: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
			want: 1,
		},
		{
			name: "AND of two compares",
			expr: ir.CondAnd{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
			},
			want: 2,
		},
		{
			name: "OR of two compares",
			expr: ir.CondOr{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
			},
			want: 2,
		},
		{
			name: "NOT of compare",
			expr: ir.CondNot{Inner: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"}},
			want: 1,
		},
		{
			name: "nested AND/OR/NOT",
			expr: ir.CondAnd{
				Left: ir.CondOr{
					Left:  ir.CondCompare{Variable: "ctx.a", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.b", Op: "=", Value: "2"},
				},
				Right: ir.CondNot{
					Inner: ir.CondCompare{Variable: "ctx.c", Op: "!=", Value: "3"},
				},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractComparisons(tt.expr)
			if len(got) != tt.want {
				t.Errorf("extractComparisons() returned %d comparisons, want %d", len(got), tt.want)
			}
		})
	}
}

// --- DIP113: Invalid retry policy ---

func TestLint_DIP113_InvalidRetryPolicy_Node(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Retry.Policy = "bogus"
	res := Lint(w)
	assertHasCode(t, res, DIP113)
}

func TestLint_DIP113_InvalidRetryPolicy_Default(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Defaults.RetryPolicy = "nope"
	res := Lint(w)
	assertHasCode(t, res, DIP113)
}

func TestLint_DIP113_ValidRetryPolicies(t *testing.T) {
	for _, policy := range []string{"standard", "aggressive", "patient", "linear", "none"} {
		t.Run(policy, func(t *testing.T) {
			w := cleanMinimalWorkflow()
			w.Nodes[0].Retry.Policy = policy
			res := Lint(w)
			assertNoCode(t, res, DIP113)
		})
	}
}

func TestLint_DIP113_EmptyPolicy_NoDiag(t *testing.T) {
	w := cleanMinimalWorkflow()
	// No retry policy set — should not warn.
	res := Lint(w)
	assertNoCode(t, res, DIP113)
}

// --- DIP114: Invalid fidelity level ---

func TestLint_DIP114_InvalidFidelity_Node(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Config = ir.AgentConfig{Prompt: "X", Fidelity: "sumary:high"}
	res := Lint(w)
	assertHasCode(t, res, DIP114)
}

func TestLint_DIP114_InvalidFidelity_Default(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Defaults.Fidelity = "hi"
	res := Lint(w)
	assertHasCode(t, res, DIP114)
}

func TestLint_DIP114_ValidFidelityLevels(t *testing.T) {
	for _, level := range []string{"full", "summary:high", "summary:medium", "summary:low", "compact", "truncate"} {
		t.Run(level, func(t *testing.T) {
			w := cleanMinimalWorkflow()
			w.Nodes[0].Config = ir.AgentConfig{Prompt: "X", Fidelity: level}
			res := Lint(w)
			assertNoCode(t, res, DIP114)
		})
	}
}

func TestLint_DIP114_EmptyFidelity_NoDiag(t *testing.T) {
	w := cleanMinimalWorkflow()
	res := Lint(w)
	assertNoCode(t, res, DIP114)
}

// --- DIP115: Goal gate without fallback ---

func TestLint_DIP115_GoalGateNoFallback(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Config = ir.AgentConfig{Prompt: "X", GoalGate: true}
	res := Lint(w)
	assertHasCode(t, res, DIP115)
}

func TestLint_DIP115_GoalGateWithRetryTarget(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Config = ir.AgentConfig{Prompt: "X", GoalGate: true}
	w.Nodes[0].Retry.RetryTarget = "Begin"
	res := Lint(w)
	assertNoCode(t, res, DIP115)
}

func TestLint_DIP115_GoalGateWithFallbackTarget(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Config = ir.AgentConfig{Prompt: "X", GoalGate: true}
	w.Nodes[0].Retry.FallbackTarget = "End"
	res := Lint(w)
	assertNoCode(t, res, DIP115)
}

func TestLint_DIP115_NoGoalGate_NoDiag(t *testing.T) {
	w := cleanMinimalWorkflow()
	res := Lint(w)
	assertNoCode(t, res, DIP115)
}

// --- DIP114: BranchConfig fidelity ---

func TestLint_DIP114_InvalidBranchFidelity(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes = append(w.Nodes, &ir.Node{
		ID: "Fan", Kind: ir.NodeParallel, Config: ir.ParallelConfig{
			Branches: []ir.BranchConfig{
				{Target: "Begin", Fidelity: "bogus"},
			},
		},
	})
	res := Lint(w)
	assertHasCode(t, res, DIP114)
}

func TestLint_DIP114_ValidBranchFidelity(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes = append(w.Nodes, &ir.Node{
		ID: "Fan", Kind: ir.NodeParallel, Config: ir.ParallelConfig{
			Branches: []ir.BranchConfig{
				{Target: "Begin", Fidelity: "compact"},
			},
		},
	})
	res := Lint(w)
	assertNoCode(t, res, DIP114)
}

// --- DIP119: reasoning_effort ---

func TestLint_DIP119_InvalidReasoningEffort(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Config = ir.AgentConfig{Prompt: "Go.", ReasoningEffort: "hihg"}
	res := Lint(w)
	assertHasCode(t, res, DIP119)
}

func TestLint_DIP119_ValidReasoningEffort(t *testing.T) {
	for _, level := range []string{"low", "medium", "high"} {
		w := cleanMinimalWorkflow()
		w.Nodes[0].Config = ir.AgentConfig{Prompt: "Go.", ReasoningEffort: level}
		res := Lint(w)
		assertNoCode(t, res, DIP119)
	}
}

func TestLint_DIP119_EmptyReasoningEffort_NoDiag(t *testing.T) {
	w := cleanMinimalWorkflow()
	res := Lint(w)
	assertNoCode(t, res, DIP119)
}

// assertHasCode checks that a result contains at least one diagnostic with the given code.
func assertHasCode(t *testing.T, res Result, code string) {
	t.Helper()
	for _, d := range res.Diagnostics {
		if d.Code == code {
			return
		}
	}
	codes := make([]string, len(res.Diagnostics))
	for i, d := range res.Diagnostics {
		codes[i] = d.Code
	}
	t.Errorf("expected diagnostic %s, got codes: %v", code, codes)
}

// assertNoCode checks that a result does not contain any diagnostic with the given code.
func assertNoCode(t *testing.T, res Result, code string) {
	t.Helper()
	for _, d := range res.Diagnostics {
		if d.Code == code {
			t.Errorf("unexpected diagnostic %s: %s", code, d.Message)
		}
	}
}
