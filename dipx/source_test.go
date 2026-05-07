package dipx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// Compile-time assertions: Bundle and dirSource implement Source.
var _ Source = (*Bundle)(nil)
var _ Source = (*dirSource)(nil)

func TestSource_BundleImplementsInterface(t *testing.T) {
	var s Source = newTestBundle(t)
	if s.Entry() == nil {
		t.Fatal("Entry returned nil")
	}
	wf, err := s.Workflow("b.dip", "workflows/a.dip")
	if err != nil {
		t.Fatal(err)
	}
	if wf == nil {
		t.Fatal("Workflow returned nil")
	}
}

// silence unused import
var _ = ir.NodeAgent

func TestDirSource_LoadDip(t *testing.T) {
	dir := t.TempDir()
	parent := `workflow P
  goal: x
  start: S
  exit: E
  subgraph S
    ref: child.dip
  agent E
    prompt: end
  edges
    S -> E
`
	child := `workflow C
  goal: y
  start: A
  exit: A
  agent A
    prompt: child
`
	if err := os.WriteFile(filepath.Join(dir, "parent.dip"), []byte(parent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child.dip"), []byte(child), 0644); err != nil {
		t.Fatal(err)
	}
	src, err := Load(context.Background(), filepath.Join(dir, "parent.dip"))
	if err != nil {
		t.Fatal(err)
	}
	if src.Entry().Name != "P" {
		t.Fatalf("entry name = %q", src.Entry().Name)
	}
	wf, err := src.Workflow("child.dip", filepath.Join(dir, "parent.dip"))
	if err != nil {
		t.Fatal(err)
	}
	if wf.Name != "C" {
		t.Fatalf("child name = %q", wf.Name)
	}
}

func TestDirSource_RejectsEscape(t *testing.T) {
	dir := t.TempDir()
	parent := `workflow P
  goal: x
  start: A
  exit: A
  agent A
    prompt: x
`
	if err := os.WriteFile(filepath.Join(dir, "parent.dip"), []byte(parent), 0644); err != nil {
		t.Fatal(err)
	}
	src, err := Load(context.Background(), filepath.Join(dir, "parent.dip"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.Workflow("../../../etc/passwd", filepath.Join(dir, "parent.dip"))
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}
