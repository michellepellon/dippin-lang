# Scenario Testing

The `dippin test` command runs scenario-based assertions against workflow files. Test cases inject context values into the simulator and verify the execution path.

---

## Test File Format

Test files use `.test.json` extension and are auto-discovered from the workflow path:

```
pipeline.dip       → pipeline.test.json
src/flow.dip       → src/flow.test.json
```

### Schema

```json
{
  "tests": [
    {
      "name": "descriptive test name",
      "scenario": {
        "key": "value"
      },
      "expect": {
        "status": "success",
        "visited": ["NodeA", "NodeB"],
        "not_visited": ["NodeC"],
        "path_contains": ["NodeA", "NodeB"]
      }
    }
  ]
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tests` | array | yes | List of test cases |
| `tests[].name` | string | yes | Human-readable test name (shown in output) |
| `tests[].scenario` | object | no | Context key/value pairs injected into the simulator. These determine which conditional edges are taken. |
| `tests[].expect` | object | yes | Assertions to check against the simulation result |

### Expectation Fields

All expectation fields are optional. Only specified fields are checked.

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Expected simulation status: `"success"` (reached exit), `"fail"`, or `"dead_end"` |
| `visited` | string[] | Node IDs that must appear in the execution path |
| `not_visited` | string[] | Node IDs that must NOT appear in the execution path |
| `path_contains` | string[] | Node IDs that must appear **in order** in the execution path. Non-contiguous matches are allowed (other nodes may appear between them). |

---

## Example

Given a workflow `gate.dip`:

```dippin
workflow Gate
  goal: "Route based on outcome"
  start: Start
  exit: Exit

  agent Start
    label: Start

  agent Exit
    label: Exit

  agent Pass
    label: Pass
    model: claude-sonnet-4-6
    provider: anthropic
    prompt: Handle success.

  agent Fix
    label: Fix
    model: claude-sonnet-4-6
    provider: anthropic
    prompt: Handle failure.

  edges
    Start -> Pass  when ctx.outcome = success
    Start -> Fix  when ctx.outcome = fail
    Pass -> Exit
    Fix -> Exit
```

Create `gate.test.json`:

```json
{
  "tests": [
    {
      "name": "success path",
      "scenario": {"outcome": "success"},
      "expect": {
        "status": "success",
        "visited": ["Start", "Pass", "Exit"],
        "not_visited": ["Fix"]
      }
    },
    {
      "name": "failure path",
      "scenario": {"outcome": "fail"},
      "expect": {
        "status": "success",
        "visited": ["Start", "Fix", "Exit"],
        "not_visited": ["Pass"],
        "path_contains": ["Start", "Fix", "Exit"]
      }
    }
  ]
}
```

Run:

```bash
$ dippin test gate.dip
═══ Test Results ═════════════════════════════════════════════
  PASS  success path
  PASS  failure path
─── Summary ───────────────────────────────────────────────────
  2 tests: 2 passed, 0 failed

$ dippin test --verbose gate.dip
═══ Test Results ═════════════════════════════════════════════
  PASS  success path
        path: Start → Pass → Exit
  PASS  failure path
        path: Start → Fix → Exit
─── Summary ───────────────────────────────────────────────────
  2 tests: 2 passed, 0 failed
```

---

## Scenario Keys

The `scenario` object maps context keys to values. These are injected into the simulator's context before execution begins. The simulator uses these values to resolve conditional edges.

Common scenario keys:

| Key | Description |
|-----|-------------|
| `outcome` | Maps to `ctx.outcome` — the most common routing variable |
| `status` | Maps to `ctx.status` |
| `tool_stdout` | Maps to `ctx.tool_stdout` — tool command output |

The simulator resolves conditions by looking up `ctx.<key>` in the scenario context. For example, `scenario: {"outcome": "fail"}` causes `when ctx.outcome = fail` edges to match.

---

## CI Integration

Use `--format json` for machine-readable output:

```bash
dippin --format json test pipeline.dip
```

```json
{
  "results": [
    {"name": "happy path", "passed": true, "path": ["Start", "Gate", "Pass", "Exit"]},
    {"name": "error path", "passed": false, "errors": ["expected status \"fail\", got \"success\""]}
  ],
  "passed": 1,
  "failed": 1,
  "total": 2
}
```

Exit code is 0 if all tests pass, 1 if any fail.
