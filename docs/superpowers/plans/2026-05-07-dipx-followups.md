# `.dipx` v1 Follow-ups (deferred from gate reviews)

This file collects findings from the per-phase gate reviews that are NOT blocking v1 but should be addressed in v1.1 or later.

## Spec gaps in path canonicalization (Phase 2 gate)

Path canonicalization rules 1 and 4 (per spec § "Path canonicalization") cover NFC normalization and ASCII control-character rejection respectively, but do not address:

- **NFKC compatibility equivalents**: e.g., `ﬁ` (U+FB01) and `fi` (U+0066 U+0069) are visually identical but byte-distinct, both NFC-stable.
- **Format-class Unicode characters**: ZWJ (U+200D), ZWNJ (U+200C), ZWNBSP/BOM (U+FEFF) are mid-string-invisible.
- **Bidi controls**: U+202A–U+202E, U+2066–U+2069 reorder display.
- **IDN homographs**: Cyrillic `а` (U+0430) vs Latin `a` (U+0061) for visual spoofing.

These pass current Canonicalize rules and could enable visual-spoofing attacks at the operator-UI layer.

**Disposition**: defer to v1.1. Investigate adding NFKC normalization rejection or a "no format-class characters" rule. Coordinate with Tracker (which presents bundle paths to operators).

## Wider "any other separator" interpretation (Phase 2 gate)

