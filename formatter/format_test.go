package formatter

import (
	"strings"
	"testing"
	"time"

	"github.com/2389-research/dippin-lang/ir"
)

// --- Fixtures ---

func minimalWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "minimal",
		Start: "Begin",
		Exit:  "End",
		Nodes: []*ir.Node{
			{ID: "Begin", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{ID: "End", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Done."}},
		},
		Edges: []*ir.Edge{
			{From: "Begin", To: "End"},
		},
	}
}

func askAndExecuteWorkflow() *ir.Workflow {
	return &ir.Workflow{
		Name:  "ask_and_execute",
		Goal:  "Ask user for a task, implement it, review, ship",
		Start: "AskUser",
		Exit:  "Done",
		Defaults: ir.WorkflowDefaults{
			Model:       "claude-opus-4-6",
			Provider:    "anthropic",
			RetryPolicy: "standard",
			Fidelity:    "summary:high",
			MaxRestarts: 5,
		},
		Nodes: []*ir.Node{
			{ID: "AskUser", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "freeform"}},
			{
				ID: "Interpret", Kind: ir.NodeAgent,
				IO: ir.NodeIO{Reads: []string{"human_response"}, Writes: []string{"plan"}},
				Config: ir.AgentConfig{
					Prompt: "You are a senior software architect.\n\nRead the user's request below and produce a clear,\nactionable implementation plan.\n\n## User Request\n${ctx.human_response}",
				},
			},
			{ID: "ImplementFanOut", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"ImplementClaude", "ImplementCodex"}}},
			{
				ID: "ImplementClaude", Kind: ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "Implement the plan.", Model: "gpt-5.4", Provider: "openai"},
			},
			{
				ID: "ImplementCodex", Kind: ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "Implement the plan.", Model: "gpt-5.3-codex", Provider: "openai"},
			},
			{ID: "ImplementJoin", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"ImplementClaude", "ImplementCodex"}}},
			{
				ID: "Validate", Kind: ir.NodeAgent,
				Config: ir.AgentConfig{
					Prompt:     "Review the implementations. Run tests.\nRespond with STATUS: success or STATUS: fail.",
					AutoStatus: true,
					GoalGate:   true,
				},
				Retry: ir.RetryConfig{MaxRetries: 2},
			},
			{ID: "Approve", Kind: ir.NodeHuman, Config: ir.HumanConfig{Mode: "choice", Default: "Yes"}},
			{ID: "Done", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "Ship it."}},
		},
		Edges: []*ir.Edge{
			{From: "AskUser", To: "Interpret"},
			{From: "Interpret", To: "ImplementFanOut"},
			{From: "ImplementJoin", To: "Validate"},
			{From: "Validate", To: "Approve", Condition: &ir.Condition{
				Raw:    "ctx.outcome = success",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			}},
			{From: "Validate", To: "Interpret", Label: "retry", Restart: true, Condition: &ir.Condition{
				Raw:    "ctx.outcome = fail",
				Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
			}},
			{From: "Approve", To: "Done"},
		},
	}
}

// --- Test helpers ---

func assertContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output does not contain %q\n\ngot:\n%s", substr, output)
	}
}

func assertNotContains(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("output unexpectedly contains %q\n\ngot:\n%s", substr, output)
	}
}

