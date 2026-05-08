---
title: "What's New in Dippin v0.24"
date: "2026-05-08"
description: "The .dipx bundle format — a deterministic, content-addressed ZIP that ships a workflow plus every transitive subgraph as a single integrity-verified artifact. Three new CLI commands, every analysis command extended."
tagStyle: "release"
tagLabel: "RELEASE"
category: "Releases"
readTime: "5 min read"
related:
  - url: "whats-new-v023.html"
    title: "What's New in v0.23"
    summary: "tool_commands_allow / tool_denylist_add defaults plus a cleaner DOT header."
  - url: "whats-new-v021-v022.html"
    title: "What's New in v0.21–v0.22"
    summary: "Human timeouts, budget caps, and the manager_loop node kind."
---

`v0.24.0` is the biggest change to the project since the IR settled in v0.20 — a whole new file format. `.dipx` is a deterministic, content-addressed ZIP that bundles a `.dip` entry workflow plus every transitively-reachable subgraph into a single integrity-verified artifact. Three new CLI commands ship with it, and every existing analysis command now accepts a `.dipx` argument transparently.

## The problem

Subgraph composition is the main reason `.dip` projects grow beyond a single file. A workflow ref like `subgraph S: ref: phases/review.dip` is resolved against the filesystem at runtime, so the runtime needs every referenced file present at the right relative path. That works fine on a developer's laptop. It gets messier as soon as you want to:

- Ship a workflow tree to a runtime (Tracker) over the network.
- Pin a particular *version* of a multi-file workflow inside a CI artifact.
- Verify on the receiving end that nothing was tampered with in transit.
- Address a workflow tree by content (cache it, deduplicate it, look it up by hash).

DOT-based pipelines did this by flattening — paste every subgraph inline, ship one file. That's a serializer concern, not a packaging story; you lose the per-file identity. `.dipx` is the packaging story.

## What it is

A `.dipx` file is a regular ZIP archive with three rules:

1. **`manifest.json` at the archive root.** Lists every file's bundle path and SHA-256, plus a `format_version` and an `entry` that points at the workflow root.
2. **Workflow files under `workflows/`.** Same directory tree as the source — refs resolve lexically against bundle paths exactly as they would on disk.
3. **Deterministic byte output.** Two `dippin pack` runs over the same source tree produce byte-identical bundles: fixed mtime (ZIP epoch), sorted entries, no platform metadata, UTF-8 filename bit always set.

Integrity is verified on every `Open`. The 9-step ordering — zip → manifest read → manifest decode → manifest shape → strict-extras check → hash verify → parse → walkRefs → normalize — is encoded in the type system: the bytes presented to the parser come from a `verifiedBytes` wrapper produced exclusively by hash verification, so "parse before verify" is a structural impossibility, not a documentation invariant.

```sh
$ dippin pack pipeline.dip
$ ls
pipeline.dip   pipeline.dipx
$ dippin inspect pipeline.dipx
format: 1
entry:  workflows/pipeline.dip
identity: sha256:0a7d9f...
files:
  workflows/pipeline.dip                       sha256:abc123…
  workflows/phases/review.dip                  sha256:def456…
status: VALID (2 files, format_version 1)
```

## Three new commands

| Command | Purpose |
|---------|---------|
| `dippin pack <entry.dip>` | Build a deterministic `.dipx` from a `.dip` entry. `-o <out>` (default: `<entry>.dipx`; `-` for stdout). `--dry-run` validates and walks refs without writing. |
| `dippin unpack <bundle.dipx>` | Atomic extract via staging dir + rename. `-o <destdir>`, `--force` overwrites with rollback-safe backup-aside swap. |
| `dippin inspect <bundle.dipx>` | Print manifest, identity hash, and file list. `--format text\|json`. |

Bundle commands return distinct exit codes — `0` ok, `1` user error, `2` integrity error, `3` I/O error, `4` cancelled — so tooling can disambiguate failures that the analysis-command `0/1/2` set collapses.

