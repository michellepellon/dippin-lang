# `.dipx` Implementation — Phase Gate Log

## Phase 0 gate (2026-05-07)

- **Reviewer:** fast-track (build only)
- **Result:** PASS
- **Summary:** Bootstrap complete. `just build` and pre-commit hook (vet, golangci-lint, race tests, complexity, validate-examples) all green. `golang.org/x/text` already transitively pinned at v0.3.3, which includes `unicode/norm`. No `go.mod`/`go.sum` changes needed. Commit `a98f369`.

## Phase 1 gate (2026-05-07)

- **Reviewer:** fast-track + spec/quality reviewers
- **Result:** PASS
- **Summary:** 15 sentinels per spec, `BundleError` with `Is`/`Unwrap` extension protocol correctly implemented, idiomatic `Error()` formatting, 5 tests pass. Spec reviewer confirmed exact match. Code-quality reviewer: no critical/important issues; stylistic minors only. `just check` clean. Implementer adjusted `TestBundleErrorUnwrap` to satisfy `errorlint` (intent preserved per spec reviewer); added `TestNewError` to keep unexported `newError` helper from tripping `unused` lint (CLAUDE.md forbids `//nolint`). Commit `9d8322b`.

## Phase 2 gate (2026-05-07)

- **Reviewer:** mandatory squad — security subagent (parallel) + PAL spot-check
- **Result:** REMEDIATED → PASS
- **Summary:** Tasks 2 (Canonicalize), 3 (resolveLexically), 4 (cycle detection) implemented and committed. Code-quality reviewer (Task 2) caught a spec-rule-6 bypass (multi-extension Windows-reserved `CON.tar.dip` accepted because `stripExt` used `LastIndexByte`); fixed in commit `2de1a20`. Phase 2 gate (security + PAL) confirmed composition is sound, cycle detection correct, recently-fixed reserved check correct. Two important findings remediated in commit `92bd4da`: ErrRefCycle now reports full cycle path (`a → b → c → a`); resolveLexically's path.Clean usage documented for the future Task 26 CI grep allowlist. 4 new tests added (full-cycle-path, at-cap-success, empty-graph, adversarial-composition). Spec-level gaps (NFKC, ZWJ/ZWNJ, bidi, IDN homographs, "any other separator" wording) deferred to v1.1 in `docs/superpowers/plans/2026-05-07-dipx-followups.md` (commit `2de4034`). 48 dipx tests passing.

## Phase 3 gate (2026-05-07)

- **Reviewer:** mandatory — crypto-discipline subagent + PAL spot-check
- **Result:** REMEDIATED → PASS
- **Summary:** Tasks 5 (Manifest types + decoder) and 6 (verifyManifestShape) implemented. Spec compliance + code quality reviews PASS (one important finding: doc comment over-promised; fixed in commit `eb40a51`). Phase 3 gate confirmed architectural soundness: duplicate-key detection at every level, top-level-only signatures rejection, integer-only `format_version` gauntlet, depth cap. One important spec gap (missing `path` returned `ErrPathUnsafe` instead of `ErrManifestInvalid`) and two test gaps remediated in commit `6c18d0d`. Deferred findings (per-sentinel `Path` semantics across layers; ErrPathUnsafe vs ErrManifestInvalid for non-canonical entry; case-fold via strings.ToLower vs Unicode `cases.Fold`) appended to followups in commit `d825fed`. 67 dipx tests passing.
