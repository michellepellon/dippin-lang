package graph_test

import (
	"os"
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/graph"
	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
)

func TestLinearLayers(t *testing.T) {
	w := parseFixture(t, "testdata/linear.dip")
	info := graph.Layers(w)

	assertLayerCount(t, info, 3)
	assertNodeInLayer(t, info, 0, "Start")
	assertNodeInLayer(t, info, 1, "Middle")
	assertNodeInLayer(t, info, 2, "Exit")
}

func TestLinearRender(t *testing.T) {
	w := parseFixture(t, "testdata/linear.dip")
	out := graph.Render(w, graph.Options{})

	assertContains(t, out, "Start")
	assertContains(t, out, "Middle")
	assertContains(t, out, "Exit")
	assertOrderInOutput(t, out, "Start", "Middle")
	assertOrderInOutput(t, out, "Middle", "Exit")
}

func TestBranchingLayers(t *testing.T) {
	w := parseFixture(t, "testdata/branching.dip")
	info := graph.Layers(w)

	assertNodeInLayer(t, info, 0, "Start")
	assertSameLayer(t, info, "Left", "Right")
	assertOrderInLayers(t, info, "Start", "Left")
	assertOrderInLayers(t, info, "Left", "Merge")
	assertOrderInLayers(t, info, "Merge", "Exit")
}

func TestCompactLinear(t *testing.T) {
	w := parseFixture(t, "testdata/linear.dip")
	out := graph.Render(w, graph.Options{Compact: true})

	assertContains(t, out, "[Start]")
	assertContains(t, out, "[Exit]")
	assertContains(t, out, "\u2192")
}

func TestCompactBranching(t *testing.T) {
	w := parseFixture(t, "testdata/branching.dip")
	out := graph.Render(w, graph.Options{Compact: true})

	assertContains(t, out, "\u2192")
	// Left and Right should be in a combined bracket
	if !strings.Contains(out, "Left | Right") && !strings.Contains(out, "Right | Left") {
		t.Errorf("expected Left and Right combined in brackets, got: %s", out)
	}
}

func TestParallelLayers(t *testing.T) {
	w := parseFixture(t, "testdata/parallel.dip")
	info := graph.Layers(w)

	assertSameLayer(t, info, "ReviewA", "ReviewB")
	assertOrderInLayers(t, info, "Start", "ReviewA")
	assertOrderInLayers(t, info, "ReviewA", "Done")
}

func TestParallelRender(t *testing.T) {
	w := parseFixture(t, "testdata/parallel.dip")
	out := graph.Render(w, graph.Options{})

	assertContains(t, out, "ReviewA")
	assertContains(t, out, "ReviewB")
	assertContains(t, out, "Done")
}

func TestParallelCompact(t *testing.T) {
	w := parseFixture(t, "testdata/parallel.dip")
	out := graph.Render(w, graph.Options{Compact: true})

	assertContains(t, out, "→")
	// ReviewA and ReviewB should be combined in a bracket
	if !strings.Contains(out, "ReviewA | ReviewB") && !strings.Contains(out, "ReviewB | ReviewA") {
		t.Errorf("expected ReviewA and ReviewB combined, got: %s", out)
	}
}

func TestRestartLoopLayers(t *testing.T) {
	w := parseFixture(t, "testdata/restart_loop.dip")
	info := graph.Layers(w)

	// All 4 nodes should be assigned to layers (restart edge excluded from DAG).
	assertOrderInLayers(t, info, "Start", "Process")
	assertOrderInLayers(t, info, "Process", "Check")
	assertOrderInLayers(t, info, "Check", "Done")
}

func TestRestartLoopRender(t *testing.T) {
	w := parseFixture(t, "testdata/restart_loop.dip")
	out := graph.Render(w, graph.Options{})

	// All nodes should appear in the full render.
	assertContains(t, out, "Start")
	assertContains(t, out, "Process")
	assertContains(t, out, "Check")
	assertContains(t, out, "Done")
}

func TestRestartLoopCompact(t *testing.T) {
	w := parseFixture(t, "testdata/restart_loop.dip")
	out := graph.Render(w, graph.Options{Compact: true})

	assertContains(t, out, "[Start]")
	assertContains(t, out, "[Done]")
	assertContains(t, out, "→")
}

func parseFixture(t *testing.T, path string) *ir.Workflow {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	p := parser.NewParser(string(src), path)
	w, err := p.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return w
}

func assertContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, output)
	}
}

func assertOrderInOutput(t *testing.T, output, first, second string) {
	t.Helper()
	i := strings.Index(output, first)
	j := strings.Index(output, second)
	if i < 0 || j < 0 || i >= j {
		t.Errorf("expected %q before %q in output", first, second)
	}
}

func assertLayerCount(t *testing.T, info graph.LayerInfo, expected int) {
	t.Helper()
	if len(info.Layers) != expected {
		t.Errorf("expected %d layers, got %d: %v", expected, len(info.Layers), info.Layers)
	}
}

func assertNodeInLayer(t *testing.T, info graph.LayerInfo, layer int, nodeID string) {
	t.Helper()
	if layer >= len(info.Layers) {
		t.Fatalf("layer %d out of range (have %d layers)", layer, len(info.Layers))
	}
	for _, id := range info.Layers[layer] {
		if id == nodeID {
			return
		}
	}
	t.Errorf("expected %q in layer %d, got %v", nodeID, layer, info.Layers[layer])
}

func assertSameLayer(t *testing.T, info graph.LayerInfo, a, b string) {
	t.Helper()
	la := findLayer(info, a)
	lb := findLayer(info, b)
	if la != lb {
		t.Errorf("expected %q and %q in same layer, got layers %d and %d", a, b, la, lb)
	}
}

func assertOrderInLayers(t *testing.T, info graph.LayerInfo, first, second string) {
	t.Helper()
	la := findLayer(info, first)
	lb := findLayer(info, second)
	if la >= lb {
		t.Errorf("expected %q (layer %d) before %q (layer %d)", first, la, second, lb)
	}
}

func findLayer(info graph.LayerInfo, nodeID string) int {
	for i, layer := range info.Layers {
		for _, id := range layer {
			if id == nodeID {
				return i
			}
		}
	}
	return -1
}
