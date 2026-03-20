# Quick Reference: Complexity Refactoring

## Current State (post-round-1)

- **112 production functions** still exceed cyclomatic complexity threshold (5)
- **Top offender:** `migrate.compareConfigs` with complexity **23**
- Round 1 completed: `lexLine` (34→done), `applyNodeField` (29→done), `lintReadsWithoutUpstreamWrites` (28→done)

## Top 10 Remaining (production only)

| Complexity | Function | File |
|-----------|----------|------|
| 23 | `migrate.compareConfigs` | `migrate/parity.go:139` |
| 18 | `main.Run` | `cmd/dippin/cli.go:53` |
| 17 | `simulate.(*pathEnumerator).explore` | `simulate/simulate.go:562` |
| 17 | `simulate.(*simulator).visitNode` | `simulate/simulate.go:225` |
| 17 | `parser.(*Lexer).lex` | `parser/lexer.go:125` |
| 17 | `migrate.(*lexer).skipWhitespace` | `migrate/dot_parser.go:85` |
| 17 | `main.(*CLI).CmdSimulate` | `cmd/dippin/cli.go:596` |
| 16 | `parser.(*Parser).parseEdges` | `parser/parser.go:358` |
| 16 | `migrate.(*lexer).next` | `migrate/dot_parser.go:116` |
| 15 | `validator.lintSuccessPath` | `validator/lint.go:211` |

## Justified Exceptions (leave as-is)

1. **`migrate.tokenKindName`** (12) — simple token-to-string mapping
2. **`export.isSimpleDOTID`** (12) — character validation rules
3. **`migrate.(*lexer).skipWhitespace`** (17) — tight state machine

## Refactoring Pattern

All refactorings use **Extract Method**:

```go
// Before: one function with many responsibilities
func complexFunction() { /* 30+ complexity */ }

// After: orchestrator + focused helpers
func complexFunction() {
    doA()
    doB()
    doC()
}
```

## Tools

```bash
gocyclo -over 5 .                    # all functions over threshold
gocyclo -over 5 . | grep -v _test.go # production only
gocyclo -top 20 .                    # top 20 by complexity
go test ./...                        # verify no breakage
```
