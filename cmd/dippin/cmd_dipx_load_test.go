package main

import (
	"strings"
	"testing"
)

// TestAnalysisCommandsAcceptDipx asserts that loadWorkflow / parseFile route
// .dipx files through dipx.Load. Smoke-tests representative analysis
// subcommands by running them against a freshly-packed bundle.
func TestAnalysisCommandsAcceptDipx(t *testing.T) {
	bundle := packForTest(t)
	cases := []struct {
		name string
		args []string
	}{
		{"validate", []string{"validate", bundle}},
		{"lint", []string{"lint", bundle}},
		{"parse", []string{"parse", bundle}},
		{"check", []string{"check", "--format=text", bundle}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, tc.args...)
			if code != ExitOK {
				t.Fatalf("%s on .dipx exited %d; stderr=%s; stdout=%s", tc.name, code, stderr, stdout)
			}
		})
	}
}

// TestAnalysisRejectsBadDipx asserts the .dipx loader surfaces integrity
// failures via the standard CLI error path.
func TestAnalysisRejectsBadDipx(t *testing.T) {
	_, stderr, code := runCLI(t, "validate", "/nonexistent.dipx")
	if code == ExitOK {
		t.Fatal("expected non-zero exit for missing bundle")
	}
	if !strings.Contains(stderr, "manifest") &&
		!strings.Contains(stderr, "not readable") &&
		!strings.Contains(stderr, "missing") {
		t.Fatalf("unexpected stderr for missing bundle: %q", stderr)
	}
}
