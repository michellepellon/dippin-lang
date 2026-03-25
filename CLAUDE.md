# CLAUDE.md

## Project

dippin-lang is a DSL and toolchain for authoring AI pipeline workflows. It replaces Graphviz DOT as the authoring format for Tracker pipelines.

## Build & Test

```sh
just check          # full suite: build, vet, fmt, test, complexity, examples
just test           # go test ./...
just complexity     # cyclomatic + cognitive complexity checks
just setup-hooks    # install pre-commit hook (required for commits)
```

Single package: `go test ./validator/ -run TestLint -count=1`

## Code Quality

Pre-commit hook enforces:
- Cyclomatic complexity ≤ 5 per function (`gocyclo`)
- Cognitive complexity ≤ 7 per function (`gocognit`)
- `gofmt` canonical formatting
- All tests pass
- All example `.dip` files validate

When a function exceeds complexity: extract helpers, don't add `//nolint` directives.

## Architecture

Everything flows through `ir.Workflow`. Packages import `ir` but not each other (except analysis packages that compose: doctor → validator + coverage + cost).

Key gotcha: The parser stores edge conditions as `Condition.Raw` (plain text). `Condition.Parsed` (AST) is only populated by `simulate.EnsureConditionsParsed()`. Any code reading `Condition.Parsed` must ensure it's been called first — `Lint()` does this automatically.

## Versioning

Tag semver releases after batches of meaningful changes. The tracker team installs via `go install ...@latest` and needs stable versions to pin to.

```sh
git tag -a v0.X.0 -m "description" && git push origin v0.X.0
```

## Model Catalog & Pricing

Model names and pricing in `validator/lint_model.go` and `cost/pricing.go` must be verified against official provider documentation before committing. Source URLs are maintained as code comments. Never use training data for pricing — it goes stale.

## Lint Rules

30 diagnostic codes: DIP001-DIP009 (structural errors), DIP101-DIP120 (semantic warnings). DIP101/DIP102 suppress automatically when source node conditions are exhaustive (success/fail pairs, contains/not-contains complementary pairs).

## Testing

Test fixtures should match real parser output. If the parser doesn't populate a field, tests shouldn't either. The DIP101 bug was caused by tests pre-populating `Condition.Parsed` by hand, masking that production code never set it.