func assertIdempotent(t *testing.T, w *ir.Workflow) {
	t.Helper()
	first := Format(w)
	second := Format(w)
	if first != second {
		t.Errorf("Format is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// --- Table-driven tests ---

func TestFormatHappyPath(t *testing.T) {
	tests := []struct {
		name     string
		workflow *ir.Workflow
		checks   []string // substrings that must appear
	}{
		{
			name:     "minimal workflow",
			workflow: minimalWorkflow(),
			checks: []string{
				"workflow minimal",
				"  start: Begin",
				"  exit: End",
				"  human Begin",
				"    mode: freeform",
				"  agent End",
				"    prompt:",
				"      Done.",
				"  edges",
				"    Begin -> End",
			},
		},
		{
			name:     "full ask_and_execute",
			workflow: askAndExecuteWorkflow(),
			checks: []string{
				"workflow ask_and_execute",
				`  goal: "Ask user for a task, implement it, review, ship"`,
				"  start: AskUser",
				"  exit: Done",
				"  defaults",
				"    model: claude-opus-4-6",
				"    provider: anthropic",
				"    retry_policy: standard",
				"    fidelity: summary:high",
				"    max_restarts: 5",
				"  human AskUser",
				"  agent Interpret",
				"    reads: human_response",
				"    writes: plan",
				"    prompt:",
				"      You are a senior software architect.",
				"      ${ctx.human_response}",
				"  parallel ImplementFanOut -> ImplementClaude, ImplementCodex",
				"  agent ImplementClaude",
				"    model: gpt-5.4",
				"    provider: openai",
				"  agent ImplementCodex",
				"    model: gpt-5.3-codex",
				"  fan_in ImplementJoin <- ImplementClaude, ImplementCodex",
				"  agent Validate",
				"    goal_gate: true",
				"    auto_status: true",
				"    max_retries: 2",
				"  human Approve",
				"    mode: choice",
				"    default: Yes",
				"  agent Done",
				"  edges",
				"    AskUser -> Interpret",
				"    Validate -> Approve  when ctx.outcome = success",
				`    Validate -> Interpret  when ctx.outcome = fail  label: retry  restart: true`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := Format(tt.workflow)
			for _, check := range tt.checks {
				assertContains(t, output, check)
			}
			assertIdempotent(t, tt.workflow)
		})
	}
}

func TestFormatAgentAllFields(t *testing.T) {
	w := &ir.Workflow{
		Name:  "agent_all_fields",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{
				ID:      "A",
				Kind:    ir.NodeAgent,
				Label:   "My Agent Node",
				Classes: []string{"primary", "reviewer"},
				Config: ir.AgentConfig{
					Model:           "gpt-5.4",
					Provider:        "openai",
					ReasoningEffort: "high",
					Fidelity:        "full",
					GoalGate:        true,
					AutoStatus:      true,
					MaxTurns:        10,
					Prompt:          "Do the thing.\nDo it well.",
				},
				Retry: ir.RetryConfig{
					Policy:      "aggressive",
					MaxRetries:  5,
					RetryTarget: "Fallback",
				},
				IO: ir.NodeIO{
					Reads:  []string{"input", "context"},
					Writes: []string{"output"},
				},
			},
		},
	}

	output := Format(w)

	// Verify canonical order: label, class, model, provider, reasoning_effort,
	// fidelity, goal_gate, auto_status, max_turns, retry_policy, max_retries,
	// retry_target, reads, writes, prompt
	lines := strings.Split(output, "\n")
	fieldOrder := []string{
		"label:", "class:", "model:", "provider:", "reasoning_effort:",
		"fidelity:", "goal_gate:", "auto_status:", "max_turns:",
		"retry_policy:", "max_retries:", "retry_target:",
		"reads:", "writes:", "prompt:",
	}
	lastIdx := -1
	for _, field := range fieldOrder {
		for i, l := range lines {
			if strings.Contains(strings.TrimSpace(l), field) {
				if i <= lastIdx {
					t.Errorf("field %q at line %d appears before or at previous field line %d", field, i, lastIdx)
				}
				lastIdx = i
				break
			}
		}
	}

	assertContains(t, output, `label: "My Agent Node"`)
	assertContains(t, output, "class: primary, reviewer")
	assertContains(t, output, "model: gpt-5.4")
	assertContains(t, output, "provider: openai")
	assertContains(t, output, "reasoning_effort: high")
	assertContains(t, output, "fidelity: full")
	assertContains(t, output, "goal_gate: true")
	assertContains(t, output, "auto_status: true")
	assertContains(t, output, "max_turns: 10")
	assertContains(t, output, "retry_policy: aggressive")
	assertContains(t, output, "max_retries: 5")
	assertContains(t, output, "retry_target: Fallback")
	assertContains(t, output, "reads: input, context")
	assertContains(t, output, "writes: output")
	assertContains(t, output, "prompt:")
	assertContains(t, output, "Do the thing.")
	assertIdempotent(t, w)
}

func TestFormatHumanAllFields(t *testing.T) {
	w := &ir.Workflow{
		Name:  "human_all",
		Start: "H",
		Exit:  "H",
		Nodes: []*ir.Node{
			{
				ID:    "H",
				Kind:  ir.NodeHuman,
				Label: "User Input",
				Config: ir.HumanConfig{
					Mode:    "choice",
					Default: "Yes",
				},
				IO: ir.NodeIO{
					Reads:  []string{"prev_output"},
					Writes: []string{"human_response"},
				},
			},
		},
	}

	output := Format(w)
	assertContains(t, output, `label: "User Input"`)
	assertContains(t, output, "mode: choice")
	assertContains(t, output, "default: Yes")
	assertContains(t, output, "reads: prev_output")
	assertContains(t, output, "writes: human_response")
	assertIdempotent(t, w)
}

func TestFormatToolMultilineCommand(t *testing.T) {
	w := &ir.Workflow{
		Name:  "tool_test",
		Start: "T",
		Exit:  "T",
		Nodes: []*ir.Node{
			{
				ID:   "T",
				Kind: ir.NodeTool,
				Config: ir.ToolConfig{
					Command: "#!/bin/sh\nset -eu\nif pytest --tb=short 2>&1; then\n  printf 'pass'\nelse\n  printf 'fail'\n  exit 1\nfi",
					Timeout: 60 * time.Second,
				},
				IO: ir.NodeIO{Writes: []string{"test_result"}},
			},
		},
	}

	output := Format(w)
	assertContains(t, output, "timeout: 1m")
	assertContains(t, output, "writes: test_result")
	assertContains(t, output, "command:")
	assertContains(t, output, "      #!/bin/sh")
	assertContains(t, output, "      set -eu")
	assertContains(t, output, "      if pytest --tb=short 2>&1; then")
	assertContains(t, output, "        printf 'pass'")
	assertContains(t, output, "        printf 'fail'")
	assertIdempotent(t, w)
}

func TestFormatFieldOrdering(t *testing.T) {
	t.Run("prompt is always last", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "prompt_last",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{
					ID:   "A",
					Kind: ir.NodeAgent,
					Config: ir.AgentConfig{
						Prompt:   "Do work.",
						Model:    "gpt-5.4",
						Provider: "openai",
					},
					IO: ir.NodeIO{Reads: []string{"input"}},
				},
			},
		}

		output := Format(w)
		promptIdx := strings.Index(output, "prompt:")
		readsIdx := strings.Index(output, "reads:")
		modelIdx := strings.Index(output, "model:")
		if promptIdx < readsIdx {
			t.Error("prompt should appear after reads")
		}
		if promptIdx < modelIdx {
			t.Error("prompt should appear after model")
		}
	})

	t.Run("command is always last for tool", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "cmd_last",
			Start: "T",
			Exit:  "T",
			Nodes: []*ir.Node{
				{
					ID:   "T",
					Kind: ir.NodeTool,
					Config: ir.ToolConfig{
						Command: "echo hello",
						Timeout: 30 * time.Second,
					},
					IO: ir.NodeIO{Writes: []string{"out"}},
				},
			},
		}
		output := Format(w)
		commandIdx := strings.Index(output, "command:")
		timeoutIdx := strings.Index(output, "timeout:")
		writesIdx := strings.Index(output, "writes:")
		if commandIdx < timeoutIdx {
			t.Error("command should appear after timeout")
		}
		if commandIdx < writesIdx {
			t.Error("command should appear after writes")
		}
	})
}

