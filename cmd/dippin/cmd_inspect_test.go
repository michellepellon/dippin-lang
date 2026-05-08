package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/dipx"
)

func TestRunInspect_Text(t *testing.T) {
	bundle := packForTest(t)
	var stdout, stderr bytes.Buffer
	code := runInspect(&stdout, &stderr, []string{bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"format:", "entry:", "identity:", "files:", "status: VALID"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in text output, got:\n%s", want, out)
		}
	}
}

func TestRunInspect_JSON(t *testing.T) {
	bundle := packForTest(t)
	var stdout, stderr bytes.Buffer
	code := runInspect(&stdout, &stderr, []string{"--format=json", bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	for _, k := range []string{"format_version", "entry", "identity", "files", "status"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("missing key %q in JSON output", k)
		}
	}
	if parsed["status"] != "VALID" {
		t.Errorf("expected status=VALID, got %v", parsed["status"])
	}
}

func TestRunInspect_NoVerifyWarning(t *testing.T) {
	bundle := packForTest(t)
	var stdout, stderr bytes.Buffer
	code := runInspect(&stdout, &stderr, []string{"--no-verify", bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "full integrity check still runs") {
		t.Errorf("expected warning on stderr, got: %s", stderr.String())
	}
}

func TestRunInspect_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runInspect(&stdout, &stderr, nil)
	if code != exitDipxUserError {
		t.Fatalf("exit code = %d, expected %d", code, exitDipxUserError)
	}
}

func TestRunInspect_MissingBundle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runInspect(&stdout, &stderr, []string{"/nonexistent.dipx"})
	// ErrManifestMissing wraps an *os.PathError ("no such file or directory").
	// classifyExit's isIOErr helper detects the wrapped PathError via
	// errors.As and routes to the documented I/O exit code (3), per Phase 8
	// M2 resolution.
	if code != exitDipxIOError {
		t.Fatalf("exit code = %d, expected I/O error %d; stderr=%s", code, exitDipxIOError, stderr.String())
	}
}

func TestRunInspect_BadFormat(t *testing.T) {
	bundle := packForTest(t)
	var stdout, stderr bytes.Buffer
	// Unknown format values are rejected as user errors per spec.
	code := runInspect(&stdout, &stderr, []string{"--format=bogus", bundle})
	if code != exitDipxUserError {
		t.Fatalf("exit code = %d, expected user-error %d; stderr=%s", code, exitDipxUserError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown --format value") {
		t.Errorf("expected diagnostic on stderr, got: %s", stderr.String())
	}
}

// TestRunInspect_JSONIsParseable confirms the JSON output round-trips back into
// the public Manifest struct (Fix H3 — Manifest has JSON tags).
func TestRunInspect_JSONIsParseable(t *testing.T) {
	bundle := packForTest(t)
	var stdout, stderr bytes.Buffer
	code := runInspect(&stdout, &stderr, []string{"--format=json", bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	var got dipx.Manifest
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode into dipx.Manifest: %v\n%s", err, stdout.String())
	}
	if got.FormatVersion == 0 {
		t.Errorf("FormatVersion zero after round-trip; JSON tags missing on Manifest?")
	}
	if got.Entry == "" {
		t.Errorf("Entry empty after round-trip; JSON tags missing on Manifest?")
	}
	if len(got.Files) == 0 {
		t.Errorf("Files empty after round-trip")
	}
}
