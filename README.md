# Dippin

A language and toolchain for authoring AI pipeline workflows.

Dippin replaces Graphviz DOT as the authoring format for [Tracker](https://github.com/2389-research/tracker) pipelines. Prompts, shell scripts, model configuration, and branching logic get first-class syntax instead of being crammed into DOT string attributes.

```
workflow CodeReview
  goal: "Review a pull request with multiple models"
  start: Fetch
  exit: Done

  tool Fetch
    timeout: 30s
    command:
      #!/bin/sh
      gh pr diff $PR_NUMBER > /tmp/diff.txt
      printf 'fetched'

  parallel ReviewFan -> Claude, GPT, Gemini

  agent Claude
    model: claude-opus-4-6
    provider: anthropic
    prompt:
      Review this pull request for correctness and security.
      Be thorough but concise.

  agent GPT
    model: gpt-5.2
    provider: openai
    prompt:
      Review this pull request for performance and maintainability.

  agent Gemini
    model: gemini-3-pro
    provider: gemini
    prompt:
      Review this pull request for test coverage and edge cases.

  fan_in ReviewJoin <- Claude, GPT, Gemini

  agent Done
    prompt: "Synthesize all reviews into a final report."

  edges
    Fetch -> ReviewFan
    ReviewJoin -> Done
```

## Install

```
go install github.com/2389/dippin/cmd/dippin@latest
```

Or build from source:

```
git clone https://github.com/2389-research/dippin-lang.git
cd dippin-lang
go build -o dippin ./cmd/dippin/
```

## Commands

```
dippin parse <file>                           Parse and output IR as JSON
dippin validate <file>                        Structural validation (DIP001-DIP009)
dippin lint <file>                            Validation + semantic warnings (DIP101-DIP112)
dippin fmt [--check] [--write] <file>         Format to canonical style
dippin export-dot [--rankdir LR] <file>       Export to Graphviz DOT
dippin migrate [--output <file>] <file.dot>   Convert legacy DOT to Dippin
dippin validate-migration <old.dot> <new.dip> Verify migration parity
dippin simulate [--scenario k=v] <file>       Dry-run the execution graph
```

All commands return exit 0 (ok), 1 (errors found), or 2 (usage error).
Use `--format json` on any command for machine-readable diagnostics.

## Syntax Reference

### Workflow Header

```
workflow <Name>
  goal: "Human-readable objective"
  start: <StartNodeID>
  exit: <ExitNodeID>

  defaults
    model: claude-sonnet-4-6
    provider: anthropic
    max_retries: 3
    fidelity: summary:medium
```

### Node Types

**Agent** — LLM call:

```
  agent Analyze
    model: claude-opus-4-6
    provider: anthropic
    reasoning_effort: high
    fidelity: full:high
    goal_gate: true
    auto_status: true
    prompt:
      Multi-line prompt text.
      No escaping needed — write naturally.
```

**Tool** — shell command:

```
  tool Build
    timeout: 60s
    command:
      #!/bin/sh
      set -eu
      go build ./...
      go test ./...
      printf 'done'
```

**Human** — input gate:

```
  human Approve
    mode: choice
    default: approve
```

**Parallel / Fan-in** — concurrent execution:

```
  parallel ReviewFan -> ReviewA, ReviewB, ReviewC
  fan_in ReviewJoin <- ReviewA, ReviewB, ReviewC
```

**Subgraph** — embedded sub-workflow:

```
  subgraph Nested
    ref: other_workflow.dip
```

### Edges

```
  edges
    A -> B
    A -> C when ctx.outcome == "fail"
    A -> D when ctx.outcome == "success" and ctx.retries < 3
    Retry -> Start restart: true
    A -> B weight: 10
    A -> B label: "happy path"
```

Conditions support: `==`, `!=`, `<`, `>`, `<=`, `>=`, `and`, `or`, `not`, `contains`, `startswith`, `endswith`.

### Node Fields

| Field | Applies to | Description |
|-------|-----------|-------------|
| `label` | all | Human-readable display name |
| `model` | agent | LLM model override |
| `provider` | agent | LLM provider (anthropic, openai, gemini) |
| `prompt` | agent | Prompt text (multiline OK) |
| `system_prompt` | agent | System prompt (multiline OK) |
| `reasoning_effort` | agent | low, medium, high |
| `fidelity` | agent | summary:low, summary:medium, summary:high, full:high |
| `goal_gate` | agent | Pipeline fails if this node fails |
| `auto_status` | agent | Parse STATUS: from response |
| `max_turns` | agent | Max LLM conversation turns |
| `command` | tool | Shell command (multiline OK) |
| `timeout` | tool | Execution timeout (e.g., 60s, 5m) |
| `mode` | human | freeform or choice |
| `default` | human | Default choice value |
| `retry_policy` | all | standard, aggressive, patient, linear, none |
| `max_retries` | all | Max retry attempts |
| `retry_target` | all | Node to jump to on retry |
| `fallback_target` | all | Fallback if retries exhausted |
| `reads` | all | Context keys this node reads (advisory) |
| `writes` | all | Context keys this node writes (advisory) |

### Comments

```
  # This is a comment
  agent Foo  # Inline comment
```

### Multiline Blocks

Any field followed by a colon and a newline treats the indented block below as raw text. No escaping required — shell scripts, markdown, JSON, anything:

```
  agent Writer
    prompt:
      # This is a markdown header inside the prompt
      Write code that does:
      ```python
      print("hello -> world")
      ```

  tool Runner
    command:
      #!/bin/bash
      if [ -f go.mod ]; then
        go test ./... 2>&1
      fi
```

## Diagnostics

### Validation Errors (DIP001–DIP009)

| Code | Description |
|------|-------------|
| DIP001 | Start node not declared or missing from nodes |
| DIP002 | Exit node not declared or missing from nodes |
| DIP003 | Edge references unknown node (with typo suggestions) |
| DIP004 | Node unreachable from start |
| DIP005 | Unconditional cycle detected (restart edges excluded) |
| DIP006 | Exit node has outgoing edges |
| DIP007 | Parallel/fan_in target mismatch |
| DIP008 | Duplicate node ID |
| DIP009 | Duplicate edge |

### Lint Warnings (DIP101–DIP112)

| Code | Description |
|------|-------------|
| DIP101 | Node only reachable via conditional edges |
| DIP102 | Conditional edges with no unconditional fallback |
| DIP103 | Overlapping edge conditions |
| DIP104 | Unbounded retries (no max_retries set) |
| DIP106 | Unknown variable namespace in ${...} |
| DIP107 | Node reads context key no upstream node writes |
| DIP108 | Unknown model or provider |
| DIP110 | Agent node with empty prompt |
| DIP111 | Tool node with no timeout |
| DIP112 | I/O flow analysis — reads without upstream writes |

## Simulation

Dry-run a workflow to see the execution path without calling LLMs or running commands:

```
$ dippin simulate examples/ask_and_execute.dip
{"event":"pipeline_start","workflow":"AskAndExecute","timestamp":"..."}
{"event":"node_enter","node":"Start","kind":"agent","timestamp":"..."}
{"event":"node_exit","node":"Start","status":"success","timestamp":"..."}
{"event":"edge_traverse","from":"Start","to":"SetupWorkspace","timestamp":"..."}
...
{"event":"pipeline_end","status":"success","nodes_visited":12,"timestamp":"..."}
```

Explore different paths by injecting context values:

```
$ dippin simulate examples/stress_graph_monster.dip --scenario outcome=fail
```

## Examples

The `examples/` directory contains 15 workflows:

**Migrated from Tracker** (`.dip` + original `.dot`):
- `ask_and_execute` — multi-model implementation with human approval
- `consensus_task` / `consensus_task_parity` — consensus-driven task execution
- `human_gate_showcase` — human gate patterns
- `megaplan` / `megaplan_quality` — multi-phase sprint planning
- `semport` / `semport_thematic` — semantic porting workflows
- `sprint_exec` — sprint execution pipeline
- `vulnerability_analyzer` — security analysis pipeline

**Stress tests** (parser and graph edge cases):
- `stress_shell_hell` — here-docs, case/esac, process substitution, traps
- `stress_prompt_chaos` — markdown, code blocks, Dippin-like syntax in prompts
- `stress_graph_monster` — 20 nodes, 4 parallel groups, conditional routing, restarts
- `stress_edge_cases` — every field, empty prompts, colons in values
- `stress_adversarial` — Dippin grammar in prompts, unicode, deeply nested shells

## Architecture

```
.dip source ──→ Parser ──→ IR (Workflow) ──→ Validator (DIP001-009)
                                          ──→ Linter (DIP101-112)
                                          ──→ Formatter (canonical .dip)
                                          ──→ DOT Exporter (visualization)
                                          ──→ Simulator (dry-run)

.dot file ─────→ Migrator ──→ IR ──→ .dip source
```

All packages program against `ir.Workflow` as the central contract:

| Package | Purpose |
|---------|---------|
| `ir/` | Core types: Workflow, Node, Edge, Condition AST, typed NodeConfig |
| `parser/` | Indentation-aware lexer + recursive-descent parser |
| `validator/` | Structural validation and semantic linting |
| `formatter/` | Canonical pretty-printer (idempotent) |
| `export/` | DOT export for Graphviz visualization |
| `migrate/` | DOT-to-Dippin conversion + parity checker |
| `simulate/` | Reference executor with event protocol |
| `event/` | Canonical event types for execution protocols |
| `cmd/dippin/` | CLI |

## License

MIT
