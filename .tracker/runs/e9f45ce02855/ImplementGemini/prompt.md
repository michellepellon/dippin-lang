Implement the planned component for the Dippin toolchain. Read .tracker/current_plan.md for exactly what to build and .tracker/current_context.md for the current codebase state.

Read the design spec at ../DIPPIN_DESIGN_PLAN.md for exact specifications.
Read existing code in ir/ to match types precisely.

Rules:
- Write idiomatic Go. Standard library only (no external deps beyond what go.mod already has).
- Every exported function gets a test. Test files go next to source.
- Match the IR types in ir/ exactly. Do not modify ir/ unless the plan says to.
- Table-driven tests with edge cases from the plan.
- One responsibility per package.
- Use diagnostic codes from the spec (DIP001-DIP112) where applicable.
- Write real, complete code — no stubs, no TODOs, no placeholders.
- Run 'go build ./...' and 'go test ./...' to verify before finishing.

---
# Context from Prior Pipeline Stages

## Previous Node Output
The plan is written to `.tracker/current_plan.md` (351 lines). Here's a summary of what it covers:

## Plan Summary: Migration Tool (`migrate/`)

### Component
DOT → IR → `.dip` conversion pipeline (§16 of design spec)

### Files to Create (4 files)
| File | Purpose |
|------|---------|
| `migrate/dot_parser.go` | Minimal DOT lexer + parser for the subset used by Tracker |
| `migrate/migrate.go` | `Migrate()` and `MigrateToSource()` — DOT string → IR with all cleanup |
| `migrate/parity.go` | `CheckParity()` — structural comparison of two workflows |
| `migrate/migrate_test.go` | 36+ test cases |

### Key Design Decisions
1. **Hand-written DOT parser** — only parses the subset we actually use (no subgraphs, no ports, no HTML labels)
2. **Shape → kind mapping** — reverse of the §15 export table, with special handling for `diamond` and `Mdiamond`/`Msquare`
3. **Legacy attribute names** — `llm_model`→`model`, `llm_provider`→`provider`, `loop_restart`→`restart`, `default_max_retry`→`max_retries`, `context.`→`ctx.`
4. **Condition parsing** — handles `=`/`!=`/`contains`/`&&`/`||`/`not` with namespace prefixing
5. **Start/Exit as agent nodes** — simpler approach, consistent with how the rest of the toolchain works

### Test Coverage (36 cases)
- 10 DOT parser tests (parsing, escapes, comments, errors)
- 17 migration tests (shape mapping, un-escaping, conditions, parallel inference, round-trip)
- 8 parity checker tests (identical, missing, extra, mismatch)
- 1 integration test against real `build_dippin.dot`