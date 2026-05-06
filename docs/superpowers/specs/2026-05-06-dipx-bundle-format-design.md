# `.dipx` — Distributable bundle format for Dippin workflows

**Status:** Design v2 — revised 2026-05-06 after multi-reviewer pass (PAL + 6 expert reviewers)
**Primary use case:** Distribution / portability — share a self-contained pipeline by file, URL, or registry; recipient runs it without needing the original repo.
**Library-first:** A `dipx` Go package both the dippin CLI and downstream Tracker import directly.

## Conventions

The capitalised keywords MUST, MUST NOT, REQUIRED, SHOULD, SHOULD NOT, MAY are used in the normative sense from RFC 2119/8174. Lowercase usage is descriptive prose.

This document is a **Go library + format specification** for the dippin-lang reference implementation. Cross-language conformance (test vectors, language-neutral pseudocode, alternative-implementation interop) is **out of scope for v1**; see *Known v1 limitations*.

## Problem

A Dippin workflow's only external dependencies today are other `.dip` files referenced via `subgraph ref:` and `manager_loop subgraph_ref:`. Prompts, system prompts, and JSON response schemas are inline strings in the IR (`ir/ir.go:90-109`). There is no `@file:` / include syntax.

To share a multi-file pipeline today, you ship a directory tree and hope the recipient preserves layout. There is no:

- single artifact to email, host, or version
- integrity check on the bundle
- guarantee that a recipient won't unwittingly execute a workflow whose subgraph escaped the source root
- shared loading contract between `dippin` (authoring) and Tracker (execution)

`.dipx` is that artifact.

## Scope

In scope (v1):

- Single-file ZIP container with extension `.dipx`.
- Bundling rules: parent `.dip` plus every transitively-referenced subgraph, plus a JSON manifest with SHA-256 per file.
- Hermetic invariant: refs inside the bundle resolve only to files inside the bundle, and only to manifest-listed, hash-verified files.
- Library API in package `dipx`: `Pack`, `Open`, `OpenReader`, `OpenLax`, `Validate`, `Extract`, `Load`, plus a `Source` interface and `Bundle` type.
- CLI: new `dippin pack` / `unpack` / `inspect`; existing analysis commands transparently accept `.dipx`.
- Tracker integration: two call-site changes; identical execution semantics.
- Errors: Go error sentinels in `dipx`, no DIP codes for bundle errors.
- Reproducible Pack: same source tree → byte-identical bundle.

Out of scope (see *Known v1 limitations*):

- Cryptographic signatures.
- External asset bundling.
- Bundle-level budget aggregation.
- Distribution mechanism — networking, registries, caching.
- Cross-language conformance.
- Hash algorithm agility.

## Established design decisions

| Decision | Choice | Why |
|---|---|---|
| Container | ZIP (constrained subset) | Stdlib `archive/zip`; seekable; universal `unzip` for forensics; matches `.whl`/`.jar` precedent. |
| Bundle scope | Workflows + manifest only | No external assets in the language today; manifest extensible via tolerant decoding. |
| Integrity | SHA-256 per file in manifest | Detects accidental and adversarial *transit corruption between trusted parties*. **Does not authenticate origin** — see *Threat model*. |
| Signatures | Deferred to v2 | Don't gate v1 adoption on key-management. |
| Strict mode | **Default**: extra zip entries rejected | Executable artifacts must be strict by default. `OpenLax` is the explicit escape hatch for hand-edited bundles. |
| Hermetic | Refs cannot escape `workflows/` AND must resolve to manifest-listed entries | A `.dipx` runs standalone or not at all. Manifest tampering cannot trick the runtime into serving extras. |
| Versioning | Integer `format_version`, fail-closed on unknown | Executable artifact; "warn and try" is unsafe. |
| Reproducibility | Pack output is deterministic | Same source tree → byte-identical `.dipx`. Fixed timestamps; sorted entries; no platform-specific metadata. |

## Threat model

`.dipx` v1 is designed for distribution **between trusted parties** over channels where integrity matters but authenticity is asserted out-of-band (e.g., a developer hands a teammate a bundle; a CI artifact moves between trusted services). SHA-256 in the manifest detects accidental corruption and casual tampering by parties who do not control the channel.

**`.dipx` v1 does NOT defend against an active attacker who controls the distribution channel.** Such an attacker can rewrite both the bundle bytes and the manifest hashes. Authenticity defense is the role of cryptographic signatures, deferred to v2.

Producers and consumers MUST treat `.dipx` files received from untrusted sources (public URLs, email attachments from outside trust boundaries) as un-authenticated until v2 ships signatures. This is a deliberate v1 limitation.

## Wire format

### Container: ZIP, constrained

A `.dipx` file is a ZIP archive (PKZIP APPNOTE.TXT). The following ZIP features are restricted to keep the format predictable across tools:

