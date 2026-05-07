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
