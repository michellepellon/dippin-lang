# `.dipx` — Distributable bundle format for Dippin workflows

**Status:** Design approved 2026-05-06
**Primary use case:** Distribution / portability — share a self-contained pipeline by file, URL, or registry; recipient runs it without needing the original repo.
**Library-first:** A `dipx` Go package both the dippin CLI and downstream Tracker import directly.

## Problem

A Dippin workflow's only external dependencies today are other `.dip` files referenced via `subgraph ref:` and `manager_loop subgraph_ref:`. Prompts, system prompts, and JSON response schemas are inline strings in the IR (`ir/ir.go:90-109`). There is no `@file:` / include syntax.

To share a multi-file pipeline (parent + transitive subgraphs) with a teammate or operator today, you ship a directory tree and hope the recipient preserves layout. There is no:

- single artifact to email, host, or version
- integrity check on the bundle
- guarantee that a recipient won't unwittingly execute a workflow whose subgraph escaped the source root
- shared loading contract between `dippin` (authoring) and Tracker (execution)

`.dipx` is that artifact.

## Scope

In scope (v1):

- A single-file ZIP container with extension `.dipx`.
- Bundling rules: parent `.dip` plus every transitively-referenced subgraph, plus a JSON manifest with SHA-256 per file.
- Hermetic invariant: refs inside the bundle resolve only to files inside the bundle.
- Library API in package `dipx`: `Pack`, `Open`, `OpenReader`, `OpenStrict`, `Validate`, `Extract`, `Load`, plus a `Source` interface and `Bundle` type.
- CLI: new `dippin pack` / `unpack` / `inspect`; existing analysis commands transparently accept `.dipx`.
- Tracker integration: two call-site changes; identical execution semantics.
- Errors: Go error sentinels in `dipx`, no DIP codes for bundle errors.

Out of scope:

- Cryptographic signatures (deferred — manifest is extensible to add a `signatures` block additively).
- External asset bundling (no `@file:` / include syntax exists in the language; revisit when it does).
- Bundle-level budget aggregation (`MaxTotalTokens` etc. remain per-workflow).
- Distribution mechanism — networking, registries, caching, publishing. A `.dipx` is just a file.
- Forward-compat "warn and try" parsing — fail-closed on unknown `format_version`.
- Fuzzing, performance benchmarks, dedicated Windows CI.

## Established design decisions

| Decision | Choice | Why |
|---|---|---|
| Container | ZIP | Stdlib `archive/zip`; seekable; universal `unzip`; matches `.whl`/`.jar` precedent. |
| Bundle scope | Workflows + manifest only | No external assets exist in the language today; manifest extensible via tolerant decoding. |
| Integrity | SHA-256 per file in manifest | Detects accidental and adversarial corruption; signatures land additively later. |
| Signatures | Deferred | Don't gate v1 adoption on key-management. |
| Strictness | Hash mismatch / missing file = hard fail; extra files ignored by default, `OpenStrict` rejects them | Robust against `__MACOSX/`, `.DS_Store`; fail-closed on real corruption. |
| Hermetic | Refs cannot escape `workflows/` | A `.dipx` must run standalone or not at all. |
| Versioning | Integer `format_version`, fail-closed on unknown | Executable artifact; "warn and try" is unsafe. |

## Design

### Package layout

A new package `dipx/` at the project root, sibling to `parser`, `validator`, etc.

```
dipx/
  dipx.go        # Public API: Pack, Open, OpenReader, OpenStrict, Validate, Extract, Load
  manifest.go    # Manifest type + JSON marshaling + version checks
  resolve.go     # Path safety, canonicalization, ref walking, cycle detection
  testdata/      # Intentional fixture .dipx files
  *_test.go
```

`dipx` imports `ir` and `parser` only. This matches the architectural rule in `CLAUDE.md` ("Packages import `ir` but not each other"). Validator/cost/doctor/etc. continue operating on `ir.Workflow` after `dipx.Open` returns one.

`resolve.go` is the only file with subgraph-walking logic. `Pack` uses it to discover dependencies on disk; `Open` uses it to verify the bundle's manifest matches the workflows it claims to contain. Same code, both directions — no drift.

### Bundle layout on disk

```
manifest.json
workflows/                 # mirrors original directory structure
  api_design.dip
  interview_loop.dip
  phases/
    code_review.dip
```

No `assets/` directory in v1 (no external assets to bundle). Adding it later is non-breaking.

### Manifest schema

