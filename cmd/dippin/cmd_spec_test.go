// ABOUTME: Tests for the dippin spec command.
// ABOUTME: Verifies the embedded spec contains expected sections and markers.
package main

import (
	"strings"
	"testing"
)

func TestCmdSpec_OutputContainsExpectedSections(t *testing.T) {
	stdout, stderr, code := runCLI(t, "spec")

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Errorf("expected no stderr, got: %s", stderr)
	}

	// Verify the spec is non-trivial (at least 100 lines).
	lines := strings.Count(stdout, "\n")
	if lines < 100 {
		t.Errorf("expected at least 100 lines, got %d", lines)
	}

	// Verify key sections are present.
	markers := []string{
		"# Dippin Language Specification",
		"## Grammar",
		"## Node Kinds",
		"## Edge Conditions",
		"## File Structure",
		"## Node Types",
		"## Diagnostic Codes",
		"## CLI Reference",
		"DIP001",
		"DIP101",
		"DIP133",
	}
	for _, m := range markers {
		if !strings.Contains(stdout, m) {
			t.Errorf("spec output missing expected marker: %q", m)
		}
	}
}

func TestCmdSpec_NoFrontmatter(t *testing.T) {
	stdout, _, code := runCLI(t, "spec")

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	// Verify no skill.md frontmatter leaked through.
	if strings.Contains(stdout, "name: dippin-lang") {
		t.Error("spec output contains skill.md frontmatter (name: dippin-lang)")
	}
	if strings.Contains(stdout, "description: Use when working") {
		t.Error("spec output contains skill.md frontmatter (description)")
	}
}
