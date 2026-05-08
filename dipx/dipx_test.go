package dipx

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestPack_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	parent := `workflow P
  goal: x
  start: S
  exit: E
  subgraph S
    ref: child.dip
  agent E
    prompt: end
  edges
    S -> E
`
	child := `workflow C
  goal: y
  start: A
  exit: A
  agent A
    prompt: child
`
	if err := os.WriteFile(filepath.Join(dir, "parent.dip"), []byte(parent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.dip"), []byte(child), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	manifest, err := Pack(context.Background(), filepath.Join(dir, "parent.dip"), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Entry != "workflows/parent.dip" {
		t.Fatalf("entry = %q", manifest.Entry)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("files = %d", len(manifest.Files))
	}
	// Open the resulting bundle.
	b, err := OpenReader(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if b.Entry().Name != "P" {
		t.Fatalf("entry name = %q", b.Entry().Name)
	}
}

func TestPack_Reproducible(t *testing.T) {
	dir := t.TempDir()
	src := `workflow A
  goal: x
  start: S
  exit: S
  agent S
    prompt: hi
`
	if err := os.WriteFile(filepath.Join(dir, "a.dip"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	var buf1, buf2 bytes.Buffer
	if _, err := Pack(context.Background(), filepath.Join(dir, "a.dip"), &buf1); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(context.Background(), filepath.Join(dir, "a.dip"), &buf2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Fatal("Pack is not byte-deterministic")
	}
}

func TestPack_RejectsOversizedSource(t *testing.T) {
	dir := t.TempDir()
	// Source > maxPerFileBytes. Buffer of zeros is fast to allocate and
	// triggers the cap check before parsing.
	big := make([]byte, maxPerFileBytes+1)
	if err := os.WriteFile(filepath.Join(dir, "big.dip"), big, 0644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, err := Pack(context.Background(), filepath.Join(dir, "big.dip"), &buf)
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("err = %v, want ErrCapExceeded", err)
	}
}

func TestOpen_RejectsTooManyFiles(t *testing.T) {
	// Verify the maxFiles cap fires with ErrCapExceeded.
	//
	// We exercise verifyAllHashes directly rather than going through OpenReader:
	// constructing a manifest with maxFiles+1 entries serialized as JSON would
	// always exceed maxManifestSize (1MB) regardless of how short the paths
	// are — 10001 entries each containing a 64-char SHA-256 plus the minimal
	// JSON overhead and shortest valid "workflows/<N>.dip" path is ~1.06MB —
	// so the manifest-size cap would intercept before the file-count cap could
	// fire. Calling verifyAllHashes directly tests the intended branch without
	// hitting that ordering hazard.
	files := make([]ManifestEntry, 0, maxFiles+1)
	hash := hashOf([]byte("x"))
	for i := 0; i <= maxFiles; i++ {
		files = append(files, ManifestEntry{
			Path:   fmt.Sprintf("workflows/f%d.dip", i),
			SHA256: hash,
		})
	}
	m := Manifest{FormatVersion: 1, Entry: "workflows/f0.dip", Files: files}
	_, _, err := verifyAllHashes(nil, m, maxTotalUncompBytes)
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("err = %v, want ErrCapExceeded", err)
	}
}

func TestBundle_ConcurrentReads(t *testing.T) {
	raw := buildHappyDipx(t)
	b, err := OpenReader(context.Background(), bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	const entryPath = "workflows/hello.dip"
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Entry()
			_ = b.Manifest()
			_ = b.Identity()
			// Exercise the shared lookup/read paths too — these are what the
			// concurrency contract actually covers.
			_, _ = b.Lookup(entryPath)
			_, _ = b.Workflow("hello.dip", entryPath)
			_, _ = b.ReadFile(entryPath)
		}()
	}
	wg.Wait()
}

func TestPack_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.dip")
	if err := os.WriteFile(target, []byte(minimalDipSrc), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.dip")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported on this platform")
	}
	var buf bytes.Buffer
	_, err := Pack(context.Background(), link, &buf)
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}

// TestPack_RejectsParentSymlink covers the parent-component-symlink
// data-exfil vector: a directory in the path tree (not the leaf) is a
// symlink pointing outside rootDir. Pack must refuse rather than silently
// follow into the linked target.
func TestPack_RejectsParentSymlink(t *testing.T) {
	rootDir := t.TempDir()
	outside := t.TempDir()
	// Place an attacker-controlled .dip at the symlink target.
	if err := os.WriteFile(filepath.Join(outside, "secret.dip"), []byte(minimalDipSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a symlink under rootDir whose target is the outside directory.
	if err := os.Symlink(outside, filepath.Join(rootDir, "phases")); err != nil {
		t.Skip("symlinks not supported on this platform")
	}
	// The entry .dip references phases/secret.dip via subgraph ref. With the
	// parent-component check, Pack must refuse before reading the leaf via
	// the symlinked parent.
	entrySrc := `workflow Parent
  goal: x
  start: S
  exit: E
  subgraph S
    ref: phases/secret.dip
  agent E
    prompt: end
  edges
    S -> E
`
	entryPath := filepath.Join(rootDir, "parent.dip")
	if err := os.WriteFile(entryPath, []byte(entrySrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	_, err := Pack(context.Background(), entryPath, &buf)
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}

// TestPack_AcceptsDoubleDotPrefixDirectory covers the false-positive bug
// where strings.HasPrefix(rel, "..") rejected legitimate filenames whose
// directory name simply starts with two dots (e.g., "..foo/bar.dip").
func TestPack_AcceptsDoubleDotPrefixDirectory(t *testing.T) {
	rootDir := t.TempDir()
	subdir := filepath.Join(rootDir, "..foo")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "child.dip"), []byte(minimalDipSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	entrySrc := `workflow Parent
  goal: x
  start: S
  exit: E
  subgraph S
    ref: ..foo/child.dip
  agent E
    prompt: end
  edges
    S -> E
`
	entryPath := filepath.Join(rootDir, "parent.dip")
	if err := os.WriteFile(entryPath, []byte(entrySrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := Pack(context.Background(), entryPath, &buf); err != nil {
		t.Fatalf("unexpected error packing legitimate '..foo' subdir: %v", err)
	}
}

// TestExtract_ForcePreservesDestOnRenameFailure simulates EXDEV (cross-mount
// rename) at the staging-into-place step. With --force, the original
// destDir must be restored from the backup-aside on failure rather than
// destroyed.
func TestExtract_ForcePreservesDestOnRenameFailure(t *testing.T) {
	root := t.TempDir()
	destDir := filepath.Join(root, "out")
	staging := filepath.Join(root, "out.tmp")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("ORIGINAL CONTENT")
	if err := os.WriteFile(filepath.Join(destDir, "marker"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "marker"), []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Inject a rename that succeeds for destDir->backup but returns EXDEV for
	// staging->destDir, exactly as a cross-mount rename would.
	calls := 0
	rename := func(oldp, newp string) error {
		calls++
		if calls == 2 {
			return &os.LinkError{Op: "rename", Old: oldp, New: newp, Err: errors.New("invalid cross-device link")}
		}
		return os.Rename(oldp, newp)
	}
	if err := swapDestWithStaging(destDir, staging, rename); err == nil {
		t.Fatal("expected EXDEV-simulated rename failure")
	}
	got, err := os.ReadFile(filepath.Join(destDir, "marker"))
	if err != nil {
		t.Fatalf("destDir was destroyed on rename failure: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("destDir contents corrupted: got %q, want %q", got, original)
	}
}
