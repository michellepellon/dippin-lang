// ABOUTME: Tests for DIP139–DIP142, the lint checks covering the spec/satisfies grammar.
// ABOUTME: DIP139 flags malformed ACID refs; DIP140/141 catch orphaned spec/satisfies; DIP142 flags duplicates.
package validator

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

// --- DIP139: malformed ACID ---

func TestLint_DIP139_AcceptsValidShapes(t *testing.T) {
	valid := []string{
		"foo.BAR.1",
		"foo.BAR.1-1",
		"foo.BAR.10",
		"foo.BAR.*",
		"foo.BAR.[1-3]",
		"foo.BAR.[10-99]",
		"my-feature.AUTH.1",
		"my_feature.AUTH_TWO.1",
		"a.A.B.1", // multi-component
		"a.A.B.C.1-5",
	}
	for _, ref := range valid {
		t.Run(ref, func(t *testing.T) {
			w := cleanMinimalWorkflow()
			w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
			w.Nodes[0].Satisfies = []string{ref}
			res := Lint(w)
			assertNoCode(t, res, DIP139)
		})
	}
}

func TestLint_DIP139_RejectsMalformedShapes(t *testing.T) {
	invalid := []string{
		"",
		"foo",
		"foo.1",
		"foo.bar.1",
		"foo.BAR",
		".foo.BAR.1",
		"foo.BAR.1.",
		"foo.BAR.a",
		"foo.BAR.[3-1]a",
		"foo.BAR.[]",
		"foo.BAR.**",
	}
	for _, ref := range invalid {
		t.Run(ref, func(t *testing.T) {
			w := cleanMinimalWorkflow()
			w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
			w.Nodes[0].Satisfies = []string{ref}
			res := Lint(w)
			assertHasCode(t, res, DIP139)
		})
	}
}

// --- DIP140: satisfies without spec ---

func TestLint_DIP140_FiresWhenSpecAbsent(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	res := Lint(w)
	assertHasCode(t, res, DIP140)
}

func TestLint_DIP140_QuietWhenSpecPresent(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	res := Lint(w)
	assertNoCode(t, res, DIP140)
}

func TestLint_DIP140_QuietWhenNoSatisfies(t *testing.T) {
	w := cleanMinimalWorkflow()
	res := Lint(w)
	assertNoCode(t, res, DIP140)
}

// --- DIP141: spec without any satisfies ---

func TestLint_DIP141_FiresWhenSpecLacksConsumers(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	res := Lint(w)
	assertHasCode(t, res, DIP141)
}

func TestLint_DIP141_QuietWhenAtLeastOneSatisfies(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	res := Lint(w)
	assertNoCode(t, res, DIP141)
}

func TestLint_DIP141_QuietWhenNoSpec(t *testing.T) {
	w := cleanMinimalWorkflow()
	res := Lint(w)
	assertNoCode(t, res, DIP141)
}

// --- DIP142: duplicate ACID across satisfies lists ---

func TestLint_DIP142_FiresOnExactDuplicate(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1", "foo.BAR.2"}
	w.Nodes[1].Satisfies = []string{"foo.BAR.2"} // dup
	res := Lint(w)
	assertHasCode(t, res, DIP142)
}

func TestLint_DIP142_FiresOnDuplicateWithinOneNode(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1", "foo.BAR.1"}
	res := Lint(w)
	assertHasCode(t, res, DIP142)
}

func TestLint_DIP142_QuietOnRangeOverlapWithBare(t *testing.T) {
	// Range and wildcard semantics aren't expanded at lint time. Only literal
	// duplicates fire — the runtime is responsible for runtime expansion overlap.
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.[1-3]"}
	w.Nodes[1].Satisfies = []string{"foo.BAR.2"}
	res := Lint(w)
	assertNoCode(t, res, DIP142)
}

func TestLint_DIP142_QuietWhenAllUnique(t *testing.T) {
	w := cleanMinimalWorkflow()
	w.Spec = &ir.SpecRef{Loader: "acai", Path: "f.yaml"}
	w.Nodes[0].Satisfies = []string{"foo.BAR.1"}
	w.Nodes[1].Satisfies = []string{"foo.BAR.2"}
	res := Lint(w)
	assertNoCode(t, res, DIP142)
}