- Entries MUST use compression method `Store` (0) or `Deflate` (8). Other methods MUST be rejected.
- Encryption (any form, including ZipCrypto and AES) MUST NOT be used. Encrypted entries MUST be rejected.
- Multi-disk / spanned archives MUST NOT be used and MUST be rejected.
- ZIP64 records MUST be used when, and only when, required by entry size or count (per APPNOTE.TXT).
- The ZIP archive comment field SHOULD be empty; if present, MUST be ignored.
- Per-file extra fields are permitted but MUST NOT alter file content (see *Reproducibility* below).
- Entry filenames MUST be encoded as UTF-8 with general-purpose bit 11 set (APPNOTE.TXT 4.4.4). CP437-encoded filenames MUST be rejected.
- An entry whose external file attributes encode a symlink (Unix mode bit `S_IFLNK`, `0o120000`) MUST be rejected.
- An entry whose external file attributes encode a non-regular, non-directory file (device, FIFO, socket) MUST be rejected.
- The central directory and local file headers MUST agree on filename and uncompressed size; mismatches MUST be rejected (defense against parser-confusion).
- Duplicate entry names (case-sensitive byte equality) MUST be rejected.

### Bundle layout

```
manifest.json              # at zip root, exactly this name (lowercase ASCII)
workflows/                 # mirrors original directory structure
  api_design.dip
  interview_loop.dip
  phases/
    code_review.dip
```

`manifest.json` MUST be a zip entry named exactly `manifest.json`, at the archive root, with no leading directory.

There MUST NOT be any zip directory entries (entries whose name ends with `/`). Pack does not emit them; consumers MUST reject bundles that contain them. (Rationale: ZIP directory entries are redundant — every directory is implied by file paths — and create ambiguity for strict-mode validation.)

No `assets/` directory in v1. Adding one in a future format version is non-breaking provided the producer also bumps `format_version`.

### Manifest schema

```json
{
  "format_version": 1,
  "entry": "workflows/api_design.dip",
  "files": [
    { "path": "workflows/api_design.dip",         "sha256": "abc123…" },
    { "path": "workflows/interview_loop.dip",     "sha256": "def456…" },
    { "path": "workflows/phases/code_review.dip", "sha256": "789abc…" }
  ]
}
```

Three required keys. Schema rules:

1. **`format_version`** MUST be a non-negative integer JSON literal in `[1, 2^31-1]`. The values `1.0`, `1e0`, `"1"`, leading-zero forms, and negative values MUST be rejected. v1 accepts only `1`.
2. **`entry`** MUST be a path string that byte-equals exactly one `files[].path`. It MUST start with `workflows/` and end with `.dip`.
3. **`files[]`** MUST be a non-empty JSON array of objects. Each object MUST have exactly the required keys `path` (string) and `sha256` (string). Unknown keys inside the object are tolerated (silently ignored). Non-object members or members missing required keys MUST cause `ErrManifestInvalid`.
4. **`files[].path`** MUST start with `workflows/`, MUST end with `.dip`, MUST be byte-equal to the canonical form (see *Path canonicalization* below), and MUST be unique across the array.
5. **`files[].sha256`** MUST be lowercase hex, exactly 64 characters, computed over the **uncompressed** (logical) bytes of the file as it would be written to disk by `Extract`. Uppercase, non-hex, leading/trailing whitespace, and incorrect length MUST be rejected.

### Manifest JSON encoding

The manifest MUST be UTF-8 encoded. A leading byte-order-mark MUST NOT be emitted by Pack and MUST be rejected on Open. A trailing newline is OPTIONAL.

The decoder MUST reject:

- Duplicate top-level keys.
- Duplicate keys inside any object (including `files[]` members).
- Trailing data after the top-level JSON object.
- Manifests larger than 1 MB before parsing.

The decoder MUST tolerate (silently ignore):

- Unknown top-level keys (extensibility — see *Forward compatibility*).
- Unknown keys inside objects in `files[]`.
- Whitespace, line endings, and indentation choices.

JSON key ordering inside the manifest is not significant for v1 semantics. Pack writes a canonical key order (alphabetical at every level) and a sorted `files[]` (by `path` byte-order) to support reproducibility.

### Path canonicalization

The following rules apply identically in `Pack`, `Open`, `Source.Workflow`, and `Extract`. They are stated in language-neutral terms; the Go reference implementation expresses them via `path.Clean` plus the additional checks below.

A path is in **canonical form** if and only if all of:

1. It is a valid UTF-8 sequence in Unicode Normalization Form C (NFC).
2. Separators are forward-slash `/` only. Backslash `\` and any other separator MUST be rejected.
3. There are no leading `./` segments, no `..` segments, no empty path components (`//`), no leading `/`.
4. It contains no NUL byte (U+0000), no ASCII control characters (< 0x20 except by explicit allow-list — none in v1), no DEL (0x7F).
5. No path component has leading or trailing whitespace.
6. No path component, after stripping case and extension, equals a Windows reserved name: `CON`, `PRN`, `AUX`, `NUL`, `COM0`–`COM9`, `LPT0`–`LPT9`.
7. No path component ends in `.` or has a trailing space (Win32 strips these silently and creates collisions).
8. The path component count is ≤ 16 and total path length is ≤ 1024 bytes.
9. The path ends in `.dip` (lowercase, byte-exact). Other extensions and case variants MUST be rejected.

Path comparison is case-sensitive byte-equality of the canonical form.