func TestFormatMultilineContent(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		contains []string
		excludes []string
	}{
		{
			name:   "blank lines preserved",
			prompt: "First line.\n\nThird line.",
			contains: []string{
				"      First line.",
				"      Third line.",
			},
		},
		{
			name:   "variable references preserved",
			prompt: "Process ${ctx.human_response} now.\nUse ${graph.goal} as guide.",
			contains: []string{
				"${ctx.human_response}",
				"${graph.goal}",
			},
		},
		{
			name:   "trailing whitespace stripped",
			prompt: "line with spaces   \nnext line",
			contains: []string{
				"      line with spaces",
				"      next line",
			},
			excludes: []string{
				"line with spaces   ",
			},
		},
		{
			name:   "trailing blank lines in prompt stripped",
			prompt: "content\n\n\n",
			contains: []string{
				"      content",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &ir.Workflow{
				Name:  "test",
				Start: "A",
				Exit:  "A",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: tt.prompt}},
				},
			}
			output := Format(w)
			for _, c := range tt.contains {
				assertContains(t, output, c)
			}
			for _, e := range tt.excludes {
				assertNotContains(t, output, e)
			}
		})
	}
}

func TestFormatEdges(t *testing.T) {
	tests := []struct {
		name   string
		edges  []*ir.Edge
		checks []string
	}{
		{
			name: "simple edge",
			edges: []*ir.Edge{
				{From: "A", To: "B"},
			},
			checks: []string{"    A -> B"},
		},
		{
			name: "conditional edge",
			edges: []*ir.Edge{
				{From: "A", To: "B", Condition: &ir.Condition{
					Raw:    "ctx.outcome = success",
					Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
				}},
			},
			checks: []string{"    A -> B  when ctx.outcome = success"},
		},
		{
			name: "edge with all attributes",
			edges: []*ir.Edge{
				{
					From:  "A",
					To:    "B",
					Label: "retry",
					Condition: &ir.Condition{
						Raw:    "ctx.outcome = fail",
						Parsed: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "fail"},
					},
					Weight:  10,
					Restart: true,
				},
			},
			checks: []string{
				`    A -> B  when ctx.outcome = fail  label: retry  weight: 10  restart: true`,
			},
		},
		{
			name: "complex condition with AND",
			edges: []*ir.Edge{
				{From: "A", To: "B", Condition: &ir.Condition{
					Raw: "ctx.outcome = success and ctx.tool_stdout != empty",
					Parsed: ir.CondAnd{
						Left:  ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
						Right: ir.CondCompare{Variable: "ctx.tool_stdout", Op: "!=", Value: "empty"},
					},
				}},
			},
			checks: []string{
				"    A -> B  when ctx.outcome = success and ctx.tool_stdout != empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &ir.Workflow{
				Name:  "test",
				Start: "A",
				Exit:  "B",
				Nodes: []*ir.Node{
					{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
					{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
				},
				Edges: tt.edges,
			}
			output := Format(w)
			for _, check := range tt.checks {
				assertContains(t, output, check)
			}
		})
	}
}

