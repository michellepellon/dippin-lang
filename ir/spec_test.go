// ABOUTME: Round-trip tests for SpecRef and the optional Workflow.Spec / Node.Satisfies fields.
// ABOUTME: Establishes the IR contract for spec-first workflow authoring.
package ir_test

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestWorkflow_Spec_NilByDefault(t *testing.T) {
	w := &ir.Workflow{}
	if w.Spec != nil {
		t.Fatalf("expected Workflow.Spec nil by default, got %#v", w.Spec)
	}
}

func TestWorkflow_Spec_RoundTrip(t *testing.T) {
	w := &ir.Workflow{Spec: &ir.SpecRef{Loader: "acai", Path: "features.yaml"}}
	if w.Spec == nil {
		t.Fatalf("Spec is nil after assignment")
	}
	if w.Spec.Loader != "acai" {
		t.Errorf("Loader = %q, want acai", w.Spec.Loader)
	}
	if w.Spec.Path != "features.yaml" {
		t.Errorf("Path = %q, want features.yaml", w.Spec.Path)
	}
}

func TestNode_Satisfies_NilByDefault(t *testing.T) {
	n := &ir.Node{}
	if n.Satisfies != nil {
		t.Fatalf("expected Node.Satisfies nil by default, got %#v", n.Satisfies)
	}
}

func TestNode_Satisfies_RoundTrip(t *testing.T) {
	n := &ir.Node{Satisfies: []string{"foo.BAR.1", "foo.BAR.2-1"}}
	if len(n.Satisfies) != 2 {
		t.Fatalf("Satisfies len = %d, want 2", len(n.Satisfies))
	}
	if n.Satisfies[0] != "foo.BAR.1" || n.Satisfies[1] != "foo.BAR.2-1" {
		t.Errorf("Satisfies = %#v, want [foo.BAR.1 foo.BAR.2-1]", n.Satisfies)
	}
}
