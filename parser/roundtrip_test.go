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
	assertSpecEqual(t, a.Spec, b.Spec)
	assertNodeSatisfiesEqual(t, a.Nodes, b.Nodes)
}

func assertRequiresEqual(t *testing.T, a, b []string) {
	t.Helper()
	if (a == nil) != (b == nil) {
		t.Errorf("Requires nilness: a nil=%t vs b nil=%t", a == nil, b == nil)
		return
	}
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

func assertSpecEqual(t *testing.T, a, b *ir.SpecRef) {
	t.Helper()
	if (a == nil) != (b == nil) {
		t.Errorf("Spec nilness: a nil=%t vs b nil=%t", a == nil, b == nil)
		return
	}
	if a == nil {
		return
	}
	if a.Loader != b.Loader || a.Path != b.Path {
		t.Errorf("Spec: %#v vs %#v", a, b)
	}
}

func assertNodeSatisfiesEqual(t *testing.T, a, b []*ir.Node) {
	t.Helper()
	if len(a) != len(b) {
		return // a separate assertion already reports the mismatched count
	}
	for i := range a {
		if (a[i].Satisfies == nil) != (b[i].Satisfies == nil) {
			t.Errorf("Nodes[%d].Satisfies nilness: %t vs %t", i, a[i].Satisfies == nil, b[i].Satisfies == nil)
			continue
		}
		if len(a[i].Satisfies) != len(b[i].Satisfies) {
			t.Errorf("Nodes[%d].Satisfies len: %d vs %d", i, len(a[i].Satisfies), len(b[i].Satisfies))
			continue
		}
		for j := range a[i].Satisfies {
			if a[i].Satisfies[j] != b[i].Satisfies[j] {
				t.Errorf("Nodes[%d].Satisfies[%d]: %q vs %q", i, j, a[i].Satisfies[j], b[i].Satisfies[j])
			}
		}
	}
}
