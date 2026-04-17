# Design: `dippin spec` — Agent-Downloadable Language Reference

## Problem

Agents (AI coding assistants) need a fast, reliable way to download the full dippin language reference so they can start writing `.dip` files. Currently the reference is spread across multiple docs, a skill file, and an llms.txt that only contains links. There's no single comprehensive document an agent can consume in one fetch.

## Solution

A build-time-generated spec file embedded into the `dippin` binary via `go:embed`. The same file is published to `dippin.org/llms-full.txt` for web access.

Implementation note: `docs/generated-spec.md` remains the scratch output from the generator, while the checked-in release copies live at `cmd/dippin/generated-spec.md` and `site/static/llms-full.txt`. This keeps fresh clones and module tags buildable without requiring a pre-build generation step.

## Components

### 1. Spec Generator Script (`scripts/gen-spec.sh`)

Assembles `docs/generated-spec.md` by concatenating sections from existing source files. Single source of truth — all content comes from files that are already maintained.

**Source files and what gets extracted:**

| Section | Source | Content |
|---------|--------|---------|
| Header | Inline in script | Title, description, installation |
| Grammar | `docs/llm-reference.md` | Compact BNF, node kinds table, edge conditions, condition operators, context fields |
| Syntax details | `site/static/skill.md` | File structure, defaults block, common fields, multiline syntax, indentation rules |
| Diagnostic codes | `site/static/skill.md` | All 39 codes (DIP001-DIP009 errors, DIP101-DIP133 warnings) |
| CLI reference | `site/static/skill.md` | Command summary (authoring, export, analysis, testing) |
| Providers | `site/static/skill.md` | Supported providers list |

**Output:** `docs/generated-spec.md` scratch output plus checked-in copies at `cmd/dippin/generated-spec.md` and `site/static/llms-full.txt` (~800-1200 lines, no examples)

The script uses `sed`/`awk` to extract sections by heading markers. Each source file has stable heading structure that won't drift.

### 2. CLI Command (`cmd/dippin/cmd_spec.go`)

```go
//go:embed generated-spec.md
var specContent string

func (c *CLI) CmdSpec(args []string) ExitCode {
    fmt.Fprint(c.Stdout, specContent)
    return ExitOK
}
```

- Registered in `commandDispatch()` as `"spec"`
- Added to `printGlobalUsage()` help text
- No flags — dumps the full spec to stdout
- Piping friendly: `dippin spec | pbcopy`, `dippin spec > ref.md`

### 3. Web Distribution

During generation, `docs/generated-spec.md` is copied to `site/static/llms-full.txt`.

**Netlify headers** (added to `netlify.toml`):
```toml
[[headers]]
  for = "/llms-full.txt"
  [headers.values]
    Content-Type = "text/plain; charset=utf-8"
```

**Accessible at:** `https://dippin.org/llms-full.txt`

### 4. Build Integration

**New justfile recipes:**

```
# Generate the spec from source docs
gen-spec:
    ./scripts/gen-spec.sh

# Existing recipes updated:
build: gen-spec    # ensures embed is fresh before compile
site-build: build wasm changelog-md
    cd site && hugo --minify
```

The `gen-spec` dependency on `build` ensures the embedded content and tracked web copy are always current. CI and local builds both produce correct binaries.

### 5. Existing llms.txt

The current `llms.txt` (Hugo-generated, link-based) remains as-is. It serves the `llms.txt` convention (index of links). The new `llms-full.txt` serves the "full content" convention. Both are standard.

## File Changes

| File | Change |
|------|--------|
| `scripts/gen-spec.sh` | New — assembles spec from source docs |
| `docs/generated-spec.md` | New (generated) — the scratch spec used to refresh the checked-in copies |
| `cmd/dippin/generated-spec.md` | New (generated, tracked) — embedded source for fresh clones and module tags |
| `site/static/llms-full.txt` | Generated, tracked — published web copy kept in sync by `gen-spec` |
| `cmd/dippin/cmd_spec.go` | New — `dippin spec` command |
| `cmd/dippin/cmd_spec_test.go` | New — tests for the spec command |
| `cmd/dippin/cli.go` | Add `"spec"` to dispatch map and help text |
| `Justfile` | Add `gen-spec` recipe, update `build` and `site-build` dependencies |
| `netlify.toml` | Add `llms-full.txt` header |
| `.gitignore` | Ignore only `docs/generated-spec.md` scratch output |

## Testing

- `cmd_spec_test.go`: Verify `dippin spec` outputs non-empty content containing expected markers (e.g., "workflow", "DIP001", "agent")
- `just check` continues to pass (spec generation is part of build)
- Manual: `dippin spec | wc -l` confirms expected line count range

## Non-Goals

- No `--latest` network fetch flag
- No interactive output (no pager, no formatting beyond what's in the markdown)
- No examples in the spec (agents can use `dippin new <template>` for that)
- No changes to the existing `skill.md` or `llms.txt`
