package simulate

import (
	"testing"

	"github.com/2389/dippin/ir"
)

func TestTokenizeCondition_NoSpaces(t *testing.T) {
	// The .dip parser will produce spaces between tokens, but
	// ParseCondition may be called on raw strings that don't have spaces.
	// Currently tokenizeCondition requires spaces.
	tokens := tokenizeCondition("ctx.outcome = success")
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestParseCondition_DoubleEqual(t *testing.T) {
	expr, err := ParseCondition("ctx.outcome == success")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	cmp, ok := expr.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", expr)
	}
	if cmp.Op != "==" {
		t.Errorf("Op = %q, want ==", cmp.Op)
	}
}
