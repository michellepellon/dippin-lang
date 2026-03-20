# CLI Reference

The `dippin` command-line tool provides parsing, validation, formatting, export, and migration capabilities for `.dip` workflow files.

---

## Installation

Build from source:

```bash
go build -o dippin ./cmd/dippin
```

---

## Global Usage

```
dippin [--format text|json] <command> [args]
```

### Global Flags

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `--format` | `text`, `json` | `text` | Output format for diagnostics. `text` produces human-readable output. `json` produces machine-readable arrays for CI/tooling integration. |

---

## Exit Codes

All commands use consistent exit codes:

| Code | Meaning |
|------|---------|
| `0` | Success — no issues found, operation completed |
| `1` | Error — validation failures, parse errors, check-mode drift, parity mismatches |
| `2` | Usage error — bad flags, missing arguments, unknown command |

---

## Commands

### parse

Parse a workflow file and output the intermediate representation (IR) as JSON.

```bash
dippin parse <file>
```

**Input**: `.dip` or `.dot` file (auto-detected by extension)

**Output**: Indented JSON representation of the `ir.Workflow` struct, printed to stdout. This is useful for debugging, tooling integration, and inspecting how the parser interprets your workflow.

**Example**:
```bash
dippin parse pipeline.dip
```

```json
{
  "Name": "my_pipeline",
  "Version": "",
  "Goal": "Do a thing",
  "Start": "Ask",
  "Exit": "Done",
  "Nodes": [ ... ],
  "Edges": [ ... ]
}
```

---

### validate

Run structural validation checks (DIP001–DIP009) on a workflow.

```bash
dippin validate <file>
```

**Input**: `.dip` or `.dot` file

**Checks**: The 9 structural validation rules that must pass for a workflow to be executable. See [validation.md](validation.md) for details on each code.

**Output**:
- If all checks pass: `"validation passed"` (text mode) or empty JSON array
- If errors found: diagnostic messages to stderr

**Example**:
```bash
$ dippin validate pipeline.dip
validation passed

$ dippin validate broken.dip
error[DIP003]: unknown node reference "InterpretX" in edge
  --> broken.dip:45:5
  = help: did you mean "Interpret"?
```

---

### lint

Run both structural validation and semantic linting (DIP001–DIP009 + DIP101–DIP112).

```bash
dippin lint <file>
```

**Input**: `.dip` or `.dot` file

**Checks**: All 21 diagnostic rules. Errors (DIP001–DIP009) cause exit code 1. Warnings (DIP101–DIP112) are reported but don't affect the exit code.

**Output**: All diagnostics (errors and warnings) to stderr.

**Example**:
```bash
$ dippin lint pipeline.dip
warning[DIP111]: tool command has no timeout
  --> pipeline.dip:35:3

$ echo $?
0    # warnings don't cause failure
```

---

### fmt

Format a `.dip` file to canonical form.

```bash
dippin fmt [--check] [--write] <file>
```

**Flags**:

| Flag | Description |
|------|-------------|
| `--check` | Don't output anything. Exit 1 if the file is not already in canonical format. Useful for CI checks. |
| `--write` | Write the formatted output back to the source file in-place. |

**Default behavior** (no flags): Print the canonically formatted output to stdout.

**What canonical format means**:
- 2-space indentation
- Workflow header fields in standard order (goal, start, exit)
- Defaults block (if present) after header
- Node definitions ordered by kind
- Edges section at end
- Multiline fields (prompt, command) indented with `:` on the same line
- Deterministic, idempotent — formatting an already-formatted file produces identical output

**Examples**:
```bash
# Preview formatted output:
dippin fmt pipeline.dip

# Check in CI (fails if not formatted):
dippin fmt --check pipeline.dip

# Format in place:
dippin fmt --write pipeline.dip
```

---

### export-dot

Export a workflow to Graphviz DOT format for visualization.

```bash
dippin export-dot [--rankdir=LR|TB] [--prompts] <file>
```

**Flags**:

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `--rankdir` | `LR`, `TB` | `TB` | Graph layout direction. `TB` = top-to-bottom, `LR` = left-to-right. |
| `--prompts` | — | off | Include full prompt and command text as DOT node attributes. By default, prompts are omitted for cleaner visualization. |

**Node Shape Mapping**:

| Node Kind | DOT Shape | Note |
|-----------|-----------|------|
| `agent` | `box` | |
| `human` | `hexagon` | |
| `tool` | `parallelogram` | |
| `parallel` | `component` | |
| `fan_in` | `tripleoctagon` | |
| `subgraph` | `tab` | |
| start node | `Mdiamond` | Regardless of kind |
| exit node | `Msquare` | Regardless of kind |

**Special styling**:
- Goal gate nodes get a red filled background
- Restart edges are rendered as dashed lines

**Example**:
```bash
# Generate DOT and render as PNG:
dippin export-dot pipeline.dip | dot -Tpng -o pipeline.png

# Left-to-right layout with prompts:
dippin export-dot --rankdir=LR --prompts pipeline.dip > pipeline.dot
```

---

### migrate

Convert a DOT file to `.dip` source format.

```bash
dippin migrate [--output <file>] <file.dot>
```

**Flags**:

| Flag | Description |
|------|-------------|
| `--output <file>` | Write output to the specified file instead of stdout. |

**What it does**:
1. Parses the DOT file using a custom DOT parser
2. Maps DOT shapes to Dippin node kinds
3. Extracts graph-level attributes into a `defaults` block
4. Unescapes prompt and command text from DOT string encoding
5. Prefixes bare condition variables with `ctx.` namespace
6. Identifies start/exit from `Mdiamond`/`Msquare` shapes
7. Outputs canonical `.dip` source

**Example**:
```bash
# Preview the migration:
dippin migrate old_pipeline.dot

# Write to file:
dippin migrate --output new_pipeline.dip old_pipeline.dot
```

---

### validate-migration

Check structural parity between a DOT file and a `.dip` file to verify migration correctness.

```bash
dippin validate-migration <old.dot> <new.dip>
```

**What it checks**: Compares the IR produced from both files and reports structural differences — missing nodes, different edges, changed conditions, etc.

**Output**:
- If equivalent: `"migration parity check passed"`
- If differences found: List of differences with categories

**Example**:
```bash
$ dippin validate-migration old.dot new.dip
migration parity check passed

$ dippin validate-migration old.dot broken.dip
parity check failed: 2 difference(s) found
  [node] missing node "ReviewStep" in new file
  [edge] edge Validate->Approve has different condition
```

---

### help

Display global usage and command list.

```bash
dippin help
```

---

## JSON Output Mode

All commands support `--format json` for machine-readable output. Diagnostics are emitted as a JSON array:

```bash
dippin --format json lint pipeline.dip
```

```json
[
  {
    "code": "DIP111",
    "severity": "warning",
    "message": "tool command has no timeout",
    "location": {
      "file": "pipeline.dip",
      "line": 35,
      "column": 3,
      "end_line": 0,
      "end_column": 0
    },
    "help": "add a timeout field"
  }
]
```

---

## Auto-Detection

The CLI auto-detects input format by file extension:
- `.dip` — Parsed by the Dippin parser
- `.dot` — Parsed by the DOT migration parser

This applies to all commands that accept a file argument (`parse`, `validate`, `lint`, `export-dot`). The `migrate` command always expects a `.dot` input, and `fmt` always expects `.dip`.
