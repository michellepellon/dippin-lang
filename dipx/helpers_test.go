package dipx

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestReadManifestEntry_Happy(t *testing.T) {
	src := `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}`
	cz := buildSingleEntryZip(t, "manifest.json", []byte(src))
	raw, err := readManifestEntry(cz)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != src {
		t.Fatalf("got %q", raw)
	}
}

func TestReadManifestEntry_Missing(t *testing.T) {
	cz := buildSingleEntryZip(t, "workflows/a.dip", []byte("x"))
	_, err := readManifestEntry(cz)
	if !errors.Is(err, ErrManifestMissing) {
		t.Fatalf("err = %v, want ErrManifestMissing", err)
	}
}

func TestReadManifestEntry_OversizedRejected(t *testing.T) {
	big := bytes.Repeat([]byte("a"), maxManifestSize+10)
	cz := buildSingleEntryZip(t, "manifest.json", big)
	_, err := readManifestEntry(cz)
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}
