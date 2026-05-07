package dipx

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// Compile-time assertions: Bundle implements Source.
var _ Source = (*Bundle)(nil)

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