```json
{
  "format_version": 1,
  "entry": "workflows/api_design.dip",
  "files": [
    { "path": "workflows/api_design.dip",        "sha256": "abc123…" },
    { "path": "workflows/interview_loop.dip",    "sha256": "def456…" },
    { "path": "workflows/phases/code_review.dip","sha256": "789abc…" }
  ]
}
```

Three required keys. Spec-level rules:

1. **Tolerant decoding.** Unknown top-level keys and unknown keys inside `files[]` are silently ignored. Implementations **must not** use `json.Decoder.DisallowUnknownFields`. This is how `signatures` and similar additive blocks land later without a `format_version` bump — and only when the addition is optional. A future requirement (e.g., "presence of signatures implies mandatory verification") is a semantic change and **must** bump `format_version`.

2. **Path safety on every read** (defends against zip-slip):
   - Reject absolute paths, `..` segments, backslashes, symlinks.
   - All `files[].path` and `entry` must start with `workflows/`.

3. **Hermetic invariant.** `Pack` refuses to create a bundle where any transitive `subgraph ref:` resolves outside the source root. `Open` refuses to load a bundle where any ref resolves outside `workflows/`.

4. **Validation strictness:**
   - Hash mismatch → hard error.
   - File listed in manifest but missing from zip → hard error.
   - Entry not listed in `files[]` → hard error.
   - Extra zip entries not in manifest → ignored by default; `OpenStrict` errors.

5. **Hash format.** Lowercase hex SHA-256, 64 chars. No algorithm negotiation in v1.

### Library API

```go
package dipx

// Manifest is the parsed manifest.json.
type Manifest struct {
    FormatVersion int    `json:"format_version"`
    Entry         string `json:"entry"`
    Files         []File `json:"files"`
}

type File struct {
    Path   string `json:"path"`
    SHA256 string `json:"sha256"`
}

// SupportedFormatVersions lists format_version values this build accepts.
var SupportedFormatVersions = []int{1}

// Source loads workflows, whether from a .dip on disk (refs resolved
// against the filesystem) or from a .dipx bundle (refs resolved
// against the bundle root). Tracker programs against this interface.
//
// Source is safe for concurrent reads.
type Source interface {
    Entry() *ir.Workflow
    Workflow(fromPath, refPath string) (*ir.Workflow, error)
}

// Bundle is an opened .dipx. All workflows are parsed eagerly on Open;
// no file handles are held after Open returns. Bundle implements Source.
type Bundle struct {
    Manifest Manifest
    // unexported: workflows map[string]*ir.Workflow, files map[string][]byte
}

func (b *Bundle) Entry() *ir.Workflow
func (b *Bundle) Workflow(fromPath, refPath string) (*ir.Workflow, error)
func (b *Bundle) WorkflowAt(bundlePath string) (*ir.Workflow, error)
func (b *Bundle) Resolve(fromPath, refPath string) (string, error)
func (b *Bundle) File(bundlePath string) ([]byte, error)

// Pack builds a .dipx from an entry .dip on disk and writes it to w.
// Walks every subgraph/manager_loop ref transitively. Validates structurally.
// Refuses if any ref escapes source root or if a cycle is detected.
func Pack(entryPath string, w io.Writer) error

// Open reads a .dipx from disk. Lenient on extra zip entries.
func Open(path string) (*Bundle, error)

// OpenReader is Open from any io.ReaderAt of known size.
func OpenReader(r io.ReaderAt, size int64) (*Bundle, error)

// OpenStrict is Open but errors on any zip entry not listed in the manifest.
func OpenStrict(path string) (*Bundle, error)

// Validate opens and discards a .dipx, returning only success or error.
func Validate(path string) error

// Extract unpacks a .dipx into destDir, applying path-safety checks.
func Extract(path, destDir string) error

// Load opens either a .dip or a .dipx based on extension/sniffing.
// Returns a Source with identical semantics for both formats.
func Load(path string) (Source, error)
```

Two `Source` implementations:

- **`*Bundle`** (`.dipx`) — holds parsed manifest plus every workflow in memory; `Workflow(fromPath, refPath)` calls `Resolve` + map lookup.
- **`*dirSource`** (unexported, for `.dip` on disk) — holds the entry workflow and a base directory; `Workflow(fromPath, refPath)` does `path.Join(path.Dir(fromPath), refPath)`, parses lazily, caches per path. Same path-safety checks as `Bundle`.

### Path canonicalization

Packer and runtime must agree exactly. Rules apply identically in `Pack`, `Open`, `Source.Workflow`, and `Extract`:

