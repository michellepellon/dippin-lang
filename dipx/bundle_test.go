package dipx

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func newTestBundle(t *testing.T) *Bundle {
	t.Helper()
	hash := strings.Repeat("a", 64)
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: hash},
			{Path: "workflows/b.dip", SHA256: hash},
		},
	}
	wfA := &ir.Workflow{Name: "A", Start: "S", Exit: "E"}
	wfB := &ir.Workflow{Name: "B", Start: "S", Exit: "E"}
	return &Bundle{
		manifest:      m,
		workflows:     map[string]*ir.Workflow{"workflows/a.dip": wfA, "workflows/b.dip": wfB},
		fileBytes:     map[string][]byte{"workflows/a.dip": []byte("a-bytes"), "workflows/b.dip": []byte("b-bytes")},
		manifestBytes: []byte(`{"format_version":1}`),
	}
}

func TestBundle_Manifest_ReturnsCopy(t *testing.T) {
	b := newTestBundle(t)
	m1 := b.Manifest()
	m1.Entry = "MUTATED"
	if m1.Entry != "MUTATED" {
		t.Fatal("local mutation lost")
	}
	// Mutating files[] should also not affect the bundle.
	m1.Files[0].Path = "MUTATED-PATH"
	m2 := b.Manifest()
	if m2.Entry == "MUTATED" {
		t.Fatal("Manifest() should return a copy of Entry")
	}
	if m2.Files[0].Path == "MUTATED-PATH" {
		t.Fatal("Manifest() should return a copy of Files")
	}
}

func TestBundle_Identity(t *testing.T) {
	b := newTestBundle(t)
	id := b.Identity()
	if id == [32]byte{} {
		t.Fatal("Identity should be non-zero")
	}
}

func TestBundle_Lookup(t *testing.T) {
	b := newTestBundle(t)
	wf, err := b.Lookup("workflows/a.dip")
	if err != nil {
		t.Fatal(err)
	}
	if wf.Name != "A" {
		t.Fatalf("Name = %q", wf.Name)
	}
	_, err = b.Lookup("workflows/missing.dip")
	if !errors.Is(err, ErrFileMissing) {
		t.Fatalf("err = %v, want ErrFileMissing", err)
	}
}

func TestBundle_ReadFile(t *testing.T) {
	b := newTestBundle(t)
	got, err := b.ReadFile("workflows/a.dip")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("a-bytes")) {
		t.Fatalf("ReadFile mismatch")
	}
}

func TestBundle_Lookup_RejectsUnsafePath(t *testing.T) {
	b := newTestBundle(t)
	_, err := b.Lookup("../../../etc/passwd")
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}

func TestBundle_ReadFile_RejectsUnsafePath(t *testing.T) {
	b := newTestBundle(t)
	_, err := b.ReadFile("../../../etc/passwd")
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}

func TestBundle_Workflow(t *testing.T) {
	b := newTestBundle(t)
	wf, err := b.Workflow("b.dip", "workflows/a.dip")
	if err != nil {
		t.Fatal(err)
	}
	if wf.Name != "B" {
		t.Fatalf("Name = %q", wf.Name)
	}
}

func TestBundle_Workflow_RefMissing(t *testing.T) {
	b := newTestBundle(t)
	_, err := b.Workflow("missing.dip", "workflows/a.dip")
	if !errors.Is(err, ErrFileMissing) {
		t.Fatalf("err = %v, want ErrFileMissing", err)
	}
}

func TestBundle_Resolve(t *testing.T) {
	b := newTestBundle(t)
	got, err := b.Resolve("b.dip", "workflows/a.dip")
	if err != nil {
		t.Fatal(err)
	}
	if got != "workflows/b.dip" {
		t.Fatalf("got %q", got)
	}
}
