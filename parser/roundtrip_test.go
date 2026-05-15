package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/dippin-lang/formatter"
	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
)

func TestRoundtripTestdata(t *testing.T) {
	files, err := filepath.Glob("testdata/*.dip")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no testdata/*.dip files found")
	}

	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}

			w1, err := parser.NewParser(string(data), f).Parse()
			if err != nil {
				t.Fatalf("first parse failed: %v", err)
			}

			formatted := formatter.Format(w1)

			w2, err := parser.NewParser(formatted, f).Parse()
			if err != nil {
				t.Fatalf("second parse failed: %v\nformatted:\n%s", err, formatted)
			}

			reformatted := formatter.Format(w2)

			if formatted != reformatted {
				t.Errorf("formatter not idempotent\nfirst:\n%s\nsecond:\n%s", formatted, reformatted)
			}

			assertWorkflowsEqual(t, w1, w2)
		})
	}
}

func assertWorkflowsEqual(t *testing.T, a, b *ir.Workflow) {
	t.Helper()
	if a.Name != b.Name {
		t.Errorf("Name: %q vs %q", a.Name, b.Name)
	}
	if a.Start != b.Start {
		t.Errorf("Start: %q vs %q", a.Start, b.Start)
	}
	if a.Exit != b.Exit {
		t.Errorf("Exit: %q vs %q", a.Exit, b.Exit)
	}
	if len(a.Nodes) != len(b.Nodes) {
		t.Errorf("Nodes: %d vs %d", len(a.Nodes), len(b.Nodes))
	}
	if len(a.Edges) != len(b.Edges) {
		t.Errorf("Edges: %d vs %d", len(a.Edges), len(b.Edges))
	}
	assertRequiresEqual(t, a.Requires, b.Requires)
}

func assertRequiresEqual(t *testing.T, a, b []string) {
	t.Helper()
	if len(a) != len(b) {
		t.Errorf("Requires len: %d vs %d (a=%#v b=%#v)", len(a), len(b), a, b)
		return
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("Requires[%d]: %q vs %q", i, a[i], b[i])
		}
	}
}