- Always forward-slash separators (use `path.Clean`, not `filepath.Clean`).
- No leading `./`, no `..`, no absolute paths, no backslashes, no symlinks.
- `.dip` extension required in refs and manifest entries (no auto-append).
- Resolution: `parentPath` is bundle-relative; child path = `path.Clean(path.Join(path.Dir(parentPath), refPath))`. Result must start with `workflows/` or it is rejected.

Single canonicalization function in `resolve.go`, called from every site.

### Open post-conditions

After a successful `dipx.Open`:

- Manifest is well-formed and `format_version` is supported.
- Every file listed in `files[]` exists in the zip with matching SHA-256.
- Every workflow in the bundle parses successfully (`parser.Parse` returns no error).
- Every transitive `subgraph ref:` and `manager_loop subgraph_ref:` resolves to a workflow inside `workflows/`.
- Subgraph reference graph is acyclic.
- Total uncompressed size and file count are within soft caps.

**Not promised:** that workflows are ready for `simulate` or `lint` phases. `Condition.Parsed` is populated lazily by `simulate.EnsureConditionsParsed()` per `CLAUDE.md`; `Open` does not invoke it. Downstream tools call their own normalization passes as today.

### Soft caps

`Open` enforces (constants in `dipx.go`, bumpable in one line):

- Max files in bundle: 10,000
- Max total uncompressed size: 100 MB
- Max ref-graph depth: 64

Exceeding any cap is a hard error. These are sane defaults, not policy infrastructure.

### Concurrency

`Source` is safe for concurrent reads. Implementation:

- `Open` reads each zip entry into its own `[]byte`, closes zip internals, parses each workflow once, stores results in immutable maps, and returns. No file handles held; no mutable state remains.
- `Source.Workflow(parentPath, refPath)` is a path-validation step plus a map lookup over already-parsed IR. Lock-free.
- `*Bundle.Extract` writes to disk; not concurrent with itself, but safe alongside concurrent reads of the same `Bundle`.

This matters because `NodeParallel` and `NodeFanIn` may invoke `Source.Workflow` from multiple goroutines.

### CLI

#### New commands

```sh
dippin pack <entry.dip> [-o output.dipx]
```

Builds a `.dipx` from an entry workflow.

- Walks every transitive `subgraph ref:` and `manager_loop subgraph_ref:` from disk.
- Runs structural validation first (same checks as `dippin validate`); refuses to pack invalid input.
- Lint is not run; that's the user's call (`dippin lint && dippin pack`).
- Defaults output to `./<basename>.dipx` next to the entry file.
- Errors loudly if any ref escapes the entry's source root.

```sh
dippin unpack <bundle.dipx> [-o destdir]
```

Extracts a `.dipx` to a directory. Defaults to `./<basename>/`. Applies path-safety checks during extraction.

```sh
dippin inspect <bundle.dipx>
```

Prints the manifest — format version, entry, file list with hashes — without extracting.

```
format: 1
entry:  workflows/api_design.dip
files:
  workflows/api_design.dip                      sha256:abc123…
  workflows/interview_loop.dip                  sha256:def456…
  workflows/phases/code_review.dip              sha256:789abc…
```

#### Existing commands — uniform extension

These accept `.dip` and `.dipx` interchangeably via `dipx.Load`:

- `dippin validate <path>`
- `dippin lint <path>`
- `dippin doctor <path>`
- `dippin simulate <path>`
- `dippin parse <path>`
- `dippin cost <path>`
- `dippin coverage <path>`
- `dippin unused <path>`
- `dippin diff <a> <b>`

Per-command behavior on a bundle:

- **Default:** the command operates on the entry workflow only — preserves today's "one workflow at a time" semantics from `subgraph-composition.md`.
- **`--all`** flag: run on every workflow in the bundle and aggregate.

Commands not extended in v1: `migrate`, `export-dot`, `export-dip`, `fmt`, `new`, `watch`, `lsp`, `scaffold`. Revisit if a use case appears.

### Validation layers

| Check | `.dip` | `.dipx` |
|---|---|---|
| Workflow structural validation (DIP001–009) | ✓ | ✓ on entry; ✓ on all with `--all` |
| Manifest well-formed | — | ✓ |
| Format version recognized | — | ✓ |
| Path safety (no `..`, absolute, backslashes, symlinks) | — | ✓ |
| Files in manifest exist in zip | — | ✓ |
| SHA-256 matches | — | ✓ |
| Entry listed in `files[]` | — | ✓ |
| Hermetic invariant | — | ✓ |
| No extra zip entries | — | with `--strict` |
| Signature verification *(future)* | — | ✓ when present |