Spec rule 2 says "Backslash `\` and any other separator MUST be rejected." Current implementation only rejects `\`. Other Unicode separator-like glyphs (U+2044 FRACTION SLASH, U+FF0F FULLWIDTH SOLIDUS, U+29F8 BIG SOLIDUS, U+FF3C FULLWIDTH BACKSLASH) are not blocked. None are real path separators on supported OSes, but the spec wording is broader than the implementation.

**Disposition**: narrow the spec wording in v1.1 (recommended), or broaden the rune check in `checkRune`. Practical risk is low.

## Test coverage gaps (Phase 2 gate)

Adversarial input categories not yet tested:
- Bidi marks (U+200E/U+200F)
- ZWNBSP/BOM (U+FEFF)
- IDN homograph pairs
- Percent-encoded `..` (e.g., `%2e%2e`) — spec confirms this is treated literally; tests would lock in the behavior.

**Disposition**: add as v1.1 test additions if NFKC normalization is added.

## Manifest decoder error-context (Phase 3 gate)

The crypto-discipline gate flagged that `BundleError.Path` for `ErrManifestInvalid` and `ErrUnsupportedFormatVersion` is sometimes empty (preflight, JSON pre-pass) or carries the field name (`format_version`) rather than the bundle path. The spec § "Per-sentinel error context" table calls for the bundle path. The manifest decoder runs before the bundle path is in scope; the right architectural fix is for `Open` (Task 13) to enrich errors with the bundle path before returning. **Disposition**: revisit in Phase 5 when Open is wired up; spec wording may also need tightening to clarify that `Path` is "where the error originated" (which can be a field name) vs "the bundle file path".

## Non-canonical `entry` returns `ErrPathUnsafe` (Phase 3 gate)

`verifyEntryInFiles` returns `ErrPathUnsafe` (via `Canonicalize`) for a non-canonical `entry`. Spec is ambiguous between this and `ErrManifestInvalid`. **Disposition**: defensible interpretation; document and move on.

## Manifest case-fold uses `strings.ToLower` (Phase 3 gate)

Same Unicode case-fold concern raised in Phase 2. `verifyOneFile` uses `strings.ToLower` for case-fold-duplicate detection; full Unicode case-folding (e.g. `cases.Fold()` from `golang.org/x/text/cases`) would catch German `ß ↔ ss`, Turkish dotless `I ↔ i`, Greek final sigma. **Disposition**: bundled with the existing v1.1 case-folding upgrade.

## Cosmetic minors (Phase 3 gate, deferred)

- `assertCanonicalIntLiteral` is partly dead code (Int64 + JSON grammar already reject most non-canonical literals). Defense in depth; leave as-is.
- Missing `sha256` `Detail` message could distinguish "missing" from "malformed"; current "sha256 not 64-char lowercase hex" is acceptable.
- `assertNoTrailingTokens` `dec.More()` check is partly redundant with the EOF Token check. Belt-and-suspenders; leave.

## Phase 4 ZIP I/O findings (deferred)

### Central-directory ↔ local-header mismatch detection (Phase 4 security gate, C2)

The spec mandates that the central directory and local file headers must agree on filename and uncompressed size to defend against ZIP parser-confusion (smuggling/cloak attacks). Go's `archive/zip` reader exposes only central-directory metadata; checking the local header requires either a custom zip reader or wrapping `archive/zip`'s internals.

**Disposition:** v1.1. Practical risk in v1 is mitigated by hash verification: whatever bytes get decompressed are hashed, and if they don't match the manifest, `ErrHashMismatch` fires. The defense gap is for cross-implementation interop (a non-Go reader could disagree with our reader), but v1 is Go-only per spec § "Conventions".

### Compression-ratio cap (1000:1) not enforced (Phase 4 security gate, C3)

Spec § "Streaming cap enforcement" requires per-file compression-ratio cap of 1000:1. Currently only the absolute uncompressed-size cap (50 MB per file) is enforced. A maximally-compressed entry could expand within the absolute cap.

**Disposition:** v1.1. Implementation requires `f.OpenRaw()` plus manual deflate chain to count compressed input bytes alongside decompressed output. Significant restructure. The 50 MB absolute cap provides DoS protection in the meantime.

### Hardlink / repeated central-dir pointer detection (Phase 4 security gate, I5)

Two central-directory records pointing at the same local-header offset under different names slip through current duplicate-name detection. Defense-in-depth gap related to C2.

**Disposition:** v1.1. Bundled with C2 fix.

### `ErrZipTruncated` over-broadens (Phase 4 gates I6, L3)

Mid-stream EOF, corrupt deflate, malformed central directory, and `ErrInsecurePath` from `archive/zip` all map to `ErrZipTruncated`. Operationally distinct conditions are conflated.

**Disposition:** v1.1 polish. Distinguish via dedicated sentinels (`ErrZipFormat`, `ErrZipPathInsecure`).

### `archive/zip.ErrInsecurePath` handling (Phase 4 security gate, I7)

`zip.NewReader` may return both a reader AND `ErrInsecurePath` (Go version dependent on `GODEBUG=zipinsecurepath`). Current code treats this as fatal-zip; should map to `ErrPathUnsafe`.

**Disposition:** v1.1. Test by setting `GODEBUG=zipinsecurepath=0` in environment.

### CI grep for `verifiedBytes{` literals (Phase 4 crypto gate, M1)

The `verifiedBytes` zero-value `verifiedBytes{}` and one-arg constructor `newVerifiedBytes(buf)` are the only legitimate constructions. A future helper inside `dipx` could accidentally write `verifiedBytes{b: rawBytes}` and bypass `verifyAndReadEntry`. Spec calls for a CI grep at Task 26 final hardening.

**Disposition:** Already in plan as Task 26 step 3. No action needed here.

## Phase 5 gate findings (deferred)

### checkExtraEntries placement (Phase 5 gate, M2)

`checkExtraEntries` is an unmentioned step inserted between manifest-shape (step 4) and hash-verify (step 5). Its placement is defensible (rejects junk before we waste CPU hashing files we'd reject), but the spec's normative ordering (lines 210-225) doesn't enumerate it, and `ErrFileUnexpected` isn't in the spec's error precedence list.

**Disposition:** v1.1 spec clarification. Either add as documented step 4.5 or fold into manifest-shape validation conceptually.

### parseAllWorkflows / detectCycles strictness asymmetry (Phase 5 gate, L2/L3)

`parseAllWorkflows` parses all manifest-listed workflows (stronger than spec post-condition #4 which says "every workflow parses"). `detectCycles` only DFS's from `m.Entry`, missing cycles in unreachable workflows. Inconsistent: the parse pass is broader than necessary, the cycle pass is narrower than necessary.

**Disposition:** v1.1 polish. Both passes should iterate every manifest-listed workflow, OR both should walk only entry-reachable subgraph. Pick one convention. Currently the broader-parse + narrower-cycle behavior is harmless but inconsistent.

## Phase 6 gate findings (deferred)

### dirSource doesn't apply Canonicalize-style validation (Phase 6, H2)

`dirSource.resolveDir` performs filesystem-style validation but doesn't reject NUL bytes, control chars, NFD-encoded paths, or Windows-reserved names. The spec is ambiguous — dirSource paths are filesystem-native and "trusted by virtue of being read from local disk" per the parseDipFile spec note. **Disposition:** v1.1. If a tightening is desired, factor a Canonicalize-equivalent for filesystem paths (which would need different rules — absolute paths are valid, etc.).

### dirSource cache is unbounded with global mutex (Phase 6, M1/M2)

Spec § "Source implementations" mandates "LRU of 256 entries with singleflight.Group for cold-call coalescing." Implementation is unbounded `map[string]*ir.Workflow` guarded by sync.Mutex held across parseDipFile (disk I/O). Two issues: long-running Tracker processes leak parsed IR, and concurrent reads on different paths serialize on the global mutex. **Disposition:** v1.1. Drop in `golang.org/x/sync/singleflight` and a small LRU (e.g. `github.com/hashicorp/golang-lru/v2`) — or hand-roll. The singleflight key is the bundle-relative path; the LRU bounds memory.

### Extract atomicity edge cases (Phase 6, M3/M4)

`Extract` removes destDir before rename when `--force`, creating a window where neither old nor new exists. If `os.Rename` fails (e.g. `EXDEV` cross-device), staging is left behind. **Disposition:** v1.1 polish. Use rename-old-aside / rename-new-into-place / remove-aside pattern; defer-cleanup on rename failure.

### Source.Workflow doesn't take context (Phase 6, L4)

`Source.Workflow(refPath, relativeTo)` performs disk I/O for dirSource but no ctx. Spec is internally inconsistent: § "Cancellation" line 272 says I/O entry points take ctx, but § "Tracker integration contract" example shows ctx-less Workflow. **Disposition:** v1.1 spec clarification. Decide whether Source is a "fast lookup" or "I/O entry point." If the latter, breaking signature change.

## Phase 7 gate findings (deferred)

### Pack compression-ratio cap (Phase 7, H1 partial)

Spec § "Soft caps" lists 1000:1 compression-ratio as a producer-side limit. Pack only checks per-file uncompressed size (50MB); ratio is unchecked. Implementing requires either tracking compressed bytes during deflate (manual deflate chain + counter) or post-compression check via writer hooks. **Disposition:** v1.1. Per-file 50MB cap provides absolute DoS protection in the meantime.

### Pack parent-directory symlink check (Phase 7, M1)

`walkSourceTree` checks Lstat on the leaf file but doesn't walk parent components for directory symlinks. A directory symlink in the path tree silently re-roots `rootDir`. **Disposition:** v1.1. Practical risk is low (developer source trees, not adversarial input). Fix requires per-component Lstat or a helper.

### Pack O_NOFOLLOW (Phase 7, M2)

Spec § "Reproducible Pack" mandates `O_NOFOLLOW` on file open. Current implementation uses `os.Lstat` + `os.ReadFile`, leaving a TOCTOU window if a symlink is swapped between Lstat and ReadFile. **Disposition:** v1.1. Implementing `O_NOFOLLOW` requires platform-specific syscalls (Linux/macOS/BSD have `O_NOFOLLOW`; Windows uses different attributes). The Lstat-first approach mitigates the threat for non-adversarial source trees.

### Pack frozen golden test (Phase 7, L1, L2)

`encodeManifestCanonical` relies on Go's `json.Marshal` field-order behavior. `TestPack_Reproducible` proves in-process determinism but doesn't catch cross-Go-version drift via a frozen byte vector. Spec testing strategy mentions a `well-formed.dipx` golden file. **Disposition:** v1.1. Add as part of Task 25's hardening.

### Pack symlink coverage in tests (Phase 7, L3)

`TestPack_RejectsSymlink` only tests symlink-as-entry. Doesn't test transitive ref symlinks or directory symlinks. **Disposition:** v1.1.
