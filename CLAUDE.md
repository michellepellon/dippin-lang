# CLAUDE.md

## Project

dippin-lang is a DSL and toolchain for authoring AI pipeline workflows. It replaces Graphviz DOT as the authoring format for Tracker pipelines.

## Build & Test — always use `just`

All common operations go through the justfile. Never run raw `go build`, `go test`, `gocyclo`, etc. directly — use the corresponding `just` recipe. If you find yourself running a command repeatedly that isn't in the justfile, add a recipe for it first.

```sh
just check              # full suite: build, vet, fmt, test-race, complexity, validate-examples
just test               # go test ./... -count=1
just test-race          # go test ./... -count=1 -race
just test-pkg validator # test a single package with -v
just build              # build the dippin binary
just install            # go install to $GOBIN
just vet                # go vet ./...
just fmt                # gofmt -w .
just fmt-check          # check formatting (CI-style, exit 1 if unformatted)
just complexity         # cyclomatic ≤ 5 + cognitive ≤ 7 (excludes tests)
just validate-examples  # run dippin validate on all examples/*.dip
just lint-examples      # run dippin lint on all examples/*.dip
just cover              # generate test coverage report
just cover-html         # open coverage in browser
just setup-hooks        # install pre-commit hook (required for first checkout)
just clean              # remove build artifacts
```

## Code Quality

Pre-commit hook enforces:
- Cyclomatic complexity ≤ 5 per function (`gocyclo`)
- Cognitive complexity ≤ 7 per function (`gocognit`)
- `gofmt` canonical formatting
- All tests pass
- All example `.dip` files validate

When a function exceeds complexity: extract helpers, don't add `//nolint` directives.

## Architecture

Everything flows through `ir.Workflow`. Packages import `ir` but not each other (except analysis packages that compose: doctor → validator + coverage + cost, unused → coverage + cost).

Key gotcha: The parser stores edge conditions as `Condition.Raw` (plain text). `Condition.Parsed` (AST) is only populated by `simulate.EnsureConditionsParsed()`. Any code reading `Condition.Parsed` must ensure it's been called first — `Lint()` does this automatically.

## Versioning

Tag semver releases after batches of meaningful changes. The tracker team installs via `go install ...@latest` and needs stable versions to pin to. Update CHANGELOG.md when tagging.

```sh
git tag -a v0.X.0 -m "description" && git push origin v0.X.0
```

GoReleaser is configured (`.goreleaser.yml`) — pushing a tag triggers GitHub Actions to build cross-platform binaries and publish to Homebrew tap.

## Model Catalog & Pricing

Model names and pricing in `validator/lint_model.go` and `cost/pricing.go` must be verified against official provider documentation before committing. Source URLs and "Last verified" dates are maintained as code comments. Never use training data for pricing — it goes stale.

Supported providers: Anthropic, OpenAI, Google/Gemini, DeepSeek, xAI/Grok, Mistral, Cohere.

`TestLintExamples` in `validator/lint_examples_test.go` parses all example .dip files through the real parser and asserts zero DIP108 warnings — this catches model catalog staleness and invalid model IDs.

## Lint Rules

32 diagnostic codes: DIP001-DIP009 (structural errors), DIP101-DIP122 (semantic warnings). DIP101/DIP102 suppress automatically when source node conditions are exhaustive (success/fail pairs, contains/not-contains complementary pairs). DIP121/DIP122 only fire when source nodes declare writes/outputs (advisory metadata).

## Testing

Test fixtures should match real parser output. If the parser doesn't populate a field, tests shouldn't either. The DIP101 bug was caused by tests pre-populating `Condition.Parsed` by hand, masking that production code never set it.

Integration test `TestLintExamples` runs every example through parse → lint to catch regressions that unit tests with hand-built IR would miss.