### Subgraph ref resolution

A *ref* is the value of a `subgraph ref:` or `manager_loop subgraph_ref:` field in a workflow source. Refs MUST be static string literals; refs containing variable interpolation, expressions, or other dynamic forms are unpackable and Pack MUST reject them. (This pins the eager-closure post-condition; future ref forms require a `format_version` bump.)

Given a parent workflow at bundle-relative path `relativeTo` and a ref string `refPath`, the resolved bundle-relative path is computed as follows (pseudocode):

```
resolved := lexical-clean(join(directory(relativeTo), refPath))
```

Where:

- `directory(p)` returns `p` up to and including its last `/`, or empty if none.
- `join(a, b)` concatenates with a single `/` separator unless `a` is empty.
- `lexical-clean` collapses redundant `./`, resolves `..` relative to earlier components without consulting the filesystem, and normalizes redundant `/`.

The resolved path MUST then satisfy *all* of:

- It is in canonical form (per *Path canonicalization*).
- It starts with `workflows/`.
- It byte-equals exactly one `files[].path` in the manifest.

Use of `..` *inside* `refPath` is permitted as long as the resolved path satisfies the above (e.g., `ref: ../sibling/foo.dip` from `workflows/sub/parent.dip` resolving to `workflows/sibling/foo.dip` is legal). Use of `..` *anywhere in `files[].path` or `entry`* MUST be rejected (manifest paths must already be canonical). This resolves the apparent contradiction between path-safety and resolution algorithms in earlier drafts.

### Hash computation and verification

Each `files[].sha256` MUST be the SHA-256 digest of the **uncompressed** (logical) bytes of the file. This is the byte sequence that `Extract` would write to disk for that entry.

`Open` MUST verify hashes before any consumer (parser, ref walker, downstream tooling) sees the bytes. The required ordering is:

1. Open zip; locate `manifest.json`; cap-check size; decode (with duplicate-key rejection).
2. Validate `format_version` against `SupportedFormatVersions`. Unknown → `ErrUnsupportedFormatVersion`.
3. For each `files[]` entry: locate the zip entry by canonical path; read its decompressed bytes once into memory using a length-bounded reader; compute SHA-256; compare to manifest. Mismatch → `ErrHashMismatch`. Cap-check accumulated total size and per-file size.
4. Only after all hashes verify, parse each `.dip` file via `parser.NewParser(...)`. Parse failure on entry → `ErrEntryParse`; on subgraph → `ErrSubgraphParse`.
5. Walk the subgraph ref graph (cycle detection via tri-color DFS); reject cycles or refs to files not in `files[]` or paths failing canonicalization.
6. Normalize each workflow with `simulate.EnsureConditionsParsed` (to make `Condition.Parsed` available; see *Tracker integration*).
7. Build the immutable in-memory `Bundle` and return.

Bytes read in step 3 are the bytes used for parsing in step 4. There is no second read from the zip and no TOCTOU.

### Soft caps (split: producer / consumer)

`.dipx` distinguishes between *bundle limits* (producer-side) and *consumer floors* (reader-side):

- A conformant bundle MUST NOT exceed: 10,000 files, 100 MB total uncompressed size, ref-graph depth 64.
- A conformant reader MUST accept any bundle within those limits.
- A conformant reader MAY enforce *stricter* limits (e.g., a hardened Tracker deployment configured to 1 MB total). When stricter limits trip, the reader MUST emit `ErrCapExceeded`.
- Caps MUST be enforced via `io.LimitReader`-style streaming, not by trusting ZIP header `UncompressedSize64` fields.

Additional consumer-side caps that are not part of the bundle conformance contract but apply to all conformant readers in v1:

- Manifest size: ≤ 1 MB before parsing.
- Per-file uncompressed size: ≤ 50 MB.
- Per-file compression ratio: any entry whose decompressed size exceeds 1000× its compressed size MUST be rejected (zip-bomb defense).

### Reproducible Pack

`Pack(sameSource)` invoked twice MUST produce byte-identical `.dipx` bytes. Specifically:

- All ZIP entry mtimes MUST be set to the ZIP epoch `1980-01-01T00:00:00Z`.
- Entry order in the central directory MUST be lexicographic by canonical path, with `manifest.json` first.
- ZIP extra fields MUST NOT be emitted (no Unix UID/GID, no NTFS timestamps, no Info-ZIP Unicode path).
- File modes in external attributes MUST be `0644` for files. Directories are not emitted (see *Bundle layout*).
- The manifest is canonicalized (alphabetical keys at every object level; `files[]` sorted by `path`).
- Pack MUST NOT include OS-specific metadata files (`__MACOSX/*`, `.DS_Store`, `Thumbs.db`, `desktop.ini`, AppleDouble `._*` files, `.gitkeep`/`.gitignore` from source trees). Pack walks the source tree and only emits transitively-reachable workflow files.

## Library API (Go reference implementation)

### Package layout

A new package `dipx/` at the project root, sibling to `parser`, `validator`, etc.