Bundle-integrity checks are **not opt-out**: they are `dipx.Open`'s contract. If `Open` returns a `*Bundle`, those checks have already passed.

In `dippin validate`, integrity errors print first as a hard failure block; only if integrity is clean does the per-workflow lint run and produce DIP findings. They are never interleaved.

### Tracker integration contract

#### What Tracker keeps doing exactly as today

- Workflow execution semantics; params propagation; `manager_loop` polling, steering, cycle limits; context propagation; retry policies; fidelity; fan-in/parallel.
- Per-workflow budgets remain scoped exactly as in non-bundled execution. **`.dipx` does not introduce new budget scopes.** Aggregate run-level budgets, if ever wanted, are an orthogonal Tracker feature.
- `.dip` loading. `dipx.Load` dispatches based on file extension; `.dip` paths take the existing code path through a `dirSource`.

#### What Tracker changes — exactly two call sites

```go
// Before:
wf, err := parser.ParseFile("foo.dip")
// ... when hitting a subgraph node:
childPath := filepath.Join(filepath.Dir(parentPath), sub.Ref)
data, _ := os.ReadFile(childPath)
child, _ := parser.Parse(data)

// After:
src, err := dipx.Load(path)        // path can be "foo.dip" OR "foo.dipx"
wf := src.Entry()
// ... when hitting a subgraph node:
child, err := src.Workflow(parentPath, sub.Ref)
```

#### Format version policy

