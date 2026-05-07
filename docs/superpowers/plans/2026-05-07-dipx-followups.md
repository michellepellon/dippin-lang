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
