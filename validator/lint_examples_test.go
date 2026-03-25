package validator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/validator"
)

// TestLintExamples parses every .dip file in examples/ through the real
// parser and lints it, asserting zero DIP108 (unknown model) warnings.
// This catches model catalog staleness and invalid model IDs in examples.
func TestLintExamples(t *testing.T) {
	examples, err := filepath.Glob("../examples/*.dip")
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("no .dip files found in examples/")
	}

	for _, path := range examples {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			p := parser.NewParser(string(src), path)
			w, err := p.Parse()
			if err != nil {
				t.Fatalf("parse error in %s: %v", name, err)
			}

			result := validator.Lint(w)
			for _, d := range result.Diagnostics {
				if d.Code == validator.DIP108 {
					t.Errorf("%s: %s", name, d.Message)
				}
			}
		})
	}
}
