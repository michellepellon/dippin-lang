# Dippin LLM Reference Card

Compact reference for LLMs generating `.dip` workflow files. Paste into system prompts or tool descriptions.

---

## Grammar (simplified BNF)

```
workflow <Name>
  goal: "<text>"
  start: <NodeID>
  exit: <NodeID>

  [defaults
    model: <string>
    provider: <string>
    max_total_tokens: <int>
    max_cost_cents: <int>
    max_wall_time: <duration>
    ...]

  [vars
    <key>: <value>
    ...]

  <kind> <NodeID>
    <field>: <value>
    <multiline_field>:
      <indented content>

  parallel <ID> -> <Target1>, <Target2>[, ...]
  fan_in <ID> <- <Source1>, <Source2>[, ...]

  edges
    <From> -> <To> [when <condition>] [label: <text>] [weight: <int>] [restart: true]
```

---

## Node Kinds

| Kind | Required Fields | Optional Fields |
|------|----------------|-----------------|
| `agent` | `prompt` | `model`, `provider`, `backend`, `working_dir`, `auto_status`, `goal_gate`, `reasoning_effort`, `fidelity`, `max_turns`, `system_prompt` |
| `human` | `mode` (freeform\|choice) | `default`, `timeout` (Duration, e.g. 5m), `timeout_action` (String: fail\|default) |
| `tool` | `command` | `timeout` (e.g. 30s, 5m) |
| `parallel` | `-> Target1, Target2` (inline) | — |
| `fan_in` | `<- Source1, Source2` (inline) | — |
| `subgraph` | `ref` | `params` |

All kinds also accept: `label`, `reads`, `writes`, `retry_policy`, `max_retries`, `base_delay`, `retry_target`, `fallback_target`.

---

## Edge Conditions

```
when <variable> <op> <value>
when <expr> and <expr>
when <expr> or <expr>
when not <expr>
```

**Comparison operators:** `=`, `==`, `!=`, `contains`, `not contains`, `startswith`, `endswith`, `in` (all string comparison, no numeric ops)

**Variables:** Always namespace-qualified: `ctx.outcome`, `ctx.status`, `graph.goal`

---

## Common LLM Mistakes

| # | Mistake | Fix |
|---|---------|-----|
| 1 | Missing `start:` or `exit:` field | Every workflow needs both. They reference node IDs declared below. |
| 2 | Edge references undeclared node | Every node in an edge must be declared as `agent`, `human`, `tool`, etc. |
| 3 | `parallel` targets without matching `fan_in` sources | `parallel P -> A, B` requires `fan_in J <- A, B` with the same set. |
| 4 | Bare variable names in conditions | Use `ctx.outcome`, not `outcome`. All variables need a namespace prefix. |
| 5 | Agent node with empty prompt | Every `agent` node should have a `prompt:` field with content (except start/exit lifecycle nodes). |
| 6 | Missing tool timeout | Add `timeout: 60s` (or appropriate duration) to every `tool` node. |
| 7 | Exhaustive conditions flagged | `ctx.outcome = success` + `ctx.outcome = fail` is exhaustive — DIP101/DIP102 are auto-suppressed. No need to add a fallback edge. |

---

## Exhaustive Conditions

When outgoing edges from a node cover all possible values, DIP101 and DIP102 warnings are automatically suppressed. Known exhaustive sets:

- `ctx.outcome`: `{success, fail}` or `{success, failure}`
- `outcome`: `{success, fail}` or `{success, failure}`

This means the common pattern below is valid with zero warnings:

```
Gate -> Fix when ctx.outcome = fail
Gate -> Next when ctx.outcome = success
```

---

## Example: Conditional Routing

```
workflow ReviewPipeline
  goal: "Review code and route by outcome"
  start: Analyze
  exit: Done

  agent Analyze
    auto_status: true
    prompt:
      Analyze the code changes.
      Set STATUS: success if approved, STATUS: fail if changes needed.

  agent Approve
    prompt:
      Finalize the approved changes.

  agent RequestChanges
    prompt:
      Describe what changes are needed.

  agent Done
    prompt:
      Summarize the review outcome.

  edges
    Analyze -> Approve  when ctx.outcome = success
    Analyze -> RequestChanges  when ctx.outcome = fail
    Analyze -> Done
    Approve -> Done
    RequestChanges -> Done
```

---

## Identifiers & Reserved Words

**Identifiers:** `[a-zA-Z0-9][a-zA-Z0-9_\-./]*` — letters, digits, underscore, dash, dot, slash.

**Contextual keywords** (not reserved — usable as node IDs): `workflow`, `agent`, `human`, `tool`, `subgraph`, `parallel`, `fan_in`, `edges`, `defaults`, `when`, `and`, `or`, `not`, `true`, `false`, `restart`, `label`, `weight`.

---

## Validation with `dippin check`

Use `dippin check` in tool-calling loops to validate generated `.dip` files. It runs parse + validate + lint in one shot and outputs JSON to stdout:

```bash
dippin check my_workflow.dip
```

```json
{"valid":true,"errors":0,"warnings":0,"diagnostics":[],"suggested_actions":[]}
```

```json
{"valid":false,"errors":1,"warnings":2,"diagnostics":[{"code":"DIP003","severity":"error","message":"unknown node reference \"Nope\" in edge","line":19,"fix":""}],"suggested_actions":[]}
```

Use `valid` to decide whether to retry generation. Use `diagnostics` to feed error details back to the LLM for correction. Use `suggested_actions` for actionable fixes when available.

---

## Diagnostic Code Summary

39 diagnostic codes across two categories:

- **DIP001–DIP009** (errors): start/exit missing, unknown refs, unreachable nodes, cycles, duplicates, parallel/fan_in mismatch
- **DIP101–DIP133** (warnings): conditional reachability, missing defaults, overlapping conditions, unbounded retries, undefined variables, unknown models, empty prompts, missing timeouts, invalid policy/fidelity/reasoning_effort, stylesheet refs, namespace prefixes, condition type checking, structured output validation