func TestFormatDefaults(t *testing.T) {
	t.Run("defaults with some fields", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Defaults: ir.WorkflowDefaults{
				Model:       "gpt-5.4",
				Provider:    "openai",
				MaxRetries:  3,
				MaxRestarts: 5,
			},
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			},
		}
		output := Format(w)
		assertContains(t, output, "  defaults")
		assertContains(t, output, "    model: gpt-5.4")
		assertContains(t, output, "    provider: openai")
		assertContains(t, output, "    max_retries: 3")
		assertContains(t, output, "    max_restarts: 5")
		// Fields not set should not appear
		assertNotContains(t, output, "retry_policy:")
		assertNotContains(t, output, "fidelity:")
		assertNotContains(t, output, "cache_tools:")
		assertNotContains(t, output, "compaction:")
	})

	t.Run("no defaults omits block", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			},
		}
		output := Format(w)
		assertNotContains(t, output, "defaults")
	})
}

func TestFormatSpecialCases(t *testing.T) {
	t.Run("empty workflow", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "empty",
			Start: "S",
			Exit:  "E",
		}
		output := Format(w)
		assertContains(t, output, "workflow empty")
		assertContains(t, output, "  start: S")
		assertContains(t, output, "  exit: E")
		// Should have exactly one trailing newline
		if !strings.HasSuffix(output, "\n") {
			t.Error("output should end with newline")
		}
		trimmed := strings.TrimRight(output, "\n")
		if strings.HasSuffix(trimmed, "\n") {
			t.Error("output should have exactly one trailing newline")
		}
	})

	t.Run("parallel and fan_in inline", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "par",
			Start: "P",
			Exit:  "J",
			Nodes: []*ir.Node{
				{ID: "P", Kind: ir.NodeParallel, Config: ir.ParallelConfig{Targets: []string{"A", "B", "C"}}},
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "a."}},
				{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "b."}},
				{ID: "C", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "c."}},
				{ID: "J", Kind: ir.NodeFanIn, Config: ir.FanInConfig{Sources: []string{"A", "B", "C"}}},
			},
		}
		output := Format(w)
		assertContains(t, output, "  parallel P -> A, B, C")
		assertContains(t, output, "  fan_in J <- A, B, C")
	})

	t.Run("subgraph with params", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "sub",
			Start: "S",
			Exit:  "S",
			Nodes: []*ir.Node{
				{
					ID:   "S",
					Kind: ir.NodeSubgraph,
					Config: ir.SubgraphConfig{
						Ref:    "./review.dip",
						Params: map[string]string{"model": "gpt-5.4", "strict": "true"},
					},
				},
			},
		}
		output := Format(w)
		assertContains(t, output, "  subgraph S")
		assertContains(t, output, "    ref: ./review.dip")
		assertContains(t, output, "    params:")
		assertContains(t, output, "      model: gpt-5.4")
		assertContains(t, output, "      strict: true")
	})

	t.Run("idempotency", func(t *testing.T) {
		assertIdempotent(t, askAndExecuteWorkflow())
		assertIdempotent(t, minimalWorkflow())
	})
}