## Every analysis command accepts `.dipx`

This is the part that makes the format actually useful day-to-day. `validate`, `lint`, `doctor`, `parse`, `cost`, `coverage`, `simulate`, `optimize`, `unused`, `graph`, `diff`, `check`, `explain`, `export-dot` all accept either a `.dip` or a `.dipx` argument. Internally every one of them now goes through `dipx.Load`:

```go
src, err := dipx.Load(ctx, "pipeline.dipx")  // also accepts pipeline.dip
entry := src.Entry()                          // *ir.Workflow
```

`Source` is the interface runtimes program against. Both `.dip`-on-disk and `.dipx`-bundle paths satisfy it; argument order matches `flatten.Resolver.Resolve(refPath, relativeTo)` for codebase consistency. The recommended workflow: author and lint as `.dip`, ship `.dipx` at deploy time.

## Why content-addressed

`Bundle.Identity()` returns the SHA-256 of the manifest bytes-as-stored. That's a stable id you can use as a runtime cache key, a content-addressed lookup in distribution infrastructure, or a "did the workflow change" check without diffing source. Two byte-identical bundles have the same identity; two semantically-equivalent bundles with different formatting do not, and that's the right thing — the receiver wants to verify exact bytes.

## Threat model

`.dipx` v1 is **integrity-but-not-authenticity**. Hash-verified bundles guarantee bytes weren't corrupted between Pack and Open, but anyone who can produce a `.dipx` can produce a *valid* `.dipx`. v1 is designed for distribution between trusted parties; recipients of bundles from untrusted sources MUST treat them as un-authenticated until v2 ships cryptographic signatures (the spec carries a non-normative v2 sketch with `manifest.sig` and an `ed25519`-keyed `signatures` field).

Pack refuses symlinks anywhere in the source tree — the leaf, parent components, and ancestors — to close a host-file exfiltration vector when packing untrusted source trees (CI-runner contributor builds, mono-repo subdirs that vendor third-party `.dip` files). Extract is atomic via staging-dir + rename; with `--force`, the existing destination is renamed to `.bak` first and restored on rename failure (e.g. cross-mount EXDEV) so a failed extract can never delete the user's original directory.

## Loader-tier exemption

The new `dipx` package is the first composer that imports `parser` and `simulate` to materialize a parsed, condition-normalized workflow tree from a bundle. CLAUDE.md now documents this as a bounded exception to the "packages only depend on `ir`" rule: `dipx` may compose `ir + parser + simulate` but MUST NOT import `validator`, `cost`, `formatter`, or any analysis package — that would invert the dependency direction. Pack-time structural validation (DIP001–DIP009) therefore lives at the CLI layer in `cmd/dippin/cmd_pack.go`, not inside `dipx`.

## Known v1 limitations

See [`docs/superpowers/plans/2026-05-07-dipx-followups.md`](https://github.com/2389-research/dippin-lang/blob/main/docs/superpowers/plans/2026-05-07-dipx-followups.md) for the full list. Highlights deferred to v1.1:

- Cryptographic signatures (sketch in spec § Versioning).
- Central-directory ↔ local-header agreement detection (mitigated by hash verification in v1).
- Per-file 1000:1 compression-ratio cap (absolute 50 MB per-file cap provides DoS protection in the meantime).
- Unicode case-folding for duplicate detection (currently `strings.ToLower`).
- `DIPX_DEBUG=1` diagnostic mode.

Every deferred MUST is recorded with a disposition.

## What's next

Full notes in [CHANGELOG.md](https://github.com/2389-research/dippin-lang/blob/main/CHANGELOG.md). On the tracker side, the integration contract is the `dipx.Source` interface — Tracker can swap its disk-walking loader for `dipx.Load` and pick up content-addressed bundle support without touching the rest of its codebase.
