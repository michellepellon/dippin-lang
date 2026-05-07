package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const minimalDip = `workflow A
  goal: "Test"
  start: Ask
  exit: Done

  human Ask
    mode: freeform

  agent Done
    prompt:
      Complete the task.

  edges
    Ask -> Done
`

// writeMinimalEntry creates a temp dir with a minimal valid .dip and returns
// the dir, entry path.
func writeMinimalEntry(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	entry := filepath.Join(dir, "a.dip")
	if err := os.WriteFile(entry, []byte(minimalDip), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	return dir, entry
}

func TestRunPack_Happy(t *testing.T) {
	dir, entry := writeMinimalEntry(t)
	out := filepath.Join(dir, "a.dipx")
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, []string{"-o", out, entry})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected output: %v", err)
	}
}

func TestRunPack_DefaultOutputName(t *testing.T) {
	dir, entry := writeMinimalEntry(t)
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, []string{entry})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	defaultOut := filepath.Join(dir, "a.dipx")
	if _, err := os.Stat(defaultOut); err != nil {
		t.Fatalf("expected default output at %s: %v", defaultOut, err)
	}
}

func TestRunPack_MissingEntry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, []string{"/nonexistent.dip"})
	if code == exitDipxOK {
		t.Fatal("expected non-zero exit")
	}
}

func TestRunPack_DryRun(t *testing.T) {
	dir, entry := writeMinimalEntry(t)
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, []string{"--dry-run", entry})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	out := filepath.Join(dir, "a.dipx")
	if _, err := os.Stat(out); err == nil {
		t.Fatal("dry-run should not produce output")
	}
}

func TestRunPack_Stdout(t *testing.T) {
	_, entry := writeMinimalEntry(t)
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, []string{"-o", "-", entry})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	// Bundle starts with "PK" zip magic.
	if !bytes.HasPrefix(stdout.Bytes(), []byte{'P', 'K'}) {
		t.Fatalf("expected ZIP magic on stdout, got: %x", stdout.Bytes()[:4])
	}
}

func TestRunPack_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, nil)
	if code != exitDipxUserError {
		t.Fatalf("exit code = %d, expected %d", code, exitDipxUserError)
	}
}
