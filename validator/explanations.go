package validator

// Explanation provides detailed documentation for a single diagnostic code.
type Explanation struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
	Trigger string `json:"trigger"`
	Fix     string `json:"fix"`
	Example string `json:"example"`
}

// Explanations maps every DIP diagnostic code to its detailed explanation.
var Explanations = map[string]Explanation{
	DIP001: {
		Code:    DIP001,
		Summary: CodeDescription[DIP001],
		Trigger: "The workflow has no node with role \"start\".",
		Fix:     "Add a node with role start to serve as the entry point.",
		Example: "node Analyze { role agent }  // missing start node",
	},
	DIP002: {
		Code:    DIP002,
		Summary: CodeDescription[DIP002],
		Trigger: "The workflow has no node with role \"exit\".",
		Fix:     "Add a node with role exit to serve as the terminal node.",
		Example: "node Start { role start }\nnode Analyze { role agent }  // missing exit node",
	},
	DIP003: {
		Code:    DIP003,
		Summary: CodeDescription[DIP003],
		Trigger: "An edge references a node ID that does not exist in the workflow.",
		Fix:     "Correct the node name in the edge, or add the missing node.",
		Example: "Start -> Analize  // typo: \"Analize\" not defined",
	},
	DIP004: {
		Code:    DIP004,
		Summary: CodeDescription[DIP004],
		Trigger: "A node exists that cannot be reached by following edges from the start node.",
		Fix:     "Add an edge from an existing reachable node, or remove the orphan node.",
		Example: "node Orphan { role agent }  // no edge leads here",
	},
	DIP005: {
		Code:    DIP005,
		Summary: CodeDescription[DIP005],
		Trigger: "A cycle exists where every edge is unconditional, creating an infinite loop.",
		Fix:     "Add a condition on at least one edge in the cycle, or add a max_retries limit.",
		Example: "A -> B\nB -> A  // unconditional cycle",
	},
	DIP006: {
		Code:    DIP006,
		Summary: CodeDescription[DIP006],
		Trigger: "The exit node has one or more outgoing edges.",
		Fix:     "Remove all outgoing edges from the exit node.",
		Example: "node End { role exit }\nEnd -> Start  // exit must be terminal",
	},
	DIP007: {
		Code:    DIP007,
		Summary: CodeDescription[DIP007],
		Trigger: "A parallel fan-out does not have a matching fan-in node collecting all branches.",
		Fix:     "Add a fan_in node that collects all parallel branches before continuing.",
		Example: "node P { role parallel }\nP -> A\nP -> B\n// missing fan_in",
	},
	DIP008: {
		Code:    DIP008,
		Summary: CodeDescription[DIP008],
		Trigger: "Two or more nodes share the same ID.",
		Fix:     "Rename one of the duplicate nodes to a unique ID.",
		Example: "node Review { role agent }\nnode Review { role agent }  // duplicate",
	},
	DIP009: {
		Code:    DIP009,
		Summary: CodeDescription[DIP009],
		Trigger: "The same source-target-condition edge appears more than once.",
		Fix:     "Remove the duplicate edge declaration.",
		Example: "Start -> Analyze\nStart -> Analyze  // duplicate edge",
	},
}

func init() {
	addLintExplanations()
}

func addLintExplanations() {
	lintExplanations := buildLintExplanations()
	for k, v := range lintExplanations {
		Explanations[k] = v
	}
}

