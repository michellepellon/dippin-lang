# Dippin Lint GitHub Action

A composite GitHub Action that lints and analyzes Dippin workflow files.

## Usage

```yaml
- uses: 2389-research/dippin-lang/.github/actions/dippin-lint@main
  with:
    files: 'workflows/**/*.dip'
    fail-on-warnings: 'true'
    cost-threshold: '5.00'
```

## Inputs

| Input | Description | Default |
|-------|-------------|---------|
| `version` | Dippin version to install | `latest` |
| `files` | Glob pattern for .dip files | `**/*.dip` |
| `fail-on-warnings` | Fail if lint warnings are found | `false` |
| `cost-threshold` | Maximum expected cost in USD (0 to disable) | `0` |

## What It Does

1. Installs `dippin` CLI
2. Finds all `.dip` files matching the glob pattern
3. Runs `dippin check --format json` on each file
4. Runs `dippin cost --format json` on each file
5. Produces a summary table in the GitHub Actions step summary
6. Fails if any file has errors (or warnings, if `fail-on-warnings` is set)
7. Fails if any file exceeds the cost threshold
