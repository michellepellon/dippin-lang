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
