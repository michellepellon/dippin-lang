package dipx

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const minimalDipSrc = `workflow Hello
  goal: hello
  start: A
  exit: A

  agent A
    prompt: hi
`

func buildHappyDipx(t *testing.T) []byte {
	t.Helper()
	body := []byte(minimalDipSrc)
	bodyHash := hashOf(body)
	manifestSrc := fmt.Sprintf(`{"format_version":1,"entry":"workflows/hello.dip","files":[{"path":"workflows/hello.dip","sha256":%q}]}`, bodyHash)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, "manifest.json", []byte(manifestSrc))
	writeUTF8Entry(t, w, "workflows/hello.dip", body)
	w.Close()
	return buf.Bytes()
}

func TestOpen_Happy(t *testing.T) {
	raw := buildHappyDipx(t)
	b, err := OpenReader(context.Background(), bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if b.Entry().Name != "Hello" {
		t.Fatalf("entry name = %q", b.Entry().Name)
	}
	if b.Manifest().FormatVersion != 1 {
		t.Fatalf("format version = %d", b.Manifest().FormatVersion)
	}
}

func TestOpen_HashMismatch(t *testing.T) {
	body := []byte(minimalDipSrc)
	wrongHash := hashOf([]byte("different"))
	manifestSrc := fmt.Sprintf(`{"format_version":1,"entry":"workflows/hello.dip","files":[{"path":"workflows/hello.dip","sha256":%q}]}`, wrongHash)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, "manifest.json", []byte(manifestSrc))
	writeUTF8Entry(t, w, "workflows/hello.dip", body)
	w.Close()
	_, err := OpenReader(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("err = %v, want ErrHashMismatch", err)
	}
}

func TestOpen_ContextCancelled(t *testing.T) {
	raw := buildHappyDipx(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := OpenReader(ctx, bytes.NewReader(raw), int64(len(raw)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestExtract_Happy(t *testing.T) {
	raw := buildHappyDipx(t)
	src := filepath.Join(t.TempDir(), "h.dipx")
	if err := os.WriteFile(src, raw, 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out")
	if err := Extract(context.Background(), src, dest, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "workflows", "hello.dip")); err != nil {
		t.Fatalf("expected file extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "manifest.json")); err != nil {
		t.Fatalf("expected manifest extracted: %v", err)
	}
}

func TestExtract_RefusesExistingWithoutForce(t *testing.T) {
	raw := buildHappyDipx(t)
	src := filepath.Join(t.TempDir(), "h.dipx")
	if err := os.WriteFile(src, raw, 0644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir() // already exists
	err := Extract(context.Background(), src, dest, false)
	if err == nil {
		t.Fatal("expected error when destdir exists")
	}
}

func TestOpen_FormatVersionRejected(t *testing.T) {
	body := []byte(minimalDipSrc)
	bodyHash := hashOf(body)
	manifestSrc := fmt.Sprintf(`{"format_version":999,"entry":"workflows/hello.dip","files":[{"path":"workflows/hello.dip","sha256":%q}]}`, bodyHash)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	writeUTF8Entry(t, w, "manifest.json", []byte(manifestSrc))
	writeUTF8Entry(t, w, "workflows/hello.dip", body)
	w.Close()
	_, err := OpenReader(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if !errors.Is(err, ErrUnsupportedFormatVersion) {
		t.Fatalf("err = %v, want ErrUnsupportedFormatVersion", err)
	}
}
