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

// TestRunPack_RejectsInvalidWorkflow confirms Fix H1: pack runs structural
// validation (DIP001-DIP009) on the entry workflow first and refuses to pack
// when validation errors are present. Here the workflow declares exit: S but
// "S" has no outgoing edges (DIP004 would fire if there were unreachable nodes,
// or DIP002 if exit references a missing node, etc.). The workflow below
// references an undeclared start node, triggering DIP001.
func TestRunPack_RejectsInvalidWorkflow(t *testing.T) {
	dir := t.TempDir()
	// Structural error: start node "Missing" doesn't exist.
	invalid := `workflow A
  goal: "Test"
  start: Missing
  exit: Done

  agent Done
    prompt:
      Complete.
`
	entry := filepath.Join(dir, "a.dip")
	if err := os.WriteFile(entry, []byte(invalid), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	out := filepath.Join(dir, "a.dipx")
	var stdout, stderr bytes.Buffer
	code := runPack(&stdout, &stderr, []string{"-o", out, entry})
	if code == exitDipxOK {
		t.Fatalf("expected non-zero exit on structural validation failure; stderr=%s", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatalf("invalid workflow should not produce an output file at %s", out)
	}
}
