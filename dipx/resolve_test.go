package dipx

import (
	"errors"
	"strings"
	"testing"
)

func TestCanonicalize_Valid(t *testing.T) {
	cases := []string{
		"workflows/foo.dip",
		"workflows/sub/bar.dip",
		"workflows/a/b/c/d/e/f/g/h/i/j/k/l/m/n/o.dip", // 16 components, just at cap
	}
	for _, in := range cases {
		got, err := Canonicalize(in)
		if err != nil {
			t.Errorf("Canonicalize(%q): unexpected error: %v", in, err)
			continue
		}
		if got != in {
			t.Errorf("Canonicalize(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestCanonicalize_Rejects(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"absolute", "/workflows/foo.dip"},
		{"backslash", "workflows\\foo.dip"},
		{"dot-dot", "workflows/../etc/passwd"},
		{"leading-dot", "./workflows/foo.dip"},
		{"empty-component", "workflows//foo.dip"},
		{"nul", "workflows/foo\x00.dip"},
		{"control", "workflows/foo\x01.dip"},
		{"del", "workflows/foo\x7f.dip"},
		{"trailing-space", "workflows/foo .dip"},
		{"leading-space", "workflows/ foo.dip"},
		{"trailing-dot", "workflows/foo.dip.."},
		{"win-reserved-con", "workflows/CON.dip"},
		{"win-reserved-com1", "workflows/COM1.dip"},
		{"win-reserved-con-multi-ext", "workflows/CON.tar.dip"},
		{"win-reserved-com1-multi-ext", "workflows/COM1.foo.dip"},
		{"win-reserved-aux-with-prefix", "workflows/AUX.something.dip"},
		{"missing-extension", "workflows/foo"},
		{"wrong-extension", "workflows/foo.txt"},
		{"uppercase-extension", "workflows/foo.DIP"},
		{"too-many-components", "workflows/a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p.dip"}, // 17
		{"too-long", "workflows/" + strings.Repeat("a", 1020) + ".dip"},
		{"not-under-workflows", "other/foo.dip"},
		{"empty", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Canonicalize(c.in)
			if err == nil {
				t.Fatalf("Canonicalize(%q) succeeded, expected error", c.in)
			}
			if !errors.Is(err, ErrPathUnsafe) {
				t.Fatalf("error = %v, want ErrPathUnsafe", err)
			}
		})
	}
}

func TestCanonicalize_ErrorContext(t *testing.T) {
	_, err := Canonicalize("workflows/CON.dip")
	if err == nil {
		t.Fatal("expected error")
	}
	var be *BundleError
	if !errors.As(err, &be) {
		t.Fatalf("expected *BundleError, got %T", err)
	}
	if be.Path != "workflows/CON.dip" {
		t.Errorf("Path = %q, want full path", be.Path)
	}
	if be.Detail == "" {
		t.Errorf("Detail empty; expected mention of the offending component")
	}
}

func TestCanonicalize_RejectsNFD(t *testing.T) {
	// é as NFD: e + U+0301 (combining acute)
	in := "workflows/café.dip"
	_, err := Canonicalize(in)
	if err == nil {
		t.Fatal("expected error for NFD-encoded path")
	}
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}
