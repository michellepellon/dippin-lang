package dipx

import (
	"archive/zip"
	"bytes"
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
		fw, _ := w.Create("workflows/a.dip")
		fw.Write([]byte("x"))
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
	fw, _ := w.Create("manifest.json")
	fw.Write([]byte("{}"))
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
	w.Create("workflows/")
	fw, _ := w.Create("manifest.json")
	fw.Write([]byte("{}"))
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
