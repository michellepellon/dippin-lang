package dipx

import (
	"errors"
	"testing"
)

func TestBundleErrorIs(t *testing.T) {
	be := &BundleError{Sentinel: ErrHashMismatch, Path: "workflows/foo.dip", Detail: "expected: a; actual: b"}
	if !errors.Is(be, ErrHashMismatch) {
		t.Fatal("errors.Is should match Sentinel")
	}
	if errors.Is(be, ErrManifestInvalid) {
		t.Fatal("errors.Is should not match unrelated sentinel")
	}
}

func TestBundleErrorAs(t *testing.T) {
	be := &BundleError{Sentinel: ErrHashMismatch, Path: "workflows/foo.dip", Detail: "expected: a; actual: b"}
	var target *BundleError
	if !errors.As(be, &target) {
		t.Fatal("errors.As should populate target")
	}
	if target.Path != "workflows/foo.dip" {
		t.Fatalf("Path = %q, want workflows/foo.dip", target.Path)
	}
}

func TestBundleErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying parser error")
	be := &BundleError{Sentinel: ErrEntryParse, Path: "workflows/foo.dip", Cause: cause}
	if !errors.Is(be.Unwrap(), cause) {
		t.Fatal("Unwrap should return Cause")
	}
}

func TestBundleErrorMessage(t *testing.T) {
	be := &BundleError{Sentinel: ErrHashMismatch, Path: "workflows/foo.dip", Detail: "expected: a; actual: b"}
	got := be.Error()
	want := "hash mismatch: workflows/foo.dip: expected: a; actual: b"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestNewError(t *testing.T) {
	cause := errors.New("boom")
	err := newError(ErrEntryParse, "workflows/foo.dip", "line 1", cause)
	if !errors.Is(err, ErrEntryParse) {
		t.Fatal("newError result should match sentinel via errors.Is")
	}
	var be *BundleError
	if !errors.As(err, &be) {
		t.Fatal("newError result should be a *BundleError")
	}
	if be.Path != "workflows/foo.dip" || be.Detail != "line 1" {
		t.Fatalf("BundleError fields not populated: %+v", be)
	}
	if !errors.Is(be.Cause, cause) {
		t.Fatalf("BundleError.Cause = %v, want %v", be.Cause, cause)
	}
}

func TestEnrichBundlePath_RewritesManifestInvalidPath(t *testing.T) {
	err := newError(ErrManifestInvalid, "format_version", "must be an integer literal", nil)
	enriched := enrichBundlePath(err, "/tmp/foo.dipx")
	var be *BundleError
	if !errors.As(enriched, &be) {
		t.Fatalf("enriched = %T, want *BundleError", enriched)
	}
	if be.Path != "/tmp/foo.dipx" {
		t.Errorf("Path = %q, want %q", be.Path, "/tmp/foo.dipx")
	}
	if be.Detail != "format_version: must be an integer literal" {
		t.Errorf("Detail = %q, want %q", be.Detail, "format_version: must be an integer literal")
	}
	if !errors.Is(enriched, ErrManifestInvalid) {
		t.Error("errors.Is(enriched, ErrManifestInvalid) = false, want true")
	}
}

func TestEnrichBundlePath_RewritesUnsupportedFormatVersionPath(t *testing.T) {
	err := newError(ErrUnsupportedFormatVersion, "", "got 99; supports [1]", nil)
	enriched := enrichBundlePath(err, "/tmp/foo.dipx")
	var be *BundleError
	if !errors.As(enriched, &be) {
		t.Fatalf("enriched = %T, want *BundleError", enriched)
	}
	if be.Path != "/tmp/foo.dipx" {
		t.Errorf("Path = %q, want %q", be.Path, "/tmp/foo.dipx")
	}
	if be.Detail != "got 99; supports [1]" {
		t.Errorf("Detail = %q, want unchanged", be.Detail)
	}
}

func TestEnrichBundlePath_LeavesNonManifestErrors(t *testing.T) {
	err := newError(ErrHashMismatch, "workflows/foo.dip", "expected: X, actual: Y", nil)
	enriched := enrichBundlePath(err, "/tmp/foo.dipx")
	var be *BundleError
	if !errors.As(enriched, &be) {
		t.Fatalf("enriched = %T, want *BundleError", enriched)
	}
	if be.Path != "workflows/foo.dip" {
		t.Errorf("Path = %q, want %q (unchanged for non-manifest sentinel)", be.Path, "workflows/foo.dip")
	}
}

func TestEnrichBundlePath_PassesThroughNonBundleError(t *testing.T) {
	err := errors.New("plain error")
	enriched := enrichBundlePath(err, "/tmp/foo.dipx")
	if enriched.Error() != "plain error" {
		t.Errorf("plain error mutated: %v", enriched)
	}
}

func TestEnrichBundlePath_NilIsNil(t *testing.T) {
	if enrichBundlePath(nil, "/tmp/foo.dipx") != nil {
		t.Error("nil err should pass through as nil")
	}
}