func buildLintExplanations() map[string]Explanation {
	return map[string]Explanation{
		DIP101: {
			Code:    DIP101,
			Summary: "unreachable node after conditional branches",
			Trigger: "A node follows conditional branches that already cover all outcomes.",
			Fix:     "Route the node through a condition, or remove it if unreachable.",
			Example: "A -> B [success]\nA -> C [failure]\nA -> D  // unreachable",
		},
		DIP102: {
			Code:    DIP102,
			Summary: "routing node has no default/unconditional edge",
			Trigger: "A routing node has conditional edges but no unconditional/default fallback.",
			Fix:     "Add an unconditional edge or ensure conditions are exhaustive (success/failure pair).",
			Example: "Router -> A [success]\n// missing default edge from Router",
		},
		DIP103: {
			Code:    DIP103,
			Summary: "overlapping or contradictory conditions",
			Trigger: "Two conditions on edges from the same node can both be true simultaneously.",
			Fix:     "Make conditions mutually exclusive or remove the overlapping condition.",
			Example: "A -> B [score > 5]\nA -> C [score > 3]  // overlaps",
		},
		DIP104: {
			Code:    DIP104,
			Summary: "unbounded retry loop (no max_retries or fallback)",
			Trigger: "A retry loop has no max_retries limit and no fallback exit path.",
			Fix:     "Set max_retries on the node or add a fallback edge.",
			Example: "node Retry { role agent }\nRetry -> Retry [failure]  // no bound",
		},
		DIP105: {
			Code:    DIP105,
			Summary: "no success path from start to exit",
			Trigger: "No path of success-condition edges leads from start to exit.",
			Fix:     "Ensure at least one success path connects start to exit.",
			Example: "Start -> A [failure]\nA -> End  // no success path",
		},
		DIP106: {
			Code:    DIP106,
			Summary: "undefined variable reference in prompt",
			Trigger: "A prompt template references a variable like {{name}} that is never defined.",
			Fix:     "Add the variable to an upstream node's writes, or fix the variable name.",
			Example: "node A { prompt \"Hello {{user}}\" }  // user not defined",
		},
		DIP107: {
			Code:    DIP107,
			Summary: "unused context key (written but never read)",
			Trigger: "A node writes a context key that no downstream node ever reads.",
			Fix:     "Remove the unused write, or add a downstream read for the key.",
			Example: "node A { writes [temp_data] }  // never read downstream",
		},
		DIP108: {
			Code:    DIP108,
			Summary: "unknown model/provider combination",
			Trigger: "A node specifies a model or provider not in the known catalog.",
			Fix:     "Use a supported model/provider combination from the catalog.",
			Example: "node A { model gpt-5-turbo provider openai }  // unknown model",
		},
		DIP109: {
			Code:    DIP109,
			Summary: "namespace collision in imports",
			Trigger: "Two imports declare the same namespace prefix.",
			Fix:     "Rename one of the imports to use a unique namespace.",
			Example: "import \"a.dip\" as lib\nimport \"b.dip\" as lib  // collision",
		},
		DIP110: {
			Code:    DIP110,
			Summary: "empty prompt on agent node",
			Trigger: "An agent node has no prompt or an empty prompt string.",
			Fix:     "Add a non-empty prompt to the agent node.",
			Example: "node A { role agent prompt \"\" }  // empty prompt",
		},
		DIP111: {
			Code:    DIP111,
			Summary: "tool command has no timeout",
			Trigger: "A tool node runs a command but has no timeout configured.",
			Fix:     "Add a timeout to the tool node to prevent indefinite hangs.",
			Example: "node T { role tool command \"curl ...\" }  // no timeout",
		},
		DIP112: {
			Code:    DIP112,
			Summary: "reads key not produced by any upstream writes",
			Trigger: "A node reads a context key that no upstream node writes.",
			Fix:     "Add the key to an upstream node's writes, or fix the key name.",
			Example: "node B { reads [summary] }  // no upstream writes summary",
		},
		DIP113: {
			Code:    DIP113,
			Summary: "invalid retry policy name",
			Trigger: "A node specifies a retry_policy value not in the allowed set.",
			Fix:     "Use a valid retry policy: exponential, linear, or fixed.",
			Example: "node A { retry_policy aggressive }  // invalid policy",
		},
		DIP114: {
			Code:    DIP114,
			Summary: "invalid fidelity level",
			Trigger: "A node specifies a fidelity level not in the allowed set.",
			Fix:     "Use a valid fidelity: low, medium, or high.",
			Example: "node A { fidelity maximum }  // invalid fidelity",
		},
		DIP115: {
			Code:    DIP115,
			Summary: "goal_gate node without retry or fallback target",
			Trigger: "A goal_gate node has no retry edge or fallback target configured.",
			Fix:     "Add a retry edge or fallback target to the goal_gate node.",
			Example: "node G { role goal_gate }  // no retry or fallback",
		},
		DIP116: {
			Code:    DIP116,
			Summary: "invalid compaction threshold or on_resume value",
			Trigger: "A compaction block has an invalid threshold or on_resume value.",
			Fix:     "Use a valid threshold (0.0-1.0) and on_resume (summarize or truncate).",
			Example: "compaction { threshold 1.5 }  // out of range",
		},
		DIP117: {
			Code:    DIP117,
			Summary: "stylesheet class references undefined class",
			Trigger: "A stylesheet uses .class that is not defined in any style block.",
			Fix:     "Define the referenced class or fix the class name.",
			Example: "style .highlight { color red }\nnode A { class bold }  // .bold undefined",
		},
		DIP118: {
			Code:    DIP118,
			Summary: "stylesheet ID references unknown node",
			Trigger: "A stylesheet uses #id targeting a node that does not exist.",
			Fix:     "Fix the node ID in the stylesheet or add the missing node.",
			Example: "style #Reviewr { color blue }  // typo in node ID",
		},
		DIP119: {
			Code:    DIP119,
			Summary: "invalid reasoning_effort value",
			Trigger: "A node specifies a reasoning_effort value outside the allowed set.",
			Fix:     "Use a valid reasoning_effort: low, medium, or high.",
			Example: "node A { reasoning_effort extreme }  // invalid value",
		},
		DIP120: {
			Code:    DIP120,
			Summary: "condition variable missing namespace prefix",
			Trigger: "A condition references a variable without the required namespace prefix.",
			Fix:     "Add the namespace prefix to the variable (e.g., ctx.variable).",
			Example: "A -> B [status == done]  // should be ctx.status",
		},
		DIP121: {
			Code:    DIP121,
			Summary: "condition references variable not in source node writes",
			Trigger: "An edge condition references a variable that the source node does not declare in its writes.",
			Fix:     "Add the variable to the source node's writes, or use a reserved runtime variable.",
			Example: "node A { writes [result] }\nA -> B [ctx.score = high]  // score not in A's writes",
		},
		DIP122: {
			Code:    DIP122,
			Summary: "condition tests value not in source tool outputs",
			Trigger: "An edge condition tests a value that the source tool node does not declare in its outputs.",
			Fix:     "Add the value to the tool's outputs, or check for typos.",
			Example: "node T { role tool outputs [success, fail] }\nT -> B [ctx.outcome = retry]  // retry not declared",
		},
		DIP123: {
			Code:    DIP123,
			Summary: "tool command has shell syntax errors",
			Trigger: "bash -n reports a syntax error in the tool command block (unclosed quotes, bad redirects, etc.).",
			Fix:     "Fix the shell syntax error. Test the command with: echo '...' | bash -n",
			Example: "tool T\n  command:\n    echo \"unclosed",
		},
		DIP124: {
			Code:    DIP124,
			Summary: "tool command references runtime-only ${ctx.*} variable",
			Trigger: "A tool command contains ${ctx.*} interpolation. These are Dippin runtime variables that expand to empty in the shell.",
			Fix:     "Pass context values via environment variables or file IPC instead of ${ctx.*} in commands.",
			Example: "tool T\n  command:\n    curl ${ctx.api_url}",
		},
		DIP125: {
			Code:    DIP125,
			Summary: "tool command binary not found on PATH",
			Trigger: "The first command in the tool block references a binary not on the current PATH.",
			Fix:     "Install the missing binary or use a full path. This check is environment-dependent.",
			Example: "tool T\n  command:\n    npx create-nx-workspace",
		},
	}
}
