package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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
	if code != exitDipxIntegrityError {
		t.Fatalf("exit code = %d, expected integrity error %d", code, exitDipxIntegrityError)
	}
}

func TestRunInspect_BadFormat(t *testing.T) {
	bundle := packForTest(t)
	var stdout, stderr bytes.Buffer
	// Unknown format falls through to text rendering by design.
	code := runInspect(&stdout, &stderr, []string{"--format=bogus", bundle})
	if code != exitDipxOK {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "format:") {
		t.Errorf("expected text fallback, got: %s", stdout.String())
	}
}
