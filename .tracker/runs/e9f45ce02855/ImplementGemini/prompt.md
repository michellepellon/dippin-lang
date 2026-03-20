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
The plan is written to `.tracker/current_plan.md`. Here's a summary of what it covers:

## Plan Summary

**Component**: `validator/` — Graph structure validation (DIP001–DIP009)

### Files to Create (4 files)
| File | Purpose |
|------|---------|
| `validator/diagnostic.go` | `Diagnostic`, `Severity`, `Result` types — shared diagnostic infrastructure |
| `validator/codes.go` | Constants `DIP001`–`DIP009` with human-readable descriptions |
| `validator/validate.go` | `Validate(w *ir.Workflow) Result` entry point + 9 check functions + Levenshtein helper |
| `validator/validate_test.go` | 22 test cases |

### The 9 Checks
| Code | Check | Algorithm |
|------|-------|-----------|
| DIP001 | Start node exists | Field + lookup |
| DIP002 | Exit node exists | Field + lookup |
| DIP003 | All edge endpoints exist | Iterate edges, lookup nodes, fuzzy-match for "did you mean?" |
| DIP004 | All nodes reachable from start | BFS from start, report unvisited |
| DIP005 | No unconditional cycles | DFS with gray/black coloring on non-restart edges |
| DIP006 | Exit has no outgoing edges | `EdgesFrom(exit)` check |
| DIP007 | Parallel/fan_in pairing | Set-compare targets↔sources |
| DIP008 | No duplicate node IDs | Count map |
| DIP009 | No duplicate edges | Keyed on `(From, To, Condition.Raw)` |

### Test Coverage: 22 cases
- 4 happy-path (valid workflows including restart edges and parallel/fan_in)
- 11 individual error cases (one per rule + fuzzy match)
- 7 edge cases (empty workflow, multiple simultaneous errors, both endpoints dangling, same-endpoint-different-conditions, diagnostic formatting)

### Key Design Decisions Documented
- Restart edges excluded from cycle detection (per ADR 1)
- DIP009 dedup key includes condition raw text (conditional branches are not duplicates)
- DIP007 uses order-insensitive set matching for targets↔sources
- All checks run unconditionally (multi-diagnostic collection)
- Levenshtein ≤ 2 for "did you mean?" suggestions in DIP003