func TestFormatEdgeCases(t *testing.T) {
	t.Run("zero-value agent config", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{}},
			},
		}
		output := Format(w)
		assertContains(t, output, "  agent A")
		// No fields should be emitted for zero-value config
		assertNotContains(t, output, "model:")
		assertNotContains(t, output, "prompt:")
		assertNotContains(t, output, "goal_gate:")
		assertNotContains(t, output, "auto_status:")
	})

	t.Run("workflow with goal", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Goal:  "Do something useful",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			},
		}
		output := Format(w)
		assertContains(t, output, `  goal: "Do something useful"`)
	})

	t.Run("workflow without goal", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			},
		}
		output := Format(w)
		assertNotContains(t, output, "goal:")
	})

	t.Run("node with classes", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{
					ID:      "A",
					Kind:    ir.NodeAgent,
					Classes: []string{"cls1", "cls2"},
					Config:  ir.AgentConfig{Prompt: "go."},
				},
			},
		}
		output := Format(w)
		assertContains(t, output, "    class: cls1, cls2")
	})

	t.Run("reads and writes with multiple keys", func(t *testing.T) {
		w := &ir.Workflow{
			Name:  "test",
			Start: "A",
			Exit:  "A",
			Nodes: []*ir.Node{
				{
					ID:   "A",
					Kind: ir.NodeAgent,
					Config: ir.AgentConfig{
						Prompt: "go.",
					},
					IO: ir.NodeIO{
						Reads:  []string{"key1", "key2", "key3"},
						Writes: []string{"out1", "out2"},
					},
				},
			},
		}
		output := Format(w)
		assertContains(t, output, "    reads: key1, key2, key3")
		assertContains(t, output, "    writes: out1, out2")
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{time.Hour + 30*time.Minute + 15*time.Second, "1h30m15s"},
		{0, "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatConditions(t *testing.T) {
	tests := []struct {
		name string
		expr ir.ConditionExpr
		want string
	}{
		{
			name: "simple compare",
			expr: ir.CondCompare{Variable: "ctx.outcome", Op: "=", Value: "success"},
			want: "ctx.outcome = success",
		},
		{
			name: "AND condition",
			expr: ir.CondAnd{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
			},
			want: "ctx.x = 1 and ctx.y = 2",
		},
		{
			name: "OR condition",
			expr: ir.CondOr{
				Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
				Right: ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "2"},
			},
			want: "ctx.x = 1 or ctx.x = 2",
		},
		{
			name: "NOT condition",
			expr: ir.CondNot{
				Inner: ir.CondCompare{Variable: "ctx.done", Op: "=", Value: "true"},
			},
			want: "not ctx.done = true",
		},
		{
			name: "nested AND/OR — AND inside OR needs parens",
			expr: ir.CondOr{
				Left: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
				Right: ir.CondCompare{Variable: "ctx.z", Op: "=", Value: "3"},
			},
			want: "(ctx.x = 1 and ctx.y = 2) or ctx.z = 3",
		},
		{
			name: "NOT of compound — parens around inner AND",
			expr: ir.CondNot{
				Inner: ir.CondAnd{
					Left:  ir.CondCompare{Variable: "ctx.x", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.y", Op: "=", Value: "2"},
				},
			},
			want: "not (ctx.x = 1 and ctx.y = 2)",
		},
		{
			name: "NOT of OR — parens around inner OR",
			expr: ir.CondNot{
				Inner: ir.CondOr{
					Left:  ir.CondCompare{Variable: "ctx.a", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.b", Op: "=", Value: "2"},
				},
			},
			want: "not (ctx.a = 1 or ctx.b = 2)",
		},
		{
			name: "OR inside AND needs parens",
			expr: ir.CondAnd{
				Left: ir.CondOr{
					Left:  ir.CondCompare{Variable: "ctx.a", Op: "=", Value: "1"},
					Right: ir.CondCompare{Variable: "ctx.b", Op: "=", Value: "2"},
				},
				Right: ir.CondCompare{Variable: "ctx.c", Op: "=", Value: "3"},
			},
			want: "(ctx.a = 1 or ctx.b = 2) and ctx.c = 3",
		},
		{
			name: "nil condition",
			expr: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCondition(tt.expr)
			if got != tt.want {
				t.Errorf("formatCondition() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTrailingNewline(t *testing.T) {
	w := minimalWorkflow()
	output := Format(w)
	if !strings.HasSuffix(output, "\n") {
		t.Error("output must end with a newline")
	}
	// Must be exactly one trailing newline
	trimmed := strings.TrimRight(output, "\n")
	if output != trimmed+"\n" {
		t.Errorf("output should have exactly one trailing newline, got %d",
			len(output)-len(trimmed))
	}
}

func TestFormatNoTrailingWhitespace(t *testing.T) {
	w := askAndExecuteWorkflow()
	output := Format(w)
	for i, line := range strings.Split(output, "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("line %d has trailing whitespace: %q", i+1, line)
		}
	}
}

func TestFormatBlankLineSeparation(t *testing.T) {
	w := askAndExecuteWorkflow()
	output := Format(w)

	// No triple-blank-lines
	if strings.Contains(output, "\n\n\n") {
		t.Error("output contains triple blank lines")
	}

	// Blank line between workflow header and defaults
	assertContains(t, output, "  exit: Done\n\n  defaults")
	// Blank line between defaults and first node
	assertContains(t, output, "    max_restarts: 5\n\n  human AskUser")
	// Blank line between nodes
	assertContains(t, output, "    mode: freeform\n\n  agent Interpret")
	// Blank line before edges
	assertContains(t, output, "      Ship it.\n\n  edges")
}

func TestFormatQuoting(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"two words", `"two words"`},
		{"path/to/file.dip", "path/to/file.dip"},
		{"has:colon", "has:colon"},
		{"has spaces and stuff!", `"has spaces and stuff!"`},
		{"", `""`},
		{"under_score", "under_score"},
		{"dash-case", "dash-case"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := quoteValue(tt.input)
			if got != tt.want {
				t.Errorf("quoteValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDefaultsCacheToolsAndCompaction(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Defaults: ir.WorkflowDefaults{
			CacheTools: true,
			Compaction: "rolling",
		},
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
		},
	}
	output := Format(w)
	assertContains(t, output, "    cache_tools: true")
	assertContains(t, output, "    compaction: rolling")
}

func TestFormatDefaultsRestartTarget(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Defaults: ir.WorkflowDefaults{
			RestartTarget: "Begin",
		},
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
		},
	}
	output := Format(w)
	assertContains(t, output, "    restart_target: Begin")
}

func TestFormatSubgraphNoParams(t *testing.T) {
	w := &ir.Workflow{
		Name:  "sub",
		Start: "S",
		Exit:  "S",
		Nodes: []*ir.Node{
			{
				ID:   "S",
				Kind: ir.NodeSubgraph,
				Config: ir.SubgraphConfig{
					Ref: "./other.dip",
				},
			},
		},
	}
	output := Format(w)
	assertContains(t, output, "    ref: ./other.dip")
	assertNotContains(t, output, "params:")
}

func TestFormatEdgeWeightOnly(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "B",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "go."}},
			{ID: "B", Kind: ir.NodeAgent, Config: ir.AgentConfig{Prompt: "done."}},
		},
		Edges: []*ir.Edge{
			{From: "A", To: "B", Weight: 5},
		},
	}
	output := Format(w)
	assertContains(t, output, "    A -> B  weight: 5")
}