- Tracker compiled against `dipx` v1 supports `format_version: 1` only.
- Unknown `format_version` → `dipx.ErrUnsupportedFormatVersion` (with rich context including the build's supported versions).
- Fail-closed; no "warn and try."
- Escape valve: bumping the dipx library dependency in Tracker.

#### Bundle lifetime in Tracker

- Bundle held fully in memory after `Open` returns. No file handle. No temp dir.
- `.dip` case: lazy disk reads through the `dirSource`.

#### Distribution surface

A `.dipx` is just a file. Tracker's existing input mechanisms work without change:

- File path on disk → `dipx.Load(path)`
- HTTP/URL fetch → `dipx.OpenReader(bytes.NewReader(downloaded), int64(len(downloaded)))`
- Stdin / pipe → buffer and use `OpenReader`
- In-memory generation → `OpenReader`

`dipx` defines nothing about networking, registries, or caching.

### Versioning policy

- `format_version: 1` is the only valid value today.
- Bumping `format_version` requires: (1) a documented breaking change, (2) a migration tool, (3) the prior version stays supported for at least one release cycle.
- Reading rules across all versions: known version → read; unknown version → reject; never "warn and try."
- The `dipx` Go module ships with `dippin-lang` semver tags. Format support is documented in `SupportedFormatVersions`.

### Error model

Bundle errors are surfaced as Go errors with sentinels for programmatic discrimination — **not** DIP codes:

```go
var (
    ErrUnsupportedFormatVersion = errors.New("unsupported format_version")
    ErrManifestMissing          = errors.New("manifest.json missing or unreadable")
    ErrManifestInvalid          = errors.New("manifest.json malformed")
    ErrFileMissing              = errors.New("file listed in manifest not in zip")
    ErrFileUnexpected           = errors.New("file in zip not listed in manifest") // strict mode only
    ErrHashMismatch             = errors.New("file hash does not match manifest")
    ErrPathUnsafe               = errors.New("unsafe path: traversal, absolute, or escape from workflows/")
    ErrEntryNotInManifest       = errors.New("entry not listed in files[]")
    ErrRefEscape                = errors.New("subgraph ref escapes bundle root")
    ErrRefCycle                 = errors.New("subgraph ref cycle detected")
    ErrCapExceeded              = errors.New("bundle exceeds size or file-count cap")
    ErrEntryParse               = errors.New("entry workflow failed to parse")
    ErrSubgraphParse            = errors.New("subgraph workflow failed to parse")
)
```

Every sentinel is wrapped with rich context at the throw site:

```go
return fmt.Errorf("%w: %d (build supports %v)", ErrUnsupportedFormatVersion, m.FormatVersion, SupportedFormatVersions)
return fmt.Errorf("%w: %s (expected %s, got %s)", ErrHashMismatch, path, expectedHash, actualHash)
```

Consumers use `errors.Is` for discrimination. No diagnostic-code system; no localization hooks; no machine-readable code layered atop sentinels.

Why no DIP codes for bundle errors:

1. Different audience — DIP findings are for the workflow author iterating on their `.dip`; bundle errors are for the operator running a `.dipx`.
2. Different lifecycle — DIP codes multiply over time; bundle errors are bounded.
3. Stable Go error sentinels are what consumers actually need for programmatic handling.

## Testing strategy

### Unit tests in `dipx/`

```
dipx/
  dipx_test.go        # Pack/Open/Load round-trips, format_version handling
  manifest_test.go    # JSON marshaling, unknown-key tolerance, version parsing
  resolve_test.go     # Path safety, canonicalization, ref walking, cycle detection
  testdata/
    corrupt-manifest.dipx       # malformed manifest.json
    mismatched-hash.dipx        # file in zip with hash != manifest
    missing-file.dipx           # listed in manifest, absent in zip
    extra-file.dipx             # in zip, not in manifest (strict-mode test)
    bad-version.dipx            # format_version: 999
    escape-ref.dipx             # ref: ../../etc/passwd
    cycle-refs.dipx             # a -> b -> a
    too-big.dipx                # exceeds soft cap
    well-formed.dipx            # baseline happy path
```

Required test cases:

- **Round-trip:** Pack a `.dip` directory tree → Open the resulting bytes → verify every workflow's parsed IR is structurally equal to the source.
- **Hash sentinels:** Corrupt one byte of one file; assert `errors.Is(err, ErrHashMismatch)` and that the error mentions the file path.
- **Path safety:** Each of `..` traversal, absolute path, backslash, symlink, escape-from-`workflows/` → `errors.Is(err, ErrPathUnsafe)`.
- **Hermetic Pack:** Source tree where one ref escapes the entry's directory → Pack fails before producing output.
- **Cycle detection:** Both Pack and Open reject `a → b → a`; error mentions the cycle.
- **Format version:** `format_version: 999` → `errors.Is(err, ErrUnsupportedFormatVersion)`.
- **Tolerant decoding:** Manifest with unknown top-level key plus unknown key inside `files[]` → loads successfully.
- **Strict mode:** Bundle with `__MACOSX/` extras → `Open` succeeds, `OpenStrict` returns `ErrFileUnexpected`.
- **Soft caps:** Synthetic bundle exceeding file-count or total-size cap → `ErrCapExceeded`.
- **Concurrency:** Spin 100 goroutines calling `Source.Workflow` on a bundle; run under `-race`. No data races; all return equal IR.
- **Polymorphic Load:** Same multi-workflow tree loaded as `.dip` and as `.dipx` produces `Source` instances whose entry IR and resolved children are structurally equal.

### Integration test (extends `validator/lint_examples_test.go`)

Add `TestPackExamples`: walk `examples/`, pack each `.dip` that has subgraphs (`orchestrator.dip`, `manager_loop_demo.dip`, `api_design.dip`), round-trip through `dipx.Open`, then run the existing lint. Asserts:

1. Every example packs cleanly.
2. Round-tripped IR produces the same lint output as the on-disk source.

### CLI tests

Add `cmd/dippin/cmd_pack_test.go`, `cmd_unpack_test.go`, `cmd_inspect_test.go` following the pattern of `cmd_spec_test.go`. Smoke-test happy paths and one failure path each.

### Justfile

Add a `pack-examples` recipe that packs every example with subgraphs and verifies round-trip. Extend `just check` to call it after `validate-examples`. Pre-commit hook inherits.

### Coverage

Match project posture: `just cover` should show `dipx/` at the same coverage band as `parser/` and `validator/`. No formal threshold; the unit tests above cover every error sentinel, so unreached error paths surface in coverage.

## What this design explicitly does not do

- **Cryptographic signatures.** Manifest is extensible (tolerant decoding); a `signatures` block lands later without a `format_version` bump as long as verification remains optional.
- **External assets.** No `@file:` / include syntax in the language; revisit when it exists.
- **Bundle-level budgets.** `MaxTotalTokens` etc. remain per-workflow.
- **DIP codes for bundle errors.** Wrong category; sentinels are the right primitive.
- **Forward-compat parsing on unknown `format_version`.** Fail-closed; executable artifact.
- **Distribution mechanism.** Networking, registries, caching, publishing — out of scope.
- **Fuzzing, perf benchmarks, dedicated Windows CI.** Worth doing eventually; not v1.