```
dipx/
  dipx.go        # Public API: Pack, Open, OpenReader, OpenLax, Validate, Extract, Load
  manifest.go    # Manifest type, JSON encoding/decoding with strict rules
  resolve.go     # Path canonicalization, ref walking, cycle detection
  zipio.go       # Constrained zip reader/writer (rejects forbidden features)
  helpers.go     # Helper decomposition for Open/Pack to satisfy 5/7 complexity caps
  testdata/      # Intentional fixture .dipx files
  *_test.go
```

`dipx` imports `ir`, `parser`, **and `simulate`**. The `simulate` import is required because `Open` invokes `simulate.EnsureConditionsParsed` to render returned workflows ready for execution (see *Concurrency and immutability*). This extends the architectural rule in `CLAUDE.md` ("Packages import `ir` but not each other"). `CLAUDE.md` MUST be amended in the same change to declare a "loader" tier, parallel to the existing "analysis" tier exemption: `dipx` may compose `ir + parser + simulate`. The exemption is bounded — `dipx` MUST NOT import `validator`, `cost`, `formatter`, or any other analysis package.

`Open` and `Pack` are decomposed into helpers in `helpers.go` to keep each function under cyclomatic 5 / cognitive 7. Indicative decomposition:

```
Open  → openZip → readManifest → verifyManifestShape → verifyHashes →
        parseAllWorkflows → walkRefs → normalizeConditions → buildBundle
Pack  → walkSourceTree → resolveRefs → buildManifest → writeBundle
```

Each helper is a thin step with one purpose; the top-level `Open`/`Pack` functions are sequence orchestrators with `if err != nil` plumbing only.

### Types

```go
package dipx

// Manifest is the parsed manifest.json. Returned by Bundle.Manifest() as a value.
type Manifest struct {
    FormatVersion int
    Entry         string
    Files         []ManifestEntry
}

// ManifestEntry is one record in Manifest.Files.
type ManifestEntry struct {
    Path   string
    SHA256 string
}

// SupportedFormatVersions returns the format_version values this build accepts.
// Returns a fresh slice on every call to prevent mutation by callers.
func SupportedFormatVersions() []int { return []int{1} }

// Source loads workflows, whether from a .dip on disk (refs resolved against
// the filesystem) or from a .dipx bundle (refs resolved against the bundle root).
//
// Argument order matches flatten.Resolver.Resolve(refPath, relativeTo) for
// codebase consistency.
//
// Source is safe for concurrent reads. Returned *ir.Workflow values MUST be
// treated as read-only by callers.
type Source interface {
    Entry() *ir.Workflow
    Workflow(refPath, relativeTo string) (*ir.Workflow, error)
}

// Bundle is an opened .dipx. All workflows are parsed and normalized eagerly
// on Open; no file handles are held after Open returns. Bundle implements Source.
//
// Bundle is immutable post-Open: all fields and internal maps are frozen.
type Bundle struct {
    // unexported state
}

// Manifest returns a copy of the parsed manifest. Callers may mutate the
// returned value without affecting the bundle.
func (b *Bundle) Manifest() Manifest

func (b *Bundle) Entry() *ir.Workflow
func (b *Bundle) Workflow(refPath, relativeTo string) (*ir.Workflow, error)
func (b *Bundle) Lookup(bundlePath string) (*ir.Workflow, error)
func (b *Bundle) Resolve(refPath, relativeTo string) (string, error)
func (b *Bundle) ReadFile(bundlePath string) ([]byte, error)
```

### Constructors

```go
// Pack builds a .dipx from an entry .dip on disk and writes it to w.
// Walks every transitively-reachable subgraph/manager_loop ref. Validates
// structurally, applies all path-safety and ZIP-feature constraints, and
// produces a deterministic byte stream. Returns the resulting Manifest for
// caller logging/inspection.
func Pack(entryPath string, w io.Writer) (Manifest, error)

// Open reads a .dipx from disk in strict mode (the default). Strict mode
// rejects any zip entry not listed in the manifest. Pack-produced bundles
// always pass strict mode.
func Open(path string) (*Bundle, error)

// OpenReader is Open from any io.ReaderAt of known size. Strict.
func OpenReader(r io.ReaderAt, size int64) (*Bundle, error)

// OpenLax is Open with extra zip entries silently tolerated. For hand-edited
// bundles or uncommon producers. NEVER use in security-sensitive contexts —
// it weakens the hermetic invariant.
func OpenLax(path string) (*Bundle, error)

// Validate is Open-and-discard. Equivalent to: _, err := Open(path); return err.
// Provided for symmetry with the dippin CLI; library callers should typically
// use Open directly.
func Validate(path string) error

// Extract unpacks a .dipx into destDir. Applies all path-safety checks. Files
// are written with mode 0644 regardless of any zip metadata. Directories are
// created with 0755. Existing files are not overwritten unless allowOverwrite.
func Extract(path, destDir string, allowOverwrite bool) error

// Load opens either a .dip or a .dipx based on filename extension. Returns
// a Source with identical semantics for both formats. Callers needing
// bundle-only methods (Manifest, ReadFile, Lookup) should use Open directly.
func Load(path string) (Source, error)
```

### Source implementations

Two implementations satisfy `Source`:

- **`*Bundle`** (`.dipx`) — holds parsed manifest plus every workflow in memory. `Workflow(refPath, relativeTo)` calls internal `Resolve`, then a map lookup. The lookup returns `ErrFileMissing` if the resolved path is not in `files[]` — this is how the runtime hermetic invariant is enforced (a tampered manifest cannot serve unverified bytes).
- **`*dirSource`** (unexported, for `.dip` on disk) — holds the entry workflow and a base directory. `Workflow(refPath, relativeTo)` joins via `filepath` (OS-aware separators, preserving today's `flatten.Resolver` behavior on Windows), parses lazily, applies path-safety checks identical to the bundle case (rejecting `..` escapes from the base directory), and normalizes via `simulate.EnsureConditionsParsed` before returning.

`dirSource`'s lazy cache is bounded by an LRU of 256 entries (workflows tend to be small but a long-running Tracker process could otherwise leak indefinitely). Eviction is transparent to callers.

### Path safety on every read

`Source.Workflow`, `Bundle.Lookup`, `Bundle.ReadFile`, and `Bundle.Resolve` all re-apply path canonicalization on every call, not just at `Open`. This is defense-in-depth: even if a future bug let a non-canonical path into a workflow's ref string post-Open, runtime resolution will still reject it.

### Open post-conditions

After a successful `Open` (or `OpenReader`/`OpenLax`):

1. The manifest is well-formed and `format_version` is supported.
2. The bundle uses only permitted ZIP features (compression Store/Deflate, no encryption, no symlinks, etc.).
3. Every file listed in `files[]` exists in the zip with matching SHA-256, computed over decompressed bytes that the bundle now holds in memory.
4. Every workflow in the bundle parses successfully.
5. Every workflow is normalized: `Condition.Parsed` is populated for every edge condition and every `manager_loop` `StopCondition`/`SteerCondition`.
6. Every transitive `subgraph ref:` and `manager_loop subgraph_ref:` resolves to a manifest-listed entry inside `workflows/`.
7. Subgraph reference graph is acyclic (verified via tri-color DFS).
8. All caps are within the conformance limits and the consumer's configured limits.
9. No file handles are held; no goroutines are running; the returned `*Bundle` is immutable.

### Tracker integration

#### Behavioral contract

Workflow execution semantics, params propagation, retry policies, fidelity, fan-in/parallel, and `manager_loop` polling/steering are unchanged. Per-workflow budgets (`MaxTotalTokens`, `MaxCostCents`, `MaxWallTime`) remain scoped exactly as in non-bundled execution. **`.dipx` does not introduce new budget scopes.** Aggregate run-level budgets are an orthogonal Tracker feature; their absence in v1 is a known gap (see *Known v1 limitations*).

Hermeticity applies to **ref resolution only**, not to runtime data flow. `manager_loop` `SteerContext` injection, agent `Params` propagation, and any runtime-injected context are out of scope of the hermetic invariant.

#### Migration: two call sites

Tracker's existing code:

```go
p := parser.NewParser(input, "foo.dip")
wf, err := p.Parse()
// ... when hitting a subgraph node:
childPath := filepath.Join(filepath.Dir(parentPath), sub.Ref)
data, _ := os.ReadFile(childPath)
childParser := parser.NewParser(string(data), childPath)
child, _ := childParser.Parse()
```

After:

```go
src, err := dipx.Load(path)        // path may be "foo.dip" OR "foo.dipx"
wf := src.Entry()
// ... when hitting a subgraph node:
child, err := src.Workflow(sub.Ref, parentPath)
```

`parentPath` semantics:

- For `.dipx`: `parentPath` is bundle-relative (e.g., `workflows/api_design.dip`). `src.Entry()` is at `bundle.Manifest().Entry`; thread that path into the engine and through subgraph dispatch.
- For `.dip`: `parentPath` is whatever filesystem path the engine threads through today. `dirSource` resolves via `filepath.Join` exactly as `flatten.Resolver` does.

The contract is: pass the same `parentPath` value the engine already tracks for diagnostics/logging. If Tracker today does not track `parentPath` at the subgraph dispatch site, that is the one piece of plumbing the migration requires.

#### Why `Open` normalizes conditions

`Condition.Parsed` is populated by `simulate.EnsureConditionsParsed` from `Condition.Raw` (per `CLAUDE.md` and `simulate/condition.go`). Without normalization, two parallel goroutines that both reach the same workflow via `Source.Workflow` would race on writes to `Condition.Parsed` (it is mutated in place on the shared `*ir.Workflow`).

`Open` performs normalization once, eagerly, while no other goroutines hold the workflow. After Open returns, `*ir.Workflow` is read-only as far as `dipx` is concerned. Callers MUST treat returned workflows as read-only.

#### Format version skew

A `.dipx` with `format_version: N` where `N` is not in `SupportedFormatVersions()` returns `ErrUnsupportedFormatVersion` wrapped with the value seen and the supported set:

```go
return fmt.Errorf("%w: got %d (this build supports %v)",
    ErrUnsupportedFormatVersion, got, SupportedFormatVersions())
```

Tracker SHOULD surface this to operators with a remediation hint such as "upgrade Tracker to a build supporting format_version N". The exact UX is Tracker's responsibility.

#### Distribution surface

Tracker's existing input mechanisms work without change for `.dip` paths. `.dipx` adds:

- File path on disk → `dipx.Load(path)` (already supported by Tracker).
- HTTP/URL fetch → `dipx.OpenReader(bytes.NewReader(downloaded), int64(len(downloaded)))`. **If Tracker does not support HTTP fetch today, that is a Tracker work item, not a `.dipx` feature.**
- Stdin / pipe → buffer fully and use `OpenReader`.
- In-memory generation → `OpenReader`.

`dipx` defines nothing about networking, registries, caching, or signature verification.

## CLI

### New commands

```sh
dippin pack <entry.dip> [-o output.dipx]
```

- Walks every transitively-reachable subgraph ref from disk.
- Runs structural validation first (same checks as `dippin validate`); refuses to pack invalid input.
- Runs all path-safety checks; refuses to pack with informative errors on violations (case collisions across OS rules, Windows reserved names, etc.).
- Lint is not run; the user can run it independently.
- Defaults output to `./<basename>.dipx` next to the entry file.
- `-o -` writes to stdout.
- Errors loudly and exits non-zero on any violation.

```sh
dippin unpack <bundle.dipx> [-o destdir] [--force]
```

Extracts a `.dipx` to a directory. Defaults to `./<basename>/`. Without `--force`, refuses to overwrite existing files. All path-safety checks apply during extraction.

```sh
dippin inspect <bundle.dipx> [--no-verify]
```

Prints the manifest and a verification status footer:

```
format: 1
entry:  workflows/api_design.dip
files:
  workflows/api_design.dip                       sha256:abc123…
  workflows/interview_loop.dip                   sha256:def456…
  workflows/phases/code_review.dip               sha256:789abc…
status: VALID (8 files, 24831 bytes, format_version 1)
```

By default, `inspect` runs full `Open`-equivalent validation. `--no-verify` skips hash checks for forensic inspection of corrupt bundles (the status footer reads `INVALID: <reason>`).

### Existing commands — uniform extension

These accept `.dip` and `.dipx` interchangeably via `dipx.Load`:

`dippin validate`, `lint`, `doctor`, `simulate`, `parse`, `cost`, `coverage`, `unused`, `diff`.

Per-command behavior on a bundle:

- **Default**: operates on the entry workflow only.
- **`--all`** flag: runs on every workflow in the bundle.

### Validation layers

| Check | `.dip` | `.dipx` |
|---|---|---|
| Workflow structural validation (DIP001–009) | ✓ | ✓ on entry; ✓ on all with `--all` |
| Manifest well-formed + JSON encoding rules | — | ✓ |
| Format version recognized | — | ✓ |
| Path canonicalization (NFC, no `..`, no Windows reserved, etc.) | ✓ on resolution | ✓ |
| ZIP feature constraints (no encryption, etc.) | — | ✓ |
| Files in manifest exist in zip with matching SHA-256 | — | ✓ |
| Entry listed in `files[]` and matches byte-exact | — | ✓ |
| Hermetic invariant (refs in `workflows/`, all manifest-listed) | — | ✓ |
| No extra zip entries | — | ✓ (default; `OpenLax` tolerates) |
| Cycle and depth-cap checks on ref graph | ✓ via dirSource | ✓ |
| Conditions normalized (`Condition.Parsed`) | ✓ via dirSource | ✓ |
| Signature verification *(future)* | — | ✓ when present (v2) |

In `dippin validate`, integrity errors print first as a hard failure block; only if integrity is clean does the per-workflow lint run. They are never interleaved.

## Versioning

- `format_version: 1` is the only valid value today.
- Bumping `format_version` requires (1) a documented breaking change in this spec, (2) a migration tool, (3) the prior version stays supported for at least one release cycle.
- Reading rules across all versions: known version → read; unknown version → reject; never "warn and try."
- The `dipx` Go module ships with `dippin-lang` semver tags. Format support is documented in `SupportedFormatVersions()`.

### Forward compatibility

Tolerant decoding of unknown JSON keys is the v1 mechanism for additive evolution. Two rules:

1. **Tolerant decoding applies only after `format_version` is recognized.** A v1 reader receiving `format_version: 2` rejects before parsing further keys.
2. **Tolerant decoding accommodates only optional additions.** A future feature whose presence requires consumer behavior change (e.g., "presence of signatures implies mandatory verification") is a semantic change and MUST bump `format_version`.

The top-level key `signatures` is **reserved** for future use in v1. Readers MUST tolerate its presence without error. Writers MUST NOT emit it in v1.

## Error model

Bundle errors are surfaced as Go errors with sentinels for programmatic discrimination — not DIP codes:

```go
var (
    ErrUnsupportedFormatVersion = errors.New("unsupported format_version")
    ErrManifestMissing          = errors.New("manifest.json missing")
    ErrManifestInvalid          = errors.New("manifest.json malformed")
    ErrFileMissing              = errors.New("file listed in manifest not in zip")
    ErrFileUnexpected           = errors.New("zip entry not listed in manifest") // strict mode
    ErrHashMismatch             = errors.New("file hash does not match manifest")
    ErrPathUnsafe               = errors.New("unsafe path")
    ErrEntryNotInManifest       = errors.New("entry not listed in files[]")
    ErrRefEscape                = errors.New("subgraph ref escapes bundle root")
    ErrRefCycle                 = errors.New("subgraph ref cycle detected")
    ErrCapExceeded              = errors.New("bundle exceeds size or file-count cap")
    ErrEntryParse               = errors.New("entry workflow failed to parse")
    ErrSubgraphParse            = errors.New("subgraph workflow failed to parse")
    ErrZipFeatureForbidden      = errors.New("zip uses a forbidden feature")
)
```

Each sentinel is wrapped with rich context at the throw site (file path, expected vs actual hash, the unsupported version number, etc.). Consumers use `errors.Is` for discrimination.

Why no DIP codes for bundle errors:

1. Different audience — DIP findings are for the workflow author iterating on their `.dip`; bundle errors are for the operator running a `.dipx`.
2. Different lifecycle — DIP codes multiply over time; bundle errors are bounded.
3. Stable Go error sentinels are what consumers actually need for programmatic handling.

Tracker SHOULD log bundle errors at error level with a stable `error_class: bundle_integrity` field so operators can grep by category alongside DIP findings.

## Concurrency and immutability

`Source` is safe for concurrent reads.

- `Open` reads each zip entry into its own `[]byte`, closes zip internals, parses each workflow once, normalizes conditions once, stores results in immutable maps, and returns. No file handles held; no mutable state remains.
- `Source.Workflow(refPath, relativeTo)` is path validation + map lookup over already-parsed-and-normalized IR. Lock-free.
- `*Bundle.Extract` writes to disk and is not concurrent with itself, but is safe alongside concurrent reads of the same `Bundle`.
- `Pack` is reentrant only when called with non-overlapping outputs. Two `Pack` calls writing to the same `io.Writer` is undefined.
- `dirSource` cache is implemented with a thread-safe LRU; concurrent first-call-misses on the same path are coalesced.

Returned `*ir.Workflow` values MUST be treated as read-only by callers. Any consumer mutating IR (e.g., a future tooling pass) MUST clone the workflow first. The spec does not provide a clone helper; consumers can use `parser.NewParser` on the original source to obtain a fresh copy if needed.

## Known v1 limitations

The following are known design trade-offs and intended follow-ups, called out so adopters understand the v1 scope:

1. **No cryptographic signatures.** Authenticity over untrusted channels is unaddressed in v1. Signatures are v2 work. `.dipx` v1 is appropriate for transit between trusted parties only.
2. **No bundle-level budget aggregation.** Per-workflow budgets remain. Operators wanting a run-level token / cost / time cap need a separate Tracker feature.
3. **No cross-language conformance suite.** This spec is normative for the Go reference implementation. A Rust/Python re-implementation would need a v2 conformance suite.
4. **No hash algorithm agility.** SHA-256 is locked in v1. Migrating to SHA-3 or BLAKE3 requires `format_version` bump and is a deliberate one-way door.
5. **No external asset bundling.** Only `.dip` files are bundled. When the language grows asset-reference syntax, `.dipx` will need a corresponding extension.
6. **No streaming Open.** All workflow bytes are read into memory at Open time. Practical limit: 100 MB total uncompressed (the cap).
7. **No `OpenLite` mode that drops file bytes.** `Bundle.ReadFile` requires bytes to be retained. Tracker uses ~2× workflow bytes (raw + parsed IR) per open bundle. Memory profile of long-running Tracker processes with many concurrent bundles SHOULD be measured before scale deployment.

## Testing strategy

### Unit tests in `dipx/`

```
dipx/
  dipx_test.go        # Pack/Open/Load round-trips, format_version handling
  manifest_test.go    # JSON encoding rules, duplicate-key rejection, version
  resolve_test.go     # Path canonicalization, ref resolution, cycles
  zipio_test.go       # Forbidden ZIP features rejected
  testdata/
    well-formed.dipx              # baseline (a frozen byte-vector golden file)
    corrupt-manifest.dipx
    bom-manifest.dipx             # BOM-prefixed manifest rejected
    duplicate-keys.dipx
    duplicate-files-entry.dipx
    duplicate-zip-entry.dipx
    central-dir-mismatch.dipx
    cp437-filename.dipx
    encrypted-entry.dipx
    bzip2-compression.dipx
    symlink-entry.dipx
    mismatched-hash.dipx
    missing-file.dipx
    extra-file.dipx               # tolerated by OpenLax, rejected by Open
    bad-version.dipx              # format_version: 999
    bad-version-zero.dipx
    bad-version-negative.dipx
    bad-version-float.dipx
    bad-version-string.dipx
    escape-ref.dipx
    nfd-path.dipx                 # NFD-normalized name rejected
    windows-reserved.dipx         # CON.dip rejected
    nul-byte-path.dipx
    long-path.dipx                # > 1024 bytes
    deep-path.dipx                # > 16 components
    self-loop.dipx                # a -> a
    cycle-3.dipx                  # a -> b -> c -> a
    diamond.dipx                  # a -> b -> d, a -> c -> d (NOT a cycle)
    depth-65.dipx
    too-many-files.dipx
    too-big.dipx                  # generated in-test
    bomb-ratio.dipx               # high compression-ratio entry
    signatures-present.dipx       # tolerant decoding: signatures key tolerated
    bad-extension.dipx            # entry not .dip
```

Required test cases (mapping to spec sections):

- **Round-trip:** Pack → Open → assert byte-equality of every file's contents (`Bundle.ReadFile` matches source bytes) AND `reflect.DeepEqual` on parsed IR.
- **Reproducibility:** Pack the same source tree twice; assert byte-identical `.dipx` outputs.
- **Hash sentinels:** Corrupt one byte; assert `errors.Is(err, ErrHashMismatch)` with file path in error context.
- **Path safety (every entry):** NFD path, NUL byte, Windows reserved, leading whitespace, trailing dot, `..` in manifest path, absolute path, backslash, > 1024 bytes, > 16 components, non-`.dip` extension.
- **ZIP features:** encrypted entry, multi-disk, BZIP2, symlink mode, CP437 filename, central-dir/local-header mismatch, duplicate zip entries, directory entries — each rejected with `ErrZipFeatureForbidden` or appropriate sentinel.
- **Manifest tamper:** duplicate top-level keys, duplicate `files[].path`, BOM prefix, oversized manifest (> 1 MB), trailing data after JSON object.
- **`format_version`:** values `0`, `-1`, `1.0`, `"1"`, `999` each rejected appropriately.
- **Cycle detection:** self-loop, 3-cycle, diamond (must succeed — not a cycle), depth-65 chain.
- **Strict vs Lax:** bundle with `__MACOSX/` extra → `Open` rejects, `OpenLax` tolerates.
- **Hermetic at runtime:** crafted bundle whose manifest omits a workflow but the zip contains it; `Source.Workflow` resolves to that name → `ErrFileMissing` (the runtime hermetic check).
- **Path safety at runtime:** call `Source.Workflow("../../../etc/passwd", validParent)` on opened bundle → `ErrPathUnsafe`.
- **Concurrency:** 1000 goroutines with starter-channel adversarial timing calling `Source.Workflow` on the same `*Bundle` under `-race`. Repeat for `*dirSource` (concurrent first-call cache fill). No data races; all callers see equal IR.
- **Eager parse regression guard:** assert `Bundle.Workflow` returns identical pointers across calls (caching invariant).
- **`Condition.Parsed` is populated:** assert that for every workflow in a bundle with conditional edges, `Edge.Condition.Parsed` is non-nil after Open.
- **Polymorphic `Load`:** same multi-workflow tree loaded as `.dip` and as `.dipx` produces structurally-equal entry workflows. Asymmetry on broken-subgraph detection (`.dipx` fails at Open; `.dip` fails at first `Workflow` call) is asserted explicitly.
- **In-test cap generators:** generate too-big and too-many-files bundles in test code (no committed > 100MB testdata).
- **Frozen golden:** `well-formed.dipx` is byte-exact; any future change to Pack output that drifts the bytes fails this test (proves reproducibility holds across refactors).

### Integration test (extends `validator/lint_examples_test.go`)

Add `TestPackExamples`: walk `examples/`, pack each `.dip` that has subgraphs, round-trip through `dipx.Open`, then run lint. Asserts examples pack cleanly and lint output matches on-disk source. Plus: a synthetic deep-tree fixture (3+ levels, diamond pattern) committed under `dipx/testdata/` to exercise the multi-level case `examples/` does not cover.

### CLI tests

`cmd/dippin/cmd_pack_test.go`, `cmd_unpack_test.go`, `cmd_inspect_test.go`. At least 4–5 failure paths each, including: missing entry, entry-is-directory, validation-fails, ref-escapes-source-root, output-dir-unwritable, output-file-exists-without-force, bad extension, stdin/stdout (`-` argument), corrupt manifest (`inspect` with and without `--no-verify`), zip-slip extraction attempt.

### Justfile

Add `pack-examples` recipe that descends into `examples/**/*.dip` (not just `examples/*.dip` — current `validate-examples` misses sub-examples). Extend `just check` to call it after `validate-examples`.

### Coverage

After implementation, run `just cover` and quote actual `parser/` and `validator/` percentages as concrete targets. The unit tests above cover every error sentinel; coverage gaps surface in the report and become follow-ups.

## CLAUDE.md amendment (companion change)

The architectural rule "Packages import `ir` but not each other (except analysis packages that compose: doctor → validator + coverage + cost, unused → coverage + cost)" MUST be amended in the same change as the `dipx` package, adding:

> **Loader tier**: `dipx` may compose `ir + parser + simulate`. The exemption is bounded — `dipx` MUST NOT import `validator`, `cost`, `formatter`, or any other analysis package.

This documents the architectural rationale for `dipx` reaching into `simulate` (for `EnsureConditionsParsed`) and prevents the boundary from drifting.
