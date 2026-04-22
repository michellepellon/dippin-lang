package validator

import "testing"

func TestExplanationsCoverAllCodes(t *testing.T) {
	for code := range CodeDescription {
		exp, ok := Explanations[code]
		if !ok {
			t.Errorf("CodeDescription has %s but Explanations does not", code)
			continue
		}
		assertFieldNonEmpty(t, code, "Summary", exp.Summary)
		assertFieldNonEmpty(t, code, "Trigger", exp.Trigger)
		assertFieldNonEmpty(t, code, "Fix", exp.Fix)
		assertFieldNonEmpty(t, code, "Example", exp.Example)
	}
}

func TestExplanationsNoExtra(t *testing.T) {
	for code := range Explanations {
		if _, ok := CodeDescription[code]; !ok {
			t.Errorf("Explanations has %s but CodeDescription does not", code)
		}
	}
}

func assertFieldNonEmpty(t *testing.T, code, field, value string) {
	t.Helper()
	if value == "" {
		t.Errorf("%s: %s is empty", code, field)
	}
}

func TestManagerLoopCodesRegistered(t *testing.T) {
	for _, code := range []string{DIP135, DIP136, DIP137} {
		if _, ok := CodeDescription[code]; !ok {
			t.Errorf("%s missing from CodeDescription", code)
		}
		if _, ok := Explanations[code]; !ok {
			t.Errorf("%s missing from Explanations", code)
		}
	}
}
