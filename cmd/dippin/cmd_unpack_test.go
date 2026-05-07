package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// packForTest produces a valid .dipx in tmp from minimalDip; returns its path.
func packForTest(t *testing.T) string {
	t.Helper()
	dir, entry := writeMinimalEntry(t)
	out := filepath.Join(dir, "a.dipx")
	var so, se bytes.Buffer
	if code := runPack(&so, &se, []string{"-o", out, entry}); code != exitDipxOK {
		t.Fatalf("pack failed: %d; %s", code, se.String())
	}
	return out
}

func TestRunUnpack_Happy(t *testing.T) {
	bundle := packForTest(t)
	dest := filepath.Join(t.TempDir(), "extracted")
	var stderr bytes.Buffer
	code := runUnpack(&stderr, []string{"-o", dest, bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "manifest.json")); err != nil {
		t.Fatalf("expected manifest in dest: %v", err)
	}
}

func TestRunUnpack_NoArgs(t *testing.T) {
	var stderr bytes.Buffer
	code := runUnpack(&stderr, nil)
	if code != exitDipxUserError {
		t.Fatalf("exit code = %d, expected %d", code, exitDipxUserError)
	}
}

func TestRunUnpack_MissingBundle(t *testing.T) {
	var stderr bytes.Buffer
	code := runUnpack(&stderr, []string{"/nonexistent.dipx"})
	if code == exitDipxOK {
		t.Fatal("expected non-zero exit")
	}
}

func TestRunUnpack_RefuseExisting(t *testing.T) {
	bundle := packForTest(t)
	dest := t.TempDir() // already exists
	var stderr bytes.Buffer
	code := runUnpack(&stderr, []string{"-o", dest, bundle})
	if code == exitDipxOK {
		t.Fatal("expected non-zero exit when destination exists without --force")
	}
}

func TestRunUnpack_ForceOverwrites(t *testing.T) {
	bundle := packForTest(t)
	dest := t.TempDir() // already exists
	var stderr bytes.Buffer
	code := runUnpack(&stderr, []string{"-o", dest, "--force", bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "manifest.json")); err != nil {
		t.Fatalf("expected manifest in dest: %v", err)
	}
}