func TestFormatNilWorkflowConfig(t *testing.T) {
	// A node with nil config should not panic
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{ID: "A", Kind: ir.NodeAgent}, // Config is nil
		},
	}
	output := Format(w)
	assertContains(t, output, "  agent A")
}

func TestFormatLabelOnSubgraph(t *testing.T) {
	w := &ir.Workflow{
		Name:  "sub",
		Start: "S",
		Exit:  "S",
		Nodes: []*ir.Node{
			{
				ID:    "S",
				Kind:  ir.NodeSubgraph,
				Label: "Code Review",
				Config: ir.SubgraphConfig{
					Ref: "./review.dip",
				},
			},
		},
	}
	output := Format(w)
	assertContains(t, output, `    label: "Code Review"`)
	assertContains(t, output, "    ref: ./review.dip")
}

func TestFormatBaseDelay(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{
				ID:     "A",
				Kind:   ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "go."},
				Retry: ir.RetryConfig{
					Policy:    "aggressive",
					BaseDelay: 500 * time.Millisecond,
				},
			},
		},
	}
	output := Format(w)
	assertContains(t, output, "retry_policy: aggressive")
	assertContains(t, output, "base_delay: 500ms")
}

func TestFormatBaseDelay_Zero_Omitted(t *testing.T) {
	w := &ir.Workflow{
		Name:  "test",
		Start: "A",
		Exit:  "A",
		Nodes: []*ir.Node{
			{
				ID:     "A",
				Kind:   ir.NodeAgent,
				Config: ir.AgentConfig{Prompt: "go."},
				Retry:  ir.RetryConfig{Policy: "standard"},
			},
		},
	}
	output := Format(w)
	assertNotContains(t, output, "base_delay")
}
