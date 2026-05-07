package dipx

import (
	"archive/zip"
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

func TestVerifyAllHashes_Happy(t *testing.T) {
	contentA := []byte("a")
	contentB := []byte("b")
	manifest := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: hashOf(contentA)},
			{Path: "workflows/b.dip", SHA256: hashOf(contentB)},
		},
	}
	cz := buildMultiEntryZip(t, map[string][]byte{
		"workflows/a.dip": contentA,
		"workflows/b.dip": contentB,
		"manifest.json":   []byte("{}"),
	})
	verified, totalBytes, err := verifyAllHashes(cz, manifest, 100<<20)
	if err != nil {
		t.Fatal(err)
	}
	if len(verified) != 2 {
		t.Fatalf("verified count = %d", len(verified))
	}
	if totalBytes != int64(len(contentA)+len(contentB)) {
		t.Fatalf("totalBytes = %d", totalBytes)
	}
}

func TestVerifyAllHashes_TotalCap(t *testing.T) {
	content := bytes.Repeat([]byte("a"), 10)
	manifest := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: hashOf(content)},
			{Path: "workflows/b.dip", SHA256: hashOf(content)},
		},
	}
	cz := buildMultiEntryZip(t, map[string][]byte{
		"workflows/a.dip": content,
		"workflows/b.dip": content,
		"manifest.json":   []byte("{}"),
	})
	_, _, err := verifyAllHashes(cz, manifest, 15) // total cap below sum
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("err = %v, want ErrCapExceeded", err)
	}
}

func TestParseAllWorkflows_Happy(t *testing.T) {
	src := `workflow A
  goal: x
  start: S
  exit: S
  agent S
    prompt: hi
`
	verified := map[string]verifiedBytes{
		"workflows/a.dip": newVerifiedBytes([]byte(src)),
	}
	parsed, err := parseAllWorkflows(verified, "workflows/a.dip")
	if err != nil {
		t.Fatal(err)
	}
	if parsed["workflows/a.dip"].Name != "A" {
		t.Fatalf("name = %q", parsed["workflows/a.dip"].Name)
	}
}

func TestParseAllWorkflows_EntryParseError(t *testing.T) {
	// "workflow" with no name token triggers parser diagnostics and a non-nil
	// error from Parse(). The parser is permissive about totally unrelated
	// junk like "garbage" (it just produces an empty workflow), so we use a
	// well-formed-prefix-but-wrong shape instead.
	verified := map[string]verifiedBytes{
		"workflows/a.dip": newVerifiedBytes([]byte("workflow")),
	}
	_, err := parseAllWorkflows(verified, "workflows/a.dip")
	if !errors.Is(err, ErrEntryParse) {
		t.Fatalf("err = %v, want ErrEntryParse", err)
	}
}

func buildMultiEntryZip(t *testing.T, files map[string][]byte) *constrainedZip {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		// Use the writeUTF8Entry helper from zipio_test.go to set bit 11.
		writeUTF8Entry(t, w, name, content)
	}
	w.Close()
	cz, err := openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return cz
}
