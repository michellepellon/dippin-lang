package dipx

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestVerifiedBytes_BytesReturnsContent(t *testing.T) {
	vb := newVerifiedBytes([]byte("hello"))
	got := vb.Bytes()
	if !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("got %q", got)
	}
}

func TestOpenConstrainedZip_RejectsEncryption(t *testing.T) {
	// Build a minimal zip with an encrypted entry.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "manifest.json"}
	h.SetMode(0644)
	h.Flags |= 0x1 // bit 0 = encrypted
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("{}"))
	w.Close()
	r := bytes.NewReader(buf.Bytes())
	_, err = openConstrainedZip(r, int64(buf.Len()))
	if !errors.Is(err, ErrZipFeatureForbidden) {
		t.Fatalf("err = %v, want ErrZipFeatureForbidden", err)
	}
}

func TestOpenConstrainedZip_RejectsDuplicateEntries(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < 2; i++ {
		writeUTF8Entry(t, w, "workflows/a.dip", []byte("x"))
	}
	w.Close()
	_, err := openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if !errors.Is(err, ErrZipFeatureForbidden) {
		t.Fatalf("err = %v, want ErrZipFeatureForbidden", err)
	}
}

func TestOpenConstrainedZip_RejectsNonDeflateCompression(t *testing.T) {
	// Create with method 12 (BZIP2) which Go doesn't natively support.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "manifest.json", Method: 12}
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Skip("BZIP2 not creatable in this Go version")
	}
	fw.Write([]byte("{}"))
	w.Close()
	_, err = openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if !errors.Is(err, ErrZipFeatureForbidden) {
		t.Fatalf("err = %v, want ErrZipFeatureForbidden", err)
	}
}

func TestOpenConstrainedZip_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, "manifest.json", []byte("{}"))
	w.Close()
	cz, err := openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if cz == nil {
		t.Fatal("nil constrainedZip")
	}
}

func TestOpenConstrainedZip_IgnoresDirectoryEntries(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, "workflows/", nil)
	writeUTF8Entry(t, w, "manifest.json", []byte("{}"))
	w.Close()
	cz, err := openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	// Directory entry should not appear in cz.entries.
	if _, ok := cz.entries["workflows/"]; ok {
		t.Fatal("directory entry should be skipped")
	}
}

func TestVerifyAndReadEntry_HappyPath(t *testing.T) {
	content := []byte("workflow Hello\n  goal: x\n  start: A\n  exit: A\n  agent A\n")
	cz := buildSingleEntryZip(t, "workflows/a.dip", content)
	vb, err := verifyAndReadEntry(cz, "workflows/a.dip", hashOf(content), 50<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(vb.Bytes(), content) {
		t.Fatalf("bytes differ")
	}
}

func TestVerifyAndReadEntry_HashMismatch(t *testing.T) {
	content := []byte("real content")
	cz := buildSingleEntryZip(t, "workflows/a.dip", content)
	wrong := hashOf([]byte("different"))
	_, err := verifyAndReadEntry(cz, "workflows/a.dip", wrong, 50<<20)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("err = %v, want ErrHashMismatch", err)
	}
}

func TestVerifyAndReadEntry_PerFileCap(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 1024)
	cz := buildSingleEntryZip(t, "workflows/a.dip", big)
	_, err := verifyAndReadEntry(cz, "workflows/a.dip", hashOf(big), 100) // cap < content
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("err = %v, want ErrCapExceeded", err)
	}
}

func TestVerifyAndReadEntry_NotFound(t *testing.T) {
	cz := buildSingleEntryZip(t, "workflows/a.dip", []byte("x"))
	_, err := verifyAndReadEntry(cz, "workflows/b.dip", hashOf([]byte("x")), 50<<20)
	if !errors.Is(err, ErrFileMissing) {
		t.Fatalf("err = %v, want ErrFileMissing", err)
	}
}

func TestVerifyAndReadEntry_AtCapAccepted(t *testing.T) {
	content := bytes.Repeat([]byte("a"), 100)
	cz := buildSingleEntryZip(t, "workflows/a.dip", content)
	vb, err := verifyAndReadEntry(cz, "workflows/a.dip", hashOf(content), 100)
	if err != nil {
		t.Fatalf("at-cap should accept, got %v", err)
	}
	if !bytes.Equal(vb.Bytes(), content) {
		t.Fatal("bytes differ")
	}
}

func TestVerifyAndReadEntry_OneOverCapRejected(t *testing.T) {
	content := bytes.Repeat([]byte("a"), 101)
	cz := buildSingleEntryZip(t, "workflows/a.dip", content)
	_, err := verifyAndReadEntry(cz, "workflows/a.dip", hashOf(content), 100)
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("err = %v, want ErrCapExceeded", err)
	}
}

func TestOpenConstrainedZip_RejectsAsciiWithoutUTF8Flag(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "manifest.json"}
	h.SetMode(0644)
	// Force flags to 0 to simulate a CP437-emitting producer.
	h.Flags = 0
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("{}"))
	w.Close()
	_, err = openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if !errors.Is(err, ErrZipFeatureForbidden) {
		t.Fatalf("err = %v, want ErrZipFeatureForbidden", err)
	}
}

func TestOpenConstrainedZip_RejectsManifestSig(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, "manifest.sig", []byte("dummy"))
	writeUTF8Entry(t, w, "manifest.json", []byte("{}"))
	w.Close()
	_, err := openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if !errors.Is(err, ErrZipFeatureForbidden) {
		t.Fatalf("err = %v, want ErrZipFeatureForbidden", err)
	}
}

// helpers used across zipio tests.

func hashOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// writeUTF8Entry creates a zip entry with general-purpose bit 11 (UTF-8 name)
// set, mirroring what spec-conformant Pack output must do. Go's archive/zip
// writer does NOT auto-set bit 11 for ASCII-only names, so tests that exercise
// the constrained reader's happy path must set it explicitly.
func writeUTF8Entry(t *testing.T, w *zip.Writer, name string, content []byte) {
	t.Helper()
	h := &zip.FileHeader{Name: name, Flags: 0x800}
	h.SetMode(0644)
	fw, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) > 0 {
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
}

func buildSingleEntryZip(t *testing.T, name string, content []byte) *constrainedZip {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, name, content)
	w.Close()
	cz, err := openConstrainedZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return cz
}